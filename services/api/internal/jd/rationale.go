package jd

import (
	"regexp"
	"strconv"
	"strings"
)

// Checking that a rationale agrees with the record it was written from.
//
// Every metric the evaluation computes is about the verdict: which side
// of the gate a posting landed on, whether any below-gate posting
// outscored an above-gate one, how wide the gap is. None of them reads
// the sentence underneath, and the sentence is the part a person reads
// and the part the site promises is traceable.
//
// Run 9 on 2026-09-26 passed 9 of 9 with no inversions, and two
// judgments under it did not hold up:
//
//   - A rationale reading "over 30 years of engineering experience"
//     alongside stated_span_years of 0. The duration rule then
//     downgraded the verdict for want of a span the model had just
//     described. The code was right about what it was given; the model
//     contradicted itself between its prose and its structured field,
//     and that field is what the arithmetic uses.
//   - A rationale quoting "bachelor's degree in engineering or a related
//     field" and calling it the requirement. The requirement asked for a
//     Master's. The phrase was real, copied out of the candidate's own
//     facts sheet, so nothing was invented, but a reader is told the
//     posting asked for something it did not.
//
// As in entail.go this is not a model call. One per judgment would add
// an hour to a review that already takes four, and the same division
// applies: the model reports, the code decides. What code can decide is
// narrow and is exactly where inconsistency shows up, in the specifics.
// Prose is left alone.
//
// Nothing here changes a verdict. These are reported so a run can count
// them and a reader can see them, because the failure being addressed
// is that a wrong-but-plausible sentence passed every check there was.

// rationaleIssue is one way a rationale disagrees with its own record.
type rationaleIssue struct {
	// Kind is "span_mismatch" or "quote_unsupported".
	Kind string `json:"kind"`
	// Detail says what disagrees, in a sentence a reader can act on.
	Detail string `json:"detail"`
}

// quotedRe pulls quoted spans out of a rationale. Straight and curly
// pairs both, because a model emits either and a check that only knows
// one shape reports nothing on half the output.
//
// The apostrophe is the whole difficulty. Run 10 reported "s degree in
// Applied Mathematics and an Associate" as an unsupported quote: the
// rationale said "a Bachelor's degree in Applied Mathematics and an
// Associate's degree in Electrical Engineering Technology", and a bare
// `'...'` pattern read the two possessives as a matched pair. Nothing
// was quoted at all.
//
// So a single-quoted span must open where a quote can open, after a
// space or a bracket or at the start, and close where one can close,
// before a space or punctuation or at the end. "Bachelor's" fails the
// first test because a letter precedes the apostrophe, and "Associate's"
// fails the second because a letter follows it.
//
// Inside such a span an apostrophe is allowed when a lowercase letter
// follows it, which is what a possessive looks like and what a closing
// quote does not. That keeps the original Blue Origin case, 'bachelor's
// degree in engineering or a related field', matchable: it opens after
// a space, its internal apostrophe is followed by "s", and it closes
// before a space.
var quotedRe = regexp.MustCompile(
	`"([^"\n]{6,120})"` +
		`|\x{201c}([^\x{201d}\n]{6,120})\x{201d}` +
		`|(?:^|[\s([])'((?:[^'\n]|'[a-z]){6,120})'(?:[\s.,;:!?)\]]|$)`)

// spanInProseRe finds a duration a rationale asserts: a number next to a
// year word. Deliberately the same narrowness as RequiredYears, which
// exists so the two agree about what counts as a span.
var spanInProseRe = regexp.MustCompile(`(?i)(\d{1,2})\s*\+?\s*(?:or more\s*)?(?:years?|yrs?)`)

// proseSpanYears reports the largest span a rationale states, and
// whether it states one at all.
//
// The largest, because a rationale that mentions both "30 years of
// engineering" and "3 years leading teams" is describing a career and a
// detail inside it, and the career is what a duration requirement is
// usually asking about. Taking the smaller would manufacture a
// disagreement that is not there.
func proseSpanYears(rationale string) (int, bool) {
	best := 0
	for _, m := range spanInProseRe.FindAllStringSubmatch(rationale, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > best && n <= 60 {
			best = n
		}
	}
	for _, m := range writtenYearsRe.FindAllStringSubmatch(rationale, -1) {
		if n, ok := writtenYears[strings.ToLower(m[1])]; ok && n > best {
			best = n
		}
	}
	return best, best > 0
}

// normaliseForQuote is defined in assess.go; quoted spans are compared
// the same way source quotes are, forgiving whitespace and case and
// nothing else.

// quotedSpans returns the distinct quoted spans in a rationale.
func quotedSpans(rationale string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range quotedRe.FindAllStringSubmatch(rationale, -1) {
		for _, g := range m[1:] {
			s := strings.TrimSpace(g)
			if s == "" || seen[strings.ToLower(s)] {
				continue
			}
			seen[strings.ToLower(s)] = true
			out = append(out, s)
		}
	}
	return out
}

// Disjunction: a requirement offering alternatives, answered "unmet"
// by a rationale that asserts one of them.
//
// Run 11 on 2026-09-27 answered "unmet" to "3+ years' experience in
// automation engineering within a pharmaceutical, biotechnology, or
// regulated manufacturing environment", with the rationale "the
// candidate's experience is in distilleries and mining, which are
// regulated but not in the specified industries". It satisfied the
// third branch in its own sentence and then denied the requirement.
//
// Prompt rule 4a was generalised for this in v11, and this check is
// the part that does not depend on a prompt holding. It reports and
// never decides, so an over-eager match costs a line in a report
// rather than a verdict.

// branchStop are words too common in requirement prose to identify a
// branch. Without them "experience", "field" and "environment" match
// every rationale ever written and the check reports nothing useful.
var branchStop = map[string]bool{
	// Requirement prose.
	"experience": true, "environment": true, "environments": true,
	"related": true, "equivalent": true, "field": true, "fields": true,
	"years": true, "year": true, "within": true, "other": true,
	"similar": true, "relevant": true, "including": true,
	"industry": true, "industries": true, "sector": true, "sectors": true,
	"background": true, "discipline": true, "tools": true,
	// The frame a qualification is stated in, never the branch that
	// distinguishes one from another. "Bachelor of Science in Civil or
	// Structural Engineering" offers civil or structural; "science" and
	// "bachelor" are the frame, and a rationale reciting a different
	// degree in the same frame was reported as a contradiction until
	// these were listed.
	"bachelor": true, "bachelors": true, "master": true, "masters": true,
	"degree": true, "degrees": true, "science": true, "arts": true,
	"university": true, "college": true, "school": true, "accredited": true,
	"diploma": true, "certificate": true, "certification": true,
	"doctorate": true, "graduate": true, "undergraduate": true,
	"associate": true, "associates": true,
}

// negationRe finds a cue that what follows is being denied. Word
// bounded, so "another" and "note" do not read as negations, with
// "cannot" named because its "not" is not word bounded.
var negationRe = regexp.MustCompile(`(?i)\b(not|no|never|nothing|none|lacks?|lacking|without|absent|fails?|failed|cannot|outside|neither|nor)\b|n't`)

// sentenceSplitRe breaks a rationale into clauses. Negation does not
// carry across a full stop, and the run 11 case turns on exactly that:
// the first sentence denies the list and the second asserts a branch.
var sentenceSplitRe = regexp.MustCompile(`[.;]\s+`)

// wordRe pulls candidate branch words.
var wordRe = regexp.MustCompile(`[A-Za-z][A-Za-z'/-]{4,}`)

// orRe finds the "or" that joins a list of alternatives.
var orRe = regexp.MustCompile(`\bor\b`)

// branchWindow is how many words of a branch to read, and how many
// comma-separated items to walk back through. A list's items are
// short; prose around them is not, and reading past the item is how
// generic words get in.
const branchWindow = 3

// disjunctBranches returns distinctive words from the alternatives a
// requirement offers, or nothing when it offers none.
//
// It reads outward from the last "or", which is where the list's hinge
// is, rather than treating every comma in the requirement as a list
// separator. The looser first version took every comma-separated piece
// and pulled "tools" out of "using tools like Tekla Structures, ETABS,
// RISA, STAAD, or equivalent" and "bachelor" out of "Bachelor of
// Science in Civil or Structural Engineering". Both are prose around
// the list rather than items in it, and both produced a false report
// against runs 9 and 10.
//
// This still does not parse the list properly and does not try to.
// Missing a branch costs a report that would have been nice to have;
// inventing one costs a reader's trust in every other line, so it
// reads narrowly and stops.
func disjunctBranches(requirement string) []string {
	r := strings.ToLower(strings.TrimSpace(requirement))
	hinges := orRe.FindAllStringIndex(r, -1)
	if len(hinges) == 0 {
		return nil
	}
	h := hinges[len(hinges)-1]

	// The branch after "or", read only as far as a list item runs.
	segments := []string{firstWords(r[h[1]:], branchWindow)}

	// The items before it, walking back through commas while they stay
	// short enough to be items rather than sentences.
	left := strings.Split(strings.TrimSuffix(strings.TrimSpace(r[:h[0]]), ","), ",")
	for i := len(left) - 1; i >= 0 && len(segments) <= branchWindow; i-- {
		item := strings.TrimSpace(left[i])
		if item == "" {
			continue
		}
		if len(strings.Fields(item)) > branchWindow {
			// Too long to be a list item. Take its tail, which is where
			// the head noun sits, and stop walking.
			segments = append(segments, lastWords(item, branchWindow))
			break
		}
		segments = append(segments, item)
	}

	seen := map[string]bool{}
	var out []string
	for _, seg := range segments {
		for _, w := range wordRe.FindAllString(seg, -1) {
			if w == "or" || branchStop[w] || seen[w] {
				continue
			}
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func firstWords(s string, n int) string {
	f := strings.Fields(s)
	if len(f) > n {
		f = f[:n]
	}
	return strings.Join(f, " ")
}

func lastWords(s string, n int) string {
	f := strings.Fields(s)
	if len(f) > n {
		f = f[len(f)-n:]
	}
	return strings.Join(f, " ")
}

// isWordChar reports whether a byte can sit inside a word, so a branch
// is matched whole. Without this "regulated" matches inside
// "deregulated" and reports the opposite of what was found.
func isWordChar(b byte) bool {
	return b == '\'' || b == '-' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// inProperNounPhrase reports whether the match at clause[i:i+n] sits
// inside a capitalised multi-word name.
//
// This is what separates the one real detection in run 11 from the two
// false ones. The requirement's branches are generic categories, and a
// rationale granting one uses the word as a bare predicate: "which are
// regulated". A rationale that merely shares a word with a branch uses
// it inside the name of a specific thing the candidate actually holds:
// "Applied Mathematics" against a requirement wanting a PhD in Applied
// Math, "Electrical Engineering Technology" against one wanting Civil
// or Structural Engineering. Both name a different qualification, and
// both would otherwise be reported as contradictions.
//
// A heuristic, and named as one. It is allowed to be wrong because
// this whole check reports and never decides.
func inProperNounPhrase(clause string, i, n int) bool {
	if i >= len(clause) || clause[i] < 'A' || clause[i] > 'Z' {
		return false
	}
	// The word before.
	j := i
	for j > 0 && (clause[j-1] == ' ') {
		j--
	}
	k := j
	for k > 0 && isWordChar(clause[k-1]) {
		k--
	}
	if k < j && clause[k] >= 'A' && clause[k] <= 'Z' {
		return true
	}
	// The word after.
	e := i + n
	for e < len(clause) && clause[e] == ' ' {
		e++
	}
	return e < len(clause) && clause[e] >= 'A' && clause[e] <= 'Z'
}

// assertsBranch reports the first branch a rationale states without
// denying it, and whether it found one.
//
// A rationale answering "unmet" nearly always restates the requirement
// in order to refuse it, so the words alone prove nothing: "the
// evidence does not mention pharmaceutical, biotechnology, or
// regulated manufacturing" contains every branch and asserts none.
// Two things separate a grant from a restatement: whether a negation
// precedes the word in the same clause, and whether the word is being
// used as a category or absorbed into the name of something else.
func assertsBranch(rationale string, branches []string) (string, bool) {
	for _, clause := range sentenceSplitRe.Split(rationale, -1) {
		lower := strings.ToLower(clause)
		for _, b := range branches {
			for at := 0; ; {
				i := strings.Index(lower[at:], b)
				if i < 0 {
					break
				}
				i += at
				at = i + len(b)
				// Whole words only.
				if i > 0 && isWordChar(lower[i-1]) {
					continue
				}
				if at < len(lower) && isWordChar(lower[at]) {
					continue
				}
				if negationRe.MatchString(lower[:i]) {
					continue
				}
				if inProperNounPhrase(clause, i, len(b)) {
					continue
				}
				return b, true
			}
		}
	}
	return "", false
}

// checkRationale reports where a rationale disagrees with the
// requirement it judged, the evidence it cited, or its own structured
// fields.
//
// evidence is the text of the chunks the judgment cited. A quote found
// there is not invented, whatever the sentence around it claims about
// where it came from, and saying which source carried it is more useful
// to a reader than a bare accusation.
func checkRationale(rationale, requirementText, sourceQuote, verdict string, evidence []string, statedSpanYears float64) []rationaleIssue {
	var issues []rationaleIssue
	rationale = strings.TrimSpace(rationale)
	if rationale == "" {
		return nil
	}

	// An offered alternative, asserted and then refused. Only on the
	// model's own "unmet": a verdict the duration or relationship rule
	// weakened is the code disagreeing with the model, not the model
	// disagreeing with itself, and reporting that would blame the
	// wrong party. The posting's own wording is preferred over the
	// summary of it, because a summary is where a list gets lost.
	if verdict == "unmet" {
		source := sourceQuote
		if strings.TrimSpace(source) == "" {
			source = requirementText
		}
		if branches := disjunctBranches(source); len(branches) > 0 {
			if b, found := assertsBranch(rationale, branches); found {
				issues = append(issues, rationaleIssue{
					Kind: "disjunction_ignored",
					Detail: "the requirement offers alternatives and the rationale states " +
						strconv.Quote(b) + " without denying it, yet the verdict is unmet",
				})
			}
		}
	}

	// A span the model wrote about but did not report. The structured
	// field drives applyDurationRule, so this silently costs score on a
	// requirement the evidence may well satisfy.
	if prose, says := proseSpanYears(rationale); says && statedSpanYears <= 0 {
		issues = append(issues, rationaleIssue{
			Kind: "span_mismatch",
			Detail: "the rationale states " + strconv.Itoa(prose) +
				" years and stated_span_years is 0, so the duration rule was applied as though no span was found",
		})
	}

	// Quoted spans that came from neither the requirement nor the
	// evidence. This is the invented-quote case, and it is rarer than it
	// looks: run 9's suspicious quote turned out to be in the candidate's
	// facts sheet.
	req := normaliseForQuote(requirementText + " " + sourceQuote)
	var ev string
	if len(evidence) > 0 {
		ev = normaliseForQuote(strings.Join(evidence, " "))
	}
	for _, q := range quotedSpans(rationale) {
		n := normaliseForQuote(q)
		if n == "" || strings.Contains(req, n) {
			continue
		}
		if ev != "" && strings.Contains(ev, n) {
			// Real text, wrong attribution. Worth reporting only when
			// the sentence calls it the requirement, because quoting
			// evidence is ordinary and correct.
			if callsItTheRequirement(rationale, q) {
				issues = append(issues, rationaleIssue{
					Kind:   "quote_unsupported",
					Detail: "calls " + short(q) + " a requirement; that text is in the cited evidence, not in the requirement",
				})
			}
			continue
		}
		issues = append(issues, rationaleIssue{
			Kind:   "quote_unsupported",
			Detail: "quotes " + short(q) + ", which is in neither the requirement nor the evidence it cited",
		})
	}
	return issues
}

// callsItTheRequirement reports whether the word "requirement" sits
// next to this quote, which is what turns quoting the evidence into
// misdescribing the posting.
func callsItTheRequirement(rationale, quote string) bool {
	i := strings.Index(rationale, quote)
	if i < 0 {
		return false
	}
	start := i - 60
	if start < 0 {
		start = 0
	}
	end := i + len(quote) + 60
	if end > len(rationale) {
		end = len(rationale)
	}
	return strings.Contains(strings.ToLower(rationale[start:end]), "requirement")
}

func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 60 {
		return `"` + s[:60] + `..."`
	}
	return `"` + s + `"`
}
