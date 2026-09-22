package tenant_test

import (
	"context"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// The seam's whole contract: a writer that asks gets the owner's site
// today, and gets whatever the middleware set once there is one.
func TestFromContext(t *testing.T) {
	t.Parallel()

	t.Run("bare context is the owner's site", func(t *testing.T) {
		t.Parallel()
		if got := tenant.FromContext(context.Background()); got != tenant.Default {
			t.Fatalf("got %d, want %d", got, tenant.Default)
		}
	})

	t.Run("carries what was set", func(t *testing.T) {
		t.Parallel()
		ctx := tenant.WithID(context.Background(), 42)
		if got := tenant.FromContext(ctx); got != 42 {
			t.Fatalf("got %d, want 42", got)
		}
	})

	// A zero or negative id means "nobody set one", not "tenant zero".
	// Writing rows under an id no tenants row has would be worse than
	// writing them under the default, because the foreign key would
	// reject them and the write would fail at the point of no return.
	for _, id := range []tenant.ID{0, -1} {
		t.Run("rejects a meaningless id", func(t *testing.T) {
			t.Parallel()
			ctx := tenant.WithID(context.Background(), id)
			if got := tenant.FromContext(ctx); got != tenant.Default {
				t.Fatalf("id %d: got %d, want the default %d", id, got, tenant.Default)
			}
		})
	}

	t.Run("survives a derived context", func(t *testing.T) {
		t.Parallel()
		ctx := tenant.WithID(context.Background(), 7)
		// Background writes use WithoutCancel; the tenant must ride along.
		if got := tenant.FromContext(context.WithoutCancel(ctx)); got != 7 {
			t.Fatalf("got %d, want 7", got)
		}
	})
}
