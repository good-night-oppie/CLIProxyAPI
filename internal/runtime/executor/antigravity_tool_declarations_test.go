package executor

import (
	"bytes"
	"strings"
	"testing"

	antigravityclaude "github.com/router-for-me/CLIProxyAPI/v7/internal/translator/antigravity/claude"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
	log "github.com/sirupsen/logrus"
)

const toolModel = "claude-sonnet-4-6"

func translateClaude(t *testing.T, body string) []byte {
	t.Helper()
	return antigravityclaude.ConvertClaudeRequestToAntigravity(toolModel, []byte(body), false)
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevLevel := log.StandardLogger().Out, log.GetLevel()
	log.SetOutput(&buf)
	log.SetLevel(log.InfoLevel)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetLevel(prevLevel) })
	return &buf
}

const schema = `{"type":"object","properties":{}}`

func TestFailClosedAllToolsDeclared(t *testing.T) {
	req := `{"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"Read","input_schema":` + schema + `},{"name":"Bash","input_schema":` + schema + `}]}`
	buf := captureLog(t)
	if err := failClosedOnDroppedTools(sdktranslator.FormatClaude, toolModel, []byte(req), translateClaude(t, req)); err != nil {
		t.Fatalf("all tools declared: unexpected error %v", err)
	}
	if strings.Contains(buf.String(), "requested tools declared") {
		t.Fatalf("all tools declared: unexpected warning %q", buf.String())
	}
}

func TestFailClosedSomeToolsDroppedLogsNames(t *testing.T) {
	req := `{"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"Read","input_schema":` + schema + `},{"name":"NoSchema"}]}`
	buf := captureLog(t)
	if err := failClosedOnDroppedTools(sdktranslator.FormatClaude, toolModel, []byte(req), translateClaude(t, req)); err != nil {
		t.Fatalf("one tool left: must still send, got %v", err)
	}
	out := buf.String()
	for _, w := range []string{"1 of 2 requested tools declared", "missing: NoSchema", `tool \"NoSchema\" dropped`} {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in log %q", w, out)
		}
	}
}

func TestFailClosedAllToolsDroppedReturnsError(t *testing.T) {
	req := `{"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"A"},{"name":"B","input_schema":"not-an-object"}]}`
	payload := translateClaude(t, req)
	err := failClosedOnDroppedTools(sdktranslator.FormatClaude, toolModel, []byte(req), payload)
	if err == nil {
		t.Fatalf("all tools dropped: expected an error, payload %s", payload)
	}
	se, ok := err.(statusErr)
	if !ok || se.code != 400 || !strings.Contains(se.msg, "carried 2 tools but none") || !strings.Contains(se.msg, "A, B") {
		t.Fatalf("unexpected error %#v", err)
	}
}

func TestFailClosedNoToolsRequested(t *testing.T) {
	for name, req := range map[string]string{
		"no tools":         `{"messages":[{"role":"user","content":"hi"}]}`,
		"tool_choice none": `{"messages":[{"role":"user","content":"hi"}],"tool_choice":{"type":"none"},"tools":[{"name":"A","input_schema":` + schema + `}]}`,
		"web search only":  `{"messages":[{"role":"user","content":"hi"}],"tools":[{"type":"web_search_20250305","name":"web_search"}]}`,
	} {
		if err := failClosedOnDroppedTools(sdktranslator.FormatClaude, toolModel, []byte(req), translateClaude(t, req)); err != nil {
			t.Fatalf("%s: unexpected error %v", name, err)
		}
	}
	// Other source formats are not checked here.
	if err := failClosedOnDroppedTools(sdktranslator.FormatOpenAI, toolModel, []byte(`{"tools":[{"name":"A"}]}`), []byte(`{}`)); err != nil {
		t.Fatalf("non-claude format: unexpected error %v", err)
	}
}
