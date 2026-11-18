package cryptflare

import (
	"context"
	"net/http"
	"time"
)

// ClientOptions captures every configurable knob on `*Client`. Fields are
// populated by `Option` functions; callers never touch this struct
// directly. The struct is unexported in spirit - exported only so test
// helpers in `tests/` can build sentinel configurations.
type ClientOptions struct {
	APIKey          string
	ServiceToken    string
	TokenSource     func(context.Context) (string, error)
	BaseURL         string
	HTTPClient      *http.Client
	RoundTripper    http.RoundTripper
	Timeout         time.Duration
	Retry           RetryOptions
	UserAgentSuffix string
	Hooks           Hooks
	DefaultOrg      string
	DefaultWorkspace string
	DefaultEnvironment string
}

// Option configures `*Client` at construction time. Applied in order so
// later options override earlier ones.
type Option func(*ClientOptions) error

// WithAPIKey supplies a personal access token (`cf_pat_...`).
func WithAPIKey(token string) Option {
	return func(o *ClientOptions) error {
		if token == "" {
			return &ConfigurationError{Message: "api key cannot be empty"}
		}
		o.APIKey = token
		return nil
	}
}

// WithServiceToken supplies an organisation-level service token (`cf_svc_...`).
func WithServiceToken(token string) Option {
	return func(o *ClientOptions) error {
		if token == "" {
			return &ConfigurationError{Message: "service token cannot be empty"}
		}
		o.ServiceToken = token
		return nil
	}
}

// WithTokenSource registers an async callback that resolves the bearer
// token before each request. Use this for refreshable credentials,
// federated identity, or in-process token rotation.
func WithTokenSource(fn func(context.Context) (string, error)) Option {
	return func(o *ClientOptions) error {
		if fn == nil {
			return &ConfigurationError{Message: "token source cannot be nil"}
		}
		o.TokenSource = fn
		return nil
	}
}

// WithBaseURL overrides the default base URL.
func WithBaseURL(url string) Option {
	return func(o *ClientOptions) error {
		o.BaseURL = url
		return nil
	}
}

// WithHTTPClient swaps the underlying `*http.Client`. The supplied
// client's Transport is used as-is.
func WithHTTPClient(c *http.Client) Option {
	return func(o *ClientOptions) error {
		if c == nil {
			return &ConfigurationError{Message: "http client cannot be nil"}
		}
		o.HTTPClient = c
		return nil
	}
}

// WithRoundTripper wraps the default client's Transport. Use this to
// add tracing, metrics, or proxy support without replacing the client.
func WithRoundTripper(rt http.RoundTripper) Option {
	return func(o *ClientOptions) error {
		o.RoundTripper = rt
		return nil
	}
}

// WithTimeout sets the per-request timeout. Default 60s.
func WithTimeout(d time.Duration) Option {
	return func(o *ClientOptions) error {
		o.Timeout = d
		return nil
	}
}

// WithRetry overrides the retry policy.
func WithRetry(r RetryOptions) Option {
	return func(o *ClientOptions) error {
		o.Retry = r
		return nil
	}
}

// WithoutRetry disables all retries. Equivalent to MaxAttempts = 1.
func WithoutRetry() Option {
	return func(o *ClientOptions) error {
		o.Retry.MaxAttempts = 1
		return nil
	}
}

// WithUserAgentSuffix appends a string to the SDK's default User-Agent.
func WithUserAgentSuffix(s string) Option {
	return func(o *ClientOptions) error {
		o.UserAgentSuffix = s
		return nil
	}
}

// WithHooks installs request / response / retry hooks. See `Hooks`.
func WithHooks(h Hooks) Option {
	return func(o *ClientOptions) error {
		o.Hooks = h
		return nil
	}
}

// WithDefaultOrganisation sets the default org id used when a per-call
// option does not supply one.
func WithDefaultOrganisation(id string) Option {
	return func(o *ClientOptions) error {
		o.DefaultOrg = id
		return nil
	}
}

// WithDefaultWorkspace sets the default workspace slug.
func WithDefaultWorkspace(slug string) Option {
	return func(o *ClientOptions) error {
		o.DefaultWorkspace = slug
		return nil
	}
}

// WithDefaultEnvironment sets the default environment slug.
func WithDefaultEnvironment(slug string) Option {
	return func(o *ClientOptions) error {
		o.DefaultEnvironment = slug
		return nil
	}
}

// Hooks let callers observe request lifecycle without depending on a
// particular logging library. All hooks must be non-blocking and
// non-panicking. The SDK recovers panics and logs to stderr.
type Hooks struct {
	OnRequest  func(req *http.Request)
	OnResponse func(req *http.Request, res *http.Response, durationMs int64)
	OnRetry    func(attempt int, delay time.Duration, err error)
}

// RequestOption modifies a single resource call.
type RequestOption func(*requestConfig)

type requestConfig struct {
	IdempotencyKey string
	Headers        http.Header
}

// WithIdempotencyKey overrides the auto-generated UUID for one call.
// Use the same key across retries of the same logical operation.
func WithIdempotencyKey(key string) RequestOption {
	return func(rc *requestConfig) {
		rc.IdempotencyKey = key
	}
}

// WithRequestHeader appends a custom header to one call. Reserved
// headers (`Authorization`, `Content-Type`, `User-Agent`,
// `Idempotency-Key`) cannot be overridden and are silently ignored.
func WithRequestHeader(key, value string) RequestOption {
	return func(rc *requestConfig) {
		if rc.Headers == nil {
			rc.Headers = http.Header{}
		}
		switch key {
		case "Authorization", "Content-Type", "User-Agent", "Idempotency-Key":
			return
		}
		rc.Headers.Set(key, value)
	}
}
