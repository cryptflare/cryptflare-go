package cryptflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

// requestDescriptor captures everything a resource needs to express one
// HTTP call. Resources never touch `*http.Request` directly.
type requestDescriptor struct {
	Method string
	Path   string
	Query  url.Values
	Body   any
}

// do executes a request descriptor against the API and unmarshals the
// JSON response into `out` (which may be nil for void endpoints). It
// owns retry, idempotency, hooks, and error mapping.
func (c *Client) do(ctx context.Context, desc requestDescriptor, out any, opts ...RequestOption) error {
	cfg := requestConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	if isMutating(desc.Method) && cfg.IdempotencyKey == "" && c.opts.Retry.MaxAttempts > 1 {
		cfg.IdempotencyKey = uuid.NewString()
	}

	bodyBytes, err := encodeBody(desc.Body)
	if err != nil {
		return &ConfigurationError{Message: "could not marshal body", Cause: err}
	}

	full := strings.TrimRight(c.opts.BaseURL, "/") + desc.Path
	if len(desc.Query) > 0 {
		full += "?" + desc.Query.Encode()
	}

	shouldRetry := c.opts.Retry.ShouldRetry
	if shouldRetry == nil {
		shouldRetry = shouldRetryDefault
	}

	var lastErr error
	for attempt := 1; attempt <= c.opts.Retry.MaxAttempts; attempt++ {
		req, reqErr := c.buildRequest(ctx, desc.Method, full, bodyBytes, &cfg)
		if reqErr != nil {
			return reqErr
		}

		if c.opts.Hooks.OnRequest != nil {
			safeHook(func() { c.opts.Hooks.OnRequest(req) })
		}

		started := time.Now()
		res, sendErr := c.httpClient.Do(req)
		duration := time.Since(started).Milliseconds()
		if c.opts.Hooks.OnResponse != nil {
			safeHook(func() { c.opts.Hooks.OnResponse(req, res, duration) })
		}

		if sendErr != nil {
			lastErr = mapTransportError(sendErr)
			if attempt >= c.opts.Retry.MaxAttempts || !shouldRetry(nil, attempt) {
				return lastErr
			}
			c.sleep(ctx, attempt, 0)
			continue
		}

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			defer res.Body.Close()
			if out == nil {
				_, _ = io.Copy(io.Discard, res.Body)
				return nil
			}
			if err := json.NewDecoder(res.Body).Decode(out); err != nil && err != io.EOF {
				return &ConnectionError{Message: "could not decode response", Cause: err}
			}
			return nil
		}

		apiErr := mapAPIError(res)
		_ = res.Body.Close()
		lastErr = apiErr
		if attempt >= c.opts.Retry.MaxAttempts || !shouldRetry(apiErr, attempt) {
			return apiErr
		}
		if c.opts.Hooks.OnRetry != nil {
			delay := computeBackoff(attempt, c.opts.Retry, apiErr.RetryAfter)
			safeHook(func() { c.opts.Hooks.OnRetry(attempt, delay, apiErr) })
		}
		c.sleep(ctx, attempt, apiErr.RetryAfter)
	}
	return lastErr
}

func (c *Client) buildRequest(ctx context.Context, method, fullURL string, body []byte, cfg *requestConfig) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return nil, &ConfigurationError{Message: "could not build request", Cause: err}
	}

	token, tokenErr := c.resolveToken(ctx)
	if tokenErr != nil {
		return nil, tokenErr
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", buildUserAgent(c.opts.UserAgentSuffix))
	if cfg.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", cfg.IdempotencyKey)
	}
	for k, vs := range cfg.Headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	return req, nil
}

func (c *Client) sleep(ctx context.Context, attempt int, retryAfter time.Duration) {
	delay := computeBackoff(attempt, c.opts.Retry, retryAfter)
	if delay <= 0 {
		return
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (c *Client) resolveToken(ctx context.Context) (string, error) {
	if c.opts.APIKey != "" {
		return c.opts.APIKey, nil
	}
	if c.opts.ServiceToken != "" {
		return c.opts.ServiceToken, nil
	}
	if c.opts.TokenSource != nil {
		token, err := c.opts.TokenSource(ctx)
		if err != nil {
			return "", &ConfigurationError{Message: "token source failed", Cause: err}
		}
		return token, nil
	}
	return "", &ConfigurationError{Message: "no credential configured"}
}

func encodeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return json.Marshal(body)
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func mapTransportError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &TimeoutError{Message: err.Error(), Cause: err}
	}
	return &ConnectionError{Message: err.Error(), Cause: err}
}

func mapAPIError(res *http.Response) *APIError {
	body, _ := io.ReadAll(res.Body)
	out := &APIError{
		Status:    res.StatusCode,
		Headers:   res.Header,
		RequestID: res.Header.Get("X-Request-Id"),
		sentinel:  sentinelForStatus(res.StatusCode),
	}
	if len(body) > 0 {
		var parsed struct {
			Error     string        `json:"error"`
			Message   string        `json:"message"`
			Details   []ErrorDetail `json:"details"`
			RequestID string        `json:"requestId"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			out.Code = parsed.Error
			if parsed.Message != "" {
				out.Message = parsed.Message
			}
			out.Details = parsed.Details
			if parsed.RequestID != "" {
				out.RequestID = parsed.RequestID
			}
		}
	}
	if out.Message == "" {
		out.Message = fmt.Sprintf("HTTP %d", res.StatusCode)
	}
	out.RetryAfter = parseRetryAfter(res.Header)
	return out
}

func buildUserAgent(suffix string) string {
	base := fmt.Sprintf("cryptflare-sdk-go/%s (%s; %s)", Version, runtime.Version(), runtime.GOOS)
	if suffix == "" {
		return base
	}
	return base + " " + suffix
}

func safeHook(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(io.Discard, "cryptflare: hook panicked: %v\n", r)
		}
	}()
	fn()
}
