// Package prompts loads reusable prompt templates from ~/.luna-go/prompts/.
//
// Each template is a Markdown file with an optional YAML frontmatter block:
//
//	---
//	name: review-pr
//	description: "Review a pull request for correctness and style"
//	argumentHint: "<branch>"
//	---
//
//	Please review the changes in branch $1. Focus on ...
//
// If frontmatter is absent the filename (without extension) is used as the name.
//
// Argument substitution follows bash-like rules:
//
//	$1, $2, …   positional arguments
//	$@          all arguments joined by space
//	${@:N}      arguments from index N onward (1-based), joined by space
package prompts

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Template is a loaded prompt template.
type Template struct {
	Name         string // slash-command name (lowercase, hyphens allowed)
	Description  string // shown in /help and tab completion
	ArgumentHint string // e.g. "<file> <target>" shown next to the command
	Body         string // template body after frontmatter expansion
}

// Expand substitutes argument placeholders in the template body.
//
//	$1, $2, …  → args[0], args[1], …  (empty string if out of range)
//	$@         → all args joined by space
//	${@:N}     → args[N-1:] joined by space (1-based, empty if out of range)
func (t *Template) Expand(args []string) string {
	body := t.Body

	// ${@:N} — slice from index N-1
	body = reSlice.ReplaceAllStringFunc(body, func(m string) string {
		sub := reSlice.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		n, err := strconv.Atoi(sub[1])
		if err != nil || n < 1 {
			return m
		}
		idx := n - 1
		if idx >= len(args) {
			return ""
		}
		return strings.Join(args[idx:], " ")
	})

	// $@ — all args
	body = strings.ReplaceAll(body, "$@", strings.Join(args, " "))

	// $N — positional (replace longest match first to avoid $1 eating $10)
	for i := len(args); i >= 1; i-- {
		placeholder := "$" + strconv.Itoa(i)
		body = strings.ReplaceAll(body, placeholder, args[i-1])
	}
	// Remaining $N placeholders beyond provided args become empty.
	body = rePositional.ReplaceAllString(body, "")

	return body
}

var (
	reSlice      = regexp.MustCompile(`\$\{@:(\d+)\}`)
	rePositional = regexp.MustCompile(`\$\d+`)
)

// Load reads all *.md files from ~/.luna-go/prompts/ and returns parsed templates.
// Files that fail to parse are silently skipped.
func Load() []Template {
	dir := filepath.Join(homeDir(), ".luna-go", "prompts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // directory doesn't exist yet — normal
	}

	var templates []Template
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		t := parse(e.Name(), string(data))
		if t != nil {
			templates = append(templates, *t)
		}
	}
	return templates
}

// parse extracts frontmatter and body from a template file.
func parse(filename, content string) *Template {
	t := &Template{}

	// Try to extract YAML frontmatter between the first two "---" lines.
	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "\n---")
		if end != -1 {
			fm := content[3 : end+3]
			body := strings.TrimPrefix(content[end+7:], "\n") // skip closing ---\n

			var meta struct {
				Name         string `yaml:"name"`
				Description  string `yaml:"description"`
				ArgumentHint string `yaml:"argumentHint"`
			}
			if err := yaml.Unmarshal([]byte(fm), &meta); err == nil {
				t.Name = meta.Name
				t.Description = meta.Description
				t.ArgumentHint = meta.ArgumentHint
			}
			t.Body = body
		}
	}

	// Fall back to full content as body if no valid frontmatter found.
	if t.Body == "" {
		t.Body = content
	}

	// Fall back to filename (without extension) as name.
	if t.Name == "" {
		t.Name = strings.TrimSuffix(filename, ".md")
	}

	// Sanitise name: lowercase, keep hyphens and alphanumerics.
	t.Name = sanitiseName(t.Name)
	if t.Name == "" {
		return nil
	}

	return t
}

// sanitiseName lowercases and strips characters that are not alphanumeric or hyphen.
func sanitiseName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}
