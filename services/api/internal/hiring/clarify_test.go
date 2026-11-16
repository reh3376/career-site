package hiring

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode"
)

var clarifyNow = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func testEngagements() []Engagement {
	return []Engagement{
		{Ref: "corpus_document:52", Kind: "role", Title: "VP of Engineering & Technology", Organization: "Whiskey House", Start: 2022, End: 2026},
		{Ref: "corpus_document:53", Kind: "role", Title: "Director of Engineering", Organization: "Bardstown Bourbon", Start: 2017, End: 2022},
		{Ref: "corpus_document:54", Kind: "role", Title: "Senior Process Engineer", Organization: "Sazerac", Start: 2015, End: 2018},
		{Ref: "corpus_document:60", Kind: "project", Title: "MDEMG"},
	}
}

func testLists() GenericLists {
	code := func(vocab string, codes ...string) []Code {
		var out []Code
		for _, c := range codes {
			out = append(out, Code{Vocab: vocab, Code: c, Label: strings.ToUpper(c[:1]) + strings.ReplaceAll(c[1:], "_", " ")})
		}
		return out
	}
	return GenericLists{
		Involvement:       code(VocabInvolvementLevel, "aware", "used", "configured", "designed", "owned", "taught"),
		ScopeUnits:        code(VocabScopeUnit, "team_size", "budget_usd", "sites"),
		CredentialLevels:  code(VocabCredentialLevel, "certificate", "associate", "bachelor", "master"),
		RelationshipKinds: code(VocabRelationshipKind, "employed_by", "contracted_to", "partner_of"),
		ActivityClasses:   code(VocabActivityClass, "engineering_design", "software_development", "project_delivery"),
	}
}

// Every uncertainty code of the vocabulary has the answer type 04 §4.1
// gives it, so a new code that lands without one is asked as a pointer
// by decision and not by accident.
func TestAnswerTypeForCoversTheVocabulary(t *testing.T) {
	want := map[string]string{
		UncScopeUnknown: AnswerMagnitude, UncDurationUnknown: AnswerSpan, UncRecencyUnknown: AnswerSpan,
		UncDepthUnknown: AnswerInvolvement, UncEquivalenceUnconfirmed: AnswerEvidencePointer,
		UncCredentialUnknown: AnswerCredential, UncPartyUnknown: AnswerRelationship, UncEvidenceAmbiguous: AnswerDisambiguation,
	}
	codes := Codes(DefaultVocab().Uncertainty)
	if len(codes) != len(want) {
		t.Fatalf("the vocabulary has %d uncertainty codes and this test places %d", len(codes), len(want))
	}
	for _, c := range codes {
		if got := AnswerTypeFor(c); got != want[c] {
			t.Errorf("AnswerTypeFor(%s) = %s, want %s", c, got, want[c])
		}
	}
	if AnswerTypeFor("a_code_from_a_later_vocabulary") != AnswerEvidencePointer {
		t.Error("an unknown code is asked as a pointer to the candidate's own work")
	}
}

func TestPrimaryUncertainty(t *testing.T) {
	for name, c := range map[string]struct {
		codes []string
		basis string
		want  string
	}{
		"the one code":                      {[]string{UncEquivalenceUnconfirmed}, BasisAdjacentDomain, UncEquivalenceUnconfirmed},
		"a number a rule compares first":    {[]string{UncEquivalenceUnconfirmed, UncDurationUnknown}, BasisVocabulary, UncDurationUnknown},
		"a named party before anything":     {[]string{UncScopeUnknown, UncPartyUnknown}, BasisRelationshipUnproven, UncPartyUnknown},
		"a person's unresolved, shortfall":  {nil, BasisScopeShortfall, UncScopeUnknown},
		"a person's unresolved, lexical":    {nil, BasisToolEquivalent, UncEquivalenceUnconfirmed},
		"a person's unresolved, credential": {[]string{}, BasisExperienceForEducation, UncCredentialUnknown},
		"a person's unresolved, relation":   {nil, BasisRelationshipUnproven, UncPartyUnknown},
		"a code this build does not know":   {[]string{"later_code"}, BasisVocabulary, "later_code"},
	} {
		if got := PrimaryUncertainty(c.codes, c.basis); got != c.want {
			t.Errorf("%s: %s, want %s", name, got, c.want)
		}
	}
}

// FR-HF-24: the lexical bases and an unproven relationship ask without
// being asked; a shortfall of scale or years waits for the candidate.
func TestAsksUnprompted(t *testing.T) {
	for basis, want := range map[string]bool{
		BasisVocabulary: true, BasisToolEquivalent: true, BasisAdjacentDomain: true, BasisRelationshipUnproven: true,
		BasisScopeShortfall: false, BasisDurationShortfall: false, BasisRecency: false,
		BasisExperienceForEducation: false, BasisEducationForExperience: false, BasisDirect: false, BasisNotAddressed: false, BasisContradicted: false,
	} {
		if got := AsksUnprompted(basis); got != want {
			t.Errorf("AsksUnprompted(%s) = %v, want %v", basis, got, want)
		}
	}
}

// A question asks for evidence, never confirmation (03 §3.3): no
// template carries a figure, every one fits the column and every answer
// type has a default and its fields.
func TestQuestionTemplates(t *testing.T) {
	types := []string{AnswerSpan, AnswerMagnitude, AnswerInvolvement, AnswerCredential, AnswerRelationship, AnswerEvidencePointer, AnswerDisambiguation}
	for _, at := range types {
		if QuestionText(at, "") == "" {
			t.Errorf("answer type %s has no default question", at)
		}
		if QuestionText(at, "a_type_with_no_template") != QuestionText(at, "") {
			t.Errorf("answer type %s must fall back to its default wording", at)
		}
		fields := QuestionFields(at, testEngagements(), testLists(), []string{"Rockwell Automation", " Siemens "})
		if len(fields) == 0 {
			t.Errorf("answer type %s offers no fields", at)
		}
		for _, f := range fields {
			if (f.Kind == FieldChoice || f.Kind == FieldMulti) && len(f.Options) == 0 {
				t.Errorf("%s.%s is a choice with nothing to choose", at, f.Name)
			}
			for _, o := range f.Options {
				if o.Source != OptionOwnCorpus && o.Source != OptionGenericList && o.Source != OptionPosting {
					t.Errorf("%s.%s option %q has no stated source", at, f.Name, o.Value)
				}
			}
		}
	}
	for key, q := range questionTemplates {
		if n := len([]rune(q)); n > 300 {
			t.Errorf("template %v is %d characters; the column holds 300", key, n)
		}
		if strings.IndexFunc(q, unicode.IsDigit) >= 0 {
			t.Errorf("template %v carries a figure; a question must not repeat what a requirement is scored on: %q", key, q)
		}
		if strings.Contains(q, "—") {
			t.Errorf("template %v has an em dash", key)
		}
	}
	if QuestionText(AnswerInvolvement, "tool") == QuestionText(AnswerInvolvement, "") {
		t.Error("the tool wording of an involvement question is its own template")
	}
	// A span offers roles and not projects, which have no dates to add up.
	for _, o := range QuestionFields(AnswerSpan, testEngagements(), testLists(), nil)[0].Options {
		if strings.Contains(o.Label, "project") {
			t.Errorf("a span must not offer a project: %q", o.Label)
		}
	}
	// A relationship offers the posting's own named parties, trimmed.
	orgs := QuestionFields(AnswerRelationship, nil, testLists(), []string{"Rockwell Automation", " Siemens ", ""})[0].Options
	if len(orgs) != 2 || orgs[1].Value != "Siemens" || orgs[0].Source != OptionPosting {
		t.Errorf("relationship organisations: %+v", orgs)
	}
	if got := testEngagements()[0].Label(); got != "VP of Engineering & Technology, Whiskey House, 2022 to 2026" {
		t.Errorf("engagement label = %q", got)
	}
	if got := testEngagements()[3].Label(); got != "MDEMG, project" {
		t.Errorf("project label = %q", got)
	}
}

func TestNormaliseAnswer(t *testing.T) {
	inv := QuestionFields(AnswerInvolvement, testEngagements(), testLists(), nil)
	span := QuestionFields(AnswerSpan, testEngagements(), testLists(), nil)
	ptr := QuestionFields(AnswerEvidencePointer, testEngagements(), testLists(), nil)

	ok, err := NormaliseAnswer(AnswerInvolvement, inv, Answer{Status: ItemAnswered,
		Values: map[string]any{"level": " designed ", "engagement": "corpus_document:52", "not_a_field": "x"}, FreeText: "  Wrote the protocols.  "})
	if err != nil || ok.str("level") != "designed" || ok.FreeText != "Wrote the protocols." || len(ok.Values) != 2 {
		t.Fatalf("a good answer: %+v, %v", ok, err)
	}
	// Ticked in any order, kept in the order offered; a number may
	// arrive as text from a form.
	sp, err := NormaliseAnswer(AnswerSpan, span, Answer{Status: ItemAnswered,
		Values: map[string]any{"engagements": []any{"corpus_document:53", "corpus_document:52", "corpus_document:53"}, "years": "6.5"}})
	if err != nil || strings.Join(sp.list("engagements"), ",") != "corpus_document:52,corpus_document:53" || sp.num("years") != 6.5 {
		t.Fatalf("a span answer: %+v, %v", sp, err)
	}
	for name, c := range map[string]struct {
		at     string
		fields []Field
		a      Answer
	}{
		"a choice that was not offered":    {AnswerInvolvement, inv, Answer{Status: ItemAnswered, Values: map[string]any{"level": "expert", "engagement": "corpus_document:52"}}},
		"a required field left out":        {AnswerInvolvement, inv, Answer{Status: ItemAnswered, Values: map[string]any{"level": "designed"}}},
		"an engagement that is not theirs": {AnswerSpan, span, Answer{Status: ItemAnswered, Values: map[string]any{"engagements": []string{"corpus_document:52", "corpus_document:999"}}}},
		"nothing picked":                   {AnswerSpan, span, Answer{Status: ItemAnswered, Values: map[string]any{"engagements": []string{}}}},
		"a number out of range":            {AnswerSpan, span, Answer{Status: ItemAnswered, Values: map[string]any{"engagements": []string{"corpus_document:52"}, "years": 75.0}}},
		"a number that is not one":         {AnswerSpan, span, Answer{Status: ItemAnswered, Values: map[string]any{"engagements": []string{"corpus_document:52"}, "years": "several"}}},
		"a pointer with nothing said":      {AnswerEvidencePointer, ptr, Answer{Status: ItemAnswered, Values: map[string]any{"engagements": []string{"corpus_document:60"}}}},
		"a note that is too long":          {AnswerInvolvement, inv, Answer{Status: ItemDeclined, FreeText: strings.Repeat("x", MaxAnswerFreeText+1)}},
		"a status that is not one":         {AnswerInvolvement, inv, Answer{Status: "maybe"}},
	} {
		if _, err := NormaliseAnswer(c.at, c.fields, c.a); !errors.Is(err, ErrInvalidAnswer) {
			t.Errorf("%s: err = %v, want invalid", name, err)
		}
	}
	// The two ways out carry no values, whatever was sent with them.
	na, err := NormaliseAnswer(AnswerInvolvement, inv, Answer{Status: ItemNotApplicable, Values: map[string]any{"level": "designed"}})
	if err != nil || na.Values != nil {
		t.Errorf("not applicable: %+v, %v", na, err)
	}
}

func TestAnswerStatement(t *testing.T) {
	inv := QuestionFields(AnswerInvolvement, testEngagements(), testLists(), nil)
	a, _ := NormaliseAnswer(AnswerInvolvement, inv, Answer{Status: ItemAnswered,
		Values: map[string]any{"level": "designed", "engagement": "corpus_document:52"}, FreeText: "Wrote the validation protocols."})
	want := "The candidate's own answer to a follow-up question about this requirement, given 2026-11-15. " +
		"Your involvement: Designed. Which role or project: VP of Engineering & Technology, Whiskey House, 2022 to 2026. " +
		"The candidate adds: Wrote the validation protocols."
	if got := AnswerStatement(inv, a, clarifyNow); got != want {
		t.Errorf("statement:\n got %q\nwant %q", got, want)
	}
	if got := AnswerLines(inv, Answer{Status: ItemNotApplicable}); len(got) != 1 || got[0] != "The candidate states they have not done this." {
		t.Errorf("not applicable lines: %v", got)
	}
}

// What a structured answer states goes where the code rules read it.
func TestAnswerFacts(t *testing.T) {
	span := QuestionFields(AnswerSpan, testEngagements(), testLists(), nil)
	// 2022 to 2026 and 2017 to 2022 and 2015 to 2018: overlapping roles
	// are counted once, so 2015 to 2026 is eleven years.
	a, _ := NormaliseAnswer(AnswerSpan, span, Answer{Status: ItemAnswered,
		Values: map[string]any{"engagements": []string{"corpus_document:52", "corpus_document:53", "corpus_document:54"}}})
	f := AnswerFacts(AnswerSpan, span, a, ReportedFacts{StatedSpanYears: 2, AlternativeSatisfied: -1}, clarifyNow)
	if f.StatedSpanYears != 11 || f.MostRecentYear != 2026 || f.AlternativeSatisfied != -1 {
		t.Errorf("span facts: %+v", f)
	}
	// "Only part of my time there" is the candidate's own smaller figure.
	a.Values["years"] = 4.0
	if f := AnswerFacts(AnswerSpan, span, a, ReportedFacts{}, clarifyNow); f.StatedSpanYears != 4 {
		t.Errorf("a stated part of the span: %+v", f)
	}
	// A figure larger than the roles cover is not taken over the roles.
	a.Values["years"] = 30.0
	if f := AnswerFacts(AnswerSpan, span, a, ReportedFacts{}, clarifyNow); f.StatedSpanYears != 11 {
		t.Errorf("a figure larger than the roles: %+v", f)
	}

	mag := QuestionFields(AnswerMagnitude, nil, testLists(), nil)
	m, _ := NormaliseAnswer(AnswerMagnitude, mag, Answer{Status: ItemAnswered, Values: map[string]any{"unit": "budget_usd", "value": 6e7}})
	if f := AnswerFacts(AnswerMagnitude, mag, m, ReportedFacts{}, clarifyNow); f.StatedScope == nil || f.StatedScope.Unit != "budget_usd" || f.StatedScope.Value != 6e7 {
		t.Errorf("magnitude facts: %+v", f.StatedScope)
	}
	cred := QuestionFields(AnswerCredential, nil, testLists(), nil)
	c, _ := NormaliseAnswer(AnswerCredential, cred, Answer{Status: ItemAnswered,
		Values: map[string]any{"level": "bachelor", "field": "Applied  Mathematics", "status": "completed"}})
	if f := AnswerFacts(AnswerCredential, cred, c, ReportedFacts{}, clarifyNow); f.CredentialFound == nil || f.CredentialFound.Level != "bachelor" ||
		f.CredentialFound.Field != "Applied Mathematics" || f.CredentialFound.Status != "completed" {
		t.Errorf("credential facts: %+v", f.CredentialFound)
	}
	rel := QuestionFields(AnswerRelationship, nil, testLists(), []string{"Rockwell Automation", "Siemens"})
	r, _ := NormaliseAnswer(AnswerRelationship, rel, Answer{Status: ItemAnswered, Values: map[string]any{"orgs": []string{"Rockwell Automation"}, "kind": "partner_of"}})
	if f := AnswerFacts(AnswerRelationship, rel, r, ReportedFacts{PartiesEvidenced: []string{"rockwell automation"}}, clarifyNow); len(f.PartiesEvidenced) != 1 {
		t.Errorf("a party already evidenced is not added twice: %v", f.PartiesEvidenced)
	}
	if f := AnswerFacts(AnswerRelationship, rel, r, ReportedFacts{}, clarifyNow); len(f.PartiesEvidenced) != 1 || f.PartiesEvidenced[0] != "Rockwell Automation" {
		t.Errorf("relationship facts: %v", f.PartiesEvidenced)
	}
	// A declined item states nothing.
	if f := AnswerFacts(AnswerSpan, span, Answer{Status: ItemDeclined}, ReportedFacts{StatedSpanYears: 2}, clarifyNow); f.StatedSpanYears != 2 {
		t.Errorf("a declined item must leave the facts alone: %+v", f)
	}
}

// A judgment a code rule left unresolved is settled in code once the
// candidate states the fact: the same rule, over the model's own
// verdict, with the number in place.
func TestResettle(t *testing.T) {
	five := 5.0
	req := RuleInput{Text: "5+ years of PLC programming", Type: "experience", Thresholds: &Thresholds{MinYears: &five}}
	model := Verdict{Determination: DetMet, Basis: BasisDirect, Uncertainty: []string{}}
	// As the run left it: the model said met, the evidence stated no
	// span, the duration rule set it aside.
	weak, adjs := ApplyRules(req, model, ReportedFacts{AlternativeSatisfied: -1}, clarifyNow)
	if weak.Determination != DetUnresolved || len(adjs) != 1 || adjs[0].Rule != RuleDuration {
		t.Fatalf("the fixture must be a rule-made unresolved: %+v %+v", weak, adjs)
	}
	// Eleven years stated: met again, by the rule, with the settling
	// recorded.
	v, out, ok := Resettle(req, AnswerSpan, adjs, weak.Basis, ReportedFacts{StatedSpanYears: 11, AlternativeSatisfied: -1}, clarifyNow)
	if !ok || v.Determination != DetMet || v.Basis != BasisDirect || len(v.Uncertainty) != 0 {
		t.Fatalf("enough years stated: %+v ok=%v", v, ok)
	}
	if len(out) != 1 || out[0].Rule != RuleClarification || out[0].FromDetermination != DetUnresolved || out[0].ToDetermination != DetMet {
		t.Errorf("the settling must be the one adjustment left: %+v", out)
	}
	// Two years stated against five: a shortfall, and it says so.
	v, out, ok = Resettle(req, AnswerSpan, adjs, weak.Basis, ReportedFacts{StatedSpanYears: 2, AlternativeSatisfied: -1}, clarifyNow)
	if !ok || v.Determination != DetNotMet || v.Basis != BasisDurationShortfall {
		t.Errorf("too few years stated: %+v ok=%v", v, ok)
	}
	if len(out) != 2 || out[0].Rule != RuleDuration || out[1].Rule != RuleClarification || out[1].ToDetermination != DetNotMet {
		t.Errorf("a shortfall keeps the rule's reason and the settling: %+v", out)
	}
	// An answer of another kind does not speak to the duration rule.
	if _, _, ok := Resettle(req, AnswerInvolvement, adjs, weak.Basis, ReportedFacts{}, clarifyNow); ok {
		t.Error("an involvement answer cannot settle a missing span")
	}
	// A judgment the model itself left unresolved has no rule to run
	// again: the judge looks at the answer.
	if _, _, ok := Resettle(req, AnswerSpan, nil, BasisAdjacentDomain, ReportedFacts{StatedSpanYears: 11}, clarifyNow); ok {
		t.Error("a model's own unresolved is not settled in code")
	}
	// What ran before the rule is kept.
	before := append([]RuleAdjustment{{Rule: RuleConsistency, Reason: "repaired"}}, adjs...)
	_, out, _ = Resettle(req, AnswerSpan, before, weak.Basis, ReportedFacts{StatedSpanYears: 11, AlternativeSatisfied: -1}, clarifyNow)
	if len(out) != 2 || out[0].Rule != RuleConsistency || out[1].Rule != RuleClarification {
		t.Errorf("an earlier adjustment must survive the settling: %+v", out)
	}

	// A named party: the relationship rule, settled by the organisation
	// the candidate says they worked with.
	rel := RuleInput{Text: "work with Rockwell Automation", Type: "relationship", NamedParties: []string{"Rockwell Automation"}}
	weakRel, relAdjs := ApplyRules(rel, model, ReportedFacts{AlternativeSatisfied: -1}, clarifyNow)
	if weakRel.Determination != DetUnresolved || relAdjs[0].Rule != RuleRelationship {
		t.Fatalf("the relationship fixture: %+v %+v", weakRel, relAdjs)
	}
	v, _, ok = Resettle(rel, AnswerRelationship, relAdjs, weakRel.Basis, ReportedFacts{PartiesEvidenced: []string{"Rockwell Automation"}, AlternativeSatisfied: -1}, clarifyNow)
	if !ok || v.Determination != DetMet {
		t.Errorf("a named party stated: %+v ok=%v", v, ok)
	}

	na, adj := NotApplicableVerdict(BasisAdjacentDomain)
	if na.Determination != DetNotMet || na.Basis != BasisNotAddressed || adj.Rule != RuleClarification || adj.FromBasis != BasisAdjacentDomain {
		t.Errorf("not applicable: %+v %+v", na, adj)
	}
}

func TestNormaliseAnswerTakesEveryValueAsAList(t *testing.T) {
	fields := QuestionFields(AnswerSpan, []Engagement{
		{Ref: "corpus_document:1", Kind: "role", Title: "Director", Organization: "Acme", Start: 2018, End: 2022},
	}, GenericLists{}, nil)
	got, err := NormaliseAnswer(AnswerSpan, fields, Answer{Status: ItemAnswered, Values: map[string]any{
		"engagements": []string{"corpus_document:1"},
		"years":       []string{"2.5"},
	}})
	if err != nil {
		t.Fatalf("normalise: %v", err)
	}
	if got.num("years") != 2.5 || len(got.list("engagements")) != 1 {
		t.Errorf("values = %#v", got.Values)
	}
	// An empty list for an optional number is no value, not a zero.
	got, err = NormaliseAnswer(AnswerSpan, fields, Answer{Status: ItemAnswered, Values: map[string]any{
		"engagements": []string{"corpus_document:1"},
		"years":       []string{},
	}})
	if err != nil {
		t.Fatalf("normalise: %v", err)
	}
	if _, has := got.Values["years"]; has {
		t.Errorf("an empty years should be left out: %#v", got.Values)
	}
}
