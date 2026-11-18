package cryptflare

import (
	"context"
	"net/http"
	"net/url"
)

// Organisations is the resource handle for org-level CRUD. The bearer
// must already be a member of every org it operates on.
type Organisations struct {
	client *Client
}

// OrgItem is a row from `Organisations.List`.
type OrgItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug,omitempty"`
	Plan       string `json:"plan,omitempty"`
	Role       string `json:"role,omitempty"`
	DataRegion string `json:"dataRegion,omitempty"`
}

// OrgDetail is the richer shape `Organisations.Get` returns.
type OrgDetail struct {
	OrgItem
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// CreateOrgInput is the body for `Organisations.Create`.
type CreateOrgInput struct {
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	DataRegion string `json:"dataRegion,omitempty"`
}

// UpdateOrgInput is the body for `Organisations.Update`.
type UpdateOrgInput struct {
	ID   string  `json:"-"`
	Name *string `json:"name,omitempty"`
	Slug *string `json:"slug,omitempty"`
}

// List enumerates organisations the caller belongs to.
func (o *Organisations) List(ctx context.Context, opts ...RequestOption) ([]OrgItem, error) {
	var resp struct {
		Data []OrgItem `json:"data"`
	}
	if err := o.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   "/v1/organisations",
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// Get returns one organisation by id (UUID required for bearer tokens).
func (o *Organisations) Get(ctx context.Context, id string, opts ...RequestOption) (*OrgDetail, error) {
	if id == "" {
		return nil, &ConfigurationError{Message: "organisation id is required"}
	}
	var resp struct {
		Data OrgDetail `json:"data"`
	}
	if err := o.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   "/v1/organisations/" + url.PathEscape(id),
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Create provisions a new organisation. The caller becomes its owner.
func (o *Organisations) Create(ctx context.Context, input *CreateOrgInput, opts ...RequestOption) (*OrgDetail, error) {
	if input == nil || input.Name == "" || input.Slug == "" {
		return nil, &ConfigurationError{Message: "name and slug are required"}
	}
	var resp struct {
		Data OrgDetail `json:"data"`
	}
	if err := o.client.do(ctx, requestDescriptor{
		Method: http.MethodPost,
		Path:   "/v1/organisations",
		Body:   input,
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Update mutates the org name or slug.
func (o *Organisations) Update(ctx context.Context, input *UpdateOrgInput, opts ...RequestOption) (*OrgDetail, error) {
	if input == nil || input.ID == "" {
		return nil, &ConfigurationError{Message: "organisation id is required"}
	}
	var resp struct {
		Data OrgDetail `json:"data"`
	}
	if err := o.client.do(ctx, requestDescriptor{
		Method: http.MethodPatch,
		Path:   "/v1/organisations/" + url.PathEscape(input.ID),
		Body:   input,
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
