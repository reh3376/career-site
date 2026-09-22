// Package tenant carries which site a request belongs to.
//
// There is one tenant today and there may only ever be one. The point
// of this package is not multi-tenancy; it is that every row worth
// keeping records who it belongs to, and that the code already asks
// rather than assumes. When a second site appears, host resolution is
// added to FromContext and the middleware sets it. Nothing else moves.
//
// What this deliberately does not do: authorization. Knowing the
// tenant is not the same as enforcing it, and a seam that pretends
// otherwise is worse than none. Isolation, when it is needed, belongs
// in Postgres row-level security keyed on the same id, not in a
// forgotten `if` somewhere in a handler.
package tenant

import "context"

// ID identifies one site served by this stack.
type ID int64

// Default is the owner's own site, row 1 of the tenants table. Every
// row written before tenancy exists belongs to it.
const Default ID = 1

type contextKey struct{}

// WithID returns a context carrying the tenant. The middleware that
// resolves a tenant from the request host will call this; today
// nothing does, and FromContext falls back to Default.
func WithID(ctx context.Context, id ID) context.Context {
	if id <= 0 {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext returns the tenant this request belongs to, or Default.
//
// Callers persisting a row should use this rather than the constant,
// even now when the two are always equal. That is the whole seam: the
// day host resolution lands, every writer is already correct.
func FromContext(ctx context.Context) ID {
	if id, ok := ctx.Value(contextKey{}).(ID); ok && id > 0 {
		return id
	}
	return Default
}

// Int64 is the form the database driver wants.
func (id ID) Int64() int64 { return int64(id) }
