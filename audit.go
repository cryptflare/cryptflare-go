package cryptflare

import (
	"context"
	"net/http"
	"net/url"
	"path"
)

// Audit exposes the append-only audit log + integrity verification.
type Audit struct {
	client *Client
}

// AuditEvent is one row from `Audit.List`. `Action` matches the
// registry in `@cryptflare/shared/constants/audit-actions` (e.g.
// `secret.created`, `secret.rotated`).
type AuditEvent struct {
	ID           string                 `json:"id"`
	Action       string                 `json:"action"`
	ActorID      string                 `json:"actorId,omitempty"`
	ResourceType string                 `json:"resourceType,omitempty"`
	ResourceID   string                 `json:"resourceId,omitempty"`
	Metadata     map[string]any         `json:"metadata,omitempty"`
	OccurredAt   string                 `json:"occurredAt"`
}

// ListAuditInput is the request input for `Audit.List`. All filters are
// optional; an empty struct returns the full log paginated.
type ListAuditInput struct {
	Organisation string
	ActorID      string
	Action       string
	From         string // ISO 8601 lower bound (inclusive)
	To           string // ISO 8601 upper bound (exclusive)
}

// IntegrityReport mirrors the response shape from `Audit.IntegrityReport`.
// `ChainBroken == true` is a security incident; surface it loudly.
type IntegrityReport struct {
	Total       int    `json:"total"`
	Verified    int    `json:"verified"`
	ChainBroken bool   `json:"chainBroken"`
	BreakAt     string `json:"breakAt,omitempty"`
}

func (a *Audit) basePath(org string) string {
	return "/v1" + path.Join("/organisations", org, "audit")
}

// List walks the audit log paginated. Supports filtering by actor,
// action, and timestamp range. Cursors are opaque - never parse them.
func (a *Audit) List(ctx context.Context, input *ListAuditInput) *PageRequest[AuditEvent] {
	if input == nil {
		input = &ListAuditInput{}
	}
	return &PageRequest[AuditEvent]{
		fetcher: func(ctx context.Context, cursor string) (*Page[AuditEvent], error) {
			org, err := a.client.resolveOrg(input.Organisation)
			if err != nil {
				return nil, err
			}
			query := url.Values{}
			if cursor != "" {
				query.Set("cursor", cursor)
			}
			if input.ActorID != "" {
				query.Set("actorId", input.ActorID)
			}
			if input.Action != "" {
				query.Set("action", input.Action)
			}
			if input.From != "" {
				query.Set("from", input.From)
			}
			if input.To != "" {
				query.Set("to", input.To)
			}
			var resp struct {
				Data       []AuditEvent `json:"data"`
				NextCursor string       `json:"nextCursor"`
				HasMore    bool         `json:"hasMore"`
			}
			if err := a.client.do(ctx, requestDescriptor{
				Method: http.MethodGet,
				Path:   a.basePath(org),
				Query:  query,
			}, &resp); err != nil {
				return nil, err
			}
			page := &Page[AuditEvent]{
				Data:       resp.Data,
				NextCursor: resp.NextCursor,
				HasMore:    resp.HasMore,
			}
			page.fetcher = func(ctx context.Context, c string) (*Page[AuditEvent], error) {
				return a.List(ctx, input).fetcher(ctx, c)
			}
			return page, nil
		},
	}
}

// VerifyChain re-walks the audit chain and confirms every entry's
// hash matches the previous entry's signature. Returns the integrity
// report; `ChainBroken == true` is a real incident.
func (a *Audit) VerifyChain(ctx context.Context, organisation, from, to string, opts ...RequestOption) (*IntegrityReport, error) {
	org, err := a.client.resolveOrg(organisation)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	if from != "" {
		query.Set("from", from)
	}
	if to != "" {
		query.Set("to", to)
	}
	var resp struct {
		Data IntegrityReport `json:"data"`
	}
	if err := a.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   a.basePath(org) + "/verify-chain",
		Query:  query,
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// IntegrityReport returns the most recent verification snapshot
// without re-walking the chain. Cheap to call from a dashboard.
func (a *Audit) IntegrityReport(ctx context.Context, organisation string, opts ...RequestOption) (*IntegrityReport, error) {
	org, err := a.client.resolveOrg(organisation)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data IntegrityReport `json:"data"`
	}
	if err := a.client.do(ctx, requestDescriptor{
		Method: http.MethodGet,
		Path:   a.basePath(org) + "/integrity-report",
	}, &resp, opts...); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
