package jd

import (
	"strings"

	"github.com/reh3376/career-site/services/api/internal/prompts"
)

// Relationship requirements are decided in code, for the same reason
// duration requirements are.
//
// The failure this answers: asked whether the candidate had
// "collaborated with external partners like Anthropic, AWS and OpenAI",
// the judge answered met, reasoning that his memory-graph project
// "integrates with LLMs" and "supports collaboration with external AI
// providers through gRPC". Calling a company's API is not a working
// relationship with that company, and no amount of telling the model so
// changed its mind.
//
// The division of labour is the same as for durations. Which
// organisations a requirement names is settled once, at extraction,
// because that is a fact about the posting. Which organisations the
// evidence names is reported by the judge, because that is a fact about
// the documents. Whether the second satisfies the first is a set
// comparison, and set comparisons belong in code.

// applyRelationshipRule downgrades a "met" on a requirement that names
// organisations when the evidence names none of them.
//
// Like the duration rule it only ever weakens, and it weakens to
// "partial" rather than "unmet": the candidate may well do the kind of
// work the requirement describes, and what is missing is the named
// relationship rather than the capability.
//
// Matching is deliberately generous. A requirement naming "Amazon Web
// Services" is satisfied by evidence naming "AWS", and the comparison
// ignores case and surrounding punctuation, because the cost of a false
// downgrade is higher than the cost of letting a loose match through.
func applyRelationshipRule(req prompts.Requirement, verdict string, partiesEvidenced []string) (string, string) {
	if verdict != "met" || len(req.NamedParties) == 0 {
		return verdict, ""
	}
	for _, want := range req.NamedParties {
		for _, got := range partiesEvidenced {
			if partyMatches(want, got) {
				return verdict, ""
			}
		}
	}
	return "partial", "names " + strings.Join(req.NamedParties, ", ") +
		"; the evidence names none of them as organisations worked with"
}

// partyMatches compares two organisation names loosely enough to
// survive the ways people write them down.
func partyMatches(a, b string) bool {
	na, nb := normaliseParty(a), normaliseParty(b)
	if na == "" || nb == "" {
		return false
	}
	// Containment either way covers "AWS" against "Amazon Web Services
	// (AWS)" and "OpenAI" against "OpenAI, Inc".
	if na == nb || strings.Contains(na, nb) || strings.Contains(nb, na) {
		return true
	}
	// Then the acronym, because a posting writes "AWS" where a document
	// writes "Amazon Web Services" and neither contains the other.
	return acronymOf(na) == nb || acronymOf(nb) == na
}

// acronymOf returns the initials of a multi-word name, or "" when the
// name is a single word and so is already its own shortest form.
//
// Capped at five letters: beyond that an "acronym" is a coincidence of
// first letters rather than a name anyone uses, and matching on it
// would start joining unrelated companies together.
func acronymOf(normalised string) string {
	words := strings.Fields(normalised)
	if len(words) < 2 || len(words) > 5 {
		return ""
	}
	var b strings.Builder
	for _, w := range words {
		b.WriteByte(w[0])
	}
	return b.String()
}

// normaliseParty strips the decoration companies accumulate in prose.
func normaliseParty(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, suffix := range []string{" inc.", " inc", " llc", " ltd", " limited", " corp.", " corp", " co.", " gmbh"} {
		s = strings.TrimSuffix(s, suffix)
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune(' ')
		}
	}
	return strings.TrimSpace(b.String())
}
