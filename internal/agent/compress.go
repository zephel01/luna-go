package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

// maybeCompress checks whether history exceeds a.compressThreshold (estimated tokens)
// and, if so, summarises the middle portion using a.compressClient.
//
// Strategy:
//   - Keep: history[0] (system message) + last keepTail messages verbatim.
//   - Compress: everything in between → single summary assistant message.
//   - File lists: read/write/edit paths from the compressed portion are recorded
//     as <read-files> / <modified-files> XML tags so the LLM retains context.
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

	// Extract file operation lists from the compressed portion.
	readFiles, modifiedFiles := extractFileLists(middle)
	fileTags := buildFileTags(readFiles, modifiedFiles)

	// Build the compression prompt. Truncate individual messages to avoid
	// sending the entire context to the small model.
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation history in under 200 words.\n")
	sb.WriteString("Include: files changed, commands executed, key findings, and current state.\n\n")
	for _, m := range middle {
		content := m.Content
		if len(content) > 600 {
			content = content[:600] + "…"
		}
		if content == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, content))
	}

	compReq := llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: sb.String()},
		},
	}
	resp, err := a.compressClient.Complete(ctx, compReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠  context compress failed: %v\n", err)
		return
	}

	// Build summary content: file tags first, then the LLM summary.
	summaryContent := "[Earlier context — auto-summarised]\n"
	if fileTags != "" {
		summaryContent += fileTags
	}
	summaryContent += resp.Content

	summary := llm.Message{
		Role:    "assistant",
		Content: summaryContent,
	}

	// Rebuild history: system + summary + tail
	newHistory := make([]llm.Message, 0, 2+keepTail)
	newHistory = append(newHistory, a.history[0]) // system message preserved verbatim
	newHistory = append(newHistory, summary)
	newHistory = append(newHistory, tail...)
	a.history = newHistory

	after := estimateTokens(a.history)
	fmt.Fprintf(os.Stderr, "🗜  context compressed (~%d → ~%d estimated tokens)\n", tokens, after)
	if len(modifiedFiles) > 0 {
		fmt.Fprintf(os.Stderr, "    modified: %s\n", strings.Join(modifiedFiles, ", "))
	}
}
