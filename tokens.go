package cryptflare

import (
	"context"
	"net/http"
	"net/url"
	"path"
)

// Tokens manages personal access tokens (PATs). A PAT inherits the
// creator's role at evaluation time and can be revoked individually.
type Tokens struct {
	client *Client
}

// TokenItem is the metadata-only shape returned by `Tokens.List`. The
// plaintext token is returned exactly once on `Tokens.Create`.
type TokenItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	TokenPrefix string   `json:"tokenPrefix"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   *string  `json:"expiresAt"`
	LastUsedAt  *string  `json:"lastUsedAt"`
	CreatedAt   string   `json:"createdAt"`
}

// CreatedToken is what `Tokens.Create` returns. The `Token` field is
// the only time the plaintext is exposed - persist it immediately.
type CreatedToken struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Token       string   `json:"token"`
	TokenPrefix string   `json:"tokenPrefix"`
	Scopes      []string `json:"scopes"`
}

// CreateTokenInput is the body for `Tokens.Create`.
type CreateTokenInput struct {
	Organisation string   `json:"-"`
	Name         string   `json:"name"`
	WorkspaceID  string   `json:"workspaceId"`
	Scopes       []string `json:"scopes"`
	Environment  string   `json:"environment,omitempty"`
	ExpiresAt    string   `json:"expiresAt,omitempty"`
}

func (t *Tokens) basePath(org string) string {
	return "/v1" + path.Join("/organisations", org, "tokens")
}

// List returns every PAT in the organisation.
func (t *Tokens) List(ctx context.Context, organisation string, opts ...RequestOption) ([]TokenItem, error) {
	org, err := t.client.resolveOrg(organisation)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []TokenItem `json:"data"`
	}
	if err := t.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   t.basePath(org),
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// Create generates a new PAT. The plaintext value is returned exactly
// once; persist it before the call returns.
func (t *Tokens) Create(ctx context.Context, input *CreateTokenInput, opts ...RequestOption) (*CreatedToken, error) {
	if input == nil || input.Name == "" || input.WorkspaceID == "" || len(input.Scopes) == 0 {
		return nil, &ConfigurationError{Message: "name, workspaceId, and at least one scope are required"}
	}
	org, err := t.client.resolveOrg(input.Organisation)
	if err != nil {
		return nil, err
	}
	if input.Environment == "" {
		input.Environment = "live"
	}
	var resp struct {
		Data CreatedToken `json:"data"`
	}
	if err := t.client.do(ctx, requestDescriptor{
		Method: http.MethodPost,
		Path:   t.basePath(org),
		Body:   input,
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Revoke deletes the token. Subsequent requests using it return 401
// AUTH_INVALID_TOKEN immediately.
func (t *Tokens) Revoke(ctx context.Context, organisation, tokenID string, opts ...RequestOption) error {
	if tokenID == "" {
		return &ConfigurationError{Message: "tokenId is required"}
	}
	org, err := t.client.resolveOrg(organisation)
	if err != nil {
		return err
	}
	return t.client.do(ctx, requestDescriptor{
		Method: http.MethodDelete,
		Path:   t.basePath(org) + "/" + url.PathEscape(tokenID),
	}, nil, opts...)
}
