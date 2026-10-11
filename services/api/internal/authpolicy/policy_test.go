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

// allow_unverified in the real contract: exactly two methods, both
// MEMBER, and both of them methods a non-active member must still reach.
func TestAllowUnverifiedIsDeclaredWhereExpected(t *testing.T) {
	m, err := Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	got := map[string]bool{}
	for procedure, pol := range m {
		if pol.AllowUnverified {
			got[procedure] = true
			if pol.Level != v1.AuthLevel_AUTH_LEVEL_MEMBER {
				t.Errorf("%s declares allow_unverified at %s; BuildFrom "+
					"should have refused this", procedure, pol.Level)
			}
		}
	}

	// Pinned, because adding the option to a method widens who can call
	// it and should not pass unnoticed. Logout carried it until
	// 2026-10-11 and lost it when it became PUBLIC, where it meant
	// nothing: PUBLIC resolves no session, so there is no status to
	// waive.
	want := map[string]bool{
		"/career.v1.MemberService/GetMe":         true,
		"/career.v1.MemberService/DeleteAccount": true,
	}
	for p := range want {
		if !got[p] {
			t.Errorf("%s no longer declares allow_unverified. A non-active "+
				"member reaches GetMe through the ResetPassword path, and "+
				"without the option the interceptor refuses them.", p)
		}
	}
	for p := range got {
		if !want[p] {
			t.Errorf("%s now declares allow_unverified and is not on the "+
				"expected list. It waives the active-account check, so say "+
				"why here before adding it.", p)
		}
	}
}

// The option is only meaningful with MEMBER, and the server refuses to
// start otherwise. PUBLIC resolves no session, so there is no account
// whose status could be waived; ADMIN waiving it would let a disabled
// admin keep every admin method.
func TestBuildRejectsAllowUnverifiedAwayFromMember(t *testing.T) {
	for _, c := range []struct {
		name  string
		level v1.AuthLevel
	}{
		{"public", v1.AuthLevel_AUTH_LEVEL_PUBLIC},
		{"admin", v1.AuthLevel_AUTH_LEVEL_ADMIN},
	} {
		t.Run(c.name, func(t *testing.T) {
			files := registryWithAllowUnverified(t, c.level)

			m, err := BuildFrom(files)
			if err == nil {
				t.Fatalf("BuildFrom accepted allow_unverified at %s and "+
					"returned %d entries", c.level, len(m))
			}
			if m != nil {
				t.Error("returned both an error and a map")
			}
			if !strings.Contains(err.Error(), "allow_unverified") {
				t.Errorf("the error does not name the option:\n%v", err)
			}
			if !strings.Contains(err.Error(), "/career.v1.SyntheticService/Waived") {
				t.Errorf("the error does not name the method, so a deploy "+
					"hitting it cannot be diagnosed:\n%v", err)
			}
		})
	}
}

// MEMBER plus allow_unverified is accepted, so the test above is
// rejecting the level rather than the option.
func TestBuildAcceptsAllowUnverifiedWithMember(t *testing.T) {
	files := registryWithAllowUnverified(t, v1.AuthLevel_AUTH_LEVEL_MEMBER)

	m, err := BuildFrom(files)
	if err != nil {
		t.Fatalf("BuildFrom refused allow_unverified at MEMBER: %v", err)
	}
	pol, ok := m.Lookup("/career.v1.SyntheticService/Waived")
	if !ok {
		t.Fatal("the method is missing from the map")
	}
	if !pol.AllowUnverified {
		t.Error("allow_unverified was read as false")
	}
	if pol.Level != v1.AuthLevel_AUTH_LEVEL_MEMBER {
		t.Errorf("level = %s, want MEMBER", pol.Level)
	}
}

// registryWithAllowUnverified builds a registry holding one career.v1
// method that declares allow_unverified at the given level.
func registryWithAllowUnverified(t *testing.T, level v1.AuthLevel) *protoregistry.Files {
	t.Helper()

	opts := &descriptorpb.MethodOptions{}
	proto.SetExtension(opts, v1.E_Auth, level)
	proto.SetExtension(opts, v1.E_AllowUnverified, true)

	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("career/v1/synthetic_unverified_for_test.proto"),
		Package: proto.String("career.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("WaivedRequest")},
			{Name: proto.String("WaivedResponse")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("SyntheticService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("Waived"),
				InputType:  proto.String(".career.v1.WaivedRequest"),
				OutputType: proto.String(".career.v1.WaivedResponse"),
				Options:    opts,
			}},
		}},
	}

	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("build the synthetic file descriptor: %v", err)
	}
	files := &protoregistry.Files{}
	if err := files.RegisterFile(fd); err != nil {
		t.Fatalf("register the synthetic file: %v", err)
	}

	// Guard the guard: if the option stops round-tripping, every case
	// here reads as "not declared" and passes for the wrong reason.
	if !declaredAllowUnverified(fd.Services().Get(0).Methods().Get(0)) {
		t.Fatal("the synthetic method's allow_unverified reads as false, so " +
			"this test is not exercising the option at all")
	}
	return files
}
