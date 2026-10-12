package auditpolicy

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

func TestRealContractDeclaresEveryAdminMethod(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build against the real registry: %v", err)
	}
	if len(m) == 0 {
		t.Fatal("empty map")
	}
	var recorded int
	for _, lvl := range m {
		if lvl == v1.AuditLevel_AUDIT_LEVEL_RECORDED {
			recorded++
		}
	}
	// Not pinned to an exact number, which would fail on every new RPC
	// and teach whoever hits it to edit the expectation. Pinned to the
	// property that matters: the recorded set is not empty, which is
	// what "nothing writes audit_log" looked like.
	if recorded == 0 {
		t.Fatal("no procedure is declared AUDIT_LEVEL_RECORDED; audit_log would stay empty")
	}
	t.Logf("%s", m.Summary())
}

// The whole point of deriving "needs a declaration" from the auth level
// is that a new admin RPC cannot be added without a decision. This is
// that promise, run rather than read.
func TestAnAdminMethodWithNoAuditLevelRefusesToBuild(t *testing.T) {
	_, err := BuildFrom(syntheticRegistry(t, v1.AuthLevel_AUTH_LEVEL_ADMIN,
		v1.AuditLevel_AUDIT_LEVEL_NONE, v1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED))
	if err == nil {
		t.Fatal("an ADMIN method declaring no audit level was accepted")
	}
	if !strings.Contains(err.Error(), "Undeclared") {
		t.Fatalf("the error does not name the offending method: %v", err)
	}
	if !strings.Contains(err.Error(), "career.v1.audit") {
		t.Fatalf("the error does not name the option to add: %v", err)
	}
}

// A MEMBER or PUBLIC method saying nothing is fine and means NONE.
// Requiring 60-odd member RPCs to declare would be noise, and noise is
// how a required declaration becomes a copied one.
func TestANonAdminMethodMayDeclareNothing(t *testing.T) {
	for _, lvl := range []v1.AuthLevel{
		v1.AuthLevel_AUTH_LEVEL_PUBLIC,
		v1.AuthLevel_AUTH_LEVEL_MEMBER,
	} {
		m, err := BuildFrom(syntheticRegistry(t, lvl,
			v1.AuditLevel_AUDIT_LEVEL_NONE, v1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED))
		if err != nil {
			t.Fatalf("%v method with no audit level was rejected: %v", lvl, err)
		}
		if m.Recorded("/career.v1.SyntheticService/Undeclared") {
			t.Fatal("an undeclared method was treated as recorded")
		}
	}
}

func TestDeclaredRecordedIsReportedAsRecorded(t *testing.T) {
	m, err := BuildFrom(syntheticRegistry(t, v1.AuthLevel_AUTH_LEVEL_ADMIN,
		v1.AuditLevel_AUDIT_LEVEL_RECORDED, v1.AuditLevel_AUDIT_LEVEL_NONE))
	if err != nil {
		t.Fatalf("BuildFrom: %v", err)
	}
	if !m.Recorded("/career.v1.SyntheticService/Declared") {
		t.Fatal("a method declared RECORDED is not reported as recorded")
	}
	if m.Recorded("/career.v1.SyntheticService/Undeclared") {
		t.Fatal("a method declared NONE is reported as recorded")
	}
}

// An unknown procedure is not recorded. Health checks and anything
// outside career.v1 go through the same interceptor.
func TestUnknownProcedureIsNotRecorded(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if m.Recorded("/grpc.health.v1.Health/Check") {
		t.Fatal("a procedure outside career.v1 was treated as recorded")
	}
}

func TestEmptyRegistryIsAnError(t *testing.T) {
	if _, err := BuildFrom(&protoregistry.Files{}); err == nil {
		t.Fatal("an empty registry built a map; that is indistinguishable from recording nothing")
	}
}

// --- the target extraction, kept honest ---

// Procedures that legitimately act on nothing identifiable: global
// settings, whole-corpus jobs, and the SQL console, whose target is the
// statement itself and is recorded in the payload.
//
// This list is the point of the test. Adding to it is a deliberate
// statement that an action has no target; the alternative is rows whose
// target silently reads as empty and nobody notices until someone tries
// to answer "what was done to this member".
var noTarget = map[string]bool{
	"/career.v1.AdminService/SetJdFitBands":           true,
	"/career.v1.AdminService/SetJdSubmissionLimit":    true,
	"/career.v1.AdminService/SetDecisionTestSettings": true,
	"/career.v1.AdminService/SetSchedulerSettings":    true,
	"/career.v1.AdminService/ReindexCorpus":           true,
	"/career.v1.AdminService/SweepCorpusEmbeddings":   true,
	"/career.v1.AdminService/IngestCorpusText":        true,
	"/career.v1.AdminService/RunDbQuery":              true,
	"/career.v1.AdminService/ConnectCalendar":         true,
	"/career.v1.AdminService/DisconnectCalendar":      true,
	"/career.v1.AdminService/ExportDecisionLog":       true,
	"/career.v1.AdminService/ExportDecisionTestData":  true,
	"/career.v1.AdminService/UpsertAccessGrant":       true,
	"/career.v1.AdminService/UpsertSavedQuery":        true,
	"/career.v1.AdminService/UpsertGoldenPosting":     true,
	// Creates. The new entry's id exists only in the response, so
	// there is nothing to record as a target at request time; the
	// question text is kept in the payload instead.
	"/career.v1.AdminService/CreateQaEntry": true,
	// Acts on a job kind, not an instance. The kind is in the payload.
	"/career.v1.AdminService/RunJob": true,
}

func TestEveryRecordedProcedureResolvesATarget(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	known := map[string]bool{}
	for _, c := range targetFields {
		known[c.field] = true
	}

	var orphans []string
	for procedure, lvl := range m {
		if lvl != v1.AuditLevel_AUDIT_LEVEL_RECORDED || noTarget[procedure] {
			continue
		}
		fields, err := requestFieldsOf(procedure)
		if err != nil {
			t.Fatalf("%s: %v", procedure, err)
		}
		var found bool
		for i := range fields.Len() {
			if known[string(fields.Get(i).Name())] {
				found = true
				break
			}
		}
		if !found {
			orphans = append(orphans, procedure)
		}
	}
	if len(orphans) > 0 {
		t.Fatalf("%d recorded procedure(s) have no field this can use as a target, "+
			"so their audit rows would record an empty target_id:\n  %s\n"+
			"Either add the request's id field to targetFields in interceptor.go, "+
			"or add the procedure to noTarget here with the reason it acts on nothing.",
			len(orphans), strings.Join(orphans, "\n  "))
	}
}

// noTarget must not rot into a list of procedures that no longer exist,
// or it quietly stops excusing anything while looking like it does.
func TestNoTargetListHasNoStaleEntries(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for procedure := range noTarget {
		if _, ok := m[procedure]; !ok {
			t.Errorf("noTarget names %s, which is not a procedure any more", procedure)
		}
	}
}

// Every recorded procedure must be unary, because WrapStreamingHandler
// deliberately records nothing. If an admin mutation is ever added as a
// stream this fails, which is the intended warning.
func TestEveryRecordedProcedureIsUnary(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for procedure, lvl := range m {
		if lvl != v1.AuditLevel_AUDIT_LEVEL_RECORDED {
			continue
		}
		md, err := methodOf(procedure)
		if err != nil {
			t.Fatalf("%s: %v", procedure, err)
		}
		if md.IsStreamingClient() || md.IsStreamingServer() {
			t.Errorf("%s is declared RECORDED but is streaming; the interceptor "+
				"records only unary calls, so it would write nothing", procedure)
		}
	}
}

// --- helpers ---

func methodOf(procedure string) (protoreflect.MethodDescriptor, error) {
	parts := strings.Split(strings.TrimPrefix(procedure, "/"), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("unparseable procedure %q", procedure)
	}
	d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(parts[0]))
	if err != nil {
		return nil, err
	}
	svc, ok := d.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%s is not a service", parts[0])
	}
	md := svc.Methods().ByName(protoreflect.Name(parts[1]))
	if md == nil {
		return nil, fmt.Errorf("no method %s", procedure)
	}
	return md, nil
}

func requestFieldsOf(procedure string) (protoreflect.FieldDescriptors, error) {
	md, err := methodOf(procedure)
	if err != nil {
		return nil, err
	}
	return md.Input().Fields(), nil
}

// syntheticRegistry builds a private registry with one service holding
// two methods at the given auth level: "Declared", carrying the audit
// level passed in, and "Undeclared", carrying the second one. Passing
// AUDIT_LEVEL_UNSPECIFIED for either means no audit option is set at
// all, which is the gap the boot check exists to catch.
func syntheticRegistry(t *testing.T, auth v1.AuthLevel, declared, undeclared v1.AuditLevel) *protoregistry.Files {
	t.Helper()

	lvl := declared
	declaredOpts := &descriptorpb.MethodOptions{}
	proto.SetExtension(declaredOpts, v1.E_Auth, auth)
	if declared != v1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED {
		proto.SetExtension(declaredOpts, v1.E_Audit, declared)
	}

	undeclaredOpts := &descriptorpb.MethodOptions{}
	proto.SetExtension(undeclaredOpts, v1.E_Auth, auth)
	if undeclared != v1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED {
		proto.SetExtension(undeclaredOpts, v1.E_Audit, undeclared)
	}

	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("career/v1/synthetic_audit_for_test.proto"),
		Package: proto.String("career.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("SynthAuditRequest")},
			{Name: proto.String("SynthAuditResponse")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("SyntheticService"),
			Method: []*descriptorpb.MethodDescriptorProto{
				{
					Name:       proto.String("Declared"),
					InputType:  proto.String(".career.v1.SynthAuditRequest"),
					OutputType: proto.String(".career.v1.SynthAuditResponse"),
					Options:    declaredOpts,
				},
				{
					Name:       proto.String("Undeclared"),
					InputType:  proto.String(".career.v1.SynthAuditRequest"),
					OutputType: proto.String(".career.v1.SynthAuditResponse"),
					Options:    undeclaredOpts,
				},
			},
		}},
	}

	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("build the synthetic file descriptor: %v", err)
	}
	// Private, not GlobalFiles: registering a deliberately-broken
	// service globally would be visible to the tests asserting the real
	// contract is complete.
	files := &protoregistry.Files{}
	if err := files.RegisterFile(fd); err != nil {
		t.Fatalf("register the synthetic file: %v", err)
	}

	// Guard the guard. If the options stop round-tripping, both methods
	// read as undeclared and these tests pass for the wrong reason.
	svc := fd.Services().Get(0)
	if got := declaredAuth(svc.Methods().Get(0)); got != auth {
		t.Fatalf("the synthetic auth option did not round-trip: got %v", got)
	}
	if got := declaredAudit(svc.Methods().Get(0)); got != lvl {
		t.Fatalf("the synthetic audit option did not round-trip: got %v", got)
	}
	return files
}
