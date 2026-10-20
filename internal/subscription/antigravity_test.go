package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAntigravityIndependentFamilies(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	const fixture = `{"groups":[{"buckets":[{"bucketId":"gemini-weekly","remainingFraction":0.8,"resetTime":"2026-10-25T00:00:00Z"},{"bucketId":"gemini-5h","remainingFraction":0.9,"resetTime":"2026-10-20T05:00:00Z"}]},{"buckets":[{"bucketId":"3p-weekly","remainingFraction":0,"resetTime":"2026-10-26T00:00:00Z"},{"bucketId":"3p-5h","remainingFraction":1,"resetTime":"2026-10-20T04:00:00Z"}]}]}`
	var u AntigravityUsage
	if err := json.Unmarshal([]byte(fixture), &u); err != nil {
		t.Fatal(err)
	}
	s, err := u.Admit("gemini", now)
	if err != nil || len(s.Windows) != 4 {
		t.Fatalf("Gemini admission: %+v %v", s, err)
	}
	if s.Windows[0].ResetAt.Equal(*s.Windows[2].ResetAt) {
		t.Fatal("family reset times collapsed")
	}
	_, err = u.Admit("3p", now)
	var admission *AdmissionError
	if !errors.As(err, &admission) || admission.Code != 429 {
		t.Fatalf("Claude exhausted: %v", err)
	}
	// Observation must still show the exhausted family alongside available Gemini.
	if _, err := u.Observe(now); err != nil {
		t.Fatal(err)
	}
	u.Groups = u.Groups[1:]
	s, err = u.Admit("gemini", now)
	if !errors.As(err, &admission) || admission.Code != 503 || len(s.Windows) != 2 {
		t.Fatalf("missing Gemini: %+v %v", s, err)
	}
}

func TestAntigravityRejectsAmbiguousAndInvalidBuckets(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ name, buckets string }{
		{"missing fraction", `{"bucketId":"gemini-5h","resetTime":"2026-10-21T00:00:00Z"}`},
		{"out of range", `{"bucketId":"gemini-5h","remainingFraction":1.1,"resetTime":"2026-10-21T00:00:00Z"}`},
		{"invalid reset", `{"bucketId":"gemini-5h","remainingFraction":1,"resetTime":"bad"}`},
		{"duplicate", `{"bucketId":"gemini-5h","remainingFraction":1,"resetTime":"2026-10-21T00:00:00Z"},{"bucketId":"gemini-5h","remainingFraction":0,"resetTime":"2026-10-21T00:00:00Z"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var u AntigravityUsage
			if err := json.Unmarshal([]byte(`{"groups":[{"buckets":[`+tc.buckets+`]}]}`), &u); err != nil {
				t.Fatal(err)
			}
			if _, err := u.Observe(now); err == nil {
				t.Fatal("invalid observation accepted")
			}
		})
	}
	var partial AntigravityUsage
	_ = json.Unmarshal([]byte(`{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":1,"resetTime":"2026-10-21T00:00:00Z"}]}]}`), &partial)
	if _, err := partial.Admit("gemini", now); err == nil {
		t.Fatal("missing weekly accepted")
	}
	if _, err := partial.Admit("3p", now); err == nil {
		t.Fatal("missing family accepted")
	}
	if _, err := partial.Admit("gemini", now.Add(48*time.Hour)); err == nil {
		t.Fatal("elapsed reset treated as refill")
	}
}

type antigravityQuotaTransport func(*http.Request) (*http.Response, error)

func (f antigravityQuotaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAntigravityObserverUsesCredentialProjectAndPreservesDisabledWindow(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: antigravityQuotaTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		var body map[string]string
		if json.NewDecoder(req.Body).Decode(&body) != nil || body["project"] != "selected-project" {
			t.Fatal("credential project omitted")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"groups":[{"buckets":[{"bucketId":"3p-5h","disabled":true,"remainingFraction":1,"resetTime":"2027-01-01T00:00:00Z"},{"bucketId":"3p-weekly","remainingFraction":0,"resetTime":"2027-01-01T00:00:00Z"}]}]}`)), Header: make(http.Header)}, nil
	})}
	snapshot, err := FetchAntigravitySnapshot(context.Background(), client, "test-token", "selected-project")
	if err != nil || !snapshot.Windows[0].Disabled {
		t.Fatalf("disabled window lost: %+v %v", snapshot, err)
	}
	if _, err = FetchAntigravitySnapshot(context.Background(), client, "test-token", ""); err == nil || calls != 1 {
		t.Fatal("projectless observation sent")
	}
}
