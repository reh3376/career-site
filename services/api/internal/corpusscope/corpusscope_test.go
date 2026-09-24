package corpusscope

import (
	"context"
	"testing"
)

// The default has to be the restrictive one. Every other test here is a
// detail; this is the property the package exists for.
func TestDefaultIsPublic(t *testing.T) {
	if got := FromContext(context.Background()); got != Public {
		t.Fatalf("an unmarked context must be Public, got %v", got)
	}
	if AllowsPrivate(context.Background()) {
		t.Fatal("an unmarked context must not allow private documents")
	}
}

// A nil context reaches this code only through a programming error, and
// the answer to a programming error is still "no".
func TestNilContextIsPublic(t *testing.T) {
	var ctx context.Context // deliberately nil: the point is the fallback
	if AllowsPrivate(ctx) {
		t.Fatal("a nil context must not allow private documents")
	}
}

func TestWidenAndNarrow(t *testing.T) {
	ctx := With(context.Background(), All)
	if !AllowsPrivate(ctx) {
		t.Fatal("With(All) must allow private documents")
	}
	// Narrowing again must stick, so a caller handing a context to less
	// trusted work can hand over less than it holds.
	if AllowsPrivate(With(ctx, Public)) {
		t.Fatal("narrowing back to Public must take effect")
	}
}

// The zero value of Scope is what an unset context yields, so it must
// be the restrictive one. If someone reorders the constants this fails
// rather than quietly opening the corpus.
func TestZeroValueIsPublic(t *testing.T) {
	var s Scope
	if s != Public {
		t.Fatal("the zero value of Scope must be Public")
	}
}
