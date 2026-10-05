package handlers

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// How the query console renders a cell.
//
// The console is the one surface where the owner reads a view directly,
// and docs/metrics.md sends him here for the decision test's seven
// views. A cell that renders wrong there is not a cosmetic problem: it
// is a number or an identifier read off the screen and believed.

func TestQueryConsoleCellRendering(t *testing.T) {
	// The byte form of cf588df7-3ded-45d6-a141-e35ac0c648b7.
	uuid := [16]byte{
		0xcf, 0x58, 0x8d, 0xf7, 0x3d, 0xed, 0x45, 0xd6,
		0xa1, 0x41, 0xe3, 0x5a, 0xc0, 0xc6, 0x48, 0xb7,
	}

	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			// pgx decodes uuid to [16]byte, which is an array and so
			// misses the []byte case. It used to fall through to %v and
			// print "[207 88 141 247 ...]". Every public_id and
			// session_key in the database is a uuid.
			name: "a uuid reads as a uuid, not as a byte array",
			in:   uuid,
			want: "cf588df7-3ded-45d6-a141-e35ac0c648b7",
		},
		{
			// Still distinct from a uuid: real bytea gets the \x form so
			// it cannot be mistaken for text.
			name: "bytea keeps its escape form",
			in:   []byte{0xde, 0xad},
			want: `\xdead`,
		},
		{
			name: "a timestamp is normalised to UTC",
			in:   time.Date(2026, 10, 5, 14, 30, 0, 0, time.FixedZone("CDT", -5*3600)),
			want: "2026-10-05T19:30:00Z",
		},
		{name: "a string passes through", in: "d4_plus1", want: "d4_plus1"},
		{name: "a number renders bare", in: 42, want: "42"},
		{name: "a bool renders as a word", in: true, want: "true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stringify(tc.in); got != tc.want {
				t.Errorf("stringify(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Asserts the shape stringify is written against. If pgx ever decodes a
// uuid to something else, the [16]byte case becomes dead code while the
// live path silently goes back to printing a byte array.
func TestPgxDecodesUUIDToAByteArray(t *testing.T) {
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
		t.Fatalf("pgx decoded a uuid to %T, not [16]byte: stringify needs a case for it", v)
	}
	if got := stringify(v); got != "cf588df7-3ded-45d6-a141-e35ac0c648b7" {
		t.Errorf("a uuid straight from the codec rendered as %q", got)
	}
}
