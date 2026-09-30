package handlers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/events"
)

// Every event this package emits must be in the registry, with every
// prop it passes declared.
//
// This exists because both halves failed silently in shipped code and
// were only found by running the thing and reading a log line.
//
//   - An unregistered name is refused by events.Record and logged as a
//     warning. The RPC still succeeds, so nothing a caller sees is any
//     different, and the event is simply never recorded. Four
//     admin.qa_entry_* events shipped that way, and this test then
//     found two that had been missing far longer: jd.quota_blocked and
//     admin.jd_limit_changed.
//   - A prop outside the spec's list is dropped by Record with no
//     warning at all, which is worse. admin.decision_reviewed was
//     given "kind", "corrected" and "dimensions" precisely so the
//     growth of the training set could be read off the event stream,
//     and all three were being discarded.
//
// Both are the kind of mistake that stays invisible until someone asks
// the data a question months later and finds it was never there.
// Parsing the source is ugly, and it is the only way to check a call
// site against a registry without running every path that reaches one.
func TestEmittedEventsAreRegisteredWithTheirProps(t *testing.T) {
	fset := token.NewFileSet()
	dir, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the handlers package: %v", err)
	}

	var checked int
	for _, entry := range dir {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name != "requestEvent" || len(call.Args) < 2 {
				return true
			}
			where := fset.Position(call.Pos())

			eventName, ok := stringLit(call.Args[1])
			if !ok {
				// A computed name cannot be checked here. None exist
				// today; if one appears it needs its own guard.
				t.Errorf("%s: requestEvent called with a non-literal event name", where)
				return true
			}
			checked++

			spec, known := events.Registry[eventName]
			if !known {
				t.Errorf("%s: event %q is emitted but missing from events.Registry, so it is never recorded",
					where, eventName)
				return true
			}
			for _, key := range propKeys(call.Args) {
				if !slices.Contains(spec.Props, key) {
					t.Errorf("%s: event %q passes prop %q, which is not in its registry spec and is dropped silently",
						where, eventName, key)
				}
			}
			return true
		})
	}

	if checked == 0 {
		t.Fatal("no requestEvent calls were found; this test has stopped checking anything")
	}
}

// propKeys returns the string keys of the first map literal among the
// arguments, which is where props are passed.
func propKeys(args []ast.Expr) []string {
	for _, a := range args {
		lit, ok := a.(*ast.CompositeLit)
		if !ok {
			continue
		}
		if _, isMap := lit.Type.(*ast.MapType); !isMap {
			continue
		}
		var out []string
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if k, ok := stringLit(kv.Key); ok {
				out = append(out, k)
			}
		}
		return out
	}
	return nil
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}
