// Package subscription reads provider accounting without changing spend settings.
package subscription

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

type Window struct {
	Disabled      bool       `json:"disabled,omitempty"`
	WindowSeconds int64      `json:"windowSeconds,omitempty"`
	Pool          string     `json:"pool"`
	Period        string     `json:"period"`
	UsedPercent   float64    `json:"usedPercent"`
	ResetAt       *time.Time `json:"resetAt"`
}

type Snapshot struct {
	AdmissionAllowed  *bool     `json:"admissionAllowed,omitempty"`
	Provider          string    `json:"provider"`
	ObservedAt        time.Time `json:"observedAt"`
	Windows           []Window  `json:"windows"`
	IncludedRemaining *float64  `json:"includedRemaining,omitempty"`
	AutoBucketModels  []string  `json:"autoBucketModels"`
}

// AdmissionError contains no upstream response body or credentials.
type AdmissionError struct {
	Code    int
	Message string
}

func (e *AdmissionError) Error() string   { return e.Message }
func (e *AdmissionError) StatusCode() int { return e.Code }

func unknown() error {
	return &AdmissionError{http.StatusServiceUnavailable, "subscription quota is unknown; request not admitted"}
}

func exhausted() error {
	return &AdmissionError{http.StatusTooManyRequests, "included subscription quota exhausted; request not admitted"}
}

func percentage(value *float64) bool {
	return value != nil && !math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= 0
}

// ReadJSON uses caller cancellation, bounds the response, and never returns raw
// provider errors (which may contain account or credential information).
func ReadJSON(ctx context.Context, client *http.Client, method, endpoint, token string, target any, headers ...http.Header) error {
	return readJSONBody(ctx, client, method, endpoint, token, []byte("{}"), target, headers...)
}

func readJSONBody(ctx context.Context, client *http.Client, method, endpoint, token string, body []byte, target any, headers ...http.Header) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil || token == "" {
		return unknown()
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	for _, h := range headers {
		for key, values := range h {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	// Do not forward credentials to redirected hosts.
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := safeClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return unknown()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return unknown()
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil || len(data) > 1<<20 || json.Unmarshal(data, target) != nil {
		return unknown()
	}
	return nil
}

type CursorUsage struct {
	Enabled           bool     `json:"enabled"`
	BillingCycleStart string   `json:"billingCycleStart"`
	BillingCycleEnd   string   `json:"billingCycleEnd"`
	AutoBucketModels  []string `json:"autoBucketModels"`
	PlanUsage         struct {
		Remaining        *float64 `json:"remaining"`
		Limit            *float64 `json:"limit"`
		AutoPercentUsed  *float64 `json:"autoPercentUsed"`
		APIPercentUsed   *float64 `json:"apiPercentUsed"`
		TotalPercentUsed *float64 `json:"totalPercentUsed"`
	} `json:"planUsage"`
}

// includedRemaining returns the legacy numeric planUsage.remaining when the
// provider still sends it. Cursor's agent-era dashboard dropped that field,
// so fall back to deriving it from the percentage meter (100 - totalPercentUsed).
// Neither meter present stays fail-closed (nil).
func (u CursorUsage) includedRemaining() *float64 {
	if u.PlanUsage.Remaining != nil {
		return u.PlanUsage.Remaining
	}
	if total := u.PlanUsage.TotalPercentUsed; percentage(total) {
		derived := 100 - *total
		return &derived
	}
	return nil
}

// Admit requires a current billing period and explicit accounting for the
// selected model pool. On-demand remaining balance never grants admission.
func (u CursorUsage) Admit(model string, now time.Time) (Snapshot, error) {
	snapshot := Snapshot{Provider: "cursor", ObservedAt: now}
	start, errStart := strconv.ParseInt(u.BillingCycleStart, 10, 64)
	end, errEnd := strconv.ParseInt(u.BillingCycleEnd, 10, 64)
	remaining := u.includedRemaining()
	if !u.Enabled || model == "" || errStart != nil || errEnd != nil ||
		now.UnixMilli() < start || now.UnixMilli() >= end || start >= end ||
		u.AutoBucketModels == nil || !percentage(remaining) ||
		!percentage(u.PlanUsage.Limit) || *u.PlanUsage.Limit <= 0 ||
		!percentage(u.PlanUsage.TotalPercentUsed) {
		return snapshot, unknown()
	}
	pool, used := "api", u.PlanUsage.APIPercentUsed
	for _, id := range u.AutoBucketModels {
		if id == model {
			pool, used = "auto", u.PlanUsage.AutoPercentUsed
			break
		}
	}
	if !percentage(used) {
		return snapshot, unknown()
	}
	reset := time.UnixMilli(end).UTC()
	snapshot.Windows = []Window{
		{Pool: pool, Period: "monthly", UsedPercent: *used, ResetAt: &reset},
		{Pool: "total", Period: "monthly", UsedPercent: *u.PlanUsage.TotalPercentUsed, ResetAt: &reset},
	}
	if *used >= 100 || *u.PlanUsage.TotalPercentUsed >= 100 || *remaining <= 0 {
		return snapshot, exhausted()
	}
	return snapshot, nil
}

func RequireCursor(ctx context.Context, client *http.Client, token, model string) error {
	var usage CursorUsage
	if err := ReadJSON(ctx, client, http.MethodPost,
		"https://api2.cursor.sh/aiserver.v1.DashboardService/GetCurrentPeriodUsage", token, &usage); err != nil {
		return err
	}
	_, err := usage.Admit(model, time.Now())
	if err != nil {
		return fmt.Errorf("cursor: %w", err)
	}
	return nil
}
