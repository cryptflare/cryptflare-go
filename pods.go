package cryptflare

import (
	"context"
	"net/http"
	"net/url"
	"path"
)

// Pods is the resource handle for pod CRUD + tree traversal. Pods are
// recursive containers (max depth 5) that group related secrets inside
// an environment.
type Pods struct {
	client *Client
}

// PodItem is a single pod row returned by `Pods.List` and friends.
type PodItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parentId,omitempty"`
	Path        string `json:"path,omitempty"`
	CreatedBy   string `json:"createdBy,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// PodAncestor is one node in the breadcrumb returned by `Pods.Get`.
type PodAncestor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// PodWithAncestors is the shape `Pods.Get` returns.
type PodWithAncestors struct {
	PodItem
	Ancestors []PodAncestor `json:"ancestors,omitempty"`
}

// ListPodsInput filters by parent. Pass a non-nil ParentID to drill
// into a sub-tree; nil returns root-level pods only.
type ListPodsInput struct {
	Organisation string
	Workspace    string
	Environment  string
	ParentID     *string
}

// CreatePodInput is the body for `Pods.Create`.
type CreatePodInput struct {
	Organisation string  `json:"-"`
	Workspace    string  `json:"-"`
	Environment  string  `json:"-"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  string  `json:"description,omitempty"`
	ParentID     *string `json:"parentId,omitempty"`
}

// UpdatePodInput is the body for `Pods.Update`. All fields optional.
type UpdatePodInput struct {
	Organisation string  `json:"-"`
	Workspace    string  `json:"-"`
	Environment  string  `json:"-"`
	ID           string  `json:"-"`
	Name         *string `json:"name,omitempty"`
	Slug         *string `json:"slug,omitempty"`
	Description  *string `json:"description,omitempty"`
}

func (p *Pods) basePath(org, ws, env string) string {
	return "/v1" + path.Join("/organisations", org, "workspaces", ws, "environments", env, "pods")
}

// List enumerates pods at one level. Nil ParentID = root-level pods.
func (p *Pods) List(ctx context.Context, input *ListPodsInput) *PageRequest[PodItem] {
	if input == nil {
		input = &ListPodsInput{}
	}
	return &PageRequest[PodItem]{
		fetcher: func(ctx context.Context, cursor string) (*Page[PodItem], error) {
			org, err := p.client.resolveOrg(input.Organisation)
			if err != nil {
				return nil, err
			}
			ws, err := p.client.resolveWorkspace(input.Workspace)
			if err != nil {
				return nil, err
			}
			env, err := p.client.resolveEnvironment(input.Environment)
			if err != nil {
				return nil, err
			}
			query := url.Values{}
			if cursor != "" {
				query.Set("cursor", cursor)
			}
			if input.ParentID != nil {
				query.Set("parentId", *input.ParentID)
			}
			var resp struct {
				Data       []PodItem `json:"data"`
				NextCursor string    `json:"nextCursor"`
				HasMore    bool      `json:"hasMore"`
			}
			if err := p.client.do(ctx, requestDescriptor{
				Method: http.MethodGet,
				Path:   p.basePath(org, ws, env),
				Query:  query,
			}, &resp); err != nil {
				return nil, err
			}
			page := &Page[PodItem]{
				Data:       resp.Data,
				NextCursor: resp.NextCursor,
				HasMore:    resp.HasMore,
			}
			page.fetcher = func(ctx context.Context, c string) (*Page[PodItem], error) {
				return p.List(ctx, input).fetcher(ctx, c)
			}
			return page, nil
		},
	}
}

// Get returns the pod plus its ancestor breadcrumb.
func (p *Pods) Get(ctx context.Context, organisation, workspace, environment, id string, opts ...RequestOption) (*PodWithAncestors, error) {
	if id == "" {
		return nil, &ConfigurationError{Message: "pod id is required"}
	}
	org, err := p.client.resolveOrg(organisation)
	if err != nil {
		return nil, err
	}
	ws, err := p.client.resolveWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	env, err := p.client.resolveEnvironment(environment)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data PodWithAncestors `json:"data"`
	}
	if err := p.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   p.basePath(org, ws, env) + "/" + url.PathEscape(id),
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Create makes a new pod, optionally nested under a parent.
func (p *Pods) Create(ctx context.Context, input *CreatePodInput, opts ...RequestOption) (*PodItem, error) {
	if input == nil || input.Name == "" || input.Slug == "" {
		return nil, &ConfigurationError{Message: "name and slug are required"}
	}
	org, err := p.client.resolveOrg(input.Organisation)
	if err != nil {
		return nil, err
	}
	ws, err := p.client.resolveWorkspace(input.Workspace)
	if err != nil {
		return nil, err
	}
	env, err := p.client.resolveEnvironment(input.Environment)
	if err != nil {
		return nil, err
	}
	var item PodItem
	if err := p.client.do(ctx, requestDescriptor{
		Method: http.MethodPost,
		Path:   p.basePath(org, ws, env),
		Body:   input,
	}, &item, opts...); err != nil {
		return nil, err
	}
	return &item, nil
}

// Update mutates one or more pod fields in place.
func (p *Pods) Update(ctx context.Context, input *UpdatePodInput, opts ...RequestOption) (*PodItem, error) {
	if input == nil || input.ID == "" {
		return nil, &ConfigurationError{Message: "pod id is required"}
	}
	org, err := p.client.resolveOrg(input.Organisation)
	if err != nil {
		return nil, err
	}
	ws, err := p.client.resolveWorkspace(input.Workspace)
	if err != nil {
		return nil, err
	}
	env, err := p.client.resolveEnvironment(input.Environment)
	if err != nil {
		return nil, err
	}
	var item PodItem
	if err := p.client.do(ctx, requestDescriptor{
		Method: http.MethodPatch,
		Path:   p.basePath(org, ws, env) + "/" + url.PathEscape(input.ID),
		Body:   input,
	}, &item, opts...); err != nil {
		return nil, err
	}
	return &item, nil
}

// Delete removes an empty pod. Server returns 422 POD_NOT_EMPTY when
// secrets or sub-pods still live underneath.
func (p *Pods) Delete(ctx context.Context, organisation, workspace, environment, id string, opts ...RequestOption) error {
	if id == "" {
		return &ConfigurationError{Message: "pod id is required"}
	}
	org, err := p.client.resolveOrg(organisation)
	if err != nil {
		return err
	}
	ws, err := p.client.resolveWorkspace(workspace)
	if err != nil {
		return err
	}
	env, err := p.client.resolveEnvironment(environment)
	if err != nil {
		return err
	}
	return p.client.do(ctx, requestDescriptor{
		Method: http.MethodDelete,
		Path:   p.basePath(org, ws, env) + "/" + url.PathEscape(id),
	}, nil, opts...)
}
