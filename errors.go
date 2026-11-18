package cryptflare

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Sentinel errors. Match against any returned `error` with `errors.Is`.
// They wrap the underlying `*APIError`, so `errors.As` still works to
// recover the full payload.
var (
	ErrUnauthenticated  = errors.New("cryptflare: unauthenticated")
	ErrPermissionDenied = errors.New("cryptflare: permission denied")
	ErrNotFound         = errors.New("cryptflare: not found")
	ErrConflict         = errors.New("cryptflare: conflict")
	ErrUnprocessable    = errors.New("cryptflare: unprocessable entity")
	ErrRateLimited      = errors.New("cryptflare: rate limited")
	ErrInternal         = errors.New("cryptflare: internal server error")
	ErrUnknownAPI       = errors.New("cryptflare: unexpected API response")
)

// ErrorDetail mirrors a single entry from the server's RFC 9457
// `details` array.
type ErrorDetail struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// APIError is the rich error returned by every non-2xx response. The
// SDK never returns the raw transport error - either an `*APIError`,
// `*ConnectionError`, `*TimeoutError`, or `*ConfigurationError`.
type APIError struct {
	Code       string
	Message    string
	Status     int
	RequestID  string
	RetryAfter time.Duration
	Headers    http.Header
	Details    []ErrorDetail
	sentinel   error
}

// Error makes APIError satisfy the `error` interface.
func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("cryptflare: %s (status %d, request %s): %s", e.Code, e.Status, e.RequestID, e.Message)
	}
	return fmt.Sprintf("cryptflare: %s (status %d): %s", e.Code, e.Status, e.Message)
}

// Unwrap returns the matching sentinel for the status. `errors.Is` uses
// this to satisfy `errors.Is(err, cryptflare.ErrRateLimited)` style
// checks without callers having to type-assert.
func (e *APIError) Unwrap() error {
	return e.sentinel
}

// Is satisfies `errors.Is` with explicit pointer-equality fallthrough -
// useful for callers comparing against the typed value `*APIError(nil)`.
func (e *APIError) Is(target error) bool {
	_, ok := target.(*APIError)
	return ok
}

// ConfigurationError is returned synchronously from constructors / option
// validators. Never travels over the network.
type ConfigurationError struct {
	Message string
	Cause   error
}

func (e *ConfigurationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("cryptflare: configuration error: %s: %v", e.Message, e.Cause)
	}
	return "cryptflare: configuration error: " + e.Message
}

func (e *ConfigurationError) Unwrap() error { return e.Cause }

// ConnectionError wraps a transport-level failure (DNS, TLS, refused).
// `errors.Is(err, context.Canceled)` still works on the original cause.
type ConnectionError struct {
	Message string
	Cause   error
}

func (e *ConnectionError) Error() string {
	return "cryptflare: connection error: " + e.Message
}

func (e *ConnectionError) Unwrap() error { return e.Cause }

// TimeoutError wraps a per-request timeout breach.
type TimeoutError struct {
	Message string
	Cause   error
}

func (e *TimeoutError) Error() string {
	return "cryptflare: timeout: " + e.Message
}

func (e *TimeoutError) Unwrap() error { return e.Cause }

// sentinelForStatus picks the matching package-level sentinel for an
// HTTP status, used to wire `errors.Is` chains.
func sentinelForStatus(status int) error {
	switch status {
	case 401:
		return ErrUnauthenticated
	case 403:
		return ErrPermissionDenied
	case 404:
		return ErrNotFound
	case 409:
		return ErrConflict
	case 422:
		return ErrUnprocessable
	case 429:
		return ErrRateLimited
	}
	if status >= 500 && status < 600 {
		return ErrInternal
	}
	return ErrUnknownAPI
}
