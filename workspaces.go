package cryptflare

import (
	"context"
	"net/http"
	"net/url"
	"path"
)

// Workspaces is the resource handle for workspace CRUD inside one
// organisation. Workspaces own environments, which own secrets.
type Workspaces struct {
	client *Client
}

// WorkspaceItem is the shape returned by `Workspaces.List`.
type WorkspaceItem struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	OrganisationID string `json:"organisation_id,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
}

// CreateWorkspaceInput is the body for `Workspaces.Create`.
type CreateWorkspaceInput struct {
	Organisation string `json:"-"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
}

func (w *Workspaces) basePath(org string) string {
	return "/v1" + path.Join("/organisations", org, "workspaces")
}

// List enumerates workspaces under one organisation.
func (w *Workspaces) List(ctx context.Context, organisation string, opts ...RequestOption) ([]WorkspaceItem, error) {
	org, err := w.client.resolveOrg(organisation)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []WorkspaceItem `json:"data"`
	}
	if err := w.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   w.basePath(org),
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// Get returns one workspace by id or slug.
func (w *Workspaces) Get(ctx context.Context, organisation, idOrSlug string, opts ...RequestOption) (*WorkspaceItem, error) {
	if idOrSlug == "" {
		return nil, &ConfigurationError{Message: "workspace id or slug is required"}
	}
	org, err := w.client.resolveOrg(organisation)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data WorkspaceItem `json:"data"`
	}
	if err := w.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   w.basePath(org) + "/" + url.PathEscape(idOrSlug),
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Create makes a new workspace.
func (w *Workspaces) Create(ctx context.Context, input *CreateWorkspaceInput, opts ...RequestOption) (*WorkspaceItem, error) {
	if input == nil || input.Name == "" || input.Slug == "" {
		return nil, &ConfigurationError{Message: "name and slug are required"}
	}
	org, err := w.client.resolveOrg(input.Organisation)
	if err != nil {
		return nil, err
	}
	var item WorkspaceItem
	if err := w.client.do(ctx, requestDescriptor{
		Method: http.MethodPost,
		Path:   w.basePath(org),
		Body:   input,
	}, &item, opts...); err != nil {
		return nil, err
	}
	return &item, nil
}

// Delete removes a workspace and every environment + secret it owns.
// Irreversible. Use the dashboard if you want a confirmation prompt.
func (w *Workspaces) Delete(ctx context.Context, organisation, idOrSlug string, opts ...RequestOption) error {
	if idOrSlug == "" {
		return &ConfigurationError{Message: "workspace id or slug is required"}
	}
	org, err := w.client.resolveOrg(organisation)
	if err != nil {
		return err
	}
	return w.client.do(ctx, requestDescriptor{
		Method: http.MethodDelete,
		Path:   w.basePath(org) + "/" + url.PathEscape(idOrSlug),
	}, nil, opts...)
}
