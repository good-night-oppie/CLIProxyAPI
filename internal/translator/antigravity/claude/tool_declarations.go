package claude

import (
	"strings"

	"github.com/tidwall/gjson"
)

// ToolDeclarationCheck compares the tools a Claude request asked for with the
// function declarations in the translated Antigravity request.
type ToolDeclarationCheck struct {
	Requested []string // Claude tools that should become declarations (web search excluded)
	Declared  []string // names in request.tools[*].functionDeclarations[*]
}

// CheckToolDeclarations reports requested vs declared tools. Typed web-search
// tools are excluded (they become googleSearch, not a declaration), and a
// tool_choice of "none" requests no tools, so nothing is counted then.
func CheckToolDeclarations(claudeRequest, antigravityRequest []byte) ToolDeclarationCheck {
	var c ToolDeclarationCheck
	tc := gjson.GetBytes(claudeRequest, "tool_choice")
	tcType := tc.String()
	if tc.IsObject() {
		tcType = tc.Get("type").String()
	}
	if strings.EqualFold(strings.TrimSpace(tcType), "none") {
		return c
	}
	for _, t := range gjson.GetBytes(claudeRequest, "tools").Array() {
		if isClaudeTypedWebSearchToolType(t.Get("type").String()) {
			continue
		}
		c.Requested = append(c.Requested, t.Get("name").String())
	}
	for _, t := range gjson.GetBytes(antigravityRequest, "request.tools").Array() {
		for _, fd := range t.Get("functionDeclarations").Array() {
			c.Declared = append(c.Declared, fd.Get("name").String())
		}
	}
	return c
}

// Dropped lists requested names with no declaration of the same name. Names
// the translator sanitized (renamed) also show here; the counts are exact.
func (c ToolDeclarationCheck) Dropped() []string {
	have := make(map[string]bool, len(c.Declared))
	for _, n := range c.Declared {
		have[n] = true
	}
	var out []string
	for _, n := range c.Requested {
		if !have[n] {
			out = append(out, n)
		}
	}
	return out
}
