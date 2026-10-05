package users

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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

// session_key and participant_key are what every row of the dataset
// joins on, and SELECT * over the view delivers them as Postgres uuids.
// pgx decodes a uuid to [16]byte rather than to a string, so without an
// explicit case they render as "[207 88 141 ...]": both join keys
// unusable, while the row count and the session count stay correct
// because a byte array stringifies consistently. Nothing about the
// export would have looked wrong, which is why this is pinned.
func TestUUIDsRenderAsUUIDs(t *testing.T) {
	// The byte form of cf588df7-3ded-45d6-a141-e35ac0c648b7.
	raw := [16]byte{
		0xcf, 0x58, 0x8d, 0xf7, 0x3d, 0xed, 0x45, 0xd6,
		0xa1, 0x41, 0xe3, 0x5a, 0xc0, 0xc6, 0x48, 0xb7,
	}
	const want = "cf588df7-3ded-45d6-a141-e35ac0c648b7"
	if got := csvValue(raw); got != want {
		t.Errorf("csvValue(uuid bytes) = %q, want %q", got, want)
	}
}

// pgx decodes a uuid to [16]byte via its codec, not to a string. If
// that ever changes the case above becomes dead and the live decode
// starts falling through to fmt.Sprint again, so assert the shape this
// code is written against rather than trusting it.
func TestPgxStillDecodesUUIDToAByteArray(t *testing.T) {
	m := pgtype.NewMap()
	ti, ok := m.TypeForOID(pgtype.UUIDOID)
	if !ok {
		t.Fatal("pgx has no uuid type registered")
	}
	raw := []byte{
		0xcf, 0x58, 0x8d, 0xf7, 0x3d, 0xed, 0x45, 0xd6,
		0xa1, 0x41, 0xe3, 0x5a, 0xc0, 0xc6, 0x48, 0xb7,
	}
	v, err := ti.Codec.DecodeValue(m, pgtype.UUIDOID, pgtype.BinaryFormatCode, raw)
	if err != nil {
		t.Fatalf("decode a uuid: %v", err)
	}
	if _, isArray := v.([16]byte); !isArray {
		t.Fatalf("pgx decoded a uuid to %T, not [16]byte: csvValue needs a case for it", v)
	}
	if got := csvValue(v); got != "cf588df7-3ded-45d6-a141-e35ac0c648b7" {
		t.Errorf("a uuid straight from the codec rendered as %q", got)
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
