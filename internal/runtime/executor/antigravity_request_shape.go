package executor

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

// antigravityRequestShapeEnv turns on one log line per upstream generate call
// describing the shape of the final body (what Antigravity actually receives).
// Unset or any value other than "1": nothing is logged and nothing is parsed.
const antigravityRequestShapeEnv = "CLIPROXY_LOG_REQUEST_SHAPE"

type antigravityRequestShape struct {
	Contents             int
	ToolEntries          int
	FunctionDeclarations int
	OtherToolEntries     int
	TotalDeclaredTools   int
	Bytes                int
}

// measureAntigravityRequestShape counts the parts of the wrapped Antigravity
// body that decide what the model sees: request.contents (the turns),
// request.tools[*].functionDeclarations (callable tools), and tool entries
// without declarations (built-ins such as googleSearch).
func measureAntigravityRequestShape(body []byte) antigravityRequestShape {
	s := antigravityRequestShape{Bytes: len(body)}
	s.Contents = len(gjson.GetBytes(body, "request.contents").Array())
	for _, t := range gjson.GetBytes(body, "request.tools").Array() {
		s.ToolEntries++
		fd := t.Get("functionDeclarations")
		if fd.IsArray() {
			s.FunctionDeclarations += len(fd.Array())
		} else {
			s.OtherToolEntries++
		}
	}
	s.TotalDeclaredTools = s.FunctionDeclarations + s.OtherToolEntries
	return s
}

func logAntigravityRequestShape(modelName string, stream bool, body []byte) {
	if os.Getenv(antigravityRequestShapeEnv) != "1" {
		return
	}
	s := measureAntigravityRequestShape(body)
	log.WithFields(log.Fields{
		"model":                 modelName,
		"stream":                stream,
		"contents":              s.Contents,
		"tool_entries":          s.ToolEntries,
		"function_declarations": s.FunctionDeclarations,
		"other_tool_entries":    s.OtherToolEntries,
		"total_declared_tools":  s.TotalDeclaredTools,
		"body_bytes":            s.Bytes,
	}).Info("antigravity request shape (final body, before upstream POST)")
}
