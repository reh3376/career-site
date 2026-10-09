package authpolicy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// S3 and S4 of docs/sprint-auth-interceptor.md: one Connect interceptor
// that reads the declared level and decides, replacing ~90 hand-written
// gate calls.
//
// It arrives in **observe** mode, deciding and enforcing nothing, so its
// decision can be compared against what the handlers already do on real
// traffic before anything changes. S1 proved the two agree statically;
// this is the only way to prove it on the session edge cases a source
// parse cannot see. The mode is config, not code, so S4 is a flag flip
// and a redeploy rather than a change to this file.

// Mode decides whether the interceptor acts on its decision.
type Mode int

const (
	// ModeObserve computes the decision, lets the request through
	// regardless, and logs when the handler disagreed.
	ModeObserve Mode = iota
	// ModeEnforce denies before the handler runs.
	ModeEnforce
)

func (m Mode) String() string {
	if m == ModeEnforce {
		return "enforce"
	}
	return "observe"
}

// ParseMode reads the mode from configuration.
//
// Anything it does not recognise is an error rather than a default,
// including the empty string, so the caller decides what "unset" means.
// A typo in AUTH_INTERCEPTOR_MODE silently selecting enforce would deny
// the admin console; silently selecting observe would leave enforcement
// off while the deploy that was supposed to turn it on reported success.
// Neither is acceptable as a guess.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "observe":
		return ModeObserve, nil
	case "enforce":
		return ModeEnforce, nil
	default:
		return ModeObserve, fmt.Errorf(
			"unknown auth interceptor mode %q, expected \"observe\" or \"enforce\"", s)
	}
}

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
type ResolveFunc func(ctx context.Context, header http.Header) (Caller, error)

type Interceptor struct {
	policy  Map
	resolve ResolveFunc
	mode    Mode
	log     *slog.Logger
}

// NewInterceptor fails rather than returning something that would allow
// everything or deny everything by accident.
//
// An empty policy denies every MEMBER and ADMIN procedure once
// enforcing, and a nil resolver cannot authenticate anybody. Both are
// programming errors, and both would present as a total outage with no
// obvious cause, so they are refused here where the message can say
// which one it was.
func NewInterceptor(policy Map, resolve ResolveFunc, mode Mode, log *slog.Logger) (*Interceptor, error) {
	if len(policy) == 0 {
		return nil, errors.New("auth interceptor needs a non-empty policy map")
	}
	if resolve == nil {
		return nil, errors.New("auth interceptor needs a session resolver")
	}
	if log == nil {
		return nil, errors.New("auth interceptor needs a logger")
	}
	return &Interceptor{policy: policy, resolve: resolve, mode: mode, log: log}, nil
}

func (i *Interceptor) Mode() Mode { return i.mode }

// decision is what the interceptor concluded, kept whole so observe
// mode can report the reasoning and not just the verdict.
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
func (i *Interceptor) decide(ctx context.Context, procedure string, header http.Header) decision {
	level, ok := i.policy.Level(procedure)
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
		}
	}

	d := decision{level: level, known: true}

	if level == v1.AuthLevel_AUTH_LEVEL_PUBLIC {
		// No session is resolved for a public procedure. That is not
		// only an optimisation: the decision test is public and is the
		// busiest surface on the site, and resolving a session it does
		// not have would add a database round trip to every answer.
		d.allow = true
		return d
	}

	caller, err := i.resolve(ctx, header)
	switch {
	case err != nil:
		// Matches what every handler returns today when the lookup
		// itself fails, which is distinct from "no session".
		d.code, d.why = connect.CodeInternal, "session lookup failed"
	case !caller.Authenticated:
		d.code, d.why = connect.CodeUnauthenticated, "not signed in"
	case !caller.Active:
		d.code, d.why = connect.CodePermissionDenied, "account is not active"
	case level == v1.AuthLevel_AUTH_LEVEL_ADMIN && !caller.Admin:
		d.code, d.why = connect.CodePermissionDenied, "admin role required"
	default:
		d.allow = true
	}
	return d
}

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := req.Spec().Procedure
		d := i.decide(ctx, procedure, req.Header())

		if i.mode == ModeEnforce && !d.allow {
			return nil, connect.NewError(d.code, errors.New(d.why))
		}

		resp, err := next(ctx, req)
		if i.mode == ModeObserve {
			i.observe(procedure, d, err)
		}
		return resp, err
	}
}

// WrapStreamingHandler covers Ask Roger, which streams its answer. Same
// decision, same comparison; the only difference is where the headers
// come from.
func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		procedure := conn.Spec().Procedure
		d := i.decide(ctx, procedure, conn.RequestHeader())

		if i.mode == ModeEnforce && !d.allow {
			return connect.NewError(d.code, errors.New(d.why))
		}

		err := next(ctx, conn)
		if i.mode == ModeObserve {
			i.observe(procedure, d, err)
		}
		return err
	}
}

// WrapStreamingClient is required by connect.Interceptor and does
// nothing: this process is the server. Outbound calls to the sidecar do
// not go through Connect.
func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// observe compares the interceptor's decision with what the handler
// actually did, and logs only where they disagree.
//
// **The signal has to stay clean or it will be ignored.** A handler
// returning an error is not the same as a handler denying access: it
// may be validating input, or reporting that the database is down. And
// on a PUBLIC procedure a denial is never about the auth level, because
// the interceptor always allows those; `Login` rejecting a declined
// account with `permission_denied` is the handler's own business rule,
// and logging it as a policy disagreement would put a warning in the
// log on every failed sign-in. So:
//
//   - PUBLIC procedures are not compared at all. S1 already proved
//     statically that no PUBLIC handler gates on a session.
//   - `would_close` means the handler served a request the interceptor
//     would have refused. This is a real behaviour change at S4 and the
//     most important thing this mode can find.
//   - `would_open` means the handler demanded a session the interceptor
//     did not. This is the dangerous direction and should never appear.
//   - `handler_denied_permission` is ambiguous: the interceptor allowed
//     and the handler refused with `permission_denied`, which may be
//     authorisation the auth level does not describe ("not your
//     resource"). Logged at info so it can be triaged without drowning
//     the two findings above.
func (i *Interceptor) observe(procedure string, d decision, handlerErr error) {
	if !d.known {
		// Worth knowing regardless of mode: the routes are serving
		// something the policy map does not cover.
		i.log.Error("auth policy: procedure is served but has no policy",
			slog.String("procedure", procedure))
		return
	}
	if d.level == v1.AuthLevel_AUTH_LEVEL_PUBLIC {
		return
	}

	handlerDenied := false
	handlerCode := connect.Code(0)
	if handlerErr != nil {
		handlerCode = connect.CodeOf(handlerErr)
		handlerDenied = handlerCode == connect.CodeUnauthenticated ||
			handlerCode == connect.CodePermissionDenied
	}

	switch {
	case !d.allow && !handlerDenied:
		i.log.Warn("auth policy disagreement",
			slog.String("kind", "would_close"),
			slog.String("procedure", procedure),
			slog.String("level", d.level.String()),
			slog.String("interceptor", d.code.String()+": "+d.why),
			slog.String("handler", handlerOutcome(handlerErr, handlerCode)))

	case d.allow && handlerCode == connect.CodeUnauthenticated:
		i.log.Warn("auth policy disagreement",
			slog.String("kind", "would_open"),
			slog.String("procedure", procedure),
			slog.String("level", d.level.String()),
			slog.String("interceptor", "allowed"),
			slog.String("handler", "unauthenticated"))

	case d.allow && handlerCode == connect.CodePermissionDenied:
		i.log.Info("auth policy: handler refused a call the policy allows",
			slog.String("kind", "handler_denied_permission"),
			slog.String("procedure", procedure),
			slog.String("level", d.level.String()))
	}
}

func handlerOutcome(err error, code connect.Code) string {
	if err == nil {
		return "served the request"
	}
	return "failed with " + code.String()
}
