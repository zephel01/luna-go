package tools

import (
	"context"
	"encoding/json"

	"github.com/zephel01/luna-go/internal/llm"
)

// Tool is the interface every luna tool must implement.
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any // OpenAI-compatible JSON Schema object
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// Registry holds the available tools and produces their LLM definitions.
type Registry struct {
	tools map[string]Tool
	order []string // insertion order for deterministic output
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool. Duplicate names silently overwrite.
func (r *Registry) Register(t Tool) {
	if _, exists := r.tools[t.Name()]; !exists {
		r.order = append(r.order, t.Name())
	}
	r.tools[t.Name()] = t
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns the list of tool definitions to pass to the LLM.
func (r *Registry) Definitions() []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		defs = append(defs, llm.ToolDef{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.InputSchema(),
			},
		})
	}
	return defs
}
