package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// dreamSystemPrompt instructs Ollama to distill buffer entries into context.md.
const dreamSystemPrompt = `You are a memory assistant for a coding AI agent called Luna.

Review the recent conversation history and produce an updated Project Context.
Extract and preserve:
- Key technical decisions and architectural patterns
- Project structure, conventions, and rules
- User preferences and working style
- Important context that will help in future sessions

Rules:
- Be concise. Keep under 400 words.
- Use bullet points in Markdown.
- Merge with Existing Context — remove duplicates, resolve contradictions.
- Respond in the same language as the conversations (Japanese if Japanese).
- Output ONLY the updated context, no preamble or explanation.`

// Dream reads buffer entries, calls Ollama to consolidate them, and updates context.md.
// If dryRun is true, the result is printed but not saved.
func Dream(store *Store, ollamaBaseURL, model string, dryRun bool) error {
	entries, err := store.ReadBuffer()
	if err != nil {
		return fmt.Errorf("read buffer: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("buffer is empty — have some conversations first, then run luna dream")
	}

	// Build combined text from buffer entries.
	var combined strings.Builder
	for i, e := range entries {
		combined.WriteString(fmt.Sprintf("### Session %d (%s)\n", i+1, e.Timestamp))
		combined.WriteString(e.Text)
		combined.WriteString("\n\n")
	}

	existing, _ := store.ReadContext()
	if existing == "" {
		existing = "(none yet)"
	}

	userPrompt := fmt.Sprintf(
		"## Existing Context\n%s\n\n## Recent Conversations\n%s\n\n## Updated Project Context:",
		existing, combined.String(),
	)

	fmt.Printf("🌙 dreaming over %d buffer entries with %s ...\n", len(entries), model)

	result, err := callOllama(ollamaBaseURL, model, dreamSystemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("ollama: %w", err)
	}

	if dryRun {
		fmt.Println("\n--- dry run result (not saved) ---")
		fmt.Println(result)
		fmt.Println("---")
		return nil
	}

	if err := store.WriteContext(result); err != nil {
		return fmt.Errorf("write context.md: %w", err)
	}
	if err := store.ClearBuffer(); err != nil {
		return fmt.Errorf("clear buffer: %w", err)
	}

	fmt.Printf("✅ context.md updated (%d entries processed, buffer cleared)\n", len(entries))
	fmt.Printf("   → %s\n", store.ContextPath())
	return nil
}

// callOllama sends a prompt to Ollama's /api/generate and returns the response text.
func callOllama(baseURL, model, system, prompt string) (string, error) {
	base := strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")

	body, err := json.Marshal(map[string]any{
		"model":  model,
		"system": system,
		"prompt": prompt,
		"stream": false,
		"options": map[string]any{
			"num_ctx":     8192,
			"num_predict": 2048,
			"temperature": 0.3,
		},
	})
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(base+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("ollama: %s", result.Error)
	}
	return strings.TrimSpace(result.Response), nil
}
