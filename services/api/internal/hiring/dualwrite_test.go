package hiring

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// A posting's first line is the title when the submitter gave no role
// hint, and it is full of multibyte punctuation; the cut must never
// leave a partial rune behind, or Postgres refuses the row and the run
// gets no rows at all.
func TestTrimTitleKeepsValidUTF8(t *testing.T) {
	long := strings.Repeat("Controls Engineer – ", 20) // the en dash is three bytes
	got := trimTitle(long + "\nsecond line")
	if !utf8.ValidString(got) {
		t.Fatalf("trimTitle produced invalid UTF-8: %q", got)
	}
	if len(got) > 160 || strings.Contains(got, "\n") {
		t.Errorf("trimTitle: len=%d, has newline=%v", len(got), strings.Contains(got, "\n"))
	}
	if trimTitle("  Plain title  \r\nrest") != "Plain title" {
		t.Errorf("trimTitle did not take the first line trimmed: %q", trimTitle("  Plain title  \r\nrest"))
	}
}

// nil and empty named parties are the same absence; if they keyed
// differently every run would mint a new version of every requirement
// whose parties the extractor left out.
func TestRequirementKeyTreatsNilAndEmptyPartiesAlike(t *testing.T) {
	a := RequirementInput{LocalID: "r1", Text: "x", Category: "must", Weight: 2}
	b := a
	b.NamedParties = []string{}
	if a.key() != b.key() {
		t.Errorf("nil and empty parties key differently:\n%s\n%s", a.key(), b.key())
	}
	c := a
	c.NamedParties = []string{"Acme"}
	if a.key() == c.key() {
		t.Error("a named party did not change the key")
	}
	d := a
	d.Type = "tool"
	if a.key() != d.key() {
		t.Error("the type is part of the key; a heuristic change would re-version every row")
	}
}
