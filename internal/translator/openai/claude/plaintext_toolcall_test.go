package claude

import (
	"context"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

const claudeToolsReq = `{
  "stream": false,
  "tools": [
    {"name":"Bash","input_schema":{"type":"object","properties":{"command":{"type":"string"},"description":{"type":"string"}},"required":["command"]}},
    {"name":"Read","input_schema":{"type":"object","properties":{"file_path":{"type":"string"}},"required":["file_path"]}}
  ]
}`

func TestPromotePlaintextOpenAIToolCallsDots(t *testing.T) {
	raw := []byte(`{"id":"c1","choices":[{"index":0,"message":{"role":"assistant","content":"<dots_function_call>\n<name>Bash</name>\n<arguments>{\"command\":\"pwd\"}</arguments>\n</dots_function_call>"},"finish_reason":"stop"}]}`)
	out := promotePlaintextOpenAIToolCalls(raw, []byte(claudeToolsReq))
	if gjson.GetBytes(out, "choices.0.finish_reason").String() != "tool_calls" {
		t.Fatalf("finish_reason = %s", out)
	}
	if got := gjson.GetBytes(out, "choices.0.message.tool_calls.0.function.name").String(); got != "Bash" {
		t.Fatalf("name = %q", got)
	}
	if got := gjson.GetBytes(out, "choices.0.message.tool_calls.0.function.arguments").String(); !strings.Contains(got, `"command":"pwd"`) {
		t.Fatalf("arguments = %q", got)
	}
}

func TestPromotePlaintextOpenAIToolCallsSkipsExistingToolCalls(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"content":"<dots_function_call><name>Bash</name><arguments>{\"command\":\"pwd\"}</arguments></dots_function_call>","tool_calls":[{"id":"x","type":"function","function":{"name":"Read","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
	out := promotePlaintextOpenAIToolCalls(raw, []byte(claudeToolsReq))
	if string(out) != string(raw) {
		t.Fatalf("expected byte-identical when tool_calls already present")
	}
}

func TestPromotePlaintextOpenAIToolCallsSlashCommandUntouched(t *testing.T) {
	raw := []byte(`{"choices":[{"message":{"content":"<command-name>/model</command-name>\n<command-args></command-args>"},"finish_reason":"stop"}]}`)
	out := promotePlaintextOpenAIToolCalls(raw, []byte(claudeToolsReq))
	if string(out) != string(raw) {
		t.Fatalf("slash commands must not become tool_calls")
	}
}

func TestConvertOpenAINonStreamingPromotesXMLToClaudeToolUse(t *testing.T) {
	raw := []byte(`{"id":"c1","model":"openrouter/free","choices":[{"index":0,"message":{"role":"assistant","content":"<invoke name=\"Read\"><parameter name=\"file_path\">/tmp/a.txt</parameter></invoke>"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`)
	var paramAny any
	emitted := ConvertOpenAIResponseToClaude(context.Background(), "", []byte(claudeToolsReq), nil, append([]byte("data: "), raw...), &paramAny)
	if len(emitted) == 0 {
		t.Fatal("expected emitted body")
	}
	body := emitted[0]
	if gjson.GetBytes(body, "stop_reason").String() != "tool_use" {
		t.Fatalf("stop_reason = %s body=%s", gjson.GetBytes(body, "stop_reason").String(), body)
	}
	found := false
	for _, block := range gjson.GetBytes(body, "content").Array() {
		if block.Get("type").String() == "tool_use" && block.Get("name").String() == "Read" {
			found = true
			if block.Get("input.file_path").String() != "/tmp/a.txt" {
				t.Fatalf("input = %s", block.Raw)
			}
		}
	}
	if !found {
		t.Fatalf("expected Read tool_use, got %s", body)
	}
}
