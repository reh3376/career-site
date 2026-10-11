package handlers

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// No method on *Admin may gate itself. The interceptor is the gate.
//
// **This test used to assert the opposite**, and the reversal is the
// point of S5 in docs/sprint-auth-interceptor.md. Until 2026-10-11 the
// proto's AUTH_LEVEL_ADMIN was documentation and each handler was
// responsible for its own check, so this file required all 69 of them to
// call `requireAdmin`. Since S4 the interceptor refuses every caller
// below the declared level before the handler runs, which makes a
// handler's own check unreachable: it cannot fire, and it can only
// disagree with the interceptor. S5 removed them, and `requireAdmin` no
// longer exists.
//
// **Why the original existed, because it is the reason the interceptor
// was built.** On 2026-10-05 ExportDecisionTestData shipped without the
// call. It returns the whole decision test dataset as a CSV, and for
// about five hours anyone could fetch it with no session at all: a plain
// unauthenticated POST returned 200 and 21,708 bytes of participant
// data. It was found by reading the handler while changing it for
// something else, which is luck rather than a process.
//
// That hole is now structurally impossible rather than tested for. A new
// admin RPC is covered the moment it is declared, because the
// interceptor reads the declaration; it does not depend on the author
// remembering a line. Verified live on 2026-10-11: three *unimplemented*
// admin procedures, which have no handler and therefore no possible
// handler check, went from 200/501 to 401 when enforcement came on.
//
// So what is guarded here is the reverse risk: a handler quietly growing
// its own check again, or looking the session up a second time after the
// resolver already did.
func TestNoAdminMethodGatesItself(t *testing.T) {
	// Match the declaration only, never the signature's shape. The
	// first version of this test required `ctx context.Context, req
	// *connect.Request[...]` on one line, which matched 12 of the 66
	// methods in this package: the other 54 wrap the parameter list
	// across lines. It reported a clean audit while checking under a
	// fifth of the surface, and the ad-hoc scan that first found the
	// ExportDecisionTestData hole had exactly the same blind spot, so
	// "only that one is missing" was never established.
	//
	// The body is then found by brace balancing from the first `{`
	// after the declaration, which is safe here because no parameter
	// list or return type in this package contains a brace.
	sig := regexp.MustCompile(`func \(a \*Admin\) ([A-Z]\w+)\(`)

	// Anything that would mean the handler is deciding for itself, or
	// resolving a session the interceptor has already resolved.
	forbidden := map[string]string{
		"requireAdmin": "the interceptor already refused every non-admin; " +
			"a second check cannot fire and can only disagree with it",
		"requireChatAdmin":   "same: the interceptor decides the level",
		"requireMember":      "same: the interceptor decides the level",
		"LookupSessionUser":  "the resolver already loaded the account and put it on the request context; read it with RequireCaller",
		"lookupSessionToken": "the resolver already loaded the account; read it with RequireCaller",
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the handlers package: %v", err)
	}

	var checked int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "admin") || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		src := string(b)

		for _, m := range sig.FindAllStringSubmatchIndex(src, -1) {
			method := src[m[2]:m[3]]
			open := strings.Index(src[m[1]:], "{")
			if open < 0 {
				t.Fatalf("%s.%s: no function body found", name, method)
			}
			body := bodyAfter(src, m[1]+open+1)
			checked++

			for call, why := range forbidden {
				if strings.Contains(body, call) {
					t.Errorf("%s.%s calls %s. Since S4 the interceptor is the "+
						"gate: %s.", name, method, call, why)
				}
			}
		}
	}
	// A floor, not the exact count, so adding an RPC does not fail the
	// build. It exists because the first version of this test matched
	// 12 methods and passed, which is the failure mode it guards
	// against: a check that asserts nothing reports success. That
	// matters more now than it did, because the assertion is a negative
	// one, and a negative assertion over an empty set is always true.
	if checked < 60 {
		t.Fatalf("only matched %d admin methods, expected at least 60: the declaration pattern has drifted "+
			"and this test is now asserting almost nothing", checked)
	}
	t.Logf("%d admin methods checked; none gate themselves", checked)
}

// The gate helpers are gone, and should stay gone. Asserted separately
// from the per-method check above because a helper can exist while being
// called from nowhere, and a dead gate invites someone to start calling
// it again.
func TestTheGateHelpersAreGone(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the handlers package: %v", err)
	}

	defRe := regexp.MustCompile(`func (?:\(h \*[A-Za-z]+\) )?require(?:Admin|Member|ChatAdmin)[\[(]`)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if loc := defRe.FindString(string(b)); loc != "" {
			t.Errorf("%s still defines a gate helper (%q). The interceptor "+
				"decides the level now; a handler-side gate can only "+
				"disagree with it, and S5 removed these.", name, loc)
		}
	}
}

// bodyAfter returns the brace-balanced function body starting just
// after the opening brace at index i.
func bodyAfter(src string, i int) string {
	depth := 1
	start := i
	for i < len(src) && depth > 0 {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
		}
		i++
	}
	return src[start:i]
}
