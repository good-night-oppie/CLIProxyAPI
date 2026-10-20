package cursor

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type refreshTransport func(*http.Request) (*http.Response, error)

func (f refreshTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRefreshTokenResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body, wantRefresh string
		status                  int
		wantError               bool
	}{
		{"rotation", `{"accessToken":"new-access","refreshToken":"new-refresh"}`, "new-refresh", 200, false},
		{"retained refresh", `{"accessToken":"new-access"}`, "old-refresh", 200, false},
		{"expired credential", `{"error":"private-secret"}`, "", 401, true},
		{"redirect", "private-secret", "", 302, true},
		{"missing access", `{"refreshToken":"private-secret"}`, "", 200, true},
		{"invalid JSON", "private-secret", "", 200, true},
		{"oversize", strings.Repeat("x", (1<<20)+1), "", 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: refreshTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost || r.URL.String() != CursorRefreshURL ||
					r.Header.Get("Authorization") != "Bearer old-refresh" {
					t.Fatal("incorrect credential exchange")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			got, err := refreshTokenWithClient(context.Background(), client, "old-refresh")
			if tc.wantError {
				if err == nil || strings.Contains(err.Error(), "private-secret") {
					t.Fatalf("expected sanitized failure, got %v", err)
				}
				return
			}
			if err != nil || got.AccessToken != "new-access" || got.RefreshToken != tc.wantRefresh {
				t.Fatalf("refresh result mismatch: %v", err)
			}
		})
	}
}

func TestRefreshTokenMissingCredentialDoesNotSend(t *testing.T) {
	client := &http.Client{Transport: refreshTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("empty credential reached transport")
		return nil, nil
	})}
	if _, err := refreshTokenWithClient(context.Background(), client, ""); err == nil {
		t.Fatal("missing credential accepted")
	}
}

func TestPollFailureDoesNotExposeVerifierOrResponse(t *testing.T) {
	for _, transportFailure := range []bool{false, true} {
		calls := 0
		client := &http.Client{Transport: refreshTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if transportFailure {
				return nil, errors.New("connection failed: " + r.URL.String())
			}
			return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("private-response-secret")), Header: make(http.Header)}, nil
		})}
		_, err := pollForAuth(context.Background(), "test-id", "private-verifier-secret", client, func(context.Context, time.Duration) error { return nil })
		if err == nil || strings.Contains(err.Error(), "private-") {
			t.Fatalf("expected sanitized poll error: %v", err)
		}
		if (transportFailure && calls != maxConsecutiveErrors) || (!transportFailure && calls != 1) {
			t.Fatalf("unexpected polling attempts: %d", calls)
		}
	}
}

func TestPollPendingThenAuthorized(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: refreshTransport(func(*http.Request) (*http.Response, error) {
		calls++
		status, body := 404, "{}"
		if calls == 2 {
			status, body = 200, `{"accessToken":"authorized"}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	got, err := pollForAuth(context.Background(), "test-id", "test-verifier", client, func(context.Context, time.Duration) error { return nil })
	if err != nil || got.AccessToken != "authorized" || calls != 2 {
		t.Fatalf("pending authorization failed: %v", err)
	}
}
