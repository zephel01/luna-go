package agent

import (
	"testing"
)

// --- cleanContent ---

func TestCleanContent_NoOp(t *testing.T) {
	input := "Hello, world!"
	got := cleanContent(input)
	if got != input {
		t.Errorf("cleanContent(%q) = %q, want %q", input, got, input)
	}
}

func TestCleanContent_StripThinkTag(t *testing.T) {
	input := "<think>reasoning here</think>Final answer"
	got := cleanContent(input)
	if got != "Final answer" {
		t.Errorf("got %q, want %q", got, "Final answer")
	}
}

func TestCleanContent_StripMultilineThink(t *testing.T) {
	input := "<think>\nline 1\nline 2\n</think>\nAnswer"
	got := cleanContent(input)
	if got != "Answer" {
		t.Errorf("got %q", got)
	}
}

func TestCleanContent_StripStopMarker_EotId(t *testing.T) {
	input := "Answer<|eot_id|>"
	got := cleanContent(input)
	if got != "Answer" {
		t.Errorf("got %q", got)
	}
}

func TestCleanContent_StripStopMarker_ImEnd(t *testing.T) {
	input := "Answer<|im_end|>"
	got := cleanContent(input)
	if got != "Answer" {
		t.Errorf("got %q", got)
	}
}

func TestCleanContent_StripStopMarker_EndOfText(t *testing.T) {
	input := "Answer<|end_of_text|>"
	got := cleanContent(input)
	if got != "Answer" {
		t.Errorf("got %q", got)
	}
}

func TestCleanContent_StripBothThinkAndStop(t *testing.T) {
	input := "<think>reasoning</think>Answer<|eot_id|>"
	got := cleanContent(input)
	if got != "Answer" {
		t.Errorf("got %q", got)
	}
}

func TestCleanContent_EmptyInput(t *testing.T) {
	got := cleanContent("")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- extractFirstJSON ---

func TestExtractFirstJSON_Object(t *testing.T) {
	input := `{"name":"read"} some trailing text`
	got := extractFirstJSON(input)
	if got != `{"name":"read"}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractFirstJSON_Array(t *testing.T) {
	input := `[{"name":"read"}] trailing`
	got := extractFirstJSON(input)
	if got != `[{"name":"read"}]` {
		t.Errorf("got %q", got)
	}
}

func TestExtractFirstJSON_Nested(t *testing.T) {
	input := `{"a":{"b":1}} extra`
	got := extractFirstJSON(input)
	if got != `{"a":{"b":1}}` {
		t.Errorf("got %q", got)
	}
}

func TestExtractFirstJSON_NoJSON(t *testing.T) {
	input := "just plain text"
	got := extractFirstJSON(input)
	// Returns the original string when no valid JSON found
	if got != input {
		t.Errorf("got %q, want %q", got, input)
	}
}

// --- parseContentToolCalls ---

func TestParseContentToolCalls_FormatA(t *testing.T) {
	content := `{"name":"read","arguments":{"path":"/tmp/foo.go"}}`
	calls := parseContentToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "read" {
		t.Errorf("name = %q, want %q", calls[0].Function.Name, "read")
	}
}

func TestParseContentToolCalls_FormatB(t *testing.T) {
	content := `{"function":{"name":"bash","arguments":{"command":"ls"}}}`
	calls := parseContentToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "bash" {
		t.Errorf("name = %q, want %q", calls[0].Function.Name, "bash")
	}
}

func TestParseContentToolCalls_Array(t *testing.T) {
	content := `[{"name":"read","arguments":{"path":"/a"}},{"name":"bash","arguments":{"command":"ls"}}]`
	calls := parseContentToolCalls(content)
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Function.Name != "read" {
		t.Errorf("first call name = %q", calls[0].Function.Name)
	}
	if calls[1].Function.Name != "bash" {
		t.Errorf("second call name = %q", calls[1].Function.Name)
	}
}

func TestParseContentToolCalls_CodeFence(t *testing.T) {
	content := "```json\n{\"name\":\"grep\",\"arguments\":{\"pattern\":\"foo\"}}\n```"
	calls := parseContentToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d: %v", len(calls), content)
	}
	if calls[0].Function.Name != "grep" {
		t.Errorf("name = %q", calls[0].Function.Name)
	}
}

func TestParseContentToolCalls_Empty(t *testing.T) {
	calls := parseContentToolCalls("")
	if calls != nil {
		t.Errorf("expected nil for empty input, got %v", calls)
	}
}

func TestParseContentToolCalls_PlainText(t *testing.T) {
	calls := parseContentToolCalls("This is just a text response.")
	if calls != nil {
		t.Errorf("expected nil for plain text, got %v", calls)
	}
}

func TestParseContentToolCalls_TrailingText(t *testing.T) {
	// Model sometimes appends explanation after JSON
	content := `{"name":"read","arguments":{"path":"/foo"}}` + "\n\nI'll read that file now."
	calls := parseContentToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "read" {
		t.Errorf("name = %q", calls[0].Function.Name)
	}
}

func TestParseContentToolCalls_WithThinkPrefix(t *testing.T) {
	// cleanContent is called before parseContentToolCalls in production,
	// but test the parser alone with already-clean input
	content := `{"name":"write","arguments":{"path":"/tmp/x","content":"hello"}}`
	calls := parseContentToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
}

// --- summarize ---

func TestSummarize_Short(t *testing.T) {
	got := summarize("hello", 80)
	if got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestSummarize_Truncate(t *testing.T) {
	input := "0123456789"
	got := summarize(input, 5)
	if got != "01234…" {
		t.Errorf("got %q", got)
	}
}

func TestSummarize_CollapseNewlines(t *testing.T) {
	got := summarize("a\nb\nc", 80)
	if got != "a b c" {
		t.Errorf("got %q", got)
	}
}
