// Package auditpolicy reads the audit level each RPC declares in the
// proto and makes it available as data the server can act on.
//
// FR-ADM-09 says "all admin actions shall be recorded in the audit
// log". The `audit_log` table has existed since migration 00001, has
// the right shape, and held zero rows on production when this was
// written on 2026-10-12: no code path anywhere inserted into it. Three
// requirement rows and four proto comments promised it anyway.
//
// The lesson from the eleven controls ADR 0031 catalogues is that the
// fix is not to add the insert to the handlers that need it. Handlers
// were how `career.v1.auth` was enforced for a year, and ~90 hand-written
// gate calls is 90 chances to forget, with the 91st RPC unprotected by
// default. So this is the same shape as internal/authpolicy: the
// contract declares, the boot refuses a gap, and an interceptor applies
// it uniformly. Adding an admin RPC without deciding whether it is
// recorded is a startup failure, not an omission nobody notices.
//
// This package builds the map and validates it. It writes nothing; the
// interceptor that consumes it is in interceptor.go, kept separate so a
// wrong map is loud at boot rather than quietly changing what is kept.
package auditpolicy

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// protoPackage is the only package this validates, for the same reason
// authpolicy restricts itself: the registry also holds google.protobuf
// and grpc health descriptors, which carry none of our options and are
// not ours to police.
const protoPackage = "career.v1"

// Map holds one level per RPC, keyed as Connect names a procedure at
// request time ("/career.v1.AdminService/SetMemberStatus", exactly
// connect.Spec.Procedure), so the interceptor does one map lookup with
// no string surgery and no second way of deriving the key.
type Map map[string]v1.AuditLevel

// Recorded reports whether a call to this procedure is written to
// audit_log.
//
// A procedure that is not in the map returns false. That is the right
// default here and the opposite of authpolicy's, where a miss must mean
// deny: this package's failure mode is a missing audit row, not an
// unguarded endpoint, and Build makes a miss impossible for anything in
// career.v1 anyway. Erring the other way would mean trying to record
// calls to procedures we know nothing about, including health checks.
func (m Map) Recorded(procedure string) bool {
	return m[procedure] == v1.AuditLevel_AUDIT_LEVEL_RECORDED
}

// Summary is a one-line description for the boot log, so the running
// configuration is observable without reading the proto.
func (m Map) Summary() string {
	var recorded, none int
	for _, lvl := range m {
		switch lvl {
		case v1.AuditLevel_AUDIT_LEVEL_RECORDED:
			recorded++
		case v1.AuditLevel_AUDIT_LEVEL_NONE:
			none++
		}
	}
	return fmt.Sprintf("%d recorded, %d not recorded, %d declared", recorded, none, len(m))
}

// Build walks the registered file descriptors and reads the declared
// audit level for every method in career.v1.
func Build() (Map, error) {
	return BuildFrom(protoregistry.GlobalFiles)
}

// BuildFrom is Build against a specific registry, so the "refuse to
// start" paths can be tested against a registry that actually contains
// the mistake. Every admin method in the real binary declares a level,
// which would otherwise leave those paths asserted by reading them
// rather than by running them.
func BuildFrom(files *protoregistry.Files) (Map, error) {
	out := Map{}
	var missing []string
	var sawCareerMethod bool

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
				sawCareerMethod = true

				lvl := declaredAudit(m)

				// Only ADMIN methods are required to declare. A PUBLIC
				// or MEMBER method that says nothing means NONE, which
				// keeps 60-odd member and public RPCs out of a decision
				// they have no stake in: FR-ADM-09 is about admin
				// actions, and audit_log is not an analytics stream.
				// The events registry already covers member behaviour.
				//
				// Deriving "needs a decision" from the auth level rather
				// than from the method's name is the point. A heuristic
				// on the name would have to guess that `PurgeEverything`
				// mutates and `GetAudit` does not, and it would guess
				// wrong the first time somebody names an RPC `RunJob`.
				if declaredAuth(m) == v1.AuthLevel_AUTH_LEVEL_ADMIN &&
					lvl == v1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED {
					missing = append(missing, procedure)
					continue
				}
				out[procedure] = lvl
			}
		}
		return true
	})

	// An empty walk looks identical to success: no methods found means
	// no missing declarations found. Enforcing on an empty map would
	// silently record nothing, which is the exact state this package
	// exists to end. Reported after `missing` is collected but checked
	// against `sawCareerMethod` so a registry whose every method was
	// rejected is still diagnosed as the contract mistake it is, not as
	// a linking problem. authpolicy learned this ordering the hard way.
	if !sawCareerMethod {
		return nil, fmt.Errorf(
			"no %s methods found in the proto registry: the descriptors are "+
				"not linked in, or the package name has changed", protoPackage)
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf(
			"%d admin method(s) declare no career.v1.audit level, so "+
				"\"all admin actions are recorded\" cannot be true:\n  %s\n"+
				"Add `option (career.v1.audit) = AUDIT_LEVEL_RECORDED;` to each "+
				"that changes something or moves member data, and "+
				"AUDIT_LEVEL_NONE to each that does not.",
			len(missing), strings.Join(missing, "\n  "))
	}
	return out, nil
}

func declaredAudit(m protoreflect.MethodDescriptor) v1.AuditLevel {
	opts := m.Options()
	if opts == nil {
		return v1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED
	}
	// An unset extension reads as the zero value, so a missing option
	// and an explicitly unspecified one are the same thing. They should
	// be: both mean nobody decided whether this call is recorded.
	lvl, ok := proto.GetExtension(opts, v1.E_Audit).(v1.AuditLevel)
	if !ok {
		return v1.AuditLevel_AUDIT_LEVEL_UNSPECIFIED
	}
	return lvl
}

func declaredAuth(m protoreflect.MethodDescriptor) v1.AuthLevel {
	opts := m.Options()
	if opts == nil {
		return v1.AuthLevel_AUTH_LEVEL_UNSPECIFIED
	}
	lvl, ok := proto.GetExtension(opts, v1.E_Auth).(v1.AuthLevel)
	if !ok {
		return v1.AuthLevel_AUTH_LEVEL_UNSPECIFIED
	}
	return lvl
}
