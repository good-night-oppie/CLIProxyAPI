package subscription

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type CodexWindow struct {
	UsedPercent   *float64 `json:"used_percent"`
	WindowSeconds int64    `json:"limit_window_seconds"`
	ResetAt       int64    `json:"reset_at"`
}
type CodexUsage struct {
	RateLimit struct {
		Allowed      *bool        `json:"allowed"`
		LimitReached *bool        `json:"limit_reached"`
		Primary      *CodexWindow `json:"primary_window"`
		Secondary    *CodexWindow `json:"secondary_window"`
	} `json:"rate_limit"`
}

// Observe retains actual durations. A weekly-only account has one window;
// neither a synthetic 5h window nor a derived burn-rate budget is added.
func (u CodexUsage) Observe(now time.Time) (Snapshot, error) {
	s := Snapshot{Provider: "codex", ObservedAt: now, Windows: []Window{}}
	if u.RateLimit.Allowed == nil || u.RateLimit.LimitReached == nil {
		return Snapshot{}, unknown()
	}
	allowed := *u.RateLimit.Allowed && !*u.RateLimit.LimitReached
	s.AdmissionAllowed = &allowed
	for i, w := range []*CodexWindow{u.RateLimit.Primary, u.RateLimit.Secondary} {
		if w == nil {
			continue
		}
		if !percentage(w.UsedPercent) || w.WindowSeconds <= 0 || w.ResetAt <= 0 {
			return Snapshot{}, unknown()
		}
		period := fmt.Sprintf("%ds", w.WindowSeconds)
		switch w.WindowSeconds {
		case 18000:
			period = "5h"
		case 604800:
			period = "weekly"
		}
		reset := time.Unix(w.ResetAt, 0)
		pool := []string{"primary", "secondary"}[i]
		s.Windows = append(s.Windows, Window{Pool: pool, Period: period, WindowSeconds: w.WindowSeconds, UsedPercent: *w.UsedPercent, ResetAt: &reset})
	}
	if len(s.Windows) == 0 {
		return Snapshot{}, unknown()
	}
	return s, nil
}
func FetchCodexSnapshot(ctx context.Context, client *http.Client, token, accountID string) (Snapshot, error) {
	if accountID == "" {
		return Snapshot{}, unknown()
	}
	var usage CodexUsage
	if err := ReadJSON(ctx, client, http.MethodGet, "https://chatgpt.com/backend-api/wham/usage", token, &usage, http.Header{
		"Chatgpt-Account-Id": {accountID}, "Originator": {"Codex Desktop"}, "User-Agent": {"codex-quota-probe/2.0"},
	}); err != nil {
		return Snapshot{}, err
	}
	return usage.Observe(time.Now())
}
