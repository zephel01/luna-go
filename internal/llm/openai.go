package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient implements Client for any OpenAI-compatible API,
// including Ollama (http://localhost:11434/v1) and OpenAI proper.
type OpenAIClient struct {
	baseURL    string
	apiKey     string
	model      string
	http       *http.Client
	numCtx     int // 0 = auto (32768 for Ollama), -1 = disabled
	numPredict int // 0 = auto (4096 for Ollama), -1 = disabled
}

// ClientOptions configures an OpenAIClient.
type ClientOptions struct {
	NumCtx     int // Ollama num_ctx (context window). 0 = auto-default, -1 = off.
	NumPredict int // Ollama num_predict (max output tokens). 0 = auto-default, -1 = off.
}

// NewOpenAIClient creates a client. baseURL can be the Ollama root URL
// (http://localhost:11434) or already include /v1 — both work.
func NewOpenAIClient(baseURL, apiKey, model string, opts ...ClientOptions) *OpenAIClient {
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}
	c := &OpenAIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 5 * time.Minute},
	}
	if len(opts) > 0 {
		c.numCtx = opts[0].NumCtx
		c.numPredict = opts[0].NumPredict
	}
	return c
}

// isOllamaEndpoint returns true when the base URL looks like a local Ollama instance.
func isOllamaEndpoint(baseURL string) bool {
	return strings.Contains(baseURL, ":11434") ||
		strings.Contains(baseURL, "localhost") ||
		strings.Contains(baseURL, "127.0.0.1")
}

// --- OpenAI wire types ---

type oaiRequest struct {
	Model     string         `json:"model"`
	Messages  []oaiMessage   `json:"messages"`
	Tools     []ToolDef      `json:"tools,omitempty"`
	Stream    bool           `json:"stream"`
	ExtraBody map[string]any `json:"extra_body,omitempty"` // Ollama-specific options
}

// ollamaExtraBody builds the extra_body for Ollama when num_ctx / num_predict are configured.
// Returns nil when the client is not pointed at an Ollama endpoint or both values are -1 (disabled).
func (c *OpenAIClient) ollamaExtraBody() map[string]any {
	if !isOllamaEndpoint(c.baseURL) {
		return nil
	}
	numCtx := c.numCtx
	if numCtx == 0 {
		numCtx = 32768 // safe default: covers long system prompts + tool declarations
	}
	numPredict := c.numPredict
	if numPredict == 0 {
		numPredict = 4096 // safe default: enough for any tool_call JSON
	}
	if numCtx == -1 && numPredict == -1 {
		return nil // both explicitly disabled
	}
	opts := map[string]any{}
	if numCtx != -1 {
		opts["num_ctx"] = numCtx
	}
	if numPredict != -1 {
		opts["num_predict"] = numPredict
	}
	return map[string]any{"options": opts}
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    any           `json:"content"` // string | null
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	Name       string        `json:"name,omitempty"`
}

type oaiToolCall struct {
	Index    int             `json:"index,omitempty"`
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type,omitempty"`
	Function oaiToolCallFunc `json:"function"`
}

type oaiToolCallFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type oaiResponse struct {
	Choices []oaiChoice `json:"choices"`
	Error   *oaiError   `json:"error,omitempty"`
}

type oaiChoice struct {
	Message      oaiMessage `json:"message"`
	Delta        oaiMessage `json:"delta"`
	FinishReason string     `json:"finish_reason"`
}

type oaiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// --- conversion helpers ---

func (c *OpenAIClient) toOAIMessages(msgs []Message) []oaiMessage {
	out := make([]oaiMessage, len(msgs))
	for i, m := range msgs {
		om := oaiMessage{
			Role:       m.Role,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		// assistant messages with tool_calls must have content=null, not ""
		if m.Role == "assistant" && len(m.ToolCalls) > 0 && m.Content == "" {
			om.Content = nil
		} else {
			om.Content = m.Content
		}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, oaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: oaiToolCallFunc{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		out[i] = om
	}
	return out
}

func fromOAIToolCalls(tcs []oaiToolCall) []ToolCall {
	out := make([]ToolCall, len(tcs))
	for i, tc := range tcs {
		out[i] = ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return out
}

func (c *OpenAIClient) post(ctx context.Context, payload oaiRequest) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return c.http.Do(req)
}

// Complete sends a non-streaming request.
func (c *OpenAIClient) Complete(ctx context.Context, req Request) (*Response, error) {
	payload := oaiRequest{
		Model:     c.model,
		Messages:  c.toOAIMessages(req.Messages),
		Tools:     req.Tools,
		Stream:    false,
		ExtraBody: c.ollamaExtraBody(),
	}
	resp, err := c.post(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	var oaiResp oaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if oaiResp.Error != nil {
		return nil, fmt.Errorf("api error [%s]: %s", oaiResp.Error.Type, oaiResp.Error.Message)
	}
	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	msg := oaiResp.Choices[0].Message
	content := ""
	if s, ok := msg.Content.(string); ok {
		content = s
	}
	return &Response{
		Content:   content,
		ToolCalls: fromOAIToolCalls(msg.ToolCalls),
	}, nil
}

// Stream sends a streaming request (SSE), writes content tokens to out,
// and returns the fully assembled response.
func (c *OpenAIClient) Stream(ctx context.Context, req Request, out io.Writer) (*Response, error) {
	payload := oaiRequest{
		Model:     c.model,
		Messages:  c.toOAIMessages(req.Messages),
		Tools:     req.Tools,
		Stream:    true,
		ExtraBody: c.ollamaExtraBody(),
	}
	resp, err := c.post(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	var (
		contentBuf strings.Builder
		toolCalls  []oaiToolCall // indexed by delta.index
	)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk oaiResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// stream text content
		if s, ok := delta.Content.(string); ok && s != "" {
			contentBuf.WriteString(s)
			fmt.Fprint(out, s)
		}

		// accumulate tool call deltas by index
		for _, dtc := range delta.ToolCalls {
			idx := dtc.Index
			for len(toolCalls) <= idx {
				toolCalls = append(toolCalls, oaiToolCall{})
			}
			if dtc.ID != "" {
				toolCalls[idx].ID = dtc.ID
			}
			if dtc.Type != "" {
				toolCalls[idx].Type = dtc.Type
			}
			toolCalls[idx].Function.Name += dtc.Function.Name
			toolCalls[idx].Function.Arguments += dtc.Function.Arguments
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream read: %w", err)
	}

	return &Response{
		Content:   contentBuf.String(),
		ToolCalls: fromOAIToolCalls(toolCalls),
	}, nil
}
