package jd

import (
	"testing"

	"github.com/reh3376/career-site/services/api/internal/prompts"
)

func req(text string, parties ...string) prompts.Requirement {
	return prompts.Requirement{ID: "r1", Text: text, Category: "must", Weight: 3, NamedParties: parties}
}

func TestApplyRelationshipRule(t *testing.T) {
	t.Parallel()

	// The failure this rule was written for. The judge answered "met"
	// because the candidate's project calls LLM APIs; the evidence names
	// none of these companies as organisations he worked with.
	t.Run("named parties absent from the evidence, met becomes partial", func(t *testing.T) {
		t.Parallel()
		r := req("Collaborate with external partners like Anthropic, AWS and OpenAI", "Anthropic", "AWS", "OpenAI")
		v, note := applyRelationshipRule(r, "met", nil)
		if v != "partial" {
			t.Fatalf("verdict = %q, want partial", v)
		}
		if note == "" {
			t.Fatal("a downgrade nobody can read is indistinguishable from the model having said so")
		}
	})

	t.Run("one named party evidenced is enough", func(t *testing.T) {
		t.Parallel()
		r := req("Collaborate with partners like Anthropic, AWS and OpenAI", "Anthropic", "AWS", "OpenAI")
		if v, note := applyRelationshipRule(r, "met", []string{"AWS"}); v != "met" || note != "" {
			t.Fatalf("got (%q, %q), want met untouched", v, note)
		}
	})

	// Most requirements name nobody, and the rule must never touch them.
	t.Run("requirement names nobody, nothing happens", func(t *testing.T) {
		t.Parallel()
		r := req("Strong understanding of software architecture and design patterns")
		if v, note := applyRelationshipRule(r, "met", nil); v != "met" || note != "" {
			t.Fatalf("got (%q, %q), want met untouched", v, note)
		}
	})

	t.Run("never strengthens a verdict", func(t *testing.T) {
		t.Parallel()
		r := req("Collaborate with Anthropic", "Anthropic")
		for _, start := range []string{"unmet", "partial"} {
			if v, _ := applyRelationshipRule(r, start, []string{"Anthropic"}); v != start {
				t.Fatalf("a %q verdict became %q; the rule must only weaken", start, v)
			}
		}
	})

	// Matching is generous on purpose: a false downgrade costs more than
	// a loose match, because it understates a real candidate.
	t.Run("names survive how people write them", func(t *testing.T) {
		t.Parallel()
		cases := []struct{ want, got string }{
			{"AWS", "Amazon Web Services (AWS)"},
			{"Amazon Web Services", "AWS"},
			{"OpenAI", "OpenAI, Inc."},
			{"Anthropic", "anthropic"},
			{"Joy Global", "Joy Global Inc"},
		}
		for _, c := range cases {
			r := req("Worked with "+c.want, c.want)
			if v, _ := applyRelationshipRule(r, "met", []string{c.got}); v != "met" {
				t.Fatalf("%q should match %q, but the verdict was downgraded", c.got, c.want)
			}
		}
	})

	t.Run("different companies do not match", func(t *testing.T) {
		t.Parallel()
		r := req("Collaborate with Anthropic", "Anthropic")
		if v, _ := applyRelationshipRule(r, "met", []string{"Microsoft", "Sazerac"}); v != "partial" {
			t.Fatalf("verdict = %q, want partial: neither name is the one asked for", v)
		}
	})

	// An empty string in either list must not match everything, which a
	// naive containment check would do.
	t.Run("empty names match nothing", func(t *testing.T) {
		t.Parallel()
		r := req("Collaborate with Anthropic", "Anthropic")
		if v, _ := applyRelationshipRule(r, "met", []string{"", "   ", "!!"}); v != "partial" {
			t.Fatalf("verdict = %q, want partial: blanks are not a match", v)
		}
	})
}
