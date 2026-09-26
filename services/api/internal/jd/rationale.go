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
var quotedRe = regexp.MustCompile(`"([^"\n]{6,120})"|'([^'\n]{6,120})'|\x{201c}([^\x{201d}\n]{6,120})\x{201d}`)

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

// checkRationale reports where a rationale disagrees with the
// requirement it judged, the evidence it cited, or its own structured
// fields.
//
// evidence is the text of the chunks the judgment cited. A quote found
// there is not invented, whatever the sentence around it claims about
// where it came from, and saying which source carried it is more useful
// to a reader than a bare accusation.
func checkRationale(rationale, requirementText, sourceQuote string, evidence []string, statedSpanYears float64) []rationaleIssue {
	var issues []rationaleIssue
	rationale = strings.TrimSpace(rationale)
	if rationale == "" {
		return nil
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
