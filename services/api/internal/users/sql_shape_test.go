package users

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every INSERT ... VALUES in this package must list as many value
// expressions as it lists columns.
//
// This exists because that invariant broke in production and nothing
// caught it. On 2026-10-05 the dt_sessions insert was changed from two
// columns (attempt_no, attempt_basis) to three (prior_by_account,
// prior_by_email, prior_by_cookie) and the VALUES list kept fifteen
// placeholders against sixteen columns. Postgres refused every insert
// with
//
//	INSERT has more target columns than expressions (SQLSTATE 42601)
//
// so StartSession returned 500 and **no participant could begin the
// test for about an hour and forty minutes.** The whole instrument was
// down, during the evening the owner had volunteers lined up.
//
// Every gate in the project passed it. `go build` and `go vet` cannot
// see inside a string literal. `gofmt` has no opinion. The migrations
// job replays the schema but never executes a line of Go. The web
// tests do not reach the api. There was no layer where this was
// catchable, which is why the fix is a test rather than more care.
//
// Parsing source is crude, and it is the only way to check this without
// a live database in the api job. The same technique already guards the
// event registry and the admin auth gate in this repository, and both
// of those also exist because the thing they check failed silently
// first.
func TestEveryInsertMatchesItsValueCount(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the users package: %v", err)
	}

	var checked int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		src := string(b)

		for _, ins := range findInserts(src) {
			checked++
			if ins.cols != ins.vals {
				t.Errorf("%s: INSERT INTO %s lists %d columns but %d values. "+
					"Postgres refuses this at run time with SQLSTATE 42601 and no build "+
					"or vet step can see inside the string literal",
					name, ins.table, ins.cols, ins.vals)
			}
		}
	}

	// A parser that stops matching would pass by checking nothing, which
	// is the failure mode these source-parsing tests must guard against
	// in themselves.
	if checked < 5 {
		t.Fatalf("only matched %d inserts with a VALUES list, expected at least 5: "+
			"the parser has drifted and this test is asserting almost nothing", checked)
	}
}

type insertShape struct {
	table      string
	cols, vals int
}

// findInserts locates every `INSERT INTO t (...) VALUES (...)` and
// counts both lists.
//
// Paren-balanced rather than regex-delimited. The first version used
// `\(([^)]*)\)` and truncated at the first closing paren, so a value
// list containing `now()` or `coalesce(a, b)` was read short and seven
// perfectly correct inserts were reported as broken. The lesson is the
// same one this file is about: a check that is wrong in the direction of
// shouting is still wrong, and it would have been ignored into
// uselessness within a week.
//
// A multi-row VALUES (...),(...) is skipped rather than guessed at;
// none exist in this package and inventing a rule for one would be
// guessing at a shape nobody has written.
func findInserts(src string) []insertShape {
	var out []insertShape
	lower := strings.ToLower(src)
	for i := 0; ; {
		j := strings.Index(lower[i:], "insert into ")
		if j < 0 {
			break
		}
		at := i + j
		i = at + len("insert into ")

		rest := src[i:]
		open := strings.Index(rest, "(")
		if open < 0 {
			continue
		}
		table := strings.TrimSpace(rest[:open])
		if table == "" || strings.ContainsAny(table, " \t\n") {
			// `INSERT INTO t SELECT ...` or similar: no column list.
			continue
		}
		cols, after := balanced(rest, open)
		if cols == "" {
			continue
		}
		// VALUES must be the next token after the column list. An
		// unbounded search found the word in a LATER statement and
		// paired `INSERT INTO eval_run_documents (...) SELECT ...` with
		// a different insert's value list, reporting 7 columns against 9
		// values on code that has run in production for a week. An
		// INSERT ... SELECT has no value list to check and is skipped.
		tail := strings.TrimLeft(rest[after:], " \t\n\r")
		if !strings.HasPrefix(strings.ToLower(tail), "values") {
			continue
		}
		vi := strings.Index(rest[after:], tail[:6])
		vopen := strings.Index(rest[after+vi:], "(")
		if vopen < 0 {
			continue
		}
		vals, vafter := balanced(rest[after+vi:], vopen)
		if vals == "" {
			continue
		}
		// Multi-row insert: skip rather than guess.
		if strings.HasPrefix(strings.TrimSpace(rest[after+vi+vafter:]), ",") {
			continue
		}
		out = append(out, insertShape{
			table: table,
			cols:  countColumns(cols),
			vals:  countColumns(vals),
		})
	}
	return out
}

// balanced returns the contents of the parenthesised group starting at
// `open`, and the index just past its closing paren.
func balanced(s string, open int) (string, int) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[open+1 : i], i + 1
			}
		}
	}
	return "", 0
}

// countColumns counts comma-separated items, ignoring commas nested
// inside parentheses so `coalesce(a, b)` and `now()` each count once,
// and ignoring line comments.
func countColumns(list string) int {
	depth, n, seen := 0, 0, false
	flush := func() {
		if seen {
			n++
			seen = false
		}
	}
	for _, line := range strings.Split(list, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		for _, r := range line {
			switch {
			case r == '(':
				depth++
				seen = true
			case r == ')':
				depth--
				seen = true
			case r == ',' && depth == 0:
				flush()
			case r != ' ' && r != '\t':
				seen = true
			}
		}
	}
	flush()
	return n
}

// A worked example of the failure, so the message above is checkable
// rather than a story. These are the exact two lists from the broken
// commit.
func TestTheShapeThatTookTheTestDown(t *testing.T) {
	cols := `participant_id, instrument_version, item_set_version, key_version,
		   audio_mode, device_class, tap_check_passed, baseline_rt_ms, baseline_rt_sd_ms,
		   is_repeat, repeat_matched_by, visitor_key, is_synthetic,
		   prior_by_account, prior_by_email, prior_by_cookie`
	if got := countColumns(cols); got != 16 {
		t.Fatalf("countColumns() = %d on the real list, want 16", got)
	}
	broken := "$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15"
	if n := len(regexp.MustCompile(`\$\d+`).FindAllString(broken, -1)); n != 15 {
		t.Fatalf("the broken list had %d placeholders, want 15", n)
	}
	// 16 columns against 15 values is what production refused for an
	// hour and forty minutes, and what the test above now fails the
	// build for.
	if countColumns(cols) == countColumns(broken) {
		t.Fatal("the worked example no longer demonstrates the mismatch it describes")
	}
}
