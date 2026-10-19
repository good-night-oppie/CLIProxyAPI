package helps

import (
	"net/http"
	"testing"
)

type pure100TestTransport struct{ hits int }

func (t *pure100TestTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.hits++
	return &http.Response{StatusCode: http.StatusOK}, nil
}
func TestPure100FingerprintTransportDoesNotChangeOtherProviders(t *testing.T) {
	for _, target := range []struct {
		url    string
		chrome bool
	}{
		{"https://pure100.org/v1/messages", true},
		{"https://www.pure100.org/v1/messages", true},
		{"https://api.anthropic.com/v1/messages", false},
		{"https://example.com/v1/messages", false},
		{"http://pure100.org/v1/messages", false},
	} {
		t.Run(target.url, func(t *testing.T) {
			chrome, standard := &pure100TestTransport{}, &pure100TestTransport{}
			rt := &pure100FingerprintRoundTripper{chrome: chrome, fallback: standard}
			req, err := http.NewRequest(http.MethodPost, target.url, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := rt.RoundTrip(req); err != nil {
				t.Fatal(err)
			}
			if (chrome.hits == 1) != target.chrome || chrome.hits+standard.hits != 1 {
				t.Fatal("wrong transport selected")
			}
		})
	}
}
