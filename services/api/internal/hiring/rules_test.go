package hiring

import (
	"testing"
	"time"
)

var now2026 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func evidencedMet() Verdict { return Verdict{Determination: DetMet, Basis: BasisDirect} }

// How many years a requirement asks for is read from the posting's own
// words, so these are the words postings actually use.
func TestRequiredYears(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		want int
		asks bool
	}{
		{"plus form", "5+ years in building products that apply Machine Learning Models", 5, true},
		{"spaced plus", "2 + years working with Large Language Models", 2, true},
		{"capitalised", "10+ Years of Software and systems engineering", 10, true},
		{"or more", "7 or more years in process manufacturing", 7, true},
		{"spelled out", "at least three years leading development teams", 3, true},
		{"abbreviated", "8 yrs of controls experience", 8, true},
		{"compound takes the larger", "10+ Years of Software and systems engineering in fast paced environments with 3+ years leading development teams", 10, true},
		{"no duration", "Strong understanding of software architecture and design patterns", 0, false},
		{"degree not a duration", "Masters or equivalent experience in Computer Science", 0, false},
		{"travel percentage", "Willingness to travel up to 10% of the time", 0, false},
		{"team size", "Lead a team of 12 engineers", 0, false},
		{"a standard number", "Experience with NFPA 70E and UL 508A", 0, false},
		{"a year of grace", "Founded in 1996", 0, false},
		{"implausible span ignored", "500 years of experience", 0, false},
		// A recency window is not a span of service.
		{"recency window is not a span", "PLC programming within the last 5 years", 0, false},
		{"recency window beside a span", "7+ years of PLC programming, including work within the last 2 years", 7, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, asks := RequiredYears(c.text)
			if got != c.want || asks != c.asks {
				t.Fatalf("got (%d, %v), want (%d, %v) for %q", got, asks, c.want, c.asks, c.text)
			}
		})
	}
}

func TestRequiredRecencyAndScope(t *testing.T) {
	t.Parallel()
	if n, ok := RequiredRecencyYears("Hands-on Studio 5000 work within the last 3 years"); !ok || n != 3 {
		t.Errorf("recency = (%d, %v), want (3, true)", n, ok)
	}
	if n, ok := RequiredRecencyYears("in the past two years"); !ok || n != 2 {
		t.Errorf("recency written = (%d, %v), want (2, true)", n, ok)
	}
	if _, ok := RequiredRecencyYears("Recent experience with MES platforms"); ok {
		t.Error("'recent' with no number must not become a window")
	}
	cases := []struct {
		text  string
		unit  string
		value float64
		ok    bool
	}{
		{"Lead a team of 12 engineers", "people", 12, true},
		{"Manage 40+ direct reports across the plant", "people", 40, true},
		{"Oversee 5 manufacturing sites", "sites", 5, true},
		{"Deliver a $60M capital program", "usd", 60e6, true},
		{"Budgets of $1.2 billion", "usd", 1.2e9, true},
		{"Strong communication skills", "", 0, false},
		{"5+ years PLC programming", "", 0, false},
	}
	for _, c := range cases {
		got, ok := RequiredScope(c.text)
		if ok != c.ok || (ok && (got.Unit != c.unit || got.Value != c.value)) {
			t.Errorf("RequiredScope(%q) = (%+v, %v), want (%s %v, %v)", c.text, got, ok, c.unit, c.value, c.ok)
		}
	}
}

// The duration rule: a span the evidence does not state is a question;
// a span a little short is a question with the shortfall named; a span
// well short is not met. It never argues a candidate up.
func TestDurationRule(t *testing.T) {
	t.Parallel()
	req := RuleInput{Text: "5+ years in building products that apply Machine Learning Models", Type: "experience"}

	v, adj := ApplyRules(req, evidencedMet(), ReportedFacts{StatedSpanYears: 5, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Fatalf("span covers the ask: got %+v %+v, want met untouched", v, adj)
	}
	v, adj = ApplyRules(req, evidencedMet(), ReportedFacts{StatedSpanYears: 0, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Basis != BasisDirect || len(v.Uncertainty) != 1 || v.Uncertainty[0] != UncDurationUnknown {
		t.Fatalf("no span stated: got %+v, want unresolved on the model's basis with duration_unknown", v)
	}
	if len(adj) != 1 || adj[0].Rule != RuleDuration || adj[0].RuleVersion != RuleDurationVersion || adj[0].Reason == "" {
		t.Fatalf("adjustment = %+v, want one readable duration adjustment", adj)
	}
	// Four years against five is within a quarter: a question, not a refusal.
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{StatedSpanYears: 4, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Basis != BasisDurationShortfall || v.Uncertainty[0] != UncDurationUnknown {
		t.Fatalf("span within tolerance: got %+v, want unresolved / duration_shortfall", v)
	}
	// Two years against five is not.
	v, adj = ApplyRules(req, evidencedMet(), ReportedFacts{StatedSpanYears: 2, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetNotMet || v.Basis != BasisDurationShortfall || len(v.Uncertainty) != 0 {
		t.Fatalf("span well short: got %+v, want not_met / duration_shortfall with no uncertainty", v)
	}
	if adj[0].ToDetermination != DetNotMet || adj[0].FromDetermination != DetMet {
		t.Fatalf("adjustment from/to = %s/%s", adj[0].FromDetermination, adj[0].ToDetermination)
	}
	// A met_equivalent is weakened the same way.
	v, _ = ApplyRules(req, Verdict{Determination: DetMetEquivalent, Basis: BasisVocabulary}, ReportedFacts{StatedSpanYears: 0, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Basis != BasisVocabulary {
		t.Fatalf("met_equivalent with no span: got %+v, want unresolved keeping the suspected route", v)
	}
	// Never strengthens, and never touches a requirement without a span.
	for _, start := range []Verdict{{Determination: DetUnresolved, Basis: BasisDirect, Uncertainty: []string{UncDepthUnknown}}, {Determination: DetNotMet, Basis: BasisContradicted}} {
		v, adj := ApplyRules(req, start, ReportedFacts{StatedSpanYears: 30, AlternativeSatisfied: -1}, now2026)
		if v.Determination != start.Determination || len(adj) != 0 {
			t.Fatalf("a %s judgment with a 30 year span became %s; the rule must only weaken", start.Determination, v.Determination)
		}
	}
	v, adj = ApplyRules(RuleInput{Text: "Strong understanding of software architecture"}, evidencedMet(), ReportedFacts{AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Fatalf("no span asked: got %+v %+v", v, adj)
	}
}

func TestRecencyRule(t *testing.T) {
	t.Parallel()
	req := RuleInput{Text: "Hands-on PLC programming within the last 3 years", Type: "skill"}
	v, adj := ApplyRules(req, evidencedMet(), ReportedFacts{MostRecentYear: 2025, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Fatalf("recent enough: %+v %+v", v, adj)
	}
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{MostRecentYear: 0, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Uncertainty[0] != UncRecencyUnknown || v.Basis != BasisDirect {
		t.Fatalf("no dates: %+v", v)
	}
	// Four years ago against a three-year window: within half again.
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{MostRecentYear: 2022, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Basis != BasisRecency {
		t.Fatalf("slightly old: %+v, want unresolved / recency", v)
	}
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{MostRecentYear: 2015, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetNotMet || v.Basis != BasisRecency {
		t.Fatalf("long ago: %+v, want not_met / recency", v)
	}
}

func TestScopeRule(t *testing.T) {
	t.Parallel()
	req := RuleInput{Text: "Lead a team of 12 engineers", Type: "outcome"}
	v, adj := ApplyRules(req, evidencedMet(), ReportedFacts{StatedScope: &Scope{Unit: "engineers", Value: 15}, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Fatalf("larger team: %+v %+v", v, adj)
	}
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Uncertainty[0] != UncScopeUnknown || v.Basis != BasisDirect {
		t.Fatalf("no scale stated: %+v", v)
	}
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{StatedScope: &Scope{Unit: "people", Value: 8}, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Basis != BasisScopeShortfall {
		t.Fatalf("eight of twelve is within half: %+v", v)
	}
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{StatedScope: &Scope{Unit: "people", Value: 3}, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetNotMet || v.Basis != BasisScopeShortfall {
		t.Fatalf("three of twelve: %+v", v)
	}
	// A different measure cannot be compared and is a question.
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{StatedScope: &Scope{Unit: "sites", Value: 3}, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Basis != BasisDirect {
		t.Fatalf("unit mismatch: %+v", v)
	}
	money := RuleInput{Text: "Deliver a $60M capital program", Type: "outcome"}
	v, _ = ApplyRules(money, evidencedMet(), ReportedFacts{StatedScope: &Scope{Unit: "USD", Value: 45e6}, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Basis != BasisScopeShortfall {
		t.Fatalf("$45M of $60M: %+v", v)
	}
}

func TestCredentialRule(t *testing.T) {
	t.Parallel()
	req := RuleInput{Text: "Bachelor's degree in Electrical Engineering", Type: "credential"}
	v, adj := ApplyRules(req, evidencedMet(), ReportedFacts{CredentialFound: &Credential{Level: "bachelor", Field: "Applied Mathematics", Status: "completed"}, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Fatalf("completed credential: %+v %+v", v, adj)
	}
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Uncertainty[0] != UncCredentialUnknown {
		t.Fatalf("direct met with no credential: %+v", v)
	}
	v, _ = ApplyRules(req, evidencedMet(), ReportedFacts{CredentialFound: &Credential{Level: "bachelor", Status: "in_progress"}, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved {
		t.Fatalf("in-progress credential: %+v", v)
	}
	// The alternative the posting offered, and an equivalency, both stand.
	v, adj = ApplyRules(RuleInput{Text: "Master's degree or equivalent experience", Type: "credential"}, evidencedMet(), ReportedFacts{AlternativeSatisfied: 0}, now2026)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Fatalf("alternative satisfied: %+v %+v", v, adj)
	}
	v, adj = ApplyRules(req, Verdict{Determination: DetMetEquivalent, Basis: BasisExperienceForEducation}, ReportedFacts{AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetMetEquivalent || len(adj) != 0 {
		t.Fatalf("experience for education: %+v %+v", v, adj)
	}
	// A non-credential requirement is never touched by this rule.
	v, adj = ApplyRules(RuleInput{Text: "Degree of autonomy in a small team", Type: "skill"}, evidencedMet(), ReportedFacts{AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Fatalf("skill typed: %+v %+v", v, adj)
	}
}

func TestRelationshipRule(t *testing.T) {
	t.Parallel()
	req := RuleInput{Text: "Collaborate with external partners like Anthropic, AWS and OpenAI", Type: "relationship", NamedParties: []string{"Anthropic", "AWS", "OpenAI"}}
	// The failure this rule was written for: "met" because the project
	// calls LLM APIs, with no organisation named as worked with.
	v, adj := ApplyRules(req, evidencedMet(), ReportedFacts{AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetUnresolved || v.Basis != BasisRelationshipUnproven || v.Uncertainty[0] != UncPartyUnknown {
		t.Fatalf("no party evidenced: %+v", v)
	}
	if len(adj) != 1 || adj[0].Rule != RuleRelationship || adj[0].Reason == "" {
		t.Fatalf("adjustment: %+v", adj)
	}
	v, adj = ApplyRules(req, evidencedMet(), ReportedFacts{PartiesEvidenced: []string{"AWS"}, AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Fatalf("one party is enough: %+v %+v", v, adj)
	}
	v, adj = ApplyRules(RuleInput{Text: "Strong understanding of software architecture"}, evidencedMet(), ReportedFacts{AlternativeSatisfied: -1}, now2026)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Fatalf("names nobody: %+v %+v", v, adj)
	}
	for _, c := range []struct{ want, got string }{
		{"AWS", "Amazon Web Services (AWS)"}, {"Amazon Web Services", "AWS"}, {"OpenAI", "OpenAI, Inc."},
		{"Anthropic", "anthropic"}, {"Joy Global", "Joy Global Inc"},
	} {
		if !PartyMatches(c.want, c.got) {
			t.Errorf("%q should match %q", c.got, c.want)
		}
	}
	for _, bad := range []string{"Microsoft", "", "   ", "!!"} {
		if PartyMatches("Anthropic", bad) {
			t.Errorf("%q must not match Anthropic", bad)
		}
	}
}

// Rules run in a fixed order and at most one moves the determination,
// so the reason a reader sees is singular.
func TestOnlyOneRuleMoves(t *testing.T) {
	t.Parallel()
	req := RuleInput{Text: "5+ years leading a team of 12 engineers with Rockwell", Type: "experience", NamedParties: []string{"Rockwell"}}
	v, adj := ApplyRules(req, evidencedMet(), ReportedFacts{StatedSpanYears: 0, AlternativeSatisfied: -1}, now2026)
	if len(adj) != 1 || adj[0].Rule != RuleDuration || v.Determination != DetUnresolved {
		t.Fatalf("expected the duration rule alone to move it: %+v %+v", v, adj)
	}
}

func TestEvidenceRule(t *testing.T) {
	t.Parallel()
	v, adj := EvidenceRule(evidencedMet(), 0)
	if v.Determination != DetNotMet || v.Basis != BasisNotAddressed || adj == nil || adj.Rule != RuleEvidence {
		t.Fatalf("met citing nothing: %+v %+v", v, adj)
	}
	v, adj = EvidenceRule(Verdict{Determination: DetUnresolved, Basis: BasisVocabulary, Uncertainty: []string{UncDepthUnknown}}, 0)
	if v.Determination != DetNotMet || adj == nil {
		t.Fatalf("unresolved citing nothing: %+v", v)
	}
	if v, adj := EvidenceRule(evidencedMet(), 2); v.Determination != DetMet || adj != nil {
		t.Fatalf("met with evidence: %+v %+v", v, adj)
	}
	if v, adj := EvidenceRule(Verdict{Determination: DetNotMet, Basis: BasisNotAddressed}, 0); adj != nil || v.Determination != DetNotMet {
		t.Fatalf("not_met is left alone: %+v %+v", v, adj)
	}
}
