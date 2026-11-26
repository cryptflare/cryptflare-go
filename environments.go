package cryptflare

import (
	"context"
	"net/http"
	"net/url"
	"path"
)

// Environments is the resource handle for environment CRUD inside a
// workspace. Common environments are `production`, `staging`,
// `development` but the slug is free-form.
type Environments struct {
	client *Client
}

// EnvironmentItem is the shape returned by `Environments.List`.
type EnvironmentItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
}

// CreateEnvironmentInput is the body for `Environments.Create`.
type CreateEnvironmentInput struct {
	Organisation string `json:"-"`
	Workspace    string `json:"-"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
}

func (e *Environments) basePath(org, ws string) string {
	return "/v1" + path.Join("/organisations", org, "workspaces", ws, "environments")
}

// List enumerates environments inside a workspace.
func (e *Environments) List(ctx context.Context, organisation, workspace string, opts ...RequestOption) ([]EnvironmentItem, error) {
	org, err := e.client.resolveOrg(organisation)
	if err != nil {
		return nil, err
	}
	ws, err := e.client.resolveWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []EnvironmentItem `json:"data"`
	}
	if err := e.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   e.basePath(org, ws),
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// Get returns one environment by id or slug.
func (e *Environments) Get(ctx context.Context, organisation, workspace, idOrSlug string, opts ...RequestOption) (*EnvironmentItem, error) {
	if idOrSlug == "" {
		return nil, &ConfigurationError{Message: "environment id or slug is required"}
	}
	org, err := e.client.resolveOrg(organisation)
	if err != nil {
		return nil, err
	}
	ws, err := e.client.resolveWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data EnvironmentItem `json:"data"`
	}
	if err := e.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   e.basePath(org, ws) + "/" + url.PathEscape(idOrSlug),
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Create makes a new environment.
func (e *Environments) Create(ctx context.Context, input *CreateEnvironmentInput, opts ...RequestOption) (*EnvironmentItem, error) {
	if input == nil || input.Name == "" || input.Slug == "" {
		return nil, &ConfigurationError{Message: "name and slug are required"}
	}
	org, err := e.client.resolveOrg(input.Organisation)
	if err != nil {
		return nil, err
	}
	ws, err := e.client.resolveWorkspace(input.Workspace)
	if err != nil {
		return nil, err
	}
	var item EnvironmentItem
	if err := e.client.do(ctx, requestDescriptor{
		Method: http.MethodPost,
		Path:   e.basePath(org, ws),
		Body:   input,
	}, &item, opts...); err != nil {
		return nil, err
	}
	return &item, nil
}

// Delete removes an environment. Server returns 422 if any pods or
// secrets still live inside.
func (e *Environments) Delete(ctx context.Context, organisation, workspace, idOrSlug string, opts ...RequestOption) error {
	if idOrSlug == "" {
		return &ConfigurationError{Message: "environment id or slug is required"}
	}
	org, err := e.client.resolveOrg(organisation)
	if err != nil {
		return err
	}
	ws, err := e.client.resolveWorkspace(workspace)
	if err != nil {
		return err
	}
	return e.client.do(ctx, requestDescriptor{
		Method: http.MethodDelete,
		Path:   e.basePath(org, ws) + "/" + url.PathEscape(idOrSlug),
	}, nil, opts...)
}
