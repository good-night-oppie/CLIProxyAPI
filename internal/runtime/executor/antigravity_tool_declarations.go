package executor

import (
	"fmt"
	"net/http"
	"strings"

	antigravityclaude "github.com/router-for-me/CLIProxyAPI/v7/internal/translator/antigravity/claude"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
	log "github.com/sirupsen/logrus"
)

// failClosedOnDroppedTools runs on the final payload, just before buildRequest.
// Fewer declarations than requested tools: log the missing names. None left
// while the client sent tools: refuse instead of sending a zero-tool request
// (the model would answer that no tools exist while the client believes it
// sent all of them).
func failClosedOnDroppedTools(from sdktranslator.Format, modelName string, claudeRequest, payload []byte) error {
	if from != sdktranslator.FormatClaude {
		return nil
	}
	c := antigravityclaude.CheckToolDeclarations(claudeRequest, payload)
	if len(c.Requested) == 0 || len(c.Declared) >= len(c.Requested) {
		return nil
	}
	dropped := c.Dropped()
	log.Warnf("antigravity executor: model %s: %d of %d requested tools declared upstream; missing: %s",
		modelName, len(c.Declared), len(c.Requested), strings.Join(dropped, ", "))
	if len(c.Declared) > 0 {
		return nil
	}
	return statusErr{code: http.StatusBadRequest, msg: fmt.Sprintf(
		"antigravity: the request carried %d tools but none could be declared upstream (dropped: %s); refusing to send a request with no tools",
		len(c.Requested), strings.Join(dropped, ", "))}
}
