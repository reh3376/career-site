package hiring

import (
	"sort"
	"strings"
	"unicode"
)

// KeywordRule names the baseline's arithmetic on every record that
// carries it, so the rule can be tightened later and old records still
// say which one produced them (FR-HF-12).
const KeywordRule = "kw1"

// KeywordBaseline is the comparison the false-negative claim needs
// (13 §0 decision 5): would a plain term-overlap filter over the same
// offered evidence have found this requirement? kw1: lower-cased,
// suffix-stemmed content terms of the requirement text, found when at
// least two of them (or every one, for a one-term requirement) occur
// in the evidence. No model, no embedding, no synonym list; that is
// the point. Returns the matched terms so a reader can see what the
// filter keyed on.
func KeywordBaseline(requirement string, evidence []string) (found bool, matched []string) {
	terms := contentTerms(requirement)
	if len(terms) == 0 {
		return false, nil
	}
	have := map[string]bool{}
	for _, e := range evidence {
		for _, t := range contentTerms(e) {
			have[t] = true
		}
	}
	for _, t := range terms {
		if have[t] {
			matched = append(matched, t)
		}
	}
	sort.Strings(matched)
	need := 2
	if len(terms) < need {
		need = len(terms)
	}
	return len(matched) >= need, matched
}

// contentTerms tokenises on anything that is not a letter or digit,
// lower-cases, drops stopwords and short tokens, stems by suffix, and
// returns the distinct stems in first-seen order.
func contentTerms(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	seen := map[string]bool{}
	var out []string
	for _, f := range fields {
		if len(f) < 3 || stopwords[f] {
			continue
		}
		t := stem(f)
		if len(t) < 3 || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// stem strips the common English suffixes a posting and a résumé
// disagree on (managing / managed / manager / management all become
// manag). Crude on purpose: kw1 is the screen being compared against,
// not a competitor to the judge.
func stem(w string) string {
	for _, suf := range []string{"ements", "ement", "ations", "ation", "ingly", "ities", "ity", "ings", "ing", "ers", "er", "ies", "ied", "ed", "es", "ly", "s"} {
		if strings.HasSuffix(w, suf) && len(w)-len(suf) >= 3 {
			w = w[:len(w)-len(suf)]
			break
		}
	}
	if strings.HasSuffix(w, "i") && len(w) > 3 {
		w = w[:len(w)-1] + "y"
	}
	return w
}

var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true, "this": true, "from": true,
	"are": true, "was": true, "were": true, "have": true, "has": true, "had": true, "will": true,
	"you": true, "your": true, "our": true, "their": true, "they": true, "them": true, "who": true,
	"not": true, "but": true, "all": true, "any": true, "can": true, "may": true, "must": true,
	"should": true, "would": true, "could": true, "able": true, "ability": true, "strong": true,
	"experience": true, "experienced": true, "years": true, "year": true, "plus": true,
	"required": true, "require": true, "requires": true, "requirement": true, "requirements": true,
	"preferred": true, "prefer": true, "knowledge": true, "skills": true, "skill": true,
	"working": true, "work": true, "within": true, "into": true, "over": true, "under": true,
	"about": true, "such": true, "than": true, "then": true, "when": true, "where": true, "which": true,
	"while": true, "also": true, "very": true, "more": true, "most": true, "other": true,
	"including": true, "include": true, "includes": true, "related": true, "relevant": true,
	"etc": true, "e.g": true, "i.e": true, "well": true, "good": true, "excellent": true,
	"demonstrated": true, "proven": true, "minimum": true, "least": true, "equivalent": true,
	"degree": true, "field": true, "role": true, "position": true, "candidate": true,
	"team": true, "teams": true, "environment": true, "using": true, "use": true, "used": true,
}
