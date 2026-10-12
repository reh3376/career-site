package auditpolicy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
)

type captureWriter struct {
	entries []Entry
	err     error
}

func (c *captureWriter) WriteAudit(_ context.Context, e Entry) error {
	c.entries = append(c.entries, e)
	return c.err
}

const (
	setStatus  = "/career.v1.AdminService/SetMemberStatus"
	listMember = "/career.v1.AdminService/ListMembers"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// The interceptor is exercised through a real Connect round trip rather
// than by calling WrapUnary with a hand-built request. connect.AnyRequest
// cannot be implemented outside its package, and more to the point, the
// procedure string is the map key this whole package turns on: a test
// that supplied it by hand would be asserting against its own typing
// rather than against what Connect actually names the method.
type stubAdmin struct {
	careerv1connect.UnimplementedAdminServiceHandler
	err error
}

func (s *stubAdmin) SetMemberStatus(context.Context, *connect.Request[v1.SetMemberStatusRequest]) (*connect.Response[v1.SetMemberStatusResponse], error) {
	if s.err != nil {
		return nil, s.err
	}
	return connect.NewResponse(&v1.SetMemberStatusResponse{}), nil
}

func (s *stubAdmin) ListMembers(context.Context, *connect.Request[v1.ListMembersRequest]) (*connect.Response[v1.ListMembersResponse], error) {
	return connect.NewResponse(&v1.ListMembersResponse{}), nil
}

// newStack mounts AdminService behind the audit interceptor and returns
// a client plus the writer rows land in.
func newStack(t *testing.T, actorID int64, haveActor bool, handlerErr, writeErr error) (careerv1connect.AdminServiceClient, *captureWriter) {
	t.Helper()
	policy, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	w := &captureWriter{err: writeErr}
	intc := NewInterceptor(policy, w,
		func(context.Context) (int64, bool) { return actorID, haveActor },
		func(addr string) []byte { return []byte("hash:" + addr) },
		quietLog())

	mux := http.NewServeMux()
	path, h := careerv1connect.NewAdminServiceHandler(&stubAdmin{err: handlerErr}, connect.WithInterceptors(intc))
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return careerv1connect.NewAdminServiceClient(srv.Client(), srv.URL), w
}

func TestARecordedCallWritesOneRow(t *testing.T) {
	c, w := newStack(t, 7, true, nil, nil)
	_, err := c.SetMemberStatus(context.Background(), connect.NewRequest(&v1.SetMemberStatusRequest{
		MemberId: "42",
		Status:   v1.MemberStatus_MEMBER_STATUS_DISABLED,
		Reason:   "repeated abuse reports",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.entries) != 1 {
		t.Fatalf("wrote %d rows, want 1", len(w.entries))
	}
	e := w.entries[0]
	if e.Action != setStatus {
		t.Errorf("action = %q, want %q", e.Action, setStatus)
	}
	if e.ActorUserID == nil || *e.ActorUserID != 7 {
		t.Errorf("actor = %v, want 7", e.ActorUserID)
	}
	if e.TargetType != "member" || e.TargetID != "42" {
		t.Errorf("target = %s/%s, want member/42", e.TargetType, e.TargetID)
	}
	if e.Payload["reason"] != "repeated abuse reports" {
		t.Errorf("reason not recorded: %v", e.Payload["reason"])
	}
	if e.Payload["outcome"] != "ok" {
		t.Errorf("outcome = %v, want ok", e.Payload["outcome"])
	}
	if e.Payload["status"] != "MEMBER_STATUS_DISABLED" {
		t.Errorf("status not recorded: %v", e.Payload["status"])
	}
	if len(e.IPHash) == 0 {
		t.Error("no ip hash recorded")
	}
}

// A procedure declared NONE writes nothing. Without this the log fills
// with reads and the thing nobody can find is the mutation.
func TestANonRecordedCallWritesNothing(t *testing.T) {
	c, w := newStack(t, 7, true, nil, nil)
	if _, err := c.ListMembers(context.Background(), connect.NewRequest(&v1.ListMembersRequest{})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.entries) != 0 {
		t.Fatalf("a NONE procedure wrote %d rows", len(w.entries))
	}
}

// A refused attempt is part of the trail. An audit log holding only
// successes cannot answer "what did they try".
func TestAFailedCallIsStillRecordedWithItsCode(t *testing.T) {
	boom := connect.NewError(connect.CodePermissionDenied, errors.New("nope"))
	c, w := newStack(t, 7, true, boom, nil)
	_, err := c.SetMemberStatus(context.Background(), connect.NewRequest(&v1.SetMemberStatusRequest{MemberId: "42"}))
	if err == nil {
		t.Fatal("the handler error was swallowed")
	}
	if len(w.entries) != 1 {
		t.Fatalf("a failed call wrote %d rows, want 1", len(w.entries))
	}
	if got := w.entries[0].Payload["outcome"]; got != "permission_denied" {
		t.Errorf("outcome = %v, want permission_denied", got)
	}
}

// Losing the audit row must not lose the admin's change. The mutation
// has already happened by the time this runs.
func TestAFailedWriteDoesNotFailTheRequest(t *testing.T) {
	c, _ := newStack(t, 7, true, nil, errors.New("connection refused"))
	if _, err := c.SetMemberStatus(context.Background(),
		connect.NewRequest(&v1.SetMemberStatusRequest{MemberId: "42"})); err != nil {
		t.Fatalf("a failed audit write failed the request: %v", err)
	}
}

// An unresolved actor records a null actor rather than dropping the row.
func TestAnUnresolvedActorStillRecordsTheAction(t *testing.T) {
	c, w := newStack(t, 0, false, nil, nil)
	if _, err := c.SetMemberStatus(context.Background(),
		connect.NewRequest(&v1.SetMemberStatusRequest{MemberId: "42"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.entries) != 1 {
		t.Fatalf("wrote %d rows, want 1", len(w.entries))
	}
	if w.entries[0].ActorUserID != nil {
		t.Errorf("actor = %v, want nil", w.entries[0].ActorUserID)
	}
}

// Nothing outside the named extras is copied. ConnectCalendar carries an
// OAuth code, and an audit log is the last place a credential should
// come to rest.
func TestOnlyNamedFieldsAreCopiedIntoThePayload(t *testing.T) {
	c, w := newStack(t, 7, true, nil, nil)
	if _, err := c.SetMemberStatus(context.Background(), connect.NewRequest(&v1.SetMemberStatusRequest{
		MemberId: "42",
		Status:   v1.MemberStatus_MEMBER_STATUS_ACTIVE,
		Reason:   "reinstated",
	})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	allowed := map[string]bool{"outcome": true, "reason": true, "status": true}
	for k := range w.entries[0].Payload {
		if !allowed[k] {
			t.Errorf("payload carries an unnamed field %q", k)
		}
	}
}

func TestAnEmptyReasonIsOmittedRatherThanStored(t *testing.T) {
	c, w := newStack(t, 7, true, nil, nil)
	if _, err := c.SetMemberStatus(context.Background(),
		connect.NewRequest(&v1.SetMemberStatusRequest{MemberId: "42"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := w.entries[0].Payload["reason"]; ok {
		t.Error("an empty reason was stored; absent and blank should look the same")
	}
}

func TestALongValueIsTruncated(t *testing.T) {
	long := make([]byte, maxExtraLen+500)
	for i := range long {
		long[i] = 'x'
	}
	c, w := newStack(t, 7, true, nil, nil)
	if _, err := c.SetMemberStatus(context.Background(), connect.NewRequest(
		&v1.SetMemberStatusRequest{MemberId: "42", Reason: string(long)})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := w.entries[0].Payload["reason"].(string)
	if len(got) > maxExtraLen+len("…(truncated)") {
		t.Errorf("reason not truncated: %d chars", len(got))
	}
}
