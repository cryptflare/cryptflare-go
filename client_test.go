package cryptflare

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClientRequiresCredential(t *testing.T) {
	t.Setenv("CRYPTFLARE_API_KEY", "")
	t.Setenv("CRYPTFLARE_SERVICE_TOKEN", "")
	if _, err := NewClient(); err == nil {
		t.Fatal("expected ConfigurationError when no credential supplied")
	} else {
		var cfgErr *ConfigurationError
		if !errors.As(err, &cfgErr) {
			t.Fatalf("expected *ConfigurationError, got %T", err)
		}
	}
}

func TestNewClientRejectsMultipleCredentials(t *testing.T) {
	if _, err := NewClient(WithAPIKey("a"), WithServiceToken("b")); err == nil {
		t.Fatal("expected error when both credentials supplied")
	}
}

func TestSecretsRevealHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/secrets/DATABASE_URL") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"key":       "DATABASE_URL",
				"value":     "postgres://prod",
				"version":   3,
				"updatedAt": "2026-05-02T00:00:00Z",
			},
		})
	}))
	defer srv.Close()

	c, err := NewClient(
		WithAPIKey("cf_test_dummy"),
		WithBaseURL(srv.URL),
		WithDefaultOrganisation("org"),
		WithDefaultWorkspace("ws"),
		WithDefaultEnvironment("prod"),
		WithoutRetry(),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	revealed, err := c.Secrets.Reveal(context.Background(), &RevealSecretInput{Key: "DATABASE_URL"})
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if revealed.Value != "postgres://prod" {
		t.Fatalf("got %q", revealed.Value)
	}
}

func TestAPIErrorMappingAndSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req_xyz")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":   "RESOURCE_NOT_FOUND",
			"message": "secret not found",
			"status":  404,
		})
	}))
	defer srv.Close()

	c, _ := NewClient(
		WithAPIKey("cf_test_dummy"),
		WithBaseURL(srv.URL),
		WithDefaultOrganisation("org"),
		WithDefaultWorkspace("ws"),
		WithDefaultEnvironment("prod"),
		WithoutRetry(),
	)
	_, err := c.Secrets.Reveal(context.Background(), &RevealSecretInput{Key: "MISSING"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected errors.Is ErrNotFound, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != "RESOURCE_NOT_FOUND" {
		t.Fatalf("got code %q", apiErr.Code)
	}
	if apiErr.RequestID != "req_xyz" {
		t.Fatalf("got request id %q", apiErr.RequestID)
	}
}

func TestBackoffJitterStaysInBounds(t *testing.T) {
	opts := RetryOptions{InitialDelay: 100, MaxDelay: 800}
	for attempt := 1; attempt <= 4; attempt++ {
		for i := 0; i < 200; i++ {
			d := computeBackoff(attempt, opts, 0)
			if d < 0 || d > opts.MaxDelay {
				t.Fatalf("attempt=%d delay=%v outside bounds", attempt, d)
			}
		}
	}
}
