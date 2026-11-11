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
// whose parties the extractor left out. Everything the judge sees or
// the arithmetic reads is part of the key, so a judgment's
// requirement_id is enough to say exactly what it was judged against.
func TestJudgedKey(t *testing.T) {
	a := Draft(RequirementInput{LocalID: "r1", Text: "x", Category: "must", Weight: 2}, nil, Provenance{})
	b := a
	b.NamedParties = nil
	if !sameJudged(a, b) {
		t.Errorf("nil and empty parties key differently:\n%s\n%s", judgedKey(a), judgedKey(b))
	}
	for name, change := range map[string]func(*Requirement){
		"a named party":         func(q *Requirement) { q.NamedParties = []string{"Acme"} },
		"the type":              func(q *Requirement) { q.Type = "tool" },
		"the class":             func(q *Requirement) { q.Class = ClassIdeal },
		"the evidence standard": func(q *Requirement) { q.EvidenceStandard = "something else" },
		"the equivalency":       func(q *Requirement) { q.Equivalency = "five years in lieu" },
		"a threshold":           func(q *Requirement) { y := 3.0; q.Thresholds.MinYears = &y },
		"inclusion":             func(q *Requirement) { q.Included = !q.Included },
	} {
		c := a
		change(&c)
		if sameJudged(a, c) {
			t.Errorf("%s did not change the key", name)
		}
	}
	// Provenance is not judged: the same requirement extracted by a
	// later prompt version is the same version.
	d := Draft(RequirementInput{LocalID: "r1", Text: "x", Category: "must", Weight: 2}, nil, Provenance{PromptFingerprint: "v9:beef"})
	if !sameJudged(a, d) {
		t.Error("the extraction provenance changed the key; a prompt bump would re-version every row")
	}
}
