package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/subscription"
)

// FetchSubscriptionQuota exposes canonical observations under existing management
// authentication. It never returns provider credentials or raw upstream errors.
func (h *Handler) FetchSubscriptionQuota(c *gin.Context) {
	var body struct {
		AuthIndex string `json:"auth_index"`
	}
	if c.ShouldBindJSON(&body) != nil || body.AuthIndex == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth_index required"})
		return
	}
	auth := h.authByIndex(body.AuthIndex)
	if auth == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auth not found"})
		return
	}
	client := &http.Client{Transport: h.apiCallTransport(auth, "")}
	var snapshot subscription.Snapshot
	var err error
	switch auth.Provider {
	case "codex":
		token, _ := auth.Metadata["access_token"].(string)
		accountID, _ := auth.Metadata["account_id"].(string)
		snapshot, err = subscription.FetchCodexSnapshot(c.Request.Context(), client, token, accountID)
	case "antigravity":
		token, _ := auth.Metadata["access_token"].(string)
		project, _ := auth.Metadata["project_id"].(string)
		snapshot, err = subscription.FetchAntigravitySnapshot(c.Request.Context(), client, token, project)
	case "cursor":
		token, _ := auth.Metadata["access_token"].(string)
		snapshot, err = subscription.FetchCursorSnapshot(c.Request.Context(), client, token)
	default:
		c.JSON(http.StatusNotImplemented, gin.H{"error": "subscription observer unavailable for provider"})
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "subscription quota unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"schema": "cliproxy.subscription.v1", "snapshot": snapshot})
}
