package helps

import (
	"bytes"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestMeasureAntigravityRequestShape(t *testing.T) {
	body := []byte(`{"request":{"contents":[{"role":"user"},{"role":"model"},{"role":"user"}],` +
		`"tools":[{"functionDeclarations":[{"name":"a"},{"name":"b"}]},{"googleSearch":{}},{"functionDeclarations":[{"name":"c"}]}]}}`)
	got := measureAntigravityRequestShape(body)
	want := antigravityRequestShape{Contents: 3, ToolEntries: 3, FunctionDeclarations: 3, OtherToolEntries: 1, TotalDeclaredTools: 4, Bytes: len(body)}
	if got != want {
		t.Fatalf("shape = %+v, want %+v", got, want)
	}
	if empty := measureAntigravityRequestShape([]byte(`{"request":{}}`)); empty.Contents != 0 || empty.TotalDeclaredTools != 0 {
		t.Fatalf("empty body shape = %+v", empty)
	}
}

func TestLogAntigravityRequestShapeGate(t *testing.T) {
	var buf bytes.Buffer
	prevOut, prevLevel := log.StandardLogger().Out, log.GetLevel()
	log.SetOutput(&buf)
	log.SetLevel(log.InfoLevel)
	defer func() { log.SetOutput(prevOut); log.SetLevel(prevLevel) }()
	body := []byte(`{"request":{"contents":[{}],"tools":[{"functionDeclarations":[{},{}]}]}}`)

	t.Setenv(antigravityRequestShapeEnv, "")
	LogAntigravityRequestShape("m", false, body)
	if buf.Len() != 0 {
		t.Fatalf("flag unset: expected no log, got %q", buf.String())
	}
	t.Setenv(antigravityRequestShapeEnv, "1")
	LogAntigravityRequestShape("m", true, body)
	out := buf.String()
	for _, w := range []string{"contents=1", "function_declarations=2", "total_declared_tools=2", "body_bytes="} {
		if !bytes.Contains([]byte(out), []byte(w)) {
			t.Fatalf("flag on: log missing %q in %q", w, out)
		}
	}
}
