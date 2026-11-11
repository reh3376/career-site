package hiring

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The code rules of FR-HF-05: duration, recency, scope, credential and
// named-party comparisons are made here, from the facts the judge
// reported against what the requirement asks for, and never accepted
// from the model.
//
// The reason is the one that keeps the score out of the model. A
// comparison between two numbers is arithmetic, and arithmetic belongs
// where it can be read and tested. Three prompt versions tried to teach
// a 4B model to check a span before answering "met" and all three
// failed; the model is good at finding what the evidence says and poor
// at policing its own inference. So it reports, and this file decides.
//
// Every rule only ever weakens. A judgment the model already called
// unresolved or not_met knows something these rules do not, and a rule
// that argued a candidate up would be the model's error with the
// code's name on it. Under the v1 vocabulary a weakened judgment
// becomes unresolved with the uncertainty code naming the fact that
// would settle it, when the shortfall is within the rule's tolerance,
// or not_met with a shortfall basis when it is not (05 §2.3). Nothing
// is scored as half of anything.
//
// Each change is recorded as a RuleAdjustment, with the rule's own
// version, so a record can say exactly which comparison moved it and
// a later tolerance change does not rewrite history.

// RulesVersion names this set of rules and tolerances on the match
// record (versions.rules). It changes when any rule's logic does. v2:
// the duration, recency and scope rules compare against the
// requirement's stored thresholds rather than reading its text (step 5).
const RulesVersion = "rules-v2"

// Rule names, as written in RuleAdjustment.Rule.
const (
	RuleDuration     = "duration"
	RuleRecency      = "recency"
	RuleScope        = "scope"
	RuleCredential   = "credential"
	RuleRelationship = "relationship"
	RuleEvidence     = "evidence"
	RuleConsistency  = "consistency"
	RuleBehavioral   = "behavioral"
)

// Versions per rule. A rule's version moves when its comparison or
// tolerance changes; the set version above moves with any of them.
const (
	RuleDurationVersion     = "v2"
	RuleRecencyVersion      = "v2"
	RuleScopeVersion        = "v2"
	RuleCredentialVersion   = "v1"
	RuleRelationshipVersion = "v1"
	RuleEvidenceVersion     = "v1"
	RuleConsistencyVersion  = "v1"
	RuleBehavioralVersion   = "v1"
)

// Tolerances (05 §2.3): a span within a quarter below the ask is a
// question rather than a refusal; a scope within half of it likewise.
// Recency has no template figure; half again the window asked for is
// treated the same way as scope.
const (
	DurationTolerance = 0.25
	ScopeTolerance    = 0.5
	RecencyTolerance  = 0.5
)

// RuleAdjustment is one change a rule made to a judgment (04 §2.1):
// what it was, what it became, and why, in a sentence a reader can
// act on. An adjustment nobody can read is indistinguishable from the
// model having said so itself.
type RuleAdjustment struct {
	Rule              string   `json:"rule"`
	RuleVersion       string   `json:"rule_version"`
	FromDetermination string   `json:"from_determination"`
	ToDetermination   string   `json:"to_determination"`
	FromBasis         string   `json:"from_basis"`
	ToBasis           string   `json:"to_basis"`
	AddedUncertainty  []string `json:"added_uncertainty"`
	Reason            string   `json:"reason"`
}

// Scope is a scale the evidence or the requirement states: a unit
// (people, sites, usd) and a value.
type Scope struct {
	Unit  string  `json:"unit"`
	Value float64 `json:"value"`
}

// Credential is a degree, licence or certification the evidence
// states, as the judge reported it.
type Credential struct {
	Level  string `json:"level"`
	Field  string `json:"field"`
	Status string `json:"status"` // completed | in_progress | unknown
}

// ReportedFacts is what the judge found, for the rules to compare
// (04 §2.2). The model reports; code decides.
type ReportedFacts struct {
	StatedSpanYears      float64     `json:"stated_span_years"`
	MostRecentYear       int         `json:"most_recent_year"`
	StatedScope          *Scope      `json:"stated_scope"`
	CredentialFound      *Credential `json:"credential_found"`
	PartiesEvidenced     []string    `json:"parties_evidenced"`
	AlternativeSatisfied int         `json:"alternative_satisfied"`
	InvolvementLevel     string      `json:"involvement_level"`
}

// Verdict is the part of a judgment the rules read and may change.
type Verdict struct {
	Determination string
	Basis         string
	Uncertainty   []string
}

// Evidenced reports whether the determination counts toward coverage.
func (v Verdict) Evidenced() bool {
	return v.Determination == DetMet || v.Determination == DetMetEquivalent
}

// RuleInput is what the rules need from the requirement: its
// thresholds (how many years, how recent, how large), its type and the
// organisations it names.
//
// Since step 5 the thresholds are stored on the requirement, derived
// from its text when it is drafted and editable from then on, and the
// rules compare against those and nothing else: a hiring manager who
// removes a years threshold has removed the comparison, whatever the
// text still says. Thresholds is nil only for a requirement that was
// never drafted as a row (a run whose rows could not be written, and
// tests); the rules then read the text as they did before.
type RuleInput struct {
	Text         string
	Type         string
	NamedParties []string
	Thresholds   *Thresholds
}

// asks resolves what the requirement asks for: its stored thresholds
// when it has them, the text otherwise.
func (r RuleInput) asks() Thresholds {
	if r.Thresholds != nil {
		return *r.Thresholds
	}
	return DeriveThresholds(r.Text, r.Type, r.NamedParties)
}

// ApplyRules runs every code rule over a judgment, in a fixed order,
// and returns the judgment as the rules left it with one adjustment per
// rule that changed it. A rule that finds nothing to say leaves no
// trace. Once a rule has weakened the judgment the later rules have
// nothing to compare and stand aside, so at most one rule moves a
// determination; this keeps the reason a reader sees singular.
func ApplyRules(req RuleInput, v Verdict, facts ReportedFacts, now time.Time) (Verdict, []RuleAdjustment) {
	var out []RuleAdjustment
	for _, rule := range []func(RuleInput, Verdict, ReportedFacts, time.Time) (Verdict, *RuleAdjustment){
		durationRule, recencyRule, scopeRule, credentialRule, relationshipRule,
	} {
		next, adj := rule(req, v, facts, now)
		if adj != nil {
			out = append(out, *adj)
			v = next
			break
		}
	}
	return v, out
}

// EvidenceRule is the oldest rule in the pipeline, kept apart from
// ApplyRules because it needs to know which cited ids were actually
// offered: a judgment that cites nothing it was shown rests on
// nothing, and becomes not_met, not_addressed. The score must be
// traceable to evidence, not to the model's mood.
func EvidenceRule(v Verdict, validEvidence int) (Verdict, *RuleAdjustment) {
	if v.Determination == DetNotMet || validEvidence > 0 {
		return v, nil
	}
	adj := &RuleAdjustment{
		Rule: RuleEvidence, RuleVersion: RuleEvidenceVersion,
		FromDetermination: v.Determination, ToDetermination: DetNotMet,
		FromBasis: v.Basis, ToBasis: BasisNotAddressed,
		AddedUncertainty: []string{},
		Reason:           "cited no evidence it was shown; a judgment that rests on nothing is not_met",
	}
	return Verdict{Determination: DetNotMet, Basis: BasisNotAddressed, Uncertainty: []string{}}, adj
}

// weaken builds the judgment a rule leaves behind: unresolved with the
// code that names the missing fact when the shortfall is a question,
// not_met with the shortfall basis when it is not. The basis of an
// unresolved judgment is the suspected route to met (03 §2.2): when the
// rule knows the route (a shortfall) it names it, otherwise the model's
// basis stands.
func weaken(v Verdict, rule, version string, toDet, toBasis, code, reason string) (Verdict, *RuleAdjustment) {
	out := Verdict{Determination: toDet, Basis: toBasis, Uncertainty: []string{}}
	adj := &RuleAdjustment{
		Rule: rule, RuleVersion: version,
		FromDetermination: v.Determination, ToDetermination: toDet,
		FromBasis: v.Basis, ToBasis: toBasis,
		AddedUncertainty: []string{},
		Reason:           reason,
	}
	if toDet == DetUnresolved {
		out.Uncertainty = appendCode(v.Uncertainty, code)
		adj.AddedUncertainty = []string{code}
	}
	return out, adj
}

func appendCode(codes []string, code string) []string {
	for _, c := range codes {
		if c == code {
			return append([]string{}, codes...)
		}
	}
	return append(append([]string{}, codes...), code)
}

// --- duration -------------------------------------------------------

// requiredYearsRe matches the ways postings ask for a length of service.
// Deliberately narrow: a number next to a time word. Anything cleverer
// starts matching salaries, team sizes and version numbers.
var requiredYearsRe = regexp.MustCompile(`(?i)(\d{1,2})\s*\+?\s*(?:or more\s*)?(?:years?|yrs?)`)

// writtenYears covers the small numbers postings spell out. Beyond ten
// they are written as digits in practice.
// WrittenYears and WrittenYearsRe are exported for the rationale check
// in jd, which reads spans the same narrow way.
var WrittenYears = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

var WrittenYearsRe = regexp.MustCompile(`(?i)\b(one|two|three|four|five|six|seven|eight|nine|ten)\s+(?:or more\s+)?(?:years?|yrs?)`)

// RequiredYears reports how many years a requirement demands, and
// whether it demands any at all. When a requirement names more than
// one span ("10+ years of software engineering with 3+ years leading
// teams") the largest is returned: that is the headline ask, and taking
// the smaller number would let a candidate clear the requirement on the
// easier half.
func RequiredYears(text string) (int, bool) {
	// A recency window ("within the last 5 years") is not a span of
	// service and must not be read as one.
	text = recencyRe.ReplaceAllString(text, " ")
	best := 0
	for _, m := range requiredYearsRe.FindAllStringSubmatch(text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > best && n <= 60 {
			best = n
		}
	}
	for _, m := range WrittenYearsRe.FindAllStringSubmatch(text, -1) {
		if n, ok := WrittenYears[strings.ToLower(m[1])]; ok && n > best {
			best = n
		}
	}
	return best, best > 0
}

func durationRule(req RuleInput, v Verdict, facts ReportedFacts, _ time.Time) (Verdict, *RuleAdjustment) {
	if !v.Evidenced() {
		return v, nil
	}
	min := req.asks().MinYears
	if min == nil || *min <= 0 {
		return v, nil
	}
	need := strconv.FormatFloat(*min, 'g', -1, 64)
	span := facts.StatedSpanYears
	if span >= *min {
		return v, nil
	}
	if span <= 0 {
		return weaken(v, RuleDuration, RuleDurationVersion, DetUnresolved, v.Basis, UncDurationUnknown,
			fmt.Sprintf("asks for %s years; the evidence shows the work but states no span for it", need))
	}
	stated := strconv.FormatFloat(span, 'g', -1, 64)
	if span >= *min*(1-DurationTolerance) {
		return weaken(v, RuleDuration, RuleDurationVersion, DetUnresolved, BasisDurationShortfall, UncDurationUnknown,
			fmt.Sprintf("asks for %s years; the evidence states %s, within a quarter of the ask, so the exact span is a question rather than a refusal", need, stated))
	}
	return weaken(v, RuleDuration, RuleDurationVersion, DetNotMet, BasisDurationShortfall, "",
		fmt.Sprintf("asks for %s years; the evidence states %s", need, stated))
}

// --- recency --------------------------------------------------------

// recencyRe matches an explicit recency window: "within the last 5
// years", "in the past three years". Only the explicit form; "recent
// experience" names no number and gets no rule.
var recencyRe = regexp.MustCompile(`(?i)\b(?:within|in|over|during)\s+the\s+(?:last|past|previous)\s+(\d{1,2}|one|two|three|four|five|six|seven|eight|nine|ten)\s+(?:years?|yrs?)\b`)

// RequiredRecencyYears reports the recency window a requirement asks
// for, in years, and whether it asks for one.
func RequiredRecencyYears(text string) (int, bool) {
	m := recencyRe.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	if n, err := strconv.Atoi(m[1]); err == nil {
		return n, n > 0
	}
	n, ok := WrittenYears[strings.ToLower(m[1])]
	return n, ok
}

func recencyRule(req RuleInput, v Verdict, facts ReportedFacts, now time.Time) (Verdict, *RuleAdjustment) {
	if !v.Evidenced() {
		return v, nil
	}
	within := req.asks().RecencyYears
	if within == nil || *within <= 0 {
		return v, nil
	}
	window := int(*within)
	if facts.MostRecentYear <= 0 {
		return weaken(v, RuleRecency, RuleRecencyVersion, DetUnresolved, v.Basis, UncRecencyUnknown,
			fmt.Sprintf("asks for work within the last %d years; the evidence shows the work but states no dates for it", window))
	}
	age := now.Year() - facts.MostRecentYear
	if age <= window {
		return v, nil
	}
	if float64(age) <= float64(window)*(1+RecencyTolerance) {
		return weaken(v, RuleRecency, RuleRecencyVersion, DetUnresolved, BasisRecency, UncRecencyUnknown,
			fmt.Sprintf("asks for work within the last %d years; the most recent evidenced is %d, %d years ago, close enough that more recent work is a question", window, facts.MostRecentYear, age))
	}
	return weaken(v, RuleRecency, RuleRecencyVersion, DetNotMet, BasisRecency, "",
		fmt.Sprintf("asks for work within the last %d years; the most recent evidenced is %d, %d years ago", window, facts.MostRecentYear, age))
}

// --- scope ----------------------------------------------------------

// Up to two words may sit between the number and the unit ("12
// controls engineers", "5 manufacturing sites"). Year phrases and
// recency windows are removed before matching so "10 years across
// sites" is not read as ten sites.
var (
	scopePeopleRe = regexp.MustCompile(`(?i)\b(?:teams?\s+of\s+)?(\d{1,4})\s*\+?\s*(?:or more\s+)?(?:[a-z-]+\s+){0,2}?(direct reports|reports|engineers|people|employees|staff|technicians|team members|developers|ftes?|headcount)\b`)
	scopeSitesRe  = regexp.MustCompile(`(?i)\b(\d{1,3})\s*\+?\s*(?:or more\s+)?(?:[a-z-]+\s+){0,2}?(sites|plants|facilities|locations|factories)\b`)
	scopeMoneyRe  = regexp.MustCompile(`(?i)\$\s?(\d+(?:\.\d+)?)\s*(k|m|mm|million|b|bn|billion)\b`)
)

// RequiredScope reads the scale a requirement names: a team size, a
// number of sites, or a budget. Units are normalised to people, sites
// and usd so the judge's stated_scope can be compared with it.
func RequiredScope(text string) (Scope, bool) {
	text = recencyRe.ReplaceAllString(text, " ")
	text = requiredYearsRe.ReplaceAllString(text, " ")
	text = WrittenYearsRe.ReplaceAllString(text, " ")
	if m := scopeMoneyRe.FindStringSubmatch(text); m != nil {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil {
			return Scope{Unit: "usd", Value: n * moneyMultiplier(m[2])}, true
		}
	}
	if m := scopePeopleRe.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return Scope{Unit: "people", Value: float64(n)}, true
		}
	}
	if m := scopeSitesRe.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return Scope{Unit: "sites", Value: float64(n)}, true
		}
	}
	return Scope{}, false
}

func moneyMultiplier(suffix string) float64 {
	switch strings.ToLower(suffix) {
	case "k":
		return 1e3
	case "m", "mm", "million":
		return 1e6
	case "b", "bn", "billion":
		return 1e9
	}
	return 1
}

// NormaliseScopeUnit maps the ways a unit gets written to the three
// the rule compares. Unknown units are returned lower-cased, and a
// comparison across unknown units is refused rather than guessed.
func NormaliseScopeUnit(unit string) string {
	u := strings.ToLower(strings.TrimSpace(unit))
	switch {
	case u == "":
		return ""
	case strings.Contains(u, "people"), strings.Contains(u, "person"), strings.Contains(u, "report"),
		strings.Contains(u, "engineer"), strings.Contains(u, "employee"), strings.Contains(u, "staff"),
		strings.Contains(u, "team"), strings.Contains(u, "fte"), strings.Contains(u, "headcount"), strings.Contains(u, "technician"):
		return "people"
	case strings.Contains(u, "site"), strings.Contains(u, "plant"), strings.Contains(u, "facilit"),
		strings.Contains(u, "location"), strings.Contains(u, "factor"):
		return "sites"
	case strings.Contains(u, "usd"), strings.Contains(u, "$"), strings.Contains(u, "dollar"),
		strings.Contains(u, "budget"), strings.Contains(u, "capex"), strings.Contains(u, "revenue"):
		return "usd"
	}
	return u
}

func scopeRule(req RuleInput, v Verdict, facts ReportedFacts, _ time.Time) (Verdict, *RuleAdjustment) {
	if !v.Evidenced() {
		return v, nil
	}
	min := req.asks().MinScope
	if min == nil || min.Value <= 0 {
		return v, nil
	}
	need := Scope{Unit: NormaliseScopeUnit(min.Unit), Value: min.Value}
	if facts.StatedScope == nil || facts.StatedScope.Value <= 0 {
		return weaken(v, RuleScope, RuleScopeVersion, DetUnresolved, v.Basis, UncScopeUnknown,
			fmt.Sprintf("asks for a scale of %s; the evidence shows the work but states no scale for it", describeScope(need)))
	}
	got := *facts.StatedScope
	if NormaliseScopeUnit(got.Unit) != need.Unit {
		return weaken(v, RuleScope, RuleScopeVersion, DetUnresolved, v.Basis, UncScopeUnknown,
			fmt.Sprintf("asks for a scale of %s; the evidence states %s, which is not the same measure", describeScope(need), describeScope(got)))
	}
	if got.Value >= need.Value {
		return v, nil
	}
	if got.Value >= need.Value*(1-ScopeTolerance) {
		return weaken(v, RuleScope, RuleScopeVersion, DetUnresolved, BasisScopeShortfall, UncScopeUnknown,
			fmt.Sprintf("asks for a scale of %s; the evidence states %s, within half of the ask, so the full scale is a question rather than a refusal", describeScope(need), describeScope(got)))
	}
	return weaken(v, RuleScope, RuleScopeVersion, DetNotMet, BasisScopeShortfall, "",
		fmt.Sprintf("asks for a scale of %s; the evidence states %s", describeScope(need), describeScope(got)))
}

func describeScope(s Scope) string {
	unit := NormaliseScopeUnit(s.Unit)
	if unit == "" {
		unit = "unknown units"
	}
	switch unit {
	case "usd":
		switch {
		case s.Value >= 1e9:
			return fmt.Sprintf("$%sB", strconv.FormatFloat(s.Value/1e9, 'g', 3, 64))
		case s.Value >= 1e6:
			return fmt.Sprintf("$%sM", strconv.FormatFloat(s.Value/1e6, 'g', 3, 64))
		case s.Value >= 1e3:
			return fmt.Sprintf("$%sK", strconv.FormatFloat(s.Value/1e3, 'g', 3, 64))
		}
		return fmt.Sprintf("$%s", strconv.FormatFloat(s.Value, 'g', 3, 64))
	}
	return fmt.Sprintf("%s %s", strconv.FormatFloat(s.Value, 'g', -1, 64), unit)
}

// --- credential -----------------------------------------------------

// credentialRule applies to a requirement typed credential that the
// model called met on the direct basis: a direct met needs a completed
// credential in the facts. A met_equivalent on experience_for_education
// is the alternative rule 4a allows and stands; so does a met on an
// alternative the posting itself offered (alternative_satisfied >= 0).
func credentialRule(req RuleInput, v Verdict, facts ReportedFacts, _ time.Time) (Verdict, *RuleAdjustment) {
	if req.Type != "credential" || v.Determination != DetMet || v.Basis != BasisDirect {
		return v, nil
	}
	if facts.AlternativeSatisfied >= 0 {
		return v, nil
	}
	if facts.CredentialFound != nil && facts.CredentialFound.Status == "completed" {
		return v, nil
	}
	if facts.CredentialFound == nil {
		return weaken(v, RuleCredential, RuleCredentialVersion, DetUnresolved, v.Basis, UncCredentialUnknown,
			"names a credential and was called met directly; the evidence states no credential, so which one is held is a question")
	}
	return weaken(v, RuleCredential, RuleCredentialVersion, DetUnresolved, v.Basis, UncCredentialUnknown,
		fmt.Sprintf("names a credential and was called met directly; the evidence states one with status %q, not completed", facts.CredentialFound.Status))
}

// --- relationship ---------------------------------------------------

// relationshipRule weakens an evidenced judgment on a requirement that
// names organisations when the evidence names none of them. Calling a
// company's API is not a working relationship with that company, and
// no amount of telling the model so changed its mind; which companies
// the requirement names is settled at extraction, which the evidence
// names is reported by the judge, and whether the second satisfies the
// first is a set comparison, which belongs here.
//
// It weakens to unresolved rather than not_met: the candidate may well
// do the kind of work described, and what is missing is the named
// relationship, which one question settles.
func relationshipRule(req RuleInput, v Verdict, facts ReportedFacts, _ time.Time) (Verdict, *RuleAdjustment) {
	if !v.Evidenced() || len(req.NamedParties) == 0 {
		return v, nil
	}
	for _, want := range req.NamedParties {
		for _, got := range facts.PartiesEvidenced {
			if PartyMatches(want, got) {
				return v, nil
			}
		}
	}
	return weaken(v, RuleRelationship, RuleRelationshipVersion, DetUnresolved, BasisRelationshipUnproven, UncPartyUnknown,
		"names "+strings.Join(req.NamedParties, ", ")+"; the evidence names none of them as organisations worked with")
}

// PartyMatches compares two organisation names loosely enough to
// survive the ways people write them down: case and punctuation are
// ignored, containment either way covers "AWS" against "Amazon Web
// Services (AWS)", and the acronym covers a posting that writes "AWS"
// where a document writes the long form. Generous on purpose, because
// a false downgrade understates a real candidate.
func PartyMatches(a, b string) bool {
	na, nb := NormaliseParty(a), NormaliseParty(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb || strings.Contains(na, nb) || strings.Contains(nb, na) {
		return true
	}
	return AcronymOf(na) == nb || AcronymOf(nb) == na
}

// AcronymOf returns the initials of a multi-word name, or "" when the
// name is a single word. Capped at five letters: beyond that an
// "acronym" is a coincidence of first letters.
func AcronymOf(normalised string) string {
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

// NormaliseParty strips the decoration companies accumulate in prose.
func NormaliseParty(s string) string {
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
