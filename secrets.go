package cryptflare

import (
	"context"
	"net/http"
	"net/url"
	"path"
)

// Secrets is the resource handle for secret CRUD + reveal + rotate.
// One instance per `*Client`. Methods are thread-safe; the client owns
// the underlying transport.
type Secrets struct {
	client *Client
}

// SecretListItem is the metadata-only shape the list endpoint returns.
// Plaintext values come back via Reveal.
type SecretListItem struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Version   int    `json:"version"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// RevealedSecret is the plaintext-bearing shape of a single secret.
type RevealedSecret struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Version   int    `json:"version"`
	UpdatedAt string `json:"updatedAt"`
}

// CreateSecretInput is the request body for `Secrets.Create`.
type CreateSecretInput struct {
	// Workspace and Environment override the client-level defaults.
	Organisation string `json:"-"`
	Workspace    string `json:"-"`
	Environment  string `json:"-"`
	// Required.
	Key   string `json:"key"`
	Value string `json:"value"`
	// PodID places the secret inside a pod. Nil keeps it at root.
	PodID *string `json:"podId,omitempty"`
}

// ListSecretsInput is the request input for `Secrets.List`.
type ListSecretsInput struct {
	Organisation string
	Workspace    string
	Environment  string
	// PodID filters to one pod. Pass empty string for "root only" by
	// using the literal "root" sentinel; nil pointer means "no
	// filter" (server returns flat list across the environment).
	PodID *string
}

// RevealSecretInput is the request input for `Secrets.Reveal`.
type RevealSecretInput struct {
	Organisation string
	Workspace    string
	Environment  string
	Key          string
}

func (s *Secrets) basePath(org, ws, env string) string {
	return "/v1" + path.Join("/organisations", org, "workspaces", ws, "environments", env, "secrets")
}

// List enumerates secret metadata in an environment. The returned
// `*PageRequest` has `First` and `Each` helpers.
//
//	page, err := client.Secrets.List(ctx, &cryptflare.ListSecretsInput{}).First(ctx)
//	for _, s := range page.Data { fmt.Println(s.Key) }
//
//	err := client.Secrets.List(ctx, in).Each(ctx, func(item cryptflare.SecretListItem) error {
//	    fmt.Println(item.Key)
//	    return nil
//	})
func (s *Secrets) List(ctx context.Context, input *ListSecretsInput) *PageRequest[SecretListItem] {
	if input == nil {
		input = &ListSecretsInput{}
	}
	return &PageRequest[SecretListItem]{
		fetcher: func(ctx context.Context, cursor string) (*Page[SecretListItem], error) {
			org, err := s.client.resolveOrg(input.Organisation)
			if err != nil {
				return nil, err
			}
			ws, err := s.client.resolveWorkspace(input.Workspace)
			if err != nil {
				return nil, err
			}
			env, err := s.client.resolveEnvironment(input.Environment)
			if err != nil {
				return nil, err
			}
			query := url.Values{}
			if cursor != "" {
				query.Set("cursor", cursor)
			}
			if input.PodID != nil {
				if *input.PodID == "" {
					query.Set("podId", "root")
				} else {
					query.Set("podId", *input.PodID)
				}
			}

			var resp struct {
				Data       []SecretListItem `json:"data"`
				NextCursor string           `json:"nextCursor"`
				HasMore    bool             `json:"hasMore"`
			}
			if err := s.client.do(ctx, requestDescriptor{
				Method: http.MethodGet,
				Path:   s.basePath(org, ws, env),
				Query:  query,
			}, &resp); err != nil {
				return nil, err
			}
			page := &Page[SecretListItem]{
				Data:       resp.Data,
				NextCursor: resp.NextCursor,
				HasMore:    resp.HasMore,
			}
			page.fetcher = func(ctx context.Context, c string) (*Page[SecretListItem], error) {
				return s.List(ctx, input).fetcher(ctx, c)
			}
			return page, nil
		},
	}
}

// Reveal returns the plaintext value for one secret.
func (s *Secrets) Reveal(ctx context.Context, input *RevealSecretInput, opts ...RequestOption) (*RevealedSecret, error) {
	if input == nil || input.Key == "" {
		return nil, &ConfigurationError{Message: "secret key is required"}
	}
	org, err := s.client.resolveOrg(input.Organisation)
	if err != nil {
		return nil, err
	}
	ws, err := s.client.resolveWorkspace(input.Workspace)
	if err != nil {
		return nil, err
	}
	env, err := s.client.resolveEnvironment(input.Environment)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data RevealedSecret `json:"data"`
	}
	if err := s.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   s.basePath(org, ws, env) + "/" + url.PathEscape(input.Key),
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Create writes a new secret. Use Rotate to change an existing value -
// Create returns 409 RESOURCE_CONFLICT if the key already exists.
func (s *Secrets) Create(ctx context.Context, input *CreateSecretInput, opts ...RequestOption) (*SecretListItem, error) {
	if input == nil || input.Key == "" {
		return nil, &ConfigurationError{Message: "secret key is required"}
	}
	org, err := s.client.resolveOrg(input.Organisation)
	if err != nil {
		return nil, err
	}
	ws, err := s.client.resolveWorkspace(input.Workspace)
	if err != nil {
		return nil, err
	}
	env, err := s.client.resolveEnvironment(input.Environment)
	if err != nil {
		return nil, err
	}
	var item SecretListItem
	if err := s.client.do(ctx, requestDescriptor{
		Method: http.MethodPost,
		Path:   s.basePath(org, ws, env),
		Body:   input,
	}, &item, opts...); err != nil {
		return nil, err
	}
	return &item, nil
}

// Rotate writes a new version of an existing secret.
func (s *Secrets) Rotate(ctx context.Context, input *struct {
	Organisation string `json:"-"`
	Workspace    string `json:"-"`
	Environment  string `json:"-"`
	Key          string `json:"-"`
	Value        string `json:"value"`
}, opts ...RequestOption) (*SecretListItem, error) {
	if input == nil || input.Key == "" || input.Value == "" {
		return nil, &ConfigurationError{Message: "key and value are required"}
	}
	org, err := s.client.resolveOrg(input.Organisation)
	if err != nil {
		return nil, err
	}
	ws, err := s.client.resolveWorkspace(input.Workspace)
	if err != nil {
		return nil, err
	}
	env, err := s.client.resolveEnvironment(input.Environment)
	if err != nil {
		return nil, err
	}
	var item SecretListItem
	if err := s.client.do(ctx, requestDescriptor{
		Method: http.MethodPost,
		Path:   s.basePath(org, ws, env) + "/" + url.PathEscape(input.Key) + "/rotate",
		Body:   input,
	}, &item, opts...); err != nil {
		return nil, err
	}
	return &item, nil
}

// Delete removes a secret and its version history. Irreversible.
func (s *Secrets) Delete(ctx context.Context, input *RevealSecretInput, opts ...RequestOption) error {
	if input == nil || input.Key == "" {
		return &ConfigurationError{Message: "secret key is required"}
	}
	org, err := s.client.resolveOrg(input.Organisation)
	if err != nil {
		return err
	}
	ws, err := s.client.resolveWorkspace(input.Workspace)
	if err != nil {
		return err
	}
	env, err := s.client.resolveEnvironment(input.Environment)
	if err != nil {
		return err
	}
	return s.client.do(ctx, requestDescriptor{
		Method: http.MethodDelete,
		Path:   s.basePath(org, ws, env) + "/" + url.PathEscape(input.Key),
	}, nil, opts...)
}
