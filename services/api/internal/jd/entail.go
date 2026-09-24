package jd

import (
	"regexp"
	"sort"
	"strings"
)

// Checking that a résumé line is supported by what it cites.
//
// Until now "verified" meant the cited chunk id was one of the ids
// offered to the model. That catches an invented citation and nothing
// else: a line could cite a real document that says nothing like it and
// still be called verified. The dangerous output of this system is a
// confident sentence with a real citation attached, because a citation
// is what stops a reader checking.
//
// The check here is deliberately not a model call. One call per line
// would add twenty minutes to a review that already takes fifty, and
// the session that produced the duration and relationship rules
// established the cheaper lesson: the model reports, the code decides.
// What code can decide here is narrow but exactly where fabrication
// shows up first, which is in the specifics.
//
//   - A quantity in the line that appears nowhere in what it cites.
//     "Reduced downtime 40 %" is either in the record or it is invented,
//     and no amount of paraphrase moves the digits.
//   - An organisation named in the line that appears nowhere in what it
//     cites. Same reasoning, and the acronym matching from the
//     relationship rule already knows that AWS and Amazon Web Services
//     are one employer.
//
// Prose is left alone. "Led the team through a difficult migration" is
// not checkable this way, and pretending otherwise would drop good
// lines to look rigorous.

// unsupported is what a line claims that its sources do not carry.
type unsupported struct {
	// Numbers present in the line and absent from every cited chunk.
	Numbers []string
	// Organisations present in the line and absent from every cited
	// chunk.
	Parties []string
}

func (u unsupported) any() bool { return len(u.Numbers) > 0 || len(u.Parties) > 0 }

// numberPattern pulls the digits out of a token that is already known
// to carry no letters.
var numberPattern = regexp.MustCompile(`([0-9][0-9,]*(?:\.[0-9]+)?)`)

// checkSupport reports what a line asserts that its cited chunks do not.
//
// sourceText is the concatenation of every chunk the line cites. An
// empty sourceText means nothing was found to check against, which is
// reported as no issues rather than as everything being unsupported:
// the caller has already refused a line with no usable sources, and
// inventing failures here would hide that distinction.
func checkSupport(line, sourceText string) unsupported {
	var out unsupported
	if strings.TrimSpace(sourceText) == "" {
		return out
	}
	haystackNums := numbersIn(sourceText)
	for n := range numbersIn(line) {
		if !haystackNums[n] {
			out.Numbers = append(out.Numbers, n)
		}
	}
	sort.Strings(out.Numbers)
	src := normaliseParty(sourceText)
	for _, p := range partiesIn(line) {
		if !partySupported(p, src) {
			out.Parties = append(out.Parties, p)
		}
	}
	return out
}

// numbersIn returns the quantities in s, normalised so that 1,200 and
// 1200 are the same claim.
// A token carrying letters is a part number, a model or a standard
// (S7-1500, ControlLogix, IEEE 1584), never a quantity being claimed.
// Reading digits out of those produced the first false positive this
// check ever had, on a line that was entirely true.
func numbersIn(s string) map[string]bool {
	out := map[string]bool{}
	for _, tok := range strings.Fields(s) {
		if hasLetter(tok) {
			continue
		}
		for _, m := range numberPattern.FindAllStringSubmatch(tok, -1) {
			n := strings.TrimSuffix(strings.ReplaceAll(m[1], ",", ""), ".")
			if n == "" {
				continue
			}
			// A leading zero is noise from a version or a code, not a claim.
			if len(n) > 1 && n[0] == '0' {
				continue
			}
			out[n] = true
		}
	}
	return out
}

func hasLetter(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func hasDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// partySupported reports whether a named organisation appears in the
// normalised source text, directly or as its acronym, which is how the
// relationship rule already treats AWS against Amazon Web Services.
func partySupported(party, normalisedSource string) bool {
	p := normaliseParty(party)
	if p == "" {
		return true
	}
	if strings.Contains(normalisedSource, p) {
		return true
	}
	if a := acronymOf(p); a != "" && strings.Contains(normalisedSource, a) {
		return true
	}
	// The line may hold the acronym where the source spells it out. The
	// source is not a short list here, so the reverse test walks its
	// words rather than trying every span.
	if len(p) >= 2 && len(p) <= 6 && !strings.Contains(p, " ") {
		words := strings.Fields(normalisedSource)
		for i := 0; i+len(p) <= len(words); i++ {
			if acronymOf(strings.Join(words[i:i+len(p)], " ")) == p {
				return true
			}
		}
	}
	return false
}

// commonCapitalised are words that start a sentence or a title and say
// nothing about who an employer is. Treating them as organisations
// would drop good lines to look rigorous.
var commonCapitalised = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "for": true,
	"led": true, "built": true, "designed": true, "delivered": true,
	"managed": true, "director": true, "manager": true, "engineer": true,
	"senior": true, "lead": true, "head": true, "vice": true, "president": true,
	"process": true, "control": true, "automation": true, "systems": true,
	"engineering": true, "operations": true, "manufacturing": true,
	"reduced": true, "improved": true, "owned": true, "ran": true,
}

// partiesIn pulls the organisation-shaped names out of a line: runs of
// capitalised words, and bare acronyms.
//
// This is a heuristic and it is tuned to miss rather than over-report.
// A missed organisation costs a check that would have passed anyway; a
// false one costs a true line.
func partiesIn(line string) []string {
	var out []string
	words := strings.Fields(line)
	var run []string
	runStart := -1
	flush := func() {
		if len(run) == 0 {
			return
		}
		// The first word of the line is capitalised because it is the
		// first word, which says nothing. Dropping it costs a check on a
		// company that happens to open the sentence, which is a miss, and
		// keeping it cost a true line, which is worse.
		if runStart == 0 {
			run = run[1:]
		}
		// A single capitalised word is a sentence start as often as a
		// company, so only multi-word runs count, plus acronyms which are
		// handled separately below.
		if len(run) >= 2 {
			out = append(out, strings.Join(run, " "))
		}
		run, runStart = nil, -1
	}
	for i, w := range words {
		trimmed := strings.Trim(w, ".,;:()[]\"'")
		if trimmed == "" {
			flush()
			continue
		}
		lower := strings.ToLower(trimmed)
		if isAcronym(trimmed) && !commonCapitalised[lower] {
			out = append(out, trimmed)
			flush()
			continue
		}
		// A token with digits in it is hardware, not an employer.
		if isCapitalised(trimmed) && !hasDigit(trimmed) && !commonCapitalised[lower] {
			if len(run) == 0 {
				runStart = i
			}
			run = append(run, trimmed)
			continue
		}
		flush()
	}
	flush()
	return out
}

func isCapitalised(s string) bool {
	r := []rune(s)
	return len(r) > 0 && r[0] >= 'A' && r[0] <= 'Z'
}

// isAcronym is two to six letters, all upper case. Long enough to be a
// name, short enough not to be a shouted word.
func isAcronym(s string) bool {
	r := []rune(s)
	if len(r) < 2 || len(r) > 6 {
		return false
	}
	for _, c := range r {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// describeUnsupported renders one dropped line for the admin view, so
// the count has something behind it. The line is truncated because the
// point is which claim failed, not the whole sentence.
func describeUnsupported(line string, u unsupported) string {
	head := line
	if r := []rune(head); len(r) > 80 {
		head = string(r[:80]) + "..."
	}
	var parts []string
	if len(u.Numbers) > 0 {
		parts = append(parts, "no source carries "+strings.Join(u.Numbers, ", "))
	}
	if len(u.Parties) > 0 {
		parts = append(parts, "no source names "+strings.Join(u.Parties, ", "))
	}
	return head + " (" + strings.Join(parts, "; ") + ")"
}
