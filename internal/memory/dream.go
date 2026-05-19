package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// dreamSystemPrompt instructs Ollama to distill buffer entries into a structured
// three-section output: context, skills, and experience.
const dreamSystemPrompt = `You are a memory assistant for a coding AI agent called Luna.

Review the recent conversation history and produce an updated memory document with EXACTLY three sections.

## context
Project-specific technical decisions, architecture, conventions, and rules.
- Merge with Existing Context — remove duplicates, resolve contradictions.
- Keep under 300 words. Use bullet points.

## skills
Reusable procedural patterns that apply beyond this project (e.g. "how to run tests", "how to write a commit message").
For each skill, use this exact format:

### <skill-name>
scope: project
description: <one-line description>
---
## Steps
1. ...

- scope must be either "project" (default) or "global" (use global only if truly universal).
- Omit this section entirely if no reusable skills were found.

## experience
Implicit knowledge, intuitions, and "secret sauce" — patterns that are hard to make explicit but emerged from the conversations.
- Keep under 150 words. Freeform prose or bullets.
- Omit this section entirely if nothing noteworthy.

Rules:
- Output ALL three section headers (## context, ## skills, ## experience) even if a section is empty.
- Respond in the same language as the conversations (Japanese if Japanese).
- Output ONLY the structured memory document, no preamble or explanation.`

// dreamSkill holds a parsed skill extracted from the ## skills section.
type dreamSkill struct {
	name    string
	global  bool
	content string // full SKILL.md body including frontmatter
}

// Dream reads buffer entries, calls Ollama to consolidate them, and writes:
//   - context.md  (project-specific knowledge)
//   - SKILL.md    per skill (project or global scope)
//   - experience.md (implicit knowledge / secret sauce)
//
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

	existingCtx, _ := store.ReadContext()
	if existingCtx == "" {
		existingCtx = "(none yet)"
	}
	existingExp, _ := store.ReadExperience()
	if existingExp == "" {
		existingExp = "(none yet)"
	}

	userPrompt := fmt.Sprintf(
		"## Existing Context\n%s\n\n## Existing Experience\n%s\n\n## Recent Conversations\n%s",
		existingCtx, existingExp, combined.String(),
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

	// Parse the three-section output.
	ctx, skills, exp := parseDreamOutput(result)

	// Write context.md.
	if err := store.WriteContext(ctx); err != nil {
		return fmt.Errorf("write context.md: %w", err)
	}
	fmt.Printf("   📝 context.md updated\n")

	// Write experience.md (only when non-empty).
	if strings.TrimSpace(exp) != "" {
		if err := store.WriteExperience(exp); err != nil {
			return fmt.Errorf("write experience.md: %w", err)
		}
		fmt.Printf("   ✨ experience.md updated\n")
	}

	// Write skill files.
	for _, sk := range skills {
		scope := "project"
		if sk.global {
			scope = "global"
		}
		if err := store.WriteSkill(sk.name, sk.content, sk.global); err != nil {
			fmt.Printf("   ⚠  skill %q write failed: %v\n", sk.name, err)
			continue
		}
		fmt.Printf("   🔧 skill %q saved (%s)\n", sk.name, scope)
	}

	if err := store.ClearBuffer(); err != nil {
		return fmt.Errorf("clear buffer: %w", err)
	}

	fmt.Printf("✅ dream complete (%d entries processed, buffer cleared)\n", len(entries))
	fmt.Printf("   → %s\n", store.ContextPath())
	return nil
}

// parseDreamOutput splits the three-section dream output into its parts.
// Falls back to treating the entire output as context if no section headers are found
// (backward compatibility with older Ollama responses).
func parseDreamOutput(raw string) (ctx string, skills []dreamSkill, exp string) {
	const (
		hContext    = "## context"
		hSkills     = "## skills"
		hExperience = "## experience"
	)

	// Normalise line endings and lower-case for section detection.
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")

	type section struct {
		name  string
		lines []string
	}
	var sections []section
	current := section{name: "preamble"}

	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		switch lower {
		case hContext:
			sections = append(sections, current)
			current = section{name: "context"}
		case hSkills:
			sections = append(sections, current)
			current = section{name: "skills"}
		case hExperience:
			sections = append(sections, current)
			current = section{name: "experience"}
		default:
			current.lines = append(current.lines, line)
		}
	}
	sections = append(sections, current)

	// Check whether any known section was found.
	hasSection := false
	for _, s := range sections {
		if s.name == "context" || s.name == "skills" || s.name == "experience" {
			hasSection = true
			break
		}
	}
	if !hasSection {
		// Fallback: treat entire output as context.
		return strings.TrimSpace(raw), nil, ""
	}

	for _, s := range sections {
		body := strings.TrimSpace(strings.Join(s.lines, "\n"))
		switch s.name {
		case "context":
			ctx = body
		case "skills":
			skills = parseSkillsSection(body)
		case "experience":
			exp = body
		}
	}
	return ctx, skills, exp
}

// parseSkillsSection parses the ## skills section into individual dreamSkill entries.
// Each skill starts with a ### header.
func parseSkillsSection(body string) []dreamSkill {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	// Split on ### headers.
	var result []dreamSkill
	var current *dreamSkill

	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "### ") {
			if current != nil {
				result = append(result, finaliseDreamSkill(current))
			}
			name := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			current = &dreamSkill{name: slugify(name)}
			continue
		}
		if current != nil {
			current.content += line + "\n"
		}
	}
	if current != nil {
		result = append(result, finaliseDreamSkill(current))
	}
	return result
}

// finaliseDreamSkill extracts scope/description from the raw content block and
// assembles the SKILL.md body with proper YAML frontmatter.
func finaliseDreamSkill(sk *dreamSkill) dreamSkill {
	var (
		scope       = "project"
		description = ""
		bodyLines   []string
		pastFrontmatter = false
		inBody          = false
	)

	for _, line := range strings.Split(sk.content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Parse loose key: value lines before the --- separator.
		if !inBody {
			lower := strings.ToLower(trimmed)
			if strings.HasPrefix(lower, "scope:") {
				scope = strings.TrimSpace(trimmed[len("scope:"):])
				pastFrontmatter = true
				continue
			}
			if strings.HasPrefix(lower, "description:") {
				description = strings.TrimSpace(trimmed[len("description:"):])
				pastFrontmatter = true
				continue
			}
			if trimmed == "---" && pastFrontmatter {
				inBody = true
				continue
			}
			// If we hit a markdown heading, treat everything from here as body.
			if strings.HasPrefix(trimmed, "#") {
				inBody = true
			}
		}

		if inBody {
			bodyLines = append(bodyLines, line)
		}
	}

	if description == "" {
		description = sk.name
	}

	// Build SKILL.md content with frontmatter.
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", sk.name))
	sb.WriteString(fmt.Sprintf("description: %s\n", description))
	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimSpace(strings.Join(bodyLines, "\n")))
	sb.WriteString("\n")

	return dreamSkill{
		name:    sk.name,
		global:  strings.TrimSpace(strings.ToLower(scope)) == "global",
		content: sb.String(),
	}
}

// slugify converts a skill name to a filesystem-safe slug.
func slugify(name string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			sb.WriteRune(r)
		} else if r == ' ' || r == '_' {
			sb.WriteRune('-')
		}
	}
	return strings.Trim(sb.String(), "-")
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
