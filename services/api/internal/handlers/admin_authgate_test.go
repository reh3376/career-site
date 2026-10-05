package handlers

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every method on *Admin must gate itself on session and admin role.
//
// The proto declares AUTH_LEVEL_ADMIN on all of them and that is not
// enforcement: the auth interceptor has not landed, so the declaration
// is documentation and the handler is the gate. The comment on the
// Admin type says exactly that, and it was not enough.
//
// On 2026-10-05 ExportDecisionTestData shipped without the call. It
// returns the whole decision test dataset as a CSV, and for about five
// hours anyone could fetch it with no session at all: a plain unauthed
// POST returned 200 and 21,708 bytes of participant data. It was found
// by reading the handler while changing it for something else, which is
// luck rather than a process.
//
// Parsing the source is ugly, and it is the only way to assert "every
// method calls this" without a running server and a session per method.
// The same technique already guards the event registry in this package
// for the same reason.
func TestEveryAdminMethodGatesOnAdmin(t *testing.T) {
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
			if !strings.Contains(body, "requireAdmin") {
				t.Errorf("%s.%s does not call requireAdmin: the proto's AUTH_LEVEL_ADMIN is "+
					"documentation, not enforcement, so this RPC is open to anyone", name, method)
			}
		}
	}
	// A floor, not the exact count, so adding an RPC does not fail the
	// build. It exists because the first version of this test matched
	// 12 methods and passed, which is the failure mode it guards
	// against: a check that asserts nothing reports success.
	if checked < 60 {
		t.Fatalf("only matched %d admin methods, expected at least 60: the declaration pattern has drifted "+
			"and this test is now asserting almost nothing", checked)
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
