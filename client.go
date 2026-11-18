// Package cryptflare is the official Go SDK for the CryptFlare API.
//
// Construct a client with `NewClient` and reach for resource methods on
// the returned `*Client`. Every method takes a `context.Context` first
// argument and returns `(result, error)` last; errors are typed under
// `*APIError`, `*ConnectionError`, `*TimeoutError`, and `*ConfigurationError`.
//
// Example:
//
//	client, err := cryptflare.NewClient(
//	    cryptflare.WithAPIKey(os.Getenv("CRYPTFLARE_API_KEY")),
//	    cryptflare.WithDefaultOrganisation("org_xyz"),
//	    cryptflare.WithDefaultWorkspace("my-app"),
//	    cryptflare.WithDefaultEnvironment("production"),
//	)
//	if err != nil {
//	    return err
//	}
//
//	page, err := client.Secrets.List(ctx, &cryptflare.ListSecretsInput{}).First(ctx)
package cryptflare

import (
	"net/http"
	"os"
	"time"
)

const (
	defaultBaseURL = "https://api.cryptflare.com"
	defaultTimeout = 60 * time.Second
)

// Client is the entry point for every API call. Construct once per
// process and reuse - the client owns the credential resolver, the
// retry policy, and the underlying `*http.Client`, all of which are
// expensive to recompute per call.
type Client struct {
	opts       ClientOptions
	httpClient *http.Client

	// Resources. New resources land here as the SDK grows.
	Secrets       *Secrets
	Pods          *Pods
	Environments  *Environments
	Workspaces    *Workspaces
	Organisations *Organisations
	Tokens        *Tokens
	Audit         *Audit
}

// NewClient constructs a `*Client` from a list of functional options.
// Returns `*ConfigurationError` if the option set is incomplete or
// contradictory (e.g. multiple credential modes specified).
func NewClient(options ...Option) (*Client, error) {
	opts := ClientOptions{
		BaseURL: defaultBaseURL,
		Timeout: defaultTimeout,
		Retry: RetryOptions{
			MaxAttempts:  defaultRetry.MaxAttempts,
			InitialDelay: defaultRetry.InitialDelay,
			MaxDelay:     defaultRetry.MaxDelay,
		},
	}
	for _, opt := range options {
		if err := opt(&opts); err != nil {
			return nil, err
		}
	}

	// Env-var fallback when no explicit credential option supplied.
	if opts.APIKey == "" && opts.ServiceToken == "" && opts.TokenSource == nil {
		if v := os.Getenv("CRYPTFLARE_API_KEY"); v != "" {
			opts.APIKey = v
		} else if v := os.Getenv("CRYPTFLARE_SERVICE_TOKEN"); v != "" {
			opts.ServiceToken = v
		}
	}

	credsCount := 0
	if opts.APIKey != "" {
		credsCount++
	}
	if opts.ServiceToken != "" {
		credsCount++
	}
	if opts.TokenSource != nil {
		credsCount++
	}
	if credsCount == 0 {
		return nil, &ConfigurationError{Message: "no credential supplied (WithAPIKey / WithServiceToken / WithTokenSource, or env CRYPTFLARE_API_KEY / CRYPTFLARE_SERVICE_TOKEN)"}
	}
	if credsCount > 1 {
		return nil, &ConfigurationError{Message: "multiple credentials supplied; pick exactly one"}
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: opts.Timeout}
	}
	if opts.RoundTripper != nil {
		// Wrap the existing transport. We don't replace the client.
		base := httpClient.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		httpClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return opts.RoundTripper.RoundTrip(req)
		})
		_ = base // silenced; future tracing wraps could chain
	}

	c := &Client{opts: opts, httpClient: httpClient}
	c.Secrets = &Secrets{client: c}
	c.Pods = &Pods{client: c}
	c.Environments = &Environments{client: c}
	c.Workspaces = &Workspaces{client: c}
	c.Organisations = &Organisations{client: c}
	c.Tokens = &Tokens{client: c}
	c.Audit = &Audit{client: c}
	return c, nil
}

// roundTripperFunc adapts a function to `http.RoundTripper`.
type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip satisfies the interface.
func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// resolveOrg looks at the per-call input then the client default.
// Returns `*ConfigurationError` when neither is populated.
func (c *Client) resolveOrg(orgFromCall string) (string, error) {
	if orgFromCall != "" {
		return orgFromCall, nil
	}
	if c.opts.DefaultOrg != "" {
		return c.opts.DefaultOrg, nil
	}
	return "", &ConfigurationError{Message: "organisation is required (pass per-call or use WithDefaultOrganisation)"}
}

// resolveWorkspace mirrors resolveOrg for workspace context.
func (c *Client) resolveWorkspace(call string) (string, error) {
	if call != "" {
		return call, nil
	}
	if c.opts.DefaultWorkspace != "" {
		return c.opts.DefaultWorkspace, nil
	}
	return "", &ConfigurationError{Message: "workspace is required (pass per-call or use WithDefaultWorkspace)"}
}

// resolveEnvironment mirrors resolveOrg for environment context.
func (c *Client) resolveEnvironment(call string) (string, error) {
	if call != "" {
		return call, nil
	}
	if c.opts.DefaultEnvironment != "" {
		return c.opts.DefaultEnvironment, nil
	}
	return "", &ConfigurationError{Message: "environment is required (pass per-call or use WithDefaultEnvironment)"}
}
