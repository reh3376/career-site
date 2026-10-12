package auditpolicy

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Entry is one row to append. The interceptor builds it; the Writer
// persists it. Keeping the two apart means this package never imports
// the repository, and the repository never learns about Connect.
type Entry struct {
	ActorUserID *int64
	Action      string
	TargetType  string
	TargetID    string
	Payload     map[string]any
	IPHash      []byte
}

// Writer persists an audit entry. internal/users.Repo satisfies this
// through a small adapter in main.
type Writer interface {
	WriteAudit(ctx context.Context, e Entry) error
}

// ActorFunc resolves the acting admin from the request context. The auth
// interceptor has already put the caller there by the time this runs;
// this is a function rather than a direct call so that auditpolicy does
// not import internal/handlers, which would be a cycle.
//
// Returning ok=false records the action with a null actor rather than
// dropping it. An admin action whose actor we failed to resolve is still
// an admin action, and a missing row would be the worse outcome.
type ActorFunc func(ctx context.Context) (userID int64, ok bool)

// IPHashFunc hashes the client address with whatever salt and truncation
// the rest of the service uses, so audit_log.ip_hash is comparable with
// events.ip_hash rather than being a second incompatible scheme.
type IPHashFunc func(addr string) []byte

// Interceptor writes one audit_log row per call to a procedure declared
// AUDIT_LEVEL_RECORDED.
//
// Three choices worth stating, because each is a trade and not an
// oversight:
//
//  1. **The row is written after the handler returns**, carrying the
//     outcome. An attempt that was refused is part of an audit trail;
//     a row written beforehand would assert a change that may not have
//     happened.
//  2. **A failed write does not fail the request.** The action has
//     already happened by then, so returning an error would tell the
//     admin their change did not land when it did, and invite a retry
//     that does it twice. The cost is that a database problem loses
//     rows, so the failure is logged at ERROR naming the action.
//  3. **The row is written on the request's own context.** A cancelled
//     request therefore may not record, which is the lesser evil
//     against holding a background goroutine per admin call. Admin
//     mutations are short and the console does not cancel them.
type Interceptor struct {
	policy Map
	writer Writer
	actor  ActorFunc
	hashIP IPHashFunc
	log    *slog.Logger
}

func NewInterceptor(policy Map, w Writer, actor ActorFunc, hashIP IPHashFunc, log *slog.Logger) *Interceptor {
	return &Interceptor{policy: policy, writer: w, actor: actor, hashIP: hashIP, log: log}
}

var _ connect.Interceptor = (*Interceptor)(nil)

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if !i.policy.Recorded(req.Spec().Procedure) {
			return resp, err
		}
		i.record(ctx, req, err)
		return resp, err
	}
}

// Streaming handlers are not recorded. No procedure declared
// AUDIT_LEVEL_RECORDED is streaming today, and a stream has no single
// request message to take a target or a reason from, so recording one
// would mean a row that says only "a stream happened". If an admin
// mutation is ever added as a stream, this is where it goes, and the
// test in policy_test.go that asserts every RECORDED procedure is unary
// is what will fail first.
func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *Interceptor) record(ctx context.Context, req connect.AnyRequest, callErr error) {
	e := Entry{
		Action:  req.Spec().Procedure,
		Payload: map[string]any{"outcome": outcomeOf(callErr)},
	}
	if id, ok := i.actor(ctx); ok {
		e.ActorUserID = &id
	}
	if i.hashIP != nil {
		e.IPHash = i.hashIP(clientAddr(req))
	}

	if msg, ok := req.Any().(proto.Message); ok && msg != nil {
		t, id := targetOf(msg)
		e.TargetType, e.TargetID = t, id
		for k, v := range extrasOf(req.Spec().Procedure, msg) {
			e.Payload[k] = v
		}
	}

	if err := i.writer.WriteAudit(ctx, e); err != nil {
		// Loud, and naming the action, because this is the one failure
		// mode that silently reproduces the state this package exists
		// to end: an empty audit_log that everything claims is full.
		i.log.Error("audit row lost",
			slog.String("action", e.Action),
			slog.String("error", err.Error()))
	}
}

func outcomeOf(err error) string {
	if err == nil {
		return "ok"
	}
	return connect.CodeOf(err).String()
}

func clientAddr(req connect.AnyRequest) string {
	if xff := req.Header().Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := req.Header().Get("X-Real-IP"); xri != "" {
		return xri
	}
	return req.Peer().Addr
}

// targetFields are the request field names that name what an action
// acted on, in priority order, each mapped to the target_type recorded
// alongside it.
//
// This is a fixed, reviewable list rather than a guess at runtime. The
// test TestEveryRecordedProcedureResolvesATarget walks every procedure
// declared RECORDED and fails if it finds neither one of these fields
// nor a place on the explicit no-target list, so a new admin RPC with an
// unfamiliar id field cannot quietly record rows with an empty target.
var targetFields = []struct {
	field      string
	targetType string
}{
	{"member_id", "member"},
	{"user_id", "member"},
	{"entry_id", "qa_entry"},
	{"submission_id", "jd_submission"},
	{"posting_id", "golden_posting"},
	{"message_id", "contact_message"},
	{"grant_id", "access_grant"},
	{"query_id", "saved_query"},
	{"meeting_id", "meeting"},
	{"run_id", "run"},
	{"job_id", "job"},
	{"review_item_id", "review_item"},
	{"escalation_id", "escalation"},
	{"session_key", "decision_test_session"},
	{"item_id", "review_item"},
	{"decision_id", "decision"},
	{"phrasing_id", "qa_phrasing"},
	{"notification_id", "notification"},
	{"document_id", "corpus_document"},
	{"id", "id"},
}

func targetOf(msg proto.Message) (targetType, targetID string) {
	fields := msg.ProtoReflect().Descriptor().Fields()
	for _, cand := range targetFields {
		fd := fields.ByName(protoreflect.Name(cand.field))
		if fd == nil {
			continue
		}
		v := msg.ProtoReflect().Get(fd).String()
		if v == "" {
			continue
		}
		return cand.targetType, v
	}
	return "", ""
}

// extras are the per-procedure request fields worth keeping beyond the
// reason, named one by one.
//
// Copying the whole request would be easier and wrong: ConnectCalendar
// carries an OAuth code, and an audit log is the last place a credential
// should come to rest. Nothing is recorded here that is not listed.
var extras = map[string][]string{
	"/career.v1.AdminService/SetMemberStatus":       {"status"},
	"/career.v1.AdminService/ExtendAccess":          {"extend_days", "permanent"},
	"/career.v1.AdminService/RunDbQuery":            {"sql"},
	"/career.v1.AdminService/SetJdOutcome":          {"status"},
	"/career.v1.AdminService/RunJob":                {"kind", "source_kind"},
	"/career.v1.AdminService/CreateQaEntry":         {"question"},
	"/career.v1.AdminService/ReviewDecisionTestRun": {"status", "block_no"},
	"/career.v1.AdminService/ResolveReviewItem":     {"resolution"},
}

const maxExtraLen = 2000

func extrasOf(procedure string, msg proto.Message) map[string]any {
	out := map[string]any{}
	refl := msg.ProtoReflect()
	fields := refl.Descriptor().Fields()

	// `reason` is recorded for every procedure that has one. Four RPCs
	// have carried this field with a comment saying it is "recorded in
	// the audit log" since before there was an audit log to record it
	// in; this is the line that makes the comment true.
	if fd := fields.ByName("reason"); fd != nil {
		if v := strings.TrimSpace(refl.Get(fd).String()); v != "" {
			out["reason"] = truncate(v)
		}
	}
	for _, name := range extras[procedure] {
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil {
			continue
		}
		v := refl.Get(fd)
		switch fd.Kind() {
		case protoreflect.StringKind:
			if s := v.String(); s != "" {
				out[name] = truncate(s)
			}
		case protoreflect.BoolKind:
			out[name] = v.Bool()
		case protoreflect.EnumKind:
			out[name] = string(fd.Enum().Values().ByNumber(v.Enum()).Name())
		default:
			out[name] = fmt.Sprint(v.Interface())
		}
	}
	return out
}

func truncate(s string) string {
	if len(s) <= maxExtraLen {
		return s
	}
	return s[:maxExtraLen] + "…(truncated)"
}
