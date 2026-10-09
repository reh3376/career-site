// Package authpolicy reads the auth level each RPC declares in the proto
// and makes it available as data the server can enforce.
//
// Until now `career.v1.auth` has been documentation. 141 methods declare
// a level, nothing reads the declaration, and enforcement is ~90
// hand-written gate calls in the handlers, one per handler that
// remembered. That is the same shape as the Docker/UFW gap found on
// 2026-10-08: a control that is written down, reads as protection, and
// is not applied.
//
// This package is S2 of docs/sprint-auth-interceptor.md. It builds the
// map and refuses to let the server start with a gap in it. It does not
// enforce anything; the interceptor that consumes this arrives in S3,
// deliberately separately, so that the map can be wrong in a way that is
// loud at boot rather than wrong in a way that changes who can call what.
package authpolicy

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// protoPackage is the only package this validates. The registry also
// holds descriptors for everything linked into the binary, including
// google.protobuf and grpc health, and those carry no `career.v1.auth`
// option and are not ours to police.
const protoPackage = "career.v1"

// Map holds one level per RPC, keyed the way Connect names a procedure
// at request time: "/career.v1.AdminService/GetMembers", which is
// exactly connect.Spec.Procedure. Keying it that way means the
// interceptor in S3 does a single map lookup with no string surgery on
// the hot path, and no opportunity to derive the key differently from
// how it was derived here.
type Map map[string]v1.AuthLevel

// Level returns the declared level for a procedure.
//
// A procedure that is not in the map returns false, and the caller must
// treat that as **deny**, per D2 of the sprint plan. Build() makes an
// unknown procedure impossible for anything in career.v1, so a miss
// means either a service outside our proto package or a map that was
// not built by Build() — neither of which should be allowed through on
// the strength of a lookup that failed.
func (m Map) Level(procedure string) (v1.AuthLevel, bool) {
	lvl, ok := m[procedure]
	return lvl, ok
}

// Build walks the registered file descriptors and reads the declared
// level for every method in career.v1.
//
// It returns an error naming every method that declares no level. That
// error is fatal at startup by design: the enum's own comment in
// options.proto claims "Lint fails any method that leaves the level
// unspecified", and no such lint rule ever existed. That all 141 methods
// happen to declare one today is care, not a control. This is the
// control.
//
// Every declared method is included, including the methods of services
// that server.go does not currently mount (HomeService, ContentService,
// DownloadService as of 2026-10-09). Their routes do not exist, so they
// are unreachable rather than unprotected, but validating them anyway
// means mounting one later cannot introduce an unannotated method.
func Build() (Map, error) {
	return BuildFrom(protoregistry.GlobalFiles)
}

// BuildFrom is Build against a specific registry.
//
// It exists so the "refuse to start on an unannotated method" path can
// be tested, which needs a registry containing such a method. Every
// method in the real binary declares a level, so against GlobalFiles
// that path is unreachable and would otherwise be asserted by reading
// it rather than by running it.
func BuildFrom(files *protoregistry.Files) (Map, error) {
	out := Map{}
	var missing []string

	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if fd.Package() != protoPackage {
			return true
		}
		svcs := fd.Services()
		for i := range svcs.Len() {
			svc := svcs.Get(i)
			methods := svc.Methods()
			for j := range methods.Len() {
				m := methods.Get(j)
				procedure := fmt.Sprintf("/%s/%s", svc.FullName(), m.Name())

				lvl := declaredLevel(m)
				if lvl == v1.AuthLevel_AUTH_LEVEL_UNSPECIFIED {
					missing = append(missing, procedure)
					continue
				}
				out[procedure] = lvl
			}
		}
		return true
	})

	// An empty map is not a clean result. If the descriptors are not
	// registered, or protoPackage no longer matches what the generated
	// code declares, the walk above finds nothing and reports no
	// missing levels, which looks identical to success. Enforcing on an
	// empty map would then deny every request, and the reason would be
	// a silent no-op here rather than anything visible.
	if len(out) == 0 && len(missing) == 0 {
		return nil, fmt.Errorf(
			"no %s methods found in the proto registry: the descriptors are "+
				"not linked in, or the package name has changed", protoPackage)
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf(
			"%d method(s) declare no career.v1.auth level, so there is no "+
				"policy to enforce for them:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}

	return out, nil
}

// declaredLevel reads the option off one method descriptor.
func declaredLevel(m protoreflect.MethodDescriptor) v1.AuthLevel {
	opts := m.Options()
	if opts == nil {
		return v1.AuthLevel_AUTH_LEVEL_UNSPECIFIED
	}
	// An unset extension reads as the zero value, which is
	// AUTH_LEVEL_UNSPECIFIED, so a missing option and an explicitly
	// unspecified one are the same thing here. They should be: both
	// mean nothing said what this method requires.
	lvl, ok := proto.GetExtension(opts, v1.E_Auth).(v1.AuthLevel)
	if !ok {
		return v1.AuthLevel_AUTH_LEVEL_UNSPECIFIED
	}
	return lvl
}

// Distribution counts methods per level, for the one line the server
// logs at boot. The counts are worth having in the log rather than only
// in a document: they are how somebody notices that a change moved
// eleven methods from MEMBER to PUBLIC without reading the diff.
func (m Map) Distribution() map[v1.AuthLevel]int {
	out := map[v1.AuthLevel]int{}
	for _, lvl := range m {
		out[lvl]++
	}
	return out
}

// Summary renders the distribution as one short line, strongest first.
func (m Map) Summary() string {
	d := m.Distribution()
	parts := make([]string, 0, 3)
	for _, lvl := range []v1.AuthLevel{
		v1.AuthLevel_AUTH_LEVEL_ADMIN,
		v1.AuthLevel_AUTH_LEVEL_MEMBER,
		v1.AuthLevel_AUTH_LEVEL_PUBLIC,
	} {
		parts = append(parts, fmt.Sprintf("%s=%d",
			strings.ToLower(strings.TrimPrefix(lvl.String(), "AUTH_LEVEL_")),
			d[lvl]))
	}
	return fmt.Sprintf("%d methods: %s", len(m), strings.Join(parts, " "))
}
