package authpolicy

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// The real binary: every method declares a level, so the map covers all
// of them and Build succeeds.
func TestBuildCoversEveryDeclaredMethod(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build against the real descriptors: %v", err)
	}

	// The figures in docs/sprint-auth-interceptor.md, so that a change
	// moving methods between levels has to come here and say so.
	// Asserted exactly rather than as a floor: "at least 141" would be
	// satisfied by a map that had quietly gained a service, and the
	// point of this map is that its contents are known.
	d := m.Distribution()
	cases := []struct {
		level v1.AuthLevel
		want  int
	}{
		{v1.AuthLevel_AUTH_LEVEL_ADMIN, 81},
		{v1.AuthLevel_AUTH_LEVEL_MEMBER, 43},
		{v1.AuthLevel_AUTH_LEVEL_PUBLIC, 17},
	}
	total := 0
	for _, c := range cases {
		if got := d[c.level]; got != c.want {
			t.Errorf("%s: declared on %d methods, expected %d. If this is a "+
				"deliberate change, update the table in "+
				"docs/sprint-auth-interceptor.md too.",
				c.level, got, c.want)
		}
		total += c.want
	}
	if len(m) != total {
		t.Errorf("map holds %d methods but the three levels account for %d: "+
			"some method carries a level this test does not know about",
			len(m), total)
	}

	// The key format is the contract between this map and the
	// interceptor that will consume it: connect.Spec.Procedure. A
	// lookup that silently misses is a lookup that denies, so the shape
	// is pinned against real procedures rather than assumed. One per
	// level, so a map that collapsed every method to a single level
	// would not satisfy this.
	for _, c := range []struct {
		procedure string
		want      v1.AuthLevel
	}{
		{"/career.v1.AdminService/ListMembers", v1.AuthLevel_AUTH_LEVEL_ADMIN},
		{"/career.v1.MemberService/GetMe", v1.AuthLevel_AUTH_LEVEL_MEMBER},
		{"/career.v1.AuthService/Login", v1.AuthLevel_AUTH_LEVEL_PUBLIC},
	} {
		lvl, ok := m.Level(c.procedure)
		if !ok {
			t.Errorf("%s is not in the map: the key format no longer matches "+
				"connect.Spec.Procedure, and every lookup will miss", c.procedure)
			continue
		}
		if lvl != c.want {
			t.Errorf("%s declares %s, expected %s", c.procedure, lvl, c.want)
		}
	}
}

// An unknown procedure must report as unknown, not as some level. The
// interceptor turns a miss into a denial, so this is the difference
// between failing closed and failing open.
func TestLevelReportsUnknownProcedures(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, p := range []string{
		"/career.v1.AdminService/NoSuchMethod",
		"/career.v1.NoSuchService/GetMembers",
		"GetMembers",
		"",
	} {
		if lvl, ok := m.Level(p); ok {
			t.Errorf("Level(%q) returned %s and ok=true; an unrecognised "+
				"procedure must be reported as unknown so the caller can deny",
				p, lvl)
		}
	}
}

// The S2 exit criterion: the server refuses to start when a method
// declares no level.
//
// Run against a synthetic registry, because every method in the real
// binary declares one. Without this, the fail-to-start path would be
// asserted by reading the code, and that is exactly the kind of claim
// this sprint exists to stop making: four documents already described an
// interceptor that did not exist.
func TestBuildRejectsAMethodWithNoLevel(t *testing.T) {
	files := syntheticRegistry(t)

	m, err := BuildFrom(files)
	if err == nil {
		t.Fatalf("BuildFrom accepted a service with an unannotated method "+
			"and returned a map of %d entries. The server would start with "+
			"a method that has no policy to enforce.", len(m))
	}
	if m != nil {
		t.Errorf("BuildFrom returned both an error and a map; the caller " +
			"might use the map and ignore the error")
	}

	// The error has to name the offending method. "Some method is
	// missing a level" is not actionable on a 141-method API, and this
	// error appears at boot during a deploy, with the service down.
	if !strings.Contains(err.Error(), "/career.v1.SyntheticService/Unannotated") {
		t.Errorf("error does not name the unannotated method, so the "+
			"deploy that hits it cannot be diagnosed from the log:\n%v", err)
	}
	// And must not name the one that is fine.
	if strings.Contains(err.Error(), "Annotated") {
		t.Errorf("error names the correctly annotated method too:\n%v", err)
	}
}

// An empty walk is a failure, not a clean result: enforcing on an empty
// map denies every request, and the cause would be invisible.
func TestBuildFromRejectsAnEmptyRegistry(t *testing.T) {
	m, err := BuildFrom(&protoregistry.Files{})
	if err == nil {
		t.Fatalf("BuildFrom accepted an empty registry and returned %d "+
			"entries; enforcing on that map denies everything", len(m))
	}
	if !strings.Contains(err.Error(), "not linked in") {
		t.Errorf("the error should explain that no descriptors were found, "+
			"got: %v", err)
	}
}

func TestSummaryNamesEveryLevel(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := m.Summary()
	for _, want := range []string{"141 methods", "admin=81", "member=43", "public=17"} {
		if !strings.Contains(got, want) {
			t.Errorf("Summary() = %q, missing %q", got, want)
		}
	}
}

// syntheticRegistry builds a registry holding one career.v1 service with
// two methods: one declaring ADMIN, one declaring nothing.
func syntheticRegistry(t *testing.T) *protoregistry.Files {
	t.Helper()

	annotated := &descriptorpb.MethodOptions{}
	proto.SetExtension(annotated, v1.E_Auth, v1.AuthLevel_AUTH_LEVEL_ADMIN)

	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("career/v1/synthetic_for_test.proto"),
		Package: proto.String("career.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("SyntheticRequest")},
			{Name: proto.String("SyntheticResponse")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("SyntheticService"),
			Method: []*descriptorpb.MethodDescriptorProto{
				{
					Name:       proto.String("Annotated"),
					InputType:  proto.String(".career.v1.SyntheticRequest"),
					OutputType: proto.String(".career.v1.SyntheticResponse"),
					Options:    annotated,
				},
				{
					Name:       proto.String("Unannotated"),
					InputType:  proto.String(".career.v1.SyntheticRequest"),
					OutputType: proto.String(".career.v1.SyntheticResponse"),
					// No options at all: the gap this is about.
				},
			},
		}},
	}

	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("build the synthetic file descriptor: %v", err)
	}

	// A private registry. Registering this in protoregistry.GlobalFiles
	// would leave a service with an unannotated method visible to every
	// other test in the binary, including the one asserting the real
	// map is complete.
	files := &protoregistry.Files{}
	if err := files.RegisterFile(fd); err != nil {
		t.Fatalf("register the synthetic file: %v", err)
	}

	// Guard the guard: if the options above stop round-tripping, both
	// methods read as unannotated and this test still passes, for the
	// wrong reason.
	svc := fd.Services().Get(0)
	if lvl := declaredLevel(svc.Methods().Get(0)); lvl != v1.AuthLevel_AUTH_LEVEL_ADMIN {
		t.Fatalf("the synthetic annotated method reads as %s, so this test "+
			"is not distinguishing annotated from unannotated", lvl)
	}
	return files
}
