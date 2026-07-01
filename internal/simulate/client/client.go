// Package client is a thin typed HTTP client for the CIM API. It attaches a
// Firebase Bearer token, retries once on a 401 with a fresh token, and reports
// per-call latency/outcome centrally.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"cim-backend/internal/simulate/report"
)

// APIPrefix is the versioned API path prefix.
const APIPrefix = "/api/v1"

// Client talks to the CIM API.
type Client struct {
	baseURL string
	http    *http.Client
	tokens  *tokenProvider
	report  *report.Report
}

// Options configure a Client.
type Options struct {
	BaseURL        string
	Timeout        time.Duration
	Email          string
	Password       string
	FirebaseAPIKey string
	Report         *report.Report
}

// New constructs a Client. The token is minted lazily on the first request.
func New(opts Options) *Client {
	httpClient := &http.Client{Timeout: opts.Timeout}
	return &Client{
		baseURL: opts.BaseURL,
		http:    httpClient,
		tokens:  newTokenProvider(opts.FirebaseAPIKey, opts.Email, opts.Password, &http.Client{Timeout: opts.Timeout}),
		report:  opts.Report,
	}
}

// APIError is returned for non-2xx responses.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("status %d: %s", e.Status, e.Body)
}

// StatusOf returns the HTTP status carried by err if it is an *APIError, else 0.
func StatusOf(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status
	}
	return 0
}

// ExpectedFunc classifies a non-2xx response as an expected (tolerable) outcome
// from its status and body. Returning true records the call as a non-failure and
// suppresses the error.
type ExpectedFunc func(status int, respBody []byte) bool

// Do issues an authenticated request to path (under /api/v1) with an optional
// JSON body, decoding a 2xx response into out (if non-nil). label is the
// endpoint name recorded in the report. Any non-2xx is a failure.
func (c *Client) Do(ctx context.Context, method, path string, body, out any, label string) error {
	_, err := c.DoClassified(ctx, method, path, body, out, label, nil)
	return err
}

// DoExpectingStatus is Do with an expected non-2xx outcome keyed on the status
// code alone: a non-2xx whose status is in expectedStatuses is recorded as a
// non-failure and returned without error; any other non-2xx behaves like Do.
func (c *Client) DoExpectingStatus(ctx context.Context, method, path string, body, out any, label string, expectedStatuses ...int) (int, error) {
	return c.DoClassified(ctx, method, path, body, out, label, func(status int, _ []byte) bool {
		return containsStatus(expectedStatuses, status)
	})
}

// DoClassified is the shared core: it issues the request, records exactly one
// call in the report, and decodes a 2xx body into out (if non-nil). For a
// non-2xx, if isExpected (when non-nil) returns true the call is recorded as a
// non-failure and (status, nil) is returned; otherwise it is a failure returned
// as an *APIError.
func (c *Client) DoClassified(ctx context.Context, method, path string, body, out any, label string, isExpected ExpectedFunc) (int, error) {
	start := time.Now()
	status, respBody, err := c.do(ctx, method, path, body)

	// Expected non-2xx: record the call but not as a failure.
	if err != nil && isExpected != nil && isExpected(status, respBody) {
		c.report.RecordCall(label, status, time.Since(start), nil)
		return status, nil
	}

	c.report.RecordCall(label, status, time.Since(start), err)
	if err != nil {
		return status, err
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return status, fmt.Errorf("%s: decode response: %w", label, err)
		}
	}
	return status, nil
}

// containsStatus reports whether status is in the list.
func containsStatus(statuses []int, status int) bool {
	for _, s := range statuses {
		if s == status {
			return true
		}
	}
	return false
}

// do performs the request with one retry on 401 (after invalidating the token).
func (c *Client) do(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return 0, nil, fmt.Errorf("marshal request: %w", err)
		}
	}

	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.tokens.Token(ctx)
		if err != nil {
			return 0, nil, fmt.Errorf("auth: %w", err)
		}

		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+APIPrefix+path, reader)
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return 0, nil, err
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return resp.StatusCode, nil, readErr
		}

		// On a 401, invalidate and retry once with a new token.
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.tokens.Invalidate()
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return resp.StatusCode, data, &APIError{Status: resp.StatusCode, Body: string(data)}
		}
		return resp.StatusCode, data, nil
	}
	return 0, nil, fmt.Errorf("request to %s failed after retry", path)
}

// VerifyAuth mints a token eagerly so credential problems surface before any
// scenario runs.
func (c *Client) VerifyAuth(ctx context.Context) error {
	_, err := c.tokens.Token(ctx)
	return err
}
