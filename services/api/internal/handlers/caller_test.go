package handlers

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/reh3376/career-site/services/api/internal/users"
)

func TestCallerRoundTripsThroughTheContext(t *testing.T) {
	want := &users.User{ID: 7, Email: "someone@example.com", Role: users.RoleAdmin}
	ctx := WithCaller(context.Background(), want)

	got := CallerFromContext(ctx)
	if got != want {
		t.Fatalf("CallerFromContext returned %v, want the same pointer back", got)
	}
	if u, err := RequireCaller(ctx); err != nil || u != want {
		t.Fatalf("RequireCaller = (%v, %v), want the caller and no error", u, err)
	}
}

// An empty context is what a PUBLIC handler sees, and it must not be an
// error there: nothing is resolved for a public procedure, so nil means
// "anonymous", which is a normal state.
func TestCallerFromContextIsNilWhenNothingWasResolved(t *testing.T) {
	if got := CallerFromContext(context.Background()); got != nil {
		t.Errorf("CallerFromContext on a bare context returned %v, want nil", got)
	}
}

// RequireCaller is the invariant check for a MEMBER or ADMIN handler.
// Unreachable in enforce mode, because the interceptor refused everyone
// without a session, but it must fail rather than dereference nil: in
// observe mode the interceptor acts on nothing, so a request it would
// have refused still arrives here.
func TestRequireCallerFailsRatherThanPanicking(t *testing.T) {
	u, err := RequireCaller(context.Background())
	if err == nil {
		t.Fatalf("RequireCaller accepted an empty context and returned %v", u)
	}
	if u != nil {
		t.Error("RequireCaller returned both an error and a caller")
	}
	// `internal`, not `unauthenticated`: the request already passed the
	// policy, so no caller here means the server is misconfigured, not
	// that the caller should sign in.
	if got := connect.CodeOf(err); got != connect.CodeInternal {
		t.Errorf("code = %s, want internal. A missing caller at this point "+
			"is a server fault, not a sign-in prompt.", got)
	}
	if !strings.Contains(err.Error(), "no resolved caller") {
		t.Errorf("error %q does not say what is missing", err)
	}
}

// Attaching nil must not create an entry that later reads back as a
// non-nil interface holding a nil pointer, which is the classic way this
// pattern turns into a panic three layers away.
func TestWithCallerIgnoresNil(t *testing.T) {
	ctx := WithCaller(context.Background(), nil)
	if got := CallerFromContext(ctx); got != nil {
		t.Errorf("CallerFromContext returned %v after attaching nil", got)
	}
	if _, err := RequireCaller(ctx); err == nil {
		t.Error("RequireCaller accepted a context built from a nil caller")
	}
}
