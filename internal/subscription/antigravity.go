package subscription

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// AntigravityUsage keeps the subscription's model-family buckets independent.
// The provider calls the Claude/GPT family "3p"; retain that identifier rather
// than implying its quota is interchangeable with Gemini or account credits.
type AntigravityUsage struct {
	Groups []struct {
		Buckets []struct {
			Disabled          bool     `json:"disabled"`
			BucketID          string   `json:"bucketId"`
			RemainingFraction *float64 `json:"remainingFraction"`
			ResetTime         string   `json:"resetTime"`
		} `json:"buckets"`
	} `json:"groups"`
}

func (u AntigravityUsage) Observe(now time.Time) (Snapshot, error) {
	s := Snapshot{Provider: "antigravity", ObservedAt: now, Windows: []Window{}}
	seen := map[string]bool{}
	for _, group := range u.Groups {
		for _, bucket := range group.Buckets {
			var pool, period string
			switch bucket.BucketID {
			case "gemini-5h":
				pool, period = "gemini", "5h"
			case "gemini-weekly":
				pool, period = "gemini", "weekly"
			case "3p-5h":
				pool, period = "3p", "5h"
			case "3p-weekly":
				pool, period = "3p", "weekly"
			default:
				continue
			}
			if seen[bucket.BucketID] || !percentage(bucket.RemainingFraction) || *bucket.RemainingFraction > 1 {
				return Snapshot{}, unknown()
			}
			seen[bucket.BucketID] = true
			reset, err := time.Parse(time.RFC3339Nano, bucket.ResetTime)
			if err != nil {
				return Snapshot{}, unknown()
			}
			s.Windows = append(s.Windows, Window{Disabled: bucket.Disabled, Pool: pool, Period: period, UsedPercent: (1 - *bucket.RemainingFraction) * 100, ResetAt: &reset})
		}
	}
	if len(s.Windows) == 0 {
		return Snapshot{}, unknown()
	}
	return s, nil
}

// Admit requires both windows of the selected family. A missing or elapsed
// window needs another observation; it must never be interpreted as a refill.
func (u AntigravityUsage) Admit(pool string, now time.Time) (Snapshot, error) {
	s, err := u.Observe(now)
	if err != nil {
		return s, err
	}
	if pool != "gemini" && pool != "3p" {
		return s, unknown()
	}
	found := 0
	exhaustedPool := false
	disabledWindow := false
	for _, w := range s.Windows {
		if w.Pool != pool {
			continue
		}
		found++
		if w.ResetAt == nil || !w.ResetAt.After(now) {
			return s, unknown()
		}
		exhaustedPool = exhaustedPool || w.UsedPercent >= 100
		disabledWindow = disabledWindow || w.Disabled
	}
	if found != 2 {
		return s, unknown()
	}
	if exhaustedPool {
		return s, exhausted()
	}
	if disabledWindow {
		return s, unknown()
	}
	return s, nil
}

func FetchAntigravitySnapshot(ctx context.Context, client *http.Client, token, project string) (Snapshot, error) {
	if project == "" {
		return Snapshot{}, unknown()
	}
	body, _ := json.Marshal(map[string]string{"project": project})
	var usage AntigravityUsage
	if err := readJSONBody(ctx, client, http.MethodPost,
		"https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary", token, body, &usage,
		http.Header{"User-Agent": {"antigravity/1.21.9"}}); err != nil {
		return Snapshot{}, err
	}
	return usage.Observe(time.Now())
}
