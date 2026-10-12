package handlers

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// The caller the auth interceptor already resolved.
//
// S5 of docs/sprint-auth-interceptor.md. Until now every gated handler
// resolved the session itself: 69 `requireAdmin` calls, three separate
// `requireMember` definitions and three bare lookups, each one loading
// the account from the database and deciding. Since S4 the interceptor
// decides before the handler runs, so those decisions are unreachable,
// and the lookups are the second of two.
//
// D3 of the plan is why this exists rather than the handlers simply
// losing their gates: `requireAdmin` did two jobs, deciding and
// returning the `*users.User` the handler then used. Only the deciding
// moved to the interceptor. The resolver attaches the record it already
// loaded, and handlers read it from here.

type callerKey struct{}

// WithCaller attaches a resolved account to a request context. Called
// by the session resolver in main, which is the one place that knows how
// to turn a cookie into a user.
func WithCaller(ctx context.Context, u *users.User) context.Context {
	if u == nil {
		return ctx
	}
	return context.WithValue(ctx, callerKey{}, u)
}

// CallerFromContext returns the resolved account, or nil when the
// request carried no session. PUBLIC procedures resolve nothing, so
// every public handler sees nil here, which is correct: an anonymous
// caller has no account.
//
// Handlers that *require* a caller should use RequireCaller instead, so
// an empty context fails rather than dereferencing nil.
func CallerFromContext(ctx context.Context) *users.User {
	u, _ := ctx.Value(callerKey{}).(*users.User)
	return u
}

// RequireCaller returns the account the interceptor resolved, and an
// error if there is none.
//
// **This is an invariant check, not an authorisation decision.** For a
// MEMBER or ADMIN procedure the interceptor has already refused every
// caller without a session, so by the time a handler runs the account is
// present; the error path is unreachable in enforce mode. It exists
// because unreachable is not the same as impossible:
//
//   - in observe mode the interceptor decides and acts on nothing, so a
//     request it would have refused still reaches the handler with an
//     empty context. Returning `internal` beats a nil dereference.
//   - a service mounted without the interceptor would resolve nothing.
//     That is the hole TestTheInterceptorIsAttachedToEveryMountedService
//     guards, and this is the second line of defence behind it.
//
// The code is `internal` rather than `unauthenticated` deliberately. By
// this point the request has passed the policy, so no session here means
// the server is misconfigured, not that the caller did anything wrong,
// and it should read as a fault rather than as a sign-in prompt.
func RequireCaller(ctx context.Context) (*users.User, error) {
	u := CallerFromContext(ctx)
	if u == nil {
		return nil, connect.NewError(connect.CodeInternal,
			errors.New("no resolved caller on the request"))
	}
	return u, nil
}
