package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// tokenProvider mints and refreshes Firebase ID tokens. The sign-in logic is
// lifted from cmd/auth/main.go (which is package main and not importable); the
// cobra/clipboard bits are dropped and refresh-on-expiry is added so long runs
// survive the ~1h token lifetime.
// Default Firebase REST endpoints. Overridable on the tokenProvider for tests.
const (
	defaultSignInURL  = "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword"
	defaultRefreshURL = "https://securetoken.googleapis.com/v1/token"
)

type tokenProvider struct {
	apiKey   string
	email    string
	password string
	http     *http.Client

	// Firebase endpoints (defaults set in newTokenProvider; redirected in tests).
	signInURL  string
	refreshURL string

	mu           sync.Mutex
	idToken      string
	refreshToken string
	expiresAt    time.Time
}

func newTokenProvider(apiKey, email, password string, httpClient *http.Client) *tokenProvider {
	return &tokenProvider{
		apiKey:     apiKey,
		email:      email,
		password:   password,
		http:       httpClient,
		signInURL:  defaultSignInURL,
		refreshURL: defaultRefreshURL,
	}
}

// signInResponse mirrors the Firebase identitytoolkit signInWithPassword reply.
type signInResponse struct {
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    string `json:"expiresIn"`
}

// refreshResponse mirrors the Firebase securetoken refresh reply.
type refreshResponse struct {
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    string `json:"expires_in"`
}

// Token returns a valid ID token, signing in or refreshing as needed. It is
// safe for concurrent use.
func (t *tokenProvider) Token(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Refresh a minute early to avoid races against the server clock.
	if t.idToken != "" && time.Now().Before(t.expiresAt.Add(-time.Minute)) {
		return t.idToken, nil
	}
	if t.refreshToken != "" {
		if err := t.refresh(ctx); err == nil {
			return t.idToken, nil
		}
		// Fall through to a fresh sign-in if refresh fails.
	}
	if err := t.signIn(ctx); err != nil {
		return "", err
	}
	return t.idToken, nil
}

// Invalidate forces the next Token call to mint a new token. Called on a 401 so
// the caller can retry once with a fresh token.
func (t *tokenProvider) Invalidate() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.idToken = ""
	t.expiresAt = time.Time{}
}

func (t *tokenProvider) signIn(ctx context.Context) error {
	body, _ := json.Marshal(map[string]any{
		"email":             t.email,
		"password":          t.password,
		"returnSecureToken": true,
	})
	endpoint := t.signInURL + "?key=" + url.QueryEscape(t.apiKey)
	data, err := t.post(ctx, endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("firebase sign-in: %w", err)
	}
	var res signInResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return fmt.Errorf("firebase sign-in: decode: %w", err)
	}
	t.store(res.IDToken, res.RefreshToken, res.ExpiresIn)
	return nil
}

func (t *tokenProvider) refresh(ctx context.Context) error {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {t.refreshToken},
	}
	endpoint := t.refreshURL + "?key=" + url.QueryEscape(t.apiKey)
	data, err := t.post(ctx, endpoint, "application/x-www-form-urlencoded", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return fmt.Errorf("firebase refresh: %w", err)
	}
	var res refreshResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return fmt.Errorf("firebase refresh: decode: %w", err)
	}
	t.store(res.IDToken, res.RefreshToken, res.ExpiresIn)
	return nil
}

func (t *tokenProvider) store(idToken, refreshToken, expiresIn string) {
	t.idToken = idToken
	if refreshToken != "" {
		t.refreshToken = refreshToken
	}
	secs, _ := strconv.Atoi(expiresIn)
	if secs <= 0 {
		secs = 3600
	}
	t.expiresAt = time.Now().Add(time.Duration(secs) * time.Second)
}

func (t *tokenProvider) post(ctx context.Context, endpoint, contentType string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}
