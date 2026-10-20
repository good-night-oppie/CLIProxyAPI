package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func cursorFixture(t *testing.T) CursorUsage {
	t.Helper()
	var u CursorUsage
	err := json.Unmarshal([]byte(`{"enabled":true,"billingCycleStart":"1000","billingCycleEnd":"100000","autoBucketModels":["composer-2"],"planUsage":{"remaining":1400,"limit":2000,"autoPercentUsed":10,"apiPercentUsed":20,"totalPercentUsed":15}}`), &u)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestCursorAdmissionPoolsAndFailClosed(t *testing.T) {
	now := time.UnixMilli(5000)
	for _, tc := range []struct {
		name   string
		model  string
		change func(*CursorUsage)
		code   int
		pool   string
	}{
		{"auto", "composer-2", func(*CursorUsage) {}, 0, "auto"},
		{"api", "claude-sonnet", func(*CursorUsage) {}, 0, "api"},
		{"missing pool", "claude-sonnet", func(u *CursorUsage) { u.PlanUsage.APIPercentUsed = nil }, 503, ""},
		{"missing catalog", "claude-sonnet", func(u *CursorUsage) { u.AutoBucketModels = nil }, 503, ""},
		{"inactive", "claude-sonnet", func(u *CursorUsage) { u.Enabled = false }, 503, ""},
		{"expired period", "claude-sonnet", func(u *CursorUsage) { u.BillingCycleEnd = "4000" }, 503, ""},
		{"empty included", "claude-sonnet", func(u *CursorUsage) { *u.PlanUsage.Remaining = 0 }, 429, ""},
		{"auto exhausted", "composer-2", func(u *CursorUsage) { *u.PlanUsage.AutoPercentUsed = 100 }, 429, ""},
		{"other pool exhausted", "claude-sonnet", func(u *CursorUsage) { *u.PlanUsage.AutoPercentUsed = 100 }, 0, "api"},
		{"total exhausted", "claude-sonnet", func(u *CursorUsage) { *u.PlanUsage.TotalPercentUsed = 100 }, 429, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := cursorFixture(t)
			tc.change(&u)
			s, err := u.Admit(tc.model, now)
			if tc.code == 0 {
				if err != nil {
					t.Fatal(err)
				}
				if s.Windows[0].Pool != tc.pool || s.Windows[0].Period != "monthly" || s.Windows[0].ResetAt.UnixMilli() != 100000 {
					t.Fatalf("wrong windows: %+v", s.Windows)
				}
			} else {
				var admission *AdmissionError
				if !errors.As(err, &admission) || admission.Code != tc.code {
					t.Fatalf("error = %v, want %d", err, tc.code)
				}
			}
		})
	}
}

func TestReadJSONRejectsRedirectAndProviderErrors(t *testing.T) {
	targetCalled := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalled = true }))
	defer target.Close()
	for _, code := range []int{302, 401, 429, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(code)
				_, _ = w.Write([]byte("secret provider response"))
			}))
			defer server.Close()
			var result any
			err := ReadJSON(context.Background(), server.Client(), "POST", server.URL, "test-token", &result)
			var admission *AdmissionError
			if !errors.As(err, &admission) || admission.Code != 503 {
				t.Fatalf("error=%v", err)
			}
			if targetCalled {
				t.Fatal("redirect received credentials")
			}
		})
	}
}

func TestCursorObservationIncludesBothPoolsEvenWhenOneIsExhausted(t *testing.T) {
	u := cursorFixture(t)
	*u.PlanUsage.APIPercentUsed = 100
	s, err := u.Observe(time.UnixMilli(5000))
	if err != nil || len(s.Windows) != 3 {
		t.Fatalf("snapshot=%+v err=%v", s, err)
	}
	if s.Windows[0].Pool != "auto" || s.Windows[1].UsedPercent != 100 || s.IncludedRemaining == nil {
		t.Fatalf("incomplete snapshot: %+v", s)
	}
	if len(s.AutoBucketModels) != 1 {
		t.Fatal("model pool catalog missing")
	}
	u.PlanUsage.AutoPercentUsed = nil
	if _, err = u.Observe(time.UnixMilli(5000)); err == nil {
		t.Fatal("incomplete pool must not be exposed as known")
	}
}
