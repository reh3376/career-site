package users

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"
)

// How a cell is rendered decides what the analysis reads, and a wrong
// rendering is silent: the file parses, the column loads, and the number
// is subtly not what the database held.
//
// The structural guarantees about this export (no name, no email
// address, no chosen_index, columns following the view) are asserted in
// SQL instead, by testdata/dt_metric_assertions.sql against a real
// replay, because they are properties of v_dt_answers rather than of
// this package.

func TestCellRendering(t *testing.T) {
	ts := time.Date(2026, 10, 5, 14, 30, 0, 0, time.FixedZone("CDT", -5*3600))

	tests := []struct {
		name string
		in   any
		want string
	}{
		{"a null is an empty field, which reads as NA", nil, ""},
		{"a string passes through", "Engineer", "Engineer"},
		{"an empty string is also empty: see the note on csvValue", "", ""},
		{"true is a word, not 1: the column is a fact, not a count", true, "true"},
		{"false likewise", false, "false"},
		{"a timestamp is normalised to UTC", ts, "2026-10-05T19:30:00Z"},
		{"an int renders bare", 42, "42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := csvValue(tc.in); got != tc.want {
				t.Errorf("csvValue(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A timestamp keeping its local offset is a day lost to timezone
// archaeology later, and worse, it is the kind of thing that looks
// right. Two sessions an hour apart must not read as the same instant
// because one was recorded under a different offset.
func TestTimestampsNormaliseRatherThanCarryAnOffset(t *testing.T) {
	utc := time.Date(2026, 10, 5, 19, 30, 0, 0, time.UTC)
	local := utc.In(time.FixedZone("ACDT", 10*3600+1800))
	if csvValue(utc) != csvValue(local) {
		t.Errorf("the same instant rendered two ways: %q and %q", csvValue(utc), csvValue(local))
	}
	if strings.Contains(csvValue(local), "+") {
		t.Errorf("csvValue(%v) kept an offset: %q", local, csvValue(local))
	}
}

// Free-text fields come from participants, so the export has to survive
// an occupation with a comma, a quote or a newline in it without
// shifting every later column by one. encoding/csv handles this; the
// test is here so that a future hand-rolled writer cannot quietly
// replace it.
func TestFreeTextCannotBreakTheColumns(t *testing.T) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	nasty := []string{
		csvValue("Engineer, Process"),
		csvValue(`said "no"`),
		csvValue("two\nlines"),
		csvValue(nil),
		csvValue(true),
	}
	if err := w.Write([]string{"a", "b", "c", "d", "e"}); err != nil {
		t.Fatalf("header: %v", err)
	}
	if err := w.Write(nasty); err != nil {
		t.Fatalf("row: %v", err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	recs, err := csv.NewReader(strings.NewReader(b.String())).ReadAll()
	if err != nil {
		t.Fatalf("the file we wrote does not parse: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2 (a newline inside a field split the row)", len(recs))
	}
	if len(recs[1]) != 5 {
		t.Fatalf("got %d columns, want 5 (a comma inside a field shifted the row)", len(recs[1]))
	}
	for i, want := range []string{"Engineer, Process", `said "no"`, "two\nlines", "", "true"} {
		if recs[1][i] != want {
			t.Errorf("column %d round-tripped as %q, want %q", i, recs[1][i], want)
		}
	}
}

func TestIndexOf(t *testing.T) {
	cols := []string{"answer_id", "session_key", "block_no"}
	if got := indexOf(cols, "session_key"); got != 1 {
		t.Errorf("indexOf(session_key) = %d, want 1", got)
	}
	// Missing has to be -1 rather than 0, or the session count would
	// silently start counting the first column instead.
	if got := indexOf(cols, "nope"); got != -1 {
		t.Errorf("indexOf(missing) = %d, want -1", got)
	}
}
