package claude

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	argonHeadRe       = regexp.MustCompile(`^call:default_api:([A-Za-z0-9_.\-]+)\{`)
	commandNameRe     = regexp.MustCompile(`(?i)<command-name>\s*([^<]+?)\s*</command-name>`)
	argPairRe         = regexp.MustCompile(`(?is)<arg_key>\s*([^<]+?)\s*</arg_key>\s*<arg_value>(.*?)</arg_value>`)
	invokeNameRe      = regexp.MustCompile(`(?i)<(?:invoke|tool_call|minimax:tool_call)\b[^>]*\bname=["']([^"']+)["']`)
	invokeParamRe     = regexp.MustCompile(`(?is)<parameter\b[^>]*\bname=["']([^"']+)["'][^>]*>(.*?)</parameter>`)
	dotsNameRe        = regexp.MustCompile(`(?is)<name>\s*([^<]+?)\s*</name>`)
	dotsArgsRe        = regexp.MustCompile(`(?is)<arguments>\s*(.*?)\s*</arguments>`)
	xmlCallPrefixes   = []string{"<dots_function_call", "<command-name>", "<invoke", "<tool_call>", "<function_calls>", "<minimax:tool_call"}
)

type plaintextCall struct {
	Name   string
	Input  map[string]any
	Prefix string
}

func promotePlaintextOpenAIToolCalls(rawJSON, originalClaude []byte) []byte {
	if len(rawJSON) == 0 || !gjson.ValidBytes(rawJSON) {
		return rawJSON
	}
	choice := gjson.GetBytes(rawJSON, "choices.0")
	if !choice.Exists() {
		return rawJSON
	}
	if calls := choice.Get("message.tool_calls"); calls.Exists() && calls.IsArray() && len(calls.Array()) > 0 {
		return rawJSON
	}
	content := choice.Get("message.content").String()
	if strings.TrimSpace(content) == "" {
		return rawJSON
	}
	names := claudeToolNames(originalClaude)
	if len(names) == 0 {
		return rawJSON
	}
	call := parsePlaintextToolCall(content, names)
	if call == nil {
		return rawJSON
	}
	args, err := json.Marshal(call.Input)
	if err != nil {
		return rawJSON
	}
	id := "call_plaintext_1"
	out := rawJSON
	if call.Prefix == "" {
		out, _ = sjson.SetBytes(out, "choices.0.message.content", "")
	} else {
		out, _ = sjson.SetBytes(out, "choices.0.message.content", call.Prefix)
	}
	toolJSON := []byte(`[{"id":"","type":"function","function":{"name":"","arguments":""}}]`)
	toolJSON, _ = sjson.SetBytes(toolJSON, "0.id", id)
	toolJSON, _ = sjson.SetBytes(toolJSON, "0.function.name", call.Name)
	toolJSON, _ = sjson.SetBytes(toolJSON, "0.function.arguments", string(args))
	out, _ = sjson.SetRawBytes(out, "choices.0.message.tool_calls", toolJSON)
	out, _ = sjson.SetBytes(out, "choices.0.finish_reason", "tool_calls")
	return out
}

func claudeToolNames(original []byte) map[string]string {
	mapped := util.ToolNameMapFromClaudeRequest(original)
	if len(mapped) == 0 {
		return nil
	}
	out := make(map[string]string, len(mapped)+4)
	for _, name := range mapped {
		out[strings.ToLower(name)] = name
	}
	return out
}

func lookupClaudeTool(names map[string]string, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if name, ok := names[strings.ToLower(raw)]; ok {
		return name
	}
	return ""
}

func parsePlaintextToolCall(content string, names map[string]string) *plaintextCall {
	t := strings.TrimSpace(content)
	if t == "" {
		return nil
	}
	if call := parseExactPlaintextCall(t, names); call != nil {
		call.Prefix = ""
		return call
	}
	idx := lastXMLCallIndex(t)
	if idx <= 0 {
		return nil
	}
	call := parseExactPlaintextCall(strings.TrimSpace(t[idx:]), names)
	if call == nil {
		return nil
	}
	call.Prefix = strings.TrimRight(t[:idx], " \t\r\n")
	return call
}

func lastXMLCallIndex(t string) int {
	idx := -1
	for _, p := range xmlCallPrefixes {
		if i := strings.LastIndex(t, p); i > idx {
			idx = i
		}
	}
	return idx
}

func parseExactPlaintextCall(t string, names map[string]string) *plaintextCall {
	switch {
	case strings.HasPrefix(t, "call:default_api:"):
		return parseArgonPlaintext(t, names)
	case strings.HasPrefix(t, "<dots_function_call"):
		return parseDotsPlaintext(t, names)
	case strings.HasPrefix(t, "<invoke") || strings.HasPrefix(t, "<tool_call") || strings.HasPrefix(t, "<function_calls") || strings.HasPrefix(t, "<minimax:tool_call"):
		return parseInvokePlaintext(t, names)
	case strings.HasPrefix(t, "<command-name>"):
		return parseCommandNamePlaintext(t, names)
	default:
		return nil
	}
}

func parseArgonPlaintext(t string, names map[string]string) *plaintextCall {
	head := argonHeadRe.FindStringSubmatch(t)
	if head == nil || !strings.HasSuffix(t, "}") {
		return nil
	}
	name := lookupClaudeTool(names, head[1])
	if name == "" {
		return nil
	}
	inner := t[len(head[0]) : len(t)-1]
	input := map[string]any{}
	if err := json.Unmarshal([]byte("{"+inner+"}"), &input); err != nil {
		return nil
	}
	return &plaintextCall{Name: name, Input: input}
}

func parseDotsPlaintext(t string, names map[string]string) *plaintextCall {
	nameM := dotsNameRe.FindStringSubmatch(t)
	if nameM == nil {
		return nil
	}
	argsM := dotsArgsRe.FindStringSubmatch(t)
	if argsM == nil {
		return nil
	}
	input := map[string]any{}
	raw := strings.TrimSpace(argsM[1])
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &input); err != nil {
			return nil
		}
	}
	return finishPlaintextCall(names, nameM[1], input)
}

func parseInvokePlaintext(t string, names map[string]string) *plaintextCall {
	head := invokeNameRe.FindStringSubmatch(t)
	if head == nil {
		return nil
	}
	input := map[string]any{}
	for _, m := range invokeParamRe.FindAllStringSubmatch(t, -1) {
		input[m[1]] = strings.TrimSpace(m[2])
	}
	if len(input) == 0 {
		if argsM := dotsArgsRe.FindStringSubmatch(t); argsM != nil {
			raw := strings.TrimSpace(argsM[1])
			if raw != "" {
				if err := json.Unmarshal([]byte(raw), &input); err != nil {
					return nil
				}
			}
		}
	}
	return finishPlaintextCall(names, head[1], input)
}

func parseCommandNamePlaintext(t string, names map[string]string) *plaintextCall {
	nameM := commandNameRe.FindStringSubmatch(t)
	if nameM == nil {
		return nil
	}
	rawName := strings.TrimSpace(nameM[1])
	if rawName == "" || strings.HasPrefix(rawName, "/") {
		return nil
	}
	input := map[string]any{}
	for _, m := range argPairRe.FindAllStringSubmatch(t, -1) {
		input[strings.TrimSpace(m[1])] = strings.TrimSpace(m[2])
	}
	return finishPlaintextCall(names, rawName, input)
}

func finishPlaintextCall(names map[string]string, rawName string, input map[string]any) *plaintextCall {
	if input == nil {
		input = map[string]any{}
	}
	if name := lookupClaudeTool(names, rawName); name != "" {
		return &plaintextCall{Name: name, Input: input}
	}
	cmd, _ := input["command"].(string)
	if strings.TrimSpace(cmd) == "" {
		return nil
	}
	bash := lookupClaudeTool(names, "Bash")
	if bash == "" {
		return nil
	}
	out := map[string]any{"command": cmd}
	if desc, ok := input["description"].(string); ok && strings.TrimSpace(desc) != "" {
		out["description"] = desc
	}
	return &plaintextCall{Name: bash, Input: out}
}
