package prompts

import (
	"strings"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// The judge runs one call per requirement, fourteen of them on a real
// posting. Each call costs a full prompt evaluation unless the start of
// the prompt is byte for byte what the previous call started with, in
// which case Ollama reuses the cached KV for that span. Measured on the
// production box: 152 s cold, 2 s cached, for the same 2,515 tokens.
//
// So the profile block is not a formatting choice, it is the whole
// reason a review takes thirty minutes instead of fifty. It regressed
// once already, silently, by being gathered from retrieved evidence
// that differs per requirement. These tests fail if that happens again.

func hit(id int64, kind, text string) users.CorpusHit {
	return users.CorpusHit{
		Chunk:      users.CorpusChunk{ID: id, Text: text},
		SourceKind: kind,
		Title:      "t",
	}
}

func profileFixture() []users.CorpusHit {
	return []users.CorpusHit{
		hit(2, ProfileSourceKind, "Roger has run control systems for decades."),
		hit(1, ProfileSourceKind, "Roger is a controls and manufacturing-systems engineer."),
	}
}

// The property that matters: two different requirements, with different
// retrieved evidence, must produce prompts that begin identically.
func TestJudgePromptsShareAPrefix(t *testing.T) {
	profile := profileFixture()
	a := RenderJudgeUser(
		[]Requirement{{ID: "r1", Text: "Five years of DCS work", Category: "skill", Weight: 3}},
		map[string][]users.CorpusHit{"r1": {hit(10, "resume", "Commissioned a DCS at a chemical plant.")}},
		profile,
	)
	b := RenderJudgeUser(
		[]Requirement{{ID: "r2", Text: "Leading an automation team", Category: "leadership", Weight: 2}},
		map[string][]users.CorpusHit{"r2": {hit(11, "article", "Led a team of engineers across three sites.")}},
		profile,
	)

	shared := strings.Index(a, "Judge each requirement")
	if shared <= 0 {
		t.Fatal("the rendered prompt lost its fixed lead-in")
	}
	if a[:shared] != b[:shared] {
		t.Fatalf("judge prompts diverge before the requirement:\n--- a ---\n%s\n--- b ---\n%s",
			a[:shared], b[:shared])
	}
	if !strings.Contains(a[:shared], "<candidate_profile>") {
		t.Fatal("the shared prefix must carry the profile, which is the span worth caching")
	}
}

// The order of the profile as it arrives must not change the prompt. It
// comes from a query, and a query without an ORDER BY is entitled to
// return rows in any order it likes.
func TestProfileOrderDoesNotChangeThePrefix(t *testing.T) {
	forward := profileFixture()
	backward := []users.CorpusHit{forward[1], forward[0]}
	reqs := []Requirement{{ID: "r1", Text: "Five years of DCS work"}}
	ev := map[string][]users.CorpusHit{"r1": {hit(10, "resume", "x")}}

	if RenderJudgeUser(reqs, ev, forward) != RenderJudgeUser(reqs, ev, backward) {
		t.Fatal("the profile block must be sorted, or the prefix changes run to run")
	}
}

// Profile chunks that also arrive as retrieved evidence must not be
// printed twice: once in the shared block and again under the
// requirement, where they would push the two apart.
func TestProfileIsNotRepeatedInEvidence(t *testing.T) {
	profile := profileFixture()
	out := RenderJudgeUser(
		[]Requirement{{ID: "r1", Text: "Five years of DCS work"}},
		map[string][]users.CorpusHit{"r1": {
			profile[0],
			hit(10, "resume", "Commissioned a DCS at a chemical plant."),
		}},
		profile,
	)
	if strings.Count(out, profile[0].Chunk.Text) != 1 {
		t.Fatalf("a profile chunk appears %d times, expected once",
			strings.Count(out, profile[0].Chunk.Text))
	}
}

// With no profile at all the prompt still has to render. The facts
// sheet is a row in a table and the query for it can fail.
func TestRendersWithoutAProfile(t *testing.T) {
	out := RenderJudgeUser(
		[]Requirement{{ID: "r1", Text: "Five years of DCS work"}},
		map[string][]users.CorpusHit{"r1": {hit(10, "resume", "Commissioned a DCS.")}},
		nil,
	)
	if !strings.Contains(out, "Five years of DCS work") {
		t.Fatal("the requirement must survive an absent profile")
	}
	if strings.Contains(out, "<candidate_profile>") {
		t.Fatal("an empty profile must not render an empty block")
	}
}
