package subscription

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCodexPreservesOnlyRealWindows(t *testing.T) {
	var usage CodexUsage
	err := json.Unmarshal([]byte(`{"rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":37,"limit_window_seconds":604800,"reset_at":1800000000},"secondary_window":null}}`), &usage)
	if err != nil {
		t.Fatal(err)
	}
	s, err := usage.Observe(time.Unix(1790000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Windows) != 1 || s.Windows[0].Period != "weekly" || s.Windows[0].WindowSeconds != 604800 || s.Windows[0].UsedPercent != 37 || !*s.AdmissionAllowed {
		t.Fatalf("weekly-only snapshot: %+v", s)
	}
	p := float64(15)
	usage.RateLimit.Secondary = &CodexWindow{UsedPercent: &p, WindowSeconds: 7200, ResetAt: 1800000001}
	s, err = usage.Observe(time.Now())
	if err != nil || s.Windows[1].Period != "7200s" {
		t.Fatal("non-five-hour duration relabelled")
	}
	blocked := true
	usage.RateLimit.LimitReached = &blocked
	s, err = usage.Observe(time.Now())
	if err != nil || *s.AdmissionAllowed {
		t.Fatal("provider admission flag lost")
	}
	usage.RateLimit.Secondary.UsedPercent = nil
	if _, err = usage.Observe(time.Now()); err == nil {
		t.Fatal("missing utilization treated as zero")
	}
}
