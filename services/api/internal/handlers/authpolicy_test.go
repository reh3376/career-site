package handlers

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// S1 of docs/sprint-auth-interceptor.md: does what the contract declares
// match what the handlers actually enforce?
//
// **Why this comes before the interceptor.** Today `career.v1.auth` is
// documentation: 141 methods declare a level and nothing reads it, while
// enforcement is ~93 hand-written gate calls. Moving enforcement into an
// interceptor only preserves behaviour where the two already agree. Each
// way they can disagree is a different kind of surprise:
//
//   - declared ADMIN, handler ungated: the method is open today and the
//     interceptor CLOSES it. That reads as the interceptor breaking a
//     feature, when it is the interceptor finding a hole.
//   - declared PUBLIC, handler gated: the interceptor OPENS it.
//
// The second is the dangerous one and the reason this runs first.
//
// **Both sides are read from the real thing.** The declared side comes
// from the registered protobuf descriptors, not a regex over .proto
// files. The implemented side comes from the Go AST, not a regex over
// source. Writing this with regex first produced three separate false
// alarms in a row, including "141 mismatches" when the pattern simply
// failed to match any handler, and a report that two admin methods were
// ungated when they used a gate helper the pattern did not know about. A
// check that cries wolf is a check that gets deleted.

// gateFuncs are the helpers that decide access. A handler that calls one
// is enforcing; a handler that calls none is open.
//
// Listed explicitly rather than matched by prefix, so adding a gate means
// coming here and saying what level it enforces. A new `requireXyz` that
// this does not know about shows up as an ungated handler, which is the
// safe direction to be wrong in.
var gateFuncs = map[string]string{
	"requireAdmin":     "ADMIN",
	"requireChatAdmin": "ADMIN",
	"requireMember":    "MEMBER",
}

// lookupFuncs resolve the session cookie to a user and decide nothing.
// Whether a call to one gates depends entirely on what the handler does
// with the result, so they are classified by shape rather than by name
// (see "Gate or attribution" below).
var lookupFuncs = map[string]bool{
	"LookupSessionUser":  true,
	"lookupSessionToken": true,
}

// knownUngated records methods that declare a level but deliberately do
// not enforce it, with the reason. Every entry here is a decision, and
// every one of them is a behaviour change waiting to happen when the
// interceptor starts enforcing, so the list is short and explicit.
//
// **Empty, and that is the goal.** It held one entry, AuthService.Logout,
// which declared MEMBER and gated nothing. Rather than carry the
// exception, the owner settled it on 2026-10-09 by declaring Logout
// PUBLIC, which is what it always was: the session token the caller
// presents is the thing being revoked, so nobody can log out anyone but
// themselves, and requiring a session would have broken logout for
// exactly the expired-session case that most needs it.
//
// An empty list means every declaration now matches its handler with no
// carve-outs, so the S4 interceptor has nothing to special-case.
var knownUngated = map[string]string{}

// declaredNotImplemented are the methods the proto declares, the server
// mounts, and no handler in this package implements, so Connect's
// embedded Unimplemented*ServiceHandler answers them. Verified one by
// one on 2026-10-09: no definition for any of them exists under
// services/api/internal.
//
// **This list is pinned, and that is the point.** It is the bucket a
// broken matcher drains into: every method the AST walk fails to find
// looks exactly like a method that was never written, and a comparison
// over nothing passes. Pinning the set turns that silent pass into a
// failure, because a method that stops being found appears here and the
// set stops matching.
//
// Two things make it change, and both should fail until someone looks:
// implementing one of these (remove it, and it comes under the
// comparison), or the walk breaking (everything else to do).
var declaredNotImplemented = map[string]bool{
	// The second factor that was never built. `mfa_fresh` was removed
	// from all 79 methods on 2026-10-09 for this reason; these two RPCs
	// are the rest of that same gap.
	"AuthService.MfaEnroll": true,
	"AuthService.MfaVerify": true,

	// Account self-service, declared and not built.
	"AuthService.ChangeEmail":          true,
	"AuthService.ChangePassword":       true,
	"AuthService.LogoutAll":            true,
	"AuthService.ResendVerification":   true,
	"MemberService.DeleteAccount":      true,
	"MemberService.GetExport":          true,
	"MemberService.RequestExport":      true,
	"MemberService.SetInterests":       true,
	"MemberService.UpdateMe":           true,
	"MemberService.ListSaved":          true,
	"MemberService.SaveItem":           true,
	"MemberService.UnsaveItem":         true,
	"ContactService.GetContactOptions": true,

	// Admin surfaces declared ahead of the console that would use them.
	"AdminService.AddMemberNote":         true,
	"AdminService.GetAnalytics":          true,
	"AdminService.GetAudit":              true,
	"AdminService.GetCorpusStatus":       true,
	"AdminService.GetMemberConversation": true,
	"AdminService.GetPersona":            true,
	"AdminService.GetReviewQueue":        true,
	"AdminService.ReplyEscalation":       true,
	"AdminService.ResolveReviewItem":     true,
	"AdminService.TestRetrieval":         true,

	"ChatService.Escalate":              true,
	"ChatService.GetQuota":              true,
	"SystemService.GetGovernanceStatus": true,
}

type handlerImpl struct {
	recv  string
	file  string
	gates []string
}

func TestDeclaredAuthMatchesHandlerEnforcement(t *testing.T) {
	declared := declaredLevels(t)
	impl := handlerGates(t)

	// A walk that matches nothing passes every comparison below. Two
	// earlier versions of this check did exactly that and reported a
	// clean run, so both inputs are size-checked before they are used.
	if len(declared) < 130 {
		t.Fatalf("read %d declared methods from the descriptors, expected ~141: "+
			"the descriptor walk is broken and this test is asserting nothing",
			len(declared))
	}

	// Every handler found must correspond to a declared method. A wrong
	// entry in receiverService (mapping `Meetings` to the wrong service,
	// say) would quietly file a whole service's handlers under a key
	// nothing compares against, and the comparison would pass by
	// finding no handler for any of them.
	for _, key := range sortedImplKeys(impl) {
		if _, ok := declared[key]; !ok {
			t.Errorf("handler %s.%s (%s) matches no declared method: "+
				"receiverService probably maps %q to the wrong service",
				key.service, key.method, impl[key].file, impl[key].recv)
		}
	}

	var problems []string
	var unimplementedNames []string
	unmounted := 0
	for _, key := range sortedKeys(declared) {
		level := declared[key]
		name := key.method

		if unregisteredServices[key.service] {
			unmounted++
			continue
		}

		h, ok := impl[key]
		if !ok {
			// Declared and mounted, but no handler here: the embedded
			// Unimplemented*ServiceHandler answers `unimplemented`,
			// which is safe.
			//
			// Named rather than counted. This is the bucket a broken
			// matcher empties itself into, and a bucket of 28 anonymous
			// methods is indistinguishable from a bucket of 28 methods
			// the walk failed to find. Under -v they are listed, so the
			// claim "these really are unimplemented" is checkable
			// instead of asserted.
			unimplementedNames = append(unimplementedNames,
				key.service+"."+name)
			continue
		}

		enforced := strongest(h.gates)
		if reason, exempt := knownUngated[name]; exempt {
			if enforced != "" {
				problems = append(problems, fmt.Sprintf(
					"%s.%s is on the knownUngated list (%s) but now calls %v. "+
						"Remove the exemption.", key.service, name, reason, h.gates))
			}
			continue
		}

		switch level {
		case "ADMIN":
			if enforced != "ADMIN" {
				problems = append(problems, fmt.Sprintf(
					"%s.%s declares ADMIN but enforces %q (%s). It is reachable by "+
						"anyone who can reach the service today.",
					key.service, name, orNone(enforced), h.file))
			}
		case "MEMBER":
			if enforced == "" {
				problems = append(problems, fmt.Sprintf(
					"%s.%s declares MEMBER but enforces nothing (%s). Either gate it, "+
						"or declare it PUBLIC and say why in knownUngated.",
					key.service, name, h.file))
			}
		case "PUBLIC":
			if enforced != "" {
				problems = append(problems, fmt.Sprintf(
					"%s.%s declares PUBLIC but enforces %s (%s). An interceptor would "+
						"OPEN this method, which is the dangerous direction.",
					key.service, name, enforced, h.file))
			}
		default:
			problems = append(problems, fmt.Sprintf(
				"%s.%s declares no auth level", key.service, name))
		}
	}

	// The distribution, cross-checked against the figures in
	// docs/sprint-auth-interceptor.md. If the extension ever stops being
	// read, every method reads UNSPECIFIED and this is how it shows.
	dist := map[string]int{}
	for _, lvl := range declared {
		dist[lvl]++
	}
	t.Logf("declared levels: ADMIN=%d MEMBER=%d PUBLIC=%d other=%d",
		dist["ADMIN"], dist["MEMBER"], dist["PUBLIC"],
		len(declared)-dist["ADMIN"]-dist["MEMBER"]-dist["PUBLIC"])

	// How the 105 compared handlers actually enforce, which is the
	// figure S5 has to replace. Logged rather than asserted: it changes
	// whenever a handler is added, and the comparison above is what
	// guards correctness.
	gateUse := map[string]int{}
	for _, h := range impl {
		if len(h.gates) == 0 {
			gateUse["(none)"]++
			continue
		}
		for _, g := range h.gates {
			gateUse[g]++
		}
	}
	t.Logf("enforcement across the %d compared handlers: %s",
		len(impl), strings.Join(countsLine(gateUse), ", "))

	sort.Strings(unimplementedNames)
	t.Logf("%d methods declared; %d handlers found; %d on services the "+
		"server never mounts; %d answered by the Unimplemented embed; "+
		"%d deliberately ungated",
		len(declared), len(impl), unmounted, len(unimplementedNames),
		len(knownUngated))

	// The pinned set, checked both ways.
	found := map[string]bool{}
	for _, n := range unimplementedNames {
		found[n] = true
		if !declaredNotImplemented[n] {
			t.Errorf("%s declares an auth level and no handler was found for "+
				"it, but it is not on declaredNotImplemented. Either the AST "+
				"walk has stopped finding its handler (likely: check the "+
				"signature shape against looksLikeConnectHandler) or it is "+
				"genuinely unimplemented and belongs on the list.", n)
		}
	}
	for n := range declaredNotImplemented {
		if !found[n] {
			t.Errorf("%s is on declaredNotImplemented but a handler now "+
				"exists for it. Remove it from the list so its enforcement "+
				"is actually compared.", n)
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		t.Errorf("the contract and the handlers disagree on %d methods. "+
			"Every one of these changes behaviour when the interceptor starts "+
			"enforcing (docs/sprint-auth-interceptor.md S1):\n  %s",
			len(problems), strings.Join(problems, "\n  "))
	}
}

type methodKey struct{ service, method string }

// declaredLevels reads the auth level off the registered descriptors.
func declaredLevels(t *testing.T) map[methodKey]string {
	t.Helper()
	out := map[methodKey]string{}
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if fd.Package() != "career.v1" {
			return true
		}
		svcs := fd.Services()
		for i := range svcs.Len() {
			svc := svcs.Get(i)
			ms := svc.Methods()
			for j := range ms.Len() {
				m := ms.Get(j)
				lvl := ""
				if v := proto.GetExtension(m.Options(), v1.E_Auth); v != nil {
					if al, ok := v.(v1.AuthLevel); ok {
						lvl = strings.TrimPrefix(al.String(), "AUTH_LEVEL_")
					}
				}
				out[methodKey{string(svc.Name()), string(m.Name())}] = lvl
			}
		}
		return true
	})
	return out
}

// handlerGates walks this package's AST and records, per handler method,
// which gate helpers its body calls.
//
// Keyed by (service, method) using the receiver type to resolve the
// service, because method names are unique across services today but
// nothing guarantees they stay that way.
func handlerGates(t *testing.T) map[methodKey]handlerImpl {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the handlers directory: %v", err)
	}

	fset := token.NewFileSet()
	out := map[methodKey]handlerImpl{}
	parsed := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		parsed++

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil || !fn.Name.IsExported() {
				continue
			}
			if !looksLikeConnectHandler(fn) {
				continue
			}
			recv := receiverType(fn)
			svc, known := receiverService[recv]
			if !known {
				// AdminDecision is the non-Connect one-click route from
				// D5 of the sprint plan, not a registered service.
				continue
			}
			out[methodKey{svc, fn.Name.Name}] = handlerImpl{
				recv:  recv,
				file:  name,
				gates: gatesCalled(fn.Body),
			}
		}
	}

	if parsed < 10 {
		t.Fatalf("parsed only %d non-test .go files in the handlers package; "+
			"the directory walk is broken", parsed)
	}
	return out
}

// looksLikeConnectHandler is true for a method whose first parameter is a
// context and whose second is a *connect.Request[...]. That is the shape
// every generated service interface requires, in all its spellings:
// parameters on one line or several, named or `_`, with or without a
// trailing ServerStream for the streaming methods.
func looksLikeConnectHandler(fn *ast.FuncDecl) bool {
	params := fn.Type.Params
	if params == nil || len(params.List) < 2 {
		return false
	}
	return exprHasText(params.List[0].Type, "context.Context") &&
		exprHasText(params.List[1].Type, "connect.Request")
}

func exprHasText(e ast.Expr, want string) bool {
	var b strings.Builder
	writeExpr(&b, e)
	return strings.Contains(b.String(), want)
}

func writeExpr(b *strings.Builder, e ast.Expr) {
	switch t := e.(type) {
	case *ast.Ident:
		b.WriteString(t.Name)
	case *ast.SelectorExpr:
		writeExpr(b, t.X)
		b.WriteByte('.')
		b.WriteString(t.Sel.Name)
	case *ast.StarExpr:
		b.WriteByte('*')
		writeExpr(b, t.X)
	case *ast.IndexExpr:
		writeExpr(b, t.X)
		b.WriteByte('[')
		writeExpr(b, t.Index)
		b.WriteByte(']')
	}
}

func receiverType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	var b strings.Builder
	writeExpr(&b, fn.Recv.List[0].Type)
	return strings.TrimPrefix(b.String(), "*")
}

// receiverService maps a handler struct to the service it is registered
// as in internal/server/server.go. These eleven are every service the
// server actually mounts.
//
// Explicit rather than inferred from the name, because the names do not
// line up (`Meetings` serves `MeetingService`, `Events` serves
// `EventService`) and because a wrong guess here silently drops a
// service from the comparison. The implemented-count floor in the test
// is what stops that going unnoticed.
var receiverService = map[string]string{
	"Activity":     "ActivityService",
	"Admin":        "AdminService",
	"Auth":         "AuthService",
	"Chat":         "ChatService",
	"Contact":      "ContactService",
	"DecisionTest": "DecisionTestService",
	"Events":       "EventService",
	"Jd":           "JdService",
	"Meetings":     "MeetingService",
	"Member":       "MemberService",
	"System":       "SystemService",
}

// unregisteredServices are declared in the proto and have generated
// code, but server.go never mounts them, so none of their methods are
// reachable: Connect answers on the service's own route prefix, and
// those prefixes are not registered at all.
//
// Unreachable is the safe state, so this is not a hole. It is listed
// because an unmounted service otherwise shows up in the comparison as
// "declares ADMIN, enforces nothing", which reads alarming and is not
// true, and because the day one of these IS mounted it arrives with no
// enforcement at all. Mounting one of them means coming here.
var unregisteredServices = map[string]bool{
	"HomeService":     true,
	"ContentService":  true,
	"DownloadService": true,
}

// gatesCalled finds every gate a body applies: the named gate helpers
// wherever they appear, plus session lookups that are used to deny.
func gatesCalled(body *ast.BlockStmt) []string {
	// Lookups whose result is only used on the success path. Collected
	// first because the walk below needs to know, at each call, whether
	// that call is the one in an attribution `if`.
	attribution := map[*ast.CallExpr]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			if call := lookupInInit(ifStmt); call != nil && requiresSuccess(ifStmt.Cond) {
				attribution[call] = true
			}
		}
		return true
	})

	var found []string
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			found = append(found, name)
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := calleeName(call)
		if _, isGate := gateFuncs[name]; isGate {
			add(name)
			return true
		}
		if lookupFuncs[name] && !attribution[call] {
			// A bare lookup assigned to a variable, with the failure
			// handled by returning. That is a gate even though the
			// helper itself decides nothing.
			add(name)
		}
		return true
	})
	sort.Strings(found)
	return found
}

// Gate or attribution.
//
// A session lookup gates when its failure path returns, and attributes
// when its failure path is simply "carry on without a user". In this
// package those two intents have two distinct shapes, and the
// difference is Go idiom rather than local accident:
//
//	u, err := h.auth.LookupSessionUser(ctx, req)   // gate: deny below
//	if err != nil { return nil, connect.NewError(...) }
//
//	if u, err := h.auth.LookupSessionUser(ctx, req); err == nil && u != nil {
//	        userID = &u.ID                          // attribution: optional
//	}
//
// The second form scopes the result to the branch where it worked,
// which is only useful when not having a user is acceptable. So a
// lookup in an `if` whose condition demands success is attribution.
//
// Getting this wrong in the lenient direction would be dangerous: it
// would read an attribution lookup as enforcement and so report a
// public method as protected. It errs the other way. An unrecognised
// shape counts as a gate, which at worst reports a disagreement that
// turns out to be fine.

// lookupInInit returns the session-lookup call in an if-statement's
// init clause, or nil.
func lookupInInit(ifStmt *ast.IfStmt) *ast.CallExpr {
	assign, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return nil
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !lookupFuncs[calleeName(call)] {
		return nil
	}
	return call
}

// requiresSuccess is true when a condition demands the lookup worked,
// i.e. it contains `err == nil` anywhere in its && chain.
func requiresSuccess(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	if bin.Op == token.LAND {
		return requiresSuccess(bin.X) || requiresSuccess(bin.Y)
	}
	if bin.Op != token.EQL {
		return false
	}
	x, xok := bin.X.(*ast.Ident)
	y, yok := bin.Y.(*ast.Ident)
	return xok && yok && x.Name == "err" && y.Name == "nil"
}

func calleeName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	case *ast.IndexExpr:
		// A generic call like requireAdmin[T](...).
		return calleeName(&ast.CallExpr{Fun: f.X})
	}
	return ""
}

// strongest returns the highest level any of the called gates enforces.
//
// A gating session lookup enforces MEMBER: it is only on this list when
// gatesCalled decided its failure path denies, and what it can
// establish is that the caller has a session, not that they are an
// admin. Nothing here reads a role.
func strongest(gates []string) string {
	best := ""
	for _, g := range gates {
		level := gateFuncs[g]
		if level == "" && lookupFuncs[g] {
			level = "MEMBER"
		}
		switch level {
		case "ADMIN":
			return "ADMIN"
		case "MEMBER":
			best = "MEMBER"
		}
	}
	return best
}

func orNone(s string) string {
	if s == "" {
		return "nothing"
	}
	return s
}

func sortedKeys(m map[methodKey]string) []methodKey {
	out := make([]methodKey, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].service != out[j].service {
			return out[i].service < out[j].service
		}
		return out[i].method < out[j].method
	})
	return out
}

func shortPath(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func sortedImplKeys(m map[methodKey]handlerImpl) []methodKey {
	out := make([]methodKey, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].service != out[j].service {
			return out[i].service < out[j].service
		}
		return out[i].method < out[j].method
	})
	return out
}

func countsLine(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return out
}

// ---------------------------------------------------------------
// What the gates actually check
// ---------------------------------------------------------------

// The comparison above matches a handler to a gate by name. That is
// enough to answer "is this method gated at all", which was S1's
// question, and it is not enough for S4 or S5, because the gates do not
// agree with each other about what a level means:
//
//	requireAdmin      session + admin role
//	requireChatAdmin  session + active status + admin role
//	requireMember     session + active status
//	a bare lookup     session
//
// Login refuses every non-active status, so a session only exists for an
// account that was active when it was minted. The status checks are
// therefore about what happens *after* that: an admin suspending,
// declining or expiring a member mid-session. `requireMember` revokes
// access on the next call; `requireAdmin` and the bare lookups do not.
//
// So a suspended member can still read their own profile and history and
// record activity events, and a suspended admin keeps every admin
// method. One admin exists and he is active, so the second is
// theoretical, but both are the kind of difference an interceptor has to
// settle deliberately rather than inherit by accident.
//
// Pinned here so that the interceptor's definition of each level can be
// checked against the thing it is replacing, and so that the three
// separate `requireMember` definitions cannot drift apart unnoticed.

type gateChecks struct {
	session      bool
	activeStatus bool
	adminRole    bool
}

func (g gateChecks) String() string {
	parts := []string{}
	if g.session {
		parts = append(parts, "session")
	}
	if g.activeStatus {
		parts = append(parts, "active-status")
	}
	if g.adminRole {
		parts = append(parts, "admin-role")
	}
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, "+")
}

var wantGateSemantics = map[string]gateChecks{
	// No status check. A member whose access was revoked keeps admin
	// methods denied by role anyway, but an admin whose own account was
	// disabled would not be stopped here.
	"requireAdmin": {session: true, adminRole: true},
	// The strictest of the four, because it goes through requireMember.
	"requireChatAdmin": {session: true, activeStatus: true, adminRole: true},
	// Defined separately in chat.go, jd.go and meetings.go. All three
	// must agree; the test below fails if they stop agreeing.
	"requireMember": {session: true, activeStatus: true},
}

func TestGatesCheckWhatTheyAreDocumentedToCheck(t *testing.T) {
	funcs := gateFuncDecls(t)

	for name, want := range wantGateSemantics {
		decls := funcs[name]
		if len(decls) == 0 {
			t.Errorf("no definition of %s was found; the gate was renamed or "+
				"the AST walk is broken", name)
			continue
		}

		// Every definition of a given gate name must check the same
		// things. Three copies of requireMember exist, and a status
		// check quietly dropped from one of them would leave one
		// service's members gated more weakly than the other two.
		for i, decl := range decls {
			got := resolveChecks(decl, funcs, map[string]bool{})
			if got != want {
				t.Errorf("%s (definition %d of %d, %s) checks %s, expected %s",
					name, i+1, len(decls), decl.file, got, want)
			}
		}
	}
}

// The three definitions of requireMember are byte-for-byte equivalent in
// what they check, so a single interceptor can replace all three. Stated
// as its own test because it is the precondition for S5 collapsing them.
func TestEveryRequireMemberDefinitionAgrees(t *testing.T) {
	funcs := gateFuncDecls(t)
	decls := funcs["requireMember"]
	if len(decls) != 3 {
		t.Fatalf("found %d definitions of requireMember, expected 3 "+
			"(chat.go, jd.go, meetings.go)", len(decls))
	}

	first := resolveChecks(decls[0], funcs, map[string]bool{})
	for _, d := range decls[1:] {
		if got := resolveChecks(d, funcs, map[string]bool{}); got != first {
			t.Errorf("requireMember in %s checks %s but the one in %s checks "+
				"%s; one interceptor cannot replace both",
				d.file, got, decls[0].file, first)
		}
	}
}

type gateDecl struct {
	fn   *ast.FuncDecl
	file string
}

// gateFuncDecls indexes every function or method in the package by name,
// since a gate may be a plain generic function (requireAdmin) or a
// method (requireMember on three receivers).
func gateFuncDecls(t *testing.T) map[string][]gateDecl {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the handlers directory: %v", err)
	}
	fset := token.NewFileSet()
	out := map[string][]gateDecl{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				out[fn.Name.Name] = append(out[fn.Name.Name], gateDecl{fn: fn, file: name})
			}
		}
	}
	// Stable order, so a failure names the same "definition N" each run.
	for _, decls := range out {
		sort.Slice(decls, func(i, j int) bool { return decls[i].file < decls[j].file })
	}
	return out
}

// resolveChecks reports what a gate checks, following calls to other
// gates so that requireChatAdmin inherits requireMember's checks rather
// than appearing to check only the role.
func resolveChecks(d gateDecl, funcs map[string][]gateDecl, seen map[string]bool) gateChecks {
	if seen[d.fn.Name.Name] {
		return gateChecks{}
	}
	seen[d.fn.Name.Name] = true

	var out gateChecks
	ast.Inspect(d.fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			name := calleeName(node)
			if lookupFuncs[name] {
				out.session = true
			}
			// A gate calling another gate inherits its checks.
			if _, isGate := wantGateSemantics[name]; isGate {
				for _, inner := range funcs[name] {
					c := resolveChecks(inner, funcs, seen)
					out.session = out.session || c.session
					out.activeStatus = out.activeStatus || c.activeStatus
					out.adminRole = out.adminRole || c.adminRole
					break
				}
			}
		case *ast.BinaryExpr:
			// `u.Status != users.StatusActive` and
			// `u.Role != users.RoleAdmin`. Matched on both operands so
			// that comparing against some other status or role does not
			// read as the check it is not.
			if comparesFieldToConst(node, "Status", "StatusActive") {
				out.activeStatus = true
			}
			if comparesFieldToConst(node, "Role", "RoleAdmin") {
				out.adminRole = true
			}
		}
		return true
	})
	return out
}

// comparesFieldToConst matches `x.<field> <op> users.<constant>`.
func comparesFieldToConst(bin *ast.BinaryExpr, field, constant string) bool {
	if bin.Op != token.NEQ && bin.Op != token.EQL {
		return false
	}
	sel, ok := bin.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != field {
		return false
	}
	rhs, ok := bin.Y.(*ast.SelectorExpr)
	return ok && rhs.Sel.Name == constant
}
