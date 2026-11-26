package cryptflare

import (
	"math"
	"math/rand/v2"
	"net/http"
	"time"
)

// RetryOptions controls how the SDK reattempts failed requests.
// Defaults match the umbrella spec: 3 attempts, full jitter, base 500
// ms, cap 8 s.
type RetryOptions struct {
	// MaxAttempts is the total number of attempts including the
	// initial one. 1 disables retries.
	MaxAttempts int
	// InitialDelay is the base interval before the first retry.
	InitialDelay time.Duration
	// MaxDelay caps any single retry interval.
	MaxDelay time.Duration
	// ShouldRetry overrides the default retry matrix. Return true to
	// retry, false to surface the error immediately. The supplied
	// `*APIError` is nil for transport-level failures (look at the
	// outer error in that case).
	ShouldRetry func(err *APIError, attempt int) bool
}

// defaultRetry reflects the cross-language spec.
var defaultRetry = RetryOptions{
	MaxAttempts:  3,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay:     8 * time.Second,
}

// shouldRetryDefault implements the retry matrix per the umbrella spec:
//
//   - Always retry transport-level failures (caller surfaced as nil
//     APIError because we never sent a successful response).
//   - 408, 429, 500, 502, 503, 504 retry by default.
//   - 409 retries only when the API code is IDEMPOTENCY_KEY_COLLISION.
//   - Anything else (400, 401, 403, 404, 422 etc.) is a caller bug and
//     never retries.
func shouldRetryDefault(err *APIError, _ int) bool {
	if err == nil {
		return true // transport failure
	}
	switch err.Status {
	case 408, 429, 500, 502, 503, 504:
		return true
	case 409:
		return err.Code == "IDEMPOTENCY_KEY_COLLISION"
	}
	return false
}

// computeBackoff returns the next-attempt delay with full jitter.
// `attempt` starts at 1 for the first retry.
func computeBackoff(attempt int, opts RetryOptions, retryAfter time.Duration) time.Duration {
	// Server-side `Retry-After` always wins.
	if retryAfter > 0 {
		if retryAfter > opts.MaxDelay {
			return opts.MaxDelay
		}
		return retryAfter
	}
	cap := float64(opts.MaxDelay)
	exp := math.Pow(2, float64(attempt-1))
	upper := math.Min(cap, float64(opts.InitialDelay)*exp)
	if upper <= 0 {
		return 0
	}
	return time.Duration(rand.Float64() * upper)
}

// parseRetryAfter reads the `Retry-After` header per RFC 9110. Returns
// 0 if absent or unparseable.
func parseRetryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	// Seconds form.
	if d, err := time.ParseDuration(v + "s"); err == nil {
		return d
	}
	// HTTP-date form.
	if t, err := http.ParseTime(v); err == nil {
		delta := time.Until(t)
		if delta > 0 {
			return delta
		}
	}
	return 0
}
