package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cim-backend/internal/simulate/report"
)

// fakeFirebase stands in for the two Firebase REST endpoints so tests never hit
// the network. It always returns a token derived from a counter so refreshes
// produce distinct values.
func fakeFirebase(t *testing.T, signIns, refreshes *int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/signin", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(signIns, 1)
		_ = json.NewEncoder(w).Encode(signInResponse{
			IDToken:      "id-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    "3600",
		})
	})
	mux.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(refreshes, 1)
		_ = json.NewEncoder(w).Encode(refreshResponse{
			IDToken:      "id-token-refreshed",
			RefreshToken: "refresh-token",
			ExpiresIn:    "3600",
		})
	})
	return httptest.NewServer(mux)
}

// newTestClient builds a Client whose token provider points at fb (the fake
// Firebase server) and whose API base is api.
func newTestClient(api, fb string, rep *report.Report) *Client {
	tp := newTokenProvider("test-key", "e@x.com", "pw", &http.Client{Timeout: 5 * time.Second})
	tp.signInURL = fb + "/signin"
	tp.refreshURL = fb + "/refresh"
	return &Client{
		baseURL: api,
		http:    &http.Client{Timeout: 5 * time.Second},
		tokens:  tp,
		report:  rep,
	}
}

// TestDoRetriesOn401WithFullBody verifies the 401-retry path re-sends the
// complete request body (regression guard for an empty retried body).
func TestDoRetriesOn401WithFullBody(t *testing.T) {
	var signIns, refreshes int32
	fb := fakeFirebase(t, &signIns, &refreshes)
	defer fb.Close()

	const wantBody = `{"name":"SIM Supplier 1"}`
	var bodies []string
	var attempt int32

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		// First attempt: 401 (forces token invalidation + retry). Second: 201.
		if atomic.AddInt32(&attempt, 1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":7,"name":"SIM Supplier 1"}`))
	}))
	defer api.Close()

	c := newTestClient(api.URL, fb.URL, report.New())

	var out struct {
		ID uint `json:"id"`
	}
	body := map[string]string{"name": "SIM Supplier 1"}
	if err := c.Do(context.Background(), "POST", "/suppliers", body, &out, "POST /suppliers"); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.ID != 7 {
		t.Errorf("id = %d, want 7", out.ID)
	}
	if len(bodies) != 2 {
		t.Fatalf("server saw %d requests, want 2 (initial + retry)", len(bodies))
	}
	for i, got := range bodies {
		if got != wantBody {
			t.Errorf("request %d body = %q, want %q (retry must resend full body)", i, got, wantBody)
		}
	}
	// The 401 invalidated the ID token, so the retry re-minted one. With a
	// cached refresh token that goes through the refresh endpoint; either way a
	// re-mint must have occurred beyond the initial sign-in.
	if signIns+refreshes < 2 {
		t.Errorf("token mints = %d sign-ins + %d refreshes, want >=2 total (initial + post-401 re-mint)", signIns, refreshes)
	}
	if refreshes < 1 {
		t.Errorf("refreshes = %d, want >=1 (post-401 re-mint via cached refresh token)", refreshes)
	}
}

// TestDoSecond401ReturnsError verifies a persistent 401 isn't retried forever.
func TestDoSecond401ReturnsError(t *testing.T) {
	var signIns, refreshes int32
	fb := fakeFirebase(t, &signIns, &refreshes)
	defer fb.Close()

	var calls int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	defer api.Close()

	c := newTestClient(api.URL, fb.URL, report.New())
	err := c.Do(context.Background(), "GET", "/x", nil, nil, "GET /x")
	if err == nil {
		t.Fatal("expected error on persistent 401")
	}
	if StatusOf(err) != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", StatusOf(err))
	}
	if calls != 2 {
		t.Errorf("server calls = %d, want 2 (bounded retry)", calls)
	}
}

// TestStatusOf checks the helper used by the idempotent ref-data path.
func TestStatusOf(t *testing.T) {
	if got := StatusOf(&APIError{Status: 409}); got != 409 {
		t.Errorf("StatusOf(APIError 409) = %d, want 409", got)
	}
	if got := StatusOf(context.Canceled); got != 0 {
		t.Errorf("StatusOf(non-APIError) = %d, want 0", got)
	}
	if got := StatusOf(nil); got != 0 {
		t.Errorf("StatusOf(nil) = %d, want 0", got)
	}
}

// TestTokenProviderConcurrent exercises the mutex under -race: many goroutines
// fetch a token and invalidate concurrently.
func TestTokenProviderConcurrent(t *testing.T) {
	var signIns, refreshes int32
	fb := fakeFirebase(t, &signIns, &refreshes)
	defer fb.Close()

	tp := newTokenProvider("k", "e@x.com", "pw", &http.Client{Timeout: 5 * time.Second})
	tp.signInURL = fb.URL + "/signin"
	tp.refreshURL = fb.URL + "/refresh"

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := tp.Token(context.Background()); err != nil {
				t.Errorf("Token: %v", err)
			}
			if i%8 == 0 {
				tp.Invalidate()
			}
		}(i)
	}
	wg.Wait()

	tok, err := tp.Token(context.Background())
	if err != nil {
		t.Fatalf("final Token: %v", err)
	}
	if tok == "" {
		t.Error("token is empty after concurrent access")
	}
}
