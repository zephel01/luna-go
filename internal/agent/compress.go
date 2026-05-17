package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/zephel01/luna-go/internal/llm"
)

// estimateTokens returns a rough token count for a slice of messages.
// Uses the ~4 chars/token heuristic, which is reasonable for English + code.
func estimateTokens(msgs []llm.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Content) / 4
		for _, tc := range m.ToolCalls {
			n += (len(tc.Function.Name) + len(tc.Function.Arguments)) / 4
		}
	}
	return n
}

// extractFileLists scans tool calls in msgs and returns two deduplicated lists:
//   - readFiles:     files passed to the "read" tool
//   - modifiedFiles: files passed to "write" or "edit" tools
//
// If a file appears in both lists, it is promoted to modifiedFiles only.
func extractFileLists(msgs []llm.Message) (readFiles, modifiedFiles []string) {
	readSet := map[string]bool{}
	modSet := map[string]bool{}

	for _, m := range msgs {
		for _, tc := range m.ToolCalls {
			var args struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil || args.Path == "" {
				continue
			}
			switch tc.Function.Name {
			case "read":
				readSet[args.Path] = true
			case "write", "edit":
				modSet[args.Path] = true
			}
		}
	}

	// Files that were written/edited are not listed under read-files.
	for p := range readSet {
		if !modSet[p] {
			readFiles = append(readFiles, p)
		}
	}
	for p := range modSet {
		modifiedFiles = append(modifiedFiles, p)
	}
	return
}

// buildFileTags returns an XML block describing file operations, or "" if both lists are empty.
func buildFileTags(readFiles, modifiedFiles []string) string {
	if len(readFiles) == 0 && len(modifiedFiles) == 0 {
		return ""
	}
	var sb strings.Builder
	if len(readFiles) > 0 {
		sb.WriteString("<read-files>\n")
		for _, f := range readFiles {
			sb.WriteString(f + "\n")
		}
		sb.WriteString("</read-files>\n")
	}
	if len(modifiedFiles) > 0 {
		sb.WriteString("<modified-files>\n")
		for _, f := range modifiedFiles {
			sb.WriteString(f + "\n")
		}
		sb.WriteString("</modified-files>\n")
	}
	return sb.String()
}

// buildMessages formats a slice of messages for inclusion in a summarisation prompt.
// Each message is truncated to maxChars to avoid blowing up a small model's context.
func buildMessages(msgs []llm.Message, maxChars int) string {
	var sb strings.Builder
	for _, m := range msgs {
		content := m.Content
		if len(content) > maxChars {
			content = content[:maxChars] + "…"
		}
		if content == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, content))
	}
	return sb.String()
}

// summarise calls compressClient with the given prompt and returns the response text.
func (a *Agent) summarise(ctx context.Context, prompt string) (string, error) {
	req := llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	}
	resp, err := a.compressClient.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// findCleanCut returns the index (inclusive) of the last message in middle that
// represents a clean turn boundary — i.e. a user message or an assistant message
// without pending tool calls. Messages after that index form an incomplete "partial
// turn" that straddles the keep-tail window.
//
// Returns (len(middle)-1, false) when no split is needed (last message is already clean).
// Returns (cleanIdx, true) when a split was detected.
func findCleanCut(middle []llm.Message) (cleanIdx int, split bool) {
	last := middle[len(middle)-1]
	// Already at a clean boundary.
	if last.Role == "user" || (last.Role == "assistant" && len(last.ToolCalls) == 0) {
		return len(middle) - 1, false
	}
	// Scan backwards for the last clean boundary.
	for i := len(middle) - 2; i >= 0; i-- {
		m := middle[i]
		if m.Role == "user" || (m.Role == "assistant" && len(m.ToolCalls) == 0) {
			return i, true
		}
	}
	// No clean boundary found — treat entire middle as partial turn.
	return -1, true
}

// maybeCompress checks whether history exceeds a.compressThreshold (estimated tokens)
// and, if so, summarises the middle portion using a.compressClient.
//
// Strategy:
//
//  1. Keep: history[0] (system message) + last keepTail messages verbatim.
//  2. Find a clean cut point so we never split a turn mid-stream.
//  3. If the cut lands mid-turn (split), generate two summaries in parallel:
//     – history summary  (messages up to clean cut)
//     – turn-prefix summary (partial turn after clean cut)
//  4. Iterative updates: when a previous summary exists, pass it as context so
//     the model produces an incremental merge rather than a full re-summarise.
//  5. Prepend <read-files>/<modified-files> XML to the final summary.
//
// Non-fatal: on error a warning is printed to stderr and history is left untouched.
func (a *Agent) maybeCompress(ctx context.Context) {
	if a.compressClient == nil || a.compressThreshold <= 0 {
		return
	}
	tokens := estimateTokens(a.history)
	if tokens < a.compressThreshold {
		return
	}

	const keepTail = 6
	// Need at minimum: system(1) + at least one middle turn + keepTail
	if len(a.history) < keepTail+2 {
		return
	}

	middle := a.history[1 : len(a.history)-keepTail]
	tail := a.history[len(a.history)-keepTail:]

	// ── Step 1: find clean cut point ──────────────────────────────────────
	cleanIdx, hasSplit := findCleanCut(middle)

	var mainMiddle, partialTurn []llm.Message
	if hasSplit && cleanIdx >= 0 {
		mainMiddle = middle[:cleanIdx+1]
		partialTurn = middle[cleanIdx+1:]
	} else if hasSplit && cleanIdx < 0 {
		// Entire middle is a partial turn — compress as one unit without split.
		mainMiddle = middle
		partialTurn = nil
		hasSplit = false
	} else {
		mainMiddle = middle
	}

	// Extract file operation lists from all compressed messages.
	readFiles, modifiedFiles := extractFileLists(middle)
	fileTags := buildFileTags(readFiles, modifiedFiles)

	// ── Step 2: build summary prompt(s) ───────────────────────────────────
	const msgMaxChars = 600

	// Iterative vs. initial prompt for the main history block.
	var historyPrompt string
	if a.lastSummary != "" {
		historyPrompt = "You have an existing summary of an earlier part of this conversation:\n\n" +
			a.lastSummary + "\n\n" +
			"Update the summary to also include the new conversation below. " +
			"Keep the total under 250 words. " +
			"Preserve all key facts: files changed, commands run, decisions made, current state.\n\n" +
			buildMessages(mainMiddle, msgMaxChars)
	} else {
		historyPrompt = "Summarize the following conversation history in under 200 words.\n" +
			"Include: files changed, commands executed, key findings, and current state.\n\n" +
			buildMessages(mainMiddle, msgMaxChars)
	}

	// ── Step 3: generate summary (parallel when split) ────────────────────
	var (
		historySummary    string
		partialSummary    string
		historyErr        error
		partialErr        error
	)

	if hasSplit && len(partialTurn) > 0 {
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			historySummary, historyErr = a.summarise(ctx, historyPrompt)
		}()

		go func() {
			defer wg.Done()
			partialPrompt := "Briefly summarize (under 100 words) this incomplete turn that was in progress:\n\n" +
				buildMessages(partialTurn, msgMaxChars)
			partialSummary, partialErr = a.summarise(ctx, partialPrompt)
		}()

		wg.Wait()

		if historyErr != nil {
			fmt.Fprintf(os.Stderr, "⚠  context compress failed: %v\n", historyErr)
			return
		}
		if partialErr != nil {
			// Partial summary failure is non-fatal — fall back to history-only.
			fmt.Fprintf(os.Stderr, "⚠  partial-turn summary failed (ignored): %v\n", partialErr)
			partialSummary = ""
		}
	} else {
		historySummary, historyErr = a.summarise(ctx, historyPrompt)
		if historyErr != nil {
			fmt.Fprintf(os.Stderr, "⚠  context compress failed: %v\n", historyErr)
			return
		}
	}

	// ── Step 4: assemble final summary message ─────────────────────────────
	var summaryContent strings.Builder
	summaryContent.WriteString("[Earlier context — auto-summarised]\n")
	if fileTags != "" {
		summaryContent.WriteString(fileTags)
	}
	summaryContent.WriteString(historySummary)
	if partialSummary != "" {
		summaryContent.WriteString("\n---\n[Partial turn in progress when context was compressed]\n")
		summaryContent.WriteString(partialSummary)
	}

	// Save summary for iterative updates next time.
	a.lastSummary = summaryContent.String()

	summary := llm.Message{
		Role:    "assistant",
		Content: a.lastSummary,
	}

	// Rebuild history: system + summary + tail
	newHistory := make([]llm.Message, 0, 2+keepTail)
	newHistory = append(newHistory, a.history[0]) // system message preserved verbatim
	newHistory = append(newHistory, summary)
	newHistory = append(newHistory, tail...)
	a.history = newHistory

	after := estimateTokens(a.history)
	splitLabel := ""
	if hasSplit {
		splitLabel = " (split-turn)"
	}
	fmt.Fprintf(os.Stderr, "🗜  context compressed%s (~%d → ~%d estimated tokens)\n", splitLabel, tokens, after)
	if len(modifiedFiles) > 0 {
		fmt.Fprintf(os.Stderr, "    modified: %s\n", strings.Join(modifiedFiles, ", "))
	}
}
