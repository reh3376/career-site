package authpolicy

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// One Connect interceptor reads the declared auth level and decides,
// in place of the ~90 hand-written gate calls the handlers used to
// carry. S3, S4 and S5 of docs/sprint-auth-interceptor.md.
//
// **It always enforces, and there is no longer a mode.** It arrived in
// S3 with an observe mode that computed the decision and acted on
// nothing, so its answers could be compared against the handlers' own
// gates on real traffic before anything changed. S4 turned enforcement
// on after 1,205 comparisons produced no disagreement.
//
// S5 then removed the handler gates, and that is what removed the mode
// with them. Observe mode compared the interceptor against those gates;
// with the gates gone it compares nothing. Worse, 48 of the converted
// handlers only ever used their gate to refuse, so they now contain no
// check at all, and an interceptor that observes without acting would
// leave them open. A mode whose only remaining effect is to disable the
// sole enforcement is not a mode, it is a hole, so AUTH_INTERCEPTOR_MODE
// is gone and the observation machinery with it.

// Caller is everything the interceptor needs to know about who is
// calling. Deliberately three booleans rather than a *users.User: this
// package should not know what a user record looks like, and the three
// questions below are the whole of what the auth level turns on.
type Caller struct {
	// Authenticated is true when the request carried a session that
	// resolved to an account.
	Authenticated bool
	// Active is true when that account's status is active.
	//
	// Login already refuses every non-active status, so this only
	// differs from Authenticated when the account was suspended,
	// declined or expired *after* the session was minted. That is the
	// case `requireMember` catches today and `requireAdmin` does not.
	Active bool
	// Admin is true when the account holds the admin role.
	Admin bool
}

// ResolveFunc resolves a request's session from its headers.
//
// Headers rather than a connect.AnyRequest because the streaming path
// has no request object, only a connection, and both expose
// http.Header. One signature means one resolution path and no chance of
// the two disagreeing.
//
// **It returns a context, which is how S5 works.** The resolver has
// already loaded the account in order to answer the three questions in
// Caller, and the handler is about to need the same record. Returning a
// context lets the resolver attach it, so the interceptor can hand it
// down to the handler and the handler does not repeat the lookup. This
// package stays free of the users package: it passes the context along
// without knowing what was put in it.
//
// Implementations return the input context unchanged when there is no
// session, and must never return a nil context.
type ResolveFunc func(ctx context.Context, header http.Header) (Caller, context.Context, error)

type Interceptor struct {
	policy  Map
	resolve ResolveFunc
	log     *slog.Logger
}

// NewInterceptor fails rather than returning something that would allow
// everything or deny everything by accident.
//
// An empty policy denies every MEMBER and ADMIN procedure, and a nil
// resolver cannot authenticate anybody. Both are programming errors, and
// both would present as a total outage with no obvious cause, so they are
// refused here where the message can say which one it was.
func NewInterceptor(policy Map, resolve ResolveFunc, log *slog.Logger) (*Interceptor, error) {
	if len(policy) == 0 {
		return nil, errors.New("auth interceptor needs a non-empty policy map")
	}
	if resolve == nil {
		return nil, errors.New("auth interceptor needs a session resolver")
	}
	if log == nil {
		return nil, errors.New("auth interceptor needs a logger")
	}
	return &Interceptor{policy: policy, resolve: resolve, log: log}, nil
}

// decision is what the interceptor concluded, kept whole so a denial
// can report its reason and not just its verdict.
type decision struct {
	level v1.AuthLevel
	// known is false when the procedure is not in the policy map.
	known bool
	allow bool
	code  connect.Code
	why   string
}

// decide answers whether this caller may call this procedure.
//
// The level definitions settled by the owner on 2026-10-09, after S1
// found the existing gates disagreeing with each other:
//
//	PUBLIC  no session
//	MEMBER  a session, on an active account
//	ADMIN   a session, on an active account, holding the admin role
//
// Requiring an active account for both MEMBER and ADMIN is stricter
// than 72 of the 105 gated handlers are today, and strictness is the
// safe direction: it only ever closes access. The alternative, MEMBER
// meaning "any session", would leave S5 quietly removing the status
// check from 16 methods at the step least likely to be reviewed.
//
// **`allow_unverified` is the one exception, and it is enforced here
// rather than waived.** A MEMBER method declaring it needs a session but
// not an active account. Two methods declare it, GetMe and
// DeleteAccount, and without it the strict rule above would break a real
// path: ResetPassword mints a session and refuses only DISABLED and
// DECLINED, so a member who verified into PENDING_APPROVAL and then
// reset their password holds a session on a non-active account. GetMe
// has to answer them, because that is how the "awaiting approval" state
// is shown, and its handler answers them today via a bare lookup with no
// status check. Honouring the option makes the interceptor match that
// handler exactly instead of closing a working flow.
//
// The role check still applies, so this cannot widen ADMIN; BuildFrom
// refuses the option on any level but MEMBER.
// decide answers whether this caller may call this procedure, and
// returns the context the handler should run with: the resolver's
// enriched one where a session was resolved, the original otherwise.
func (i *Interceptor) decide(ctx context.Context, procedure string, header http.Header) (decision, context.Context) {
	policy, ok := i.policy.Lookup(procedure)
	if !ok {
		// D2: a procedure with no policy is denied. Build() makes this
		// unreachable for anything in career.v1, so reaching it means
		// the map and the routes disagree, which is a bug and not a
		// reason to serve the method.
		return decision{
			known: false,
			allow: false,
			code:  connect.CodePermissionDenied,
			why:   "no auth policy for this procedure",
		}, ctx
	}

	level := policy.Level
	d := decision{level: level, known: true}

	if level == v1.AuthLevel_AUTH_LEVEL_PUBLIC {
		// No session is resolved for a public procedure. That is not
		// only an optimisation: the decision test is public and is the
		// busiest surface on the site, and resolving a session it does
		// not have would add a database round trip to every answer.
		d.allow = true
		return d, ctx
	}

	caller, next, err := i.resolve(ctx, header)
	if next != nil {
		ctx = next
	}
	switch {
	case err != nil:
		// Matches what every handler returns today when the lookup
		// itself fails, which is distinct from "no session".
		d.code, d.why = connect.CodeInternal, "session lookup failed"
	case !caller.Authenticated:
		d.code, d.why = connect.CodeUnauthenticated, "not signed in"
	case !caller.Active && !policy.AllowUnverified:
		d.code, d.why = connect.CodePermissionDenied, "account is not active"
	case level == v1.AuthLevel_AUTH_LEVEL_ADMIN && !caller.Admin:
		d.code, d.why = connect.CodePermissionDenied, "admin role required"
	default:
		d.allow = true
	}
	return d, ctx
}

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := req.Spec().Procedure
		d, ctx := i.decide(ctx, procedure, req.Header())
		if !d.allow {
			i.logIfUnknown(procedure, d)
			return nil, connect.NewError(d.code, errors.New(d.why))
		}
		return next(ctx, req)
	}
}

// WrapStreamingHandler covers Ask Roger, which streams its answer. Same
// decision, same comparison; the only difference is where the headers
// come from.
func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		procedure := conn.Spec().Procedure
		d, ctx := i.decide(ctx, procedure, conn.RequestHeader())
		if !d.allow {
			i.logIfUnknown(procedure, d)
			return connect.NewError(d.code, errors.New(d.why))
		}
		return next(ctx, conn)
	}
}

// WrapStreamingClient is required by connect.Interceptor and does
// nothing: this process is the server. Outbound calls to the sidecar do
// not go through Connect.
func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// logIfUnknown reports a procedure that is served but carries no
// policy. It cannot happen while Build validates the map against the
// descriptors, and if it ever does, every call to that procedure is
// denied and the reason should not have to be inferred from a wall of
// permission_denied.
func (i *Interceptor) logIfUnknown(procedure string, d decision) {
	if d.known {
		return
	}
	i.log.Error("auth policy: procedure is served but has no policy",
		slog.String("procedure", procedure))
}
