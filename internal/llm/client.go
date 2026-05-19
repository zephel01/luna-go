// Package llm defines the provider-agnostic LLM interface used by the Luna agent.
// It abstracts over Ollama and OpenAI-compatible backends, exposing a unified
// Client interface for both streaming and non-streaming completions.
package llm

import (
	"context"
	"io"
)

// Message represents a single turn in the conversation.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is a single tool invocation requested by the model.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the name and JSON-encoded arguments for a tool call.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDef is the definition sent to the LLM so it knows what tools are available.
type ToolDef struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a single tool's name, description, and input schema.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Request bundles everything needed for a single LLM call.
type Request struct {
	Messages []Message
	Tools    []ToolDef
}

// Response is the normalised output from an LLM call.
type Response struct {
	Content   string
	ToolCalls []ToolCall
}

// Client is the LLM provider interface.
type Client interface {
	// Complete sends a non-streaming request and returns the full response.
	Complete(ctx context.Context, req Request) (*Response, error)
	// Stream sends a streaming request, writing content tokens to out as they
	// arrive, and returns the assembled response (including any tool calls).
	Stream(ctx context.Context, req Request, out io.Writer) (*Response, error)
}
