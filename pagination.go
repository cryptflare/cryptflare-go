package cryptflare

import (
	"context"
)

// Page is the cursor-based pagination wrapper returned by every list
// method. Iterate via `Each` (callback) or paginate manually with
// `Next`. `Data` is always populated for the current page.
type Page[T any] struct {
	// Data is the records on this page.
	Data []T
	// NextCursor is the opaque server-issued cursor for the next page,
	// or "" when this was the last page. Treat as opaque - never
	// inspect or modify.
	NextCursor string
	// HasMore mirrors the API's `hasMore` field. True when another
	// page exists.
	HasMore bool

	fetcher func(ctx context.Context, cursor string) (*Page[T], error)
}

// Next fetches the next page or returns nil when no more pages exist.
//
//	page, err := client.Secrets.List(ctx, input).First(ctx)
//	for page != nil {
//	    for _, s := range page.Data {
//	        fmt.Println(s.Key)
//	    }
//	    page, err = page.Next(ctx)
//	    if err != nil { return err }
//	}
func (p *Page[T]) Next(ctx context.Context) (*Page[T], error) {
	if !p.HasMore || p.NextCursor == "" || p.fetcher == nil {
		return nil, nil
	}
	return p.fetcher(ctx, p.NextCursor)
}

// Each iterates every record across every page. Stop early by
// returning `cryptflare.ErrStopIteration`.
//
//	err := client.Secrets.List(ctx, input).Each(func(s *Secret) error {
//	    fmt.Println(s.Key)
//	    return nil
//	})
func (p *Page[T]) Each(ctx context.Context, fn func(item T) error) error {
	current := p
	for current != nil {
		for _, item := range current.Data {
			if err := fn(item); err != nil {
				if err == ErrStopIteration {
					return nil
				}
				return err
			}
		}
		next, err := current.Next(ctx)
		if err != nil {
			return err
		}
		current = next
	}
	return nil
}

// PageRequest is a builder returned by list methods. Calling `First`
// fetches page 1; calling `Each` iterates everything.
type PageRequest[T any] struct {
	fetcher func(ctx context.Context, cursor string) (*Page[T], error)
}

// First materialises the first page.
func (r *PageRequest[T]) First(ctx context.Context) (*Page[T], error) {
	return r.fetcher(ctx, "")
}

// Each iterates every record across every page. Sugar over
// `(*PageRequest).First()` followed by `(*Page).Each(...)`.
func (r *PageRequest[T]) Each(ctx context.Context, fn func(item T) error) error {
	first, err := r.First(ctx)
	if err != nil {
		return err
	}
	return first.Each(ctx, fn)
}

// ErrStopIteration tells `Each` to stop walking pages without surfacing
// an error to the caller.
var ErrStopIteration = &iterationStopError{}

type iterationStopError struct{}

func (*iterationStopError) Error() string { return "cryptflare: stop iteration" }
