package hiring

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func f64(v float64) *float64 { return &v }

// The thresholds a draft carries are read from its text by the parsers
// the rules always used, so an unedited requirement is compared against
// exactly what it was before thresholds were stored.
func TestDeriveThresholds(t *testing.T) {
	th := DeriveThresholds("10+ years of engineering experience, 3 within the last 5 years, leading teams of 12 engineers", "experience", nil)
	if th.MinYears == nil || *th.MinYears != 10 {
		t.Errorf("min_years = %v, want 10", th.MinYears)
	}
	if th.RecencyYears == nil || *th.RecencyYears != 5 {
		t.Errorf("recency_years = %v, want 5", th.RecencyYears)
	}
	if th.MinScope == nil || th.MinScope.Unit != "team_size" || th.MinScope.Value != 12 {
		t.Errorf("min_scope = %+v, want team_size 12", th.MinScope)
	}
	if err := th.Validate(); err != nil {
		t.Errorf("derived thresholds do not validate: %v", err)
	}

	money := DeriveThresholds("Own capital projects of $50M or more", "outcome", nil)
	if money.MinScope == nil || money.MinScope.Unit != "budget_usd" || money.MinScope.Value != 50e6 {
		t.Errorf("min_scope = %+v, want budget_usd 5e7", money.MinScope)
	}

	cred := DeriveThresholds("Bachelor's degree in Electrical Engineering or equivalent experience", "credential", nil)
	if cred.CredentialLevel != "bachelor" || cred.CredentialField != "Electrical Engineering" {
		t.Errorf("credential = %q in %q, want bachelor in Electrical Engineering", cred.CredentialLevel, cred.CredentialField)
	}
	// A level word in a requirement that is not a credential is not a
	// credential threshold.
	if got := DeriveThresholds("Master the art of PLC programming", "skill", nil); got.CredentialLevel != "" {
		t.Errorf("a skill requirement got credential level %q", got.CredentialLevel)
	}
	if got := DeriveThresholds("PMP certification", "credential", nil); got.CredentialLevel != "certificate" {
		t.Errorf("PMP certification level = %q, want certificate", got.CredentialLevel)
	}
	if got := DeriveThresholds("Partner with Rockwell", "relationship", []string{"Rockwell"}); got.RelationshipKind != "any_working" {
		t.Errorf("relationship kind = %q, want any_working", got.RelationshipKind)
	}
	if got := DeriveThresholds("Experience with SCADA systems", "tool", nil); !got.IsZero() {
		t.Errorf("a requirement that states no threshold got %+v", got)
	}
}

func TestThresholdsValidate(t *testing.T) {
	for name, th := range map[string]Thresholds{
		"negative years":     {MinYears: f64(-1)},
		"absurd years":       {MinYears: f64(99)},
		"unknown unit":       {MinScope: &Scope{Unit: "parsecs", Value: 3}},
		"zero scope":         {MinScope: &Scope{Unit: "sites", Value: 0}},
		"unknown level":      {CredentialLevel: "knighthood"},
		"unknown relation":   {RelationshipKind: "friend_of"},
		"field far too long": {CredentialField: strings.Repeat("x", MaxCredentialField+1)},
	} {
		if err := th.Validate(); err == nil {
			t.Errorf("%s validated", name)
		}
	}
}

// FR-HF-23: a years or credential item counts only once what is
// equivalent to it has been defined. The stimulus rows r3 and r7 are
// the two cases the owner approved.
func TestInclusion(t *testing.T) {
	cases := []struct {
		name        string
		typ         string
		th          Thresholds
		text        string
		equivalency string
		want        bool
	}{
		{"a skill always counts", "skill", Thresholds{}, "Ladder logic programming", "", true},
		{"a bare years item does not", "experience", Thresholds{MinYears: f64(10)}, "10+ years of engineering experience", "", false},
		{"a years item with the manager's equivalency does (r3)", "experience", Thresholds{MinYears: f64(10)}, "10+ years of engineering experience", "A graduate engineering degree counts as two of the ten years.", true},
		{"a bare credential does not (r7)", "credential", Thresholds{CredentialLevel: "certificate"}, "PMP certification", "", false},
		{"a credential the posting accepts experience for does", "credential", Thresholds{CredentialLevel: "bachelor"}, "Bachelor of Science in Electrical Engineering or equivalent experience", "", true},
		{"a wider field is not an equivalency", "credential", Thresholds{CredentialLevel: "bachelor"}, "Bachelor's degree in engineering or a related field", "", false},
		{"a tool line that states years is a years item", "tool", Thresholds{MinYears: f64(5)}, "5+ years of PLC programming with Allen-Bradley or Siemens platforms", "", false},
		{"an experience line that states no years is not", "experience", Thresholds{}, "MES or manufacturing operations management experience", "", true},
		{"years the posting accepts a combination for", "experience", Thresholds{MinYears: f64(8)}, "8 years of experience, or an equivalent combination of education and experience", "", true},
		{"whitespace is not an equivalency", "credential", Thresholds{}, "PE licence", "   ", false},
	}
	for _, c := range cases {
		got, reason := Inclusion(c.typ, c.th, c.text, c.equivalency)
		if got != c.want {
			t.Errorf("%s: included = %v, want %v (%s)", c.name, got, c.want, reason)
		}
		if !got && !strings.Contains(reason, "Not included") {
			t.Errorf("%s: an excluded item must say so in words: %q", c.name, reason)
		}
	}
	for text, want := range map[string]bool{
		"Bachelor's degree or equivalent experience":                   true,
		"Master's in a STEM field, or the equivalent":                  true,
		"Degree or equivalent combination of education and experience": true,
		"Ten years of experience in lieu of a degree":                  true,
		"Bachelor's degree or comparable experience":                   true,
		"PLC, DCS or SCADA experience":                                 false,
		"Bachelor's degree in engineering or related field":            false,
		"Equivalent circuits and power electronics":                    false,
	} {
		if got := PostingOffersEquivalent(text); got != want {
			t.Errorf("PostingOffersEquivalent(%q) = %v, want %v", text, got, want)
		}
	}
}

// Every template instance fits the schema's limit, names its type, and
// carries the three outcomes a reader needs.
func TestEvidenceStandardTemplates(t *testing.T) {
	full := Thresholds{
		MinYears: f64(10), RecencyYears: f64(5), MinScope: &Scope{Unit: "budget_usd", Value: 50e6},
		CredentialLevel: "master", CredentialField: strings.Repeat("Engineering ", 9),
	}
	parties := []string{"Rockwell Automation", "Siemens", "Emerson", "Honeywell", "ABB", "Schneider Electric", "Yokogawa", "Inductive Automation"}
	for _, typ := range RequirementTypes {
		for _, th := range []Thresholds{{}, full} {
			text, ref := RenderEvidenceStandard(typ, th, parties)
			if len(text) > MaxEvidenceStandard {
				t.Errorf("%s: %d characters, over the limit of %d", typ, len(text), MaxEvidenceStandard)
			}
			if ref != typ+"@"+TemplateVersion {
				t.Errorf("%s: template reference %q", typ, ref)
			}
			if typ == "behavioral" {
				if !strings.Contains(text, "Not judged") {
					t.Errorf("behavioral standard does not say it is not judged: %q", text)
				}
				continue
			}
			for _, part := range []string{"Met when", "Unresolved when", "Not met when"} {
				if !strings.Contains(text, part) {
					t.Errorf("%s: missing %q in %q", typ, part, text)
				}
			}
			if strings.Contains(text, "—") {
				t.Errorf("%s: an em dash in a standard a person reads: %q", typ, text)
			}
		}
	}
	if text, ref := RenderEvidenceStandard("no_such_type", Thresholds{}, nil); ref != "skill@"+TemplateVersion || text == "" {
		t.Errorf("an unknown type should fall back to the skill template, got %q", ref)
	}
	if text, _ := RenderEvidenceStandard("experience", Thresholds{MinYears: f64(8)}, nil); !strings.Contains(text, "at least 8 years") {
		t.Errorf("the experience standard does not carry its threshold: %q", text)
	}
}

func testCatalog() *Catalog {
	return &Catalog{Entries: []CatalogEntry{
		{ID: "e-fel", Kind: "vocabulary", Direction: "symmetric", Strength: "full",
			Conditions: []string{"Capital-projects domain only"},
			FromLabel:  "Front-end loading (FEL)", ToLabel: "Front-end development (capital projects)",
			FromTerms: []string{"front-end loading", "fel"}, ToTerms: []string{"front-end development", "front end development"}},
		{ID: "e-sw", Kind: "vocabulary", Direction: "one_way", Strength: "none",
			Conditions: []string{"Different domains; the words coincide and the work does not"},
			FromLabel:  "Front-end development (capital projects)", ToLabel: "Front-end development (software)",
			FromTerms: []string{"front-end development"}, ToTerms: []string{"front-end development", "frontend development"}},
		{ID: "e-scada", Kind: "tool", Direction: "one_way", Strength: "full",
			Conditions: []string{"The requirement is generic"},
			FromLabel:  "Ignition SCADA", ToLabel: "SCADA platform experience (generic)",
			FromTerms: []string{"ignition"}, ToTerms: []string{"scada"}},
		{ID: "e-tia", Kind: "tool", Direction: "symmetric", Strength: "partial",
			Conditions: []string{"Ladder and structured-text concepts transfer; vendor instruction sets do not"},
			FromLabel:  "Siemens TIA Portal", ToLabel: "Rockwell Studio 5000",
			FromTerms: []string{"tia portal", "siemens"}, ToTerms: []string{"studio 5000", "rockwell"}},
		{ID: "e-comfg", Kind: "vocabulary", Direction: "symmetric", Strength: "full",
			FromLabel: "Co-manufacturing", ToLabel: "Contract manufacturing",
			FromTerms: []string{"co-manufactur*"}, ToTerms: []string{"contract manufactur*"}},
	}}
}

// The catalog speaks to a requirement through the concept it asks for,
// on word boundaries, and says what counts before what does not.
func TestCatalogNotes(t *testing.T) {
	c := testCatalog()

	notes, ids := c.Notes("Lead front-end development of major investments", "the front-end development of future major investments, including scope definition")
	if len(ids) != 2 || ids[0] != "e-fel" || ids[1] != "e-sw" {
		t.Fatalf("entries = %v, want the full FEL entry then the software exclusion", ids)
	}
	if !strings.Contains(notes, "Front-end loading (FEL) counts as front-end development (capital projects).") ||
		!strings.Contains(notes, "is not front-end development (software).") {
		t.Errorf("notes = %q", notes)
	}

	// A symmetric entry reads from either end; a one-way entry does not.
	if n, _ := c.Notes("Experience with Siemens TIA Portal", ""); !strings.Contains(n, "Rockwell Studio 5000 may count toward Siemens TIA Portal") {
		t.Errorf("symmetric entry not read from the from side: %q", n)
	}
	if n, ids := c.Notes("Experience with Ignition", ""); n != "" || ids != nil {
		t.Errorf("a one-way entry matched on its from side: %q", n)
	}
	if n, _ := c.Notes("SCADA platform experience", ""); !strings.Contains(n, "Ignition SCADA counts as SCADA platform experience (generic). The requirement is generic.") {
		t.Errorf("generic SCADA note = %q", n)
	}

	// Word boundaries: "fel" is not in "felt", and a prefix term reaches
	// "co-manufacturers".
	if n, _ := c.Notes("The team felt strongly about quality", ""); n != "" {
		t.Errorf("a term matched inside another word: %q", n)
	}
	if n, _ := c.Notes("Manage relationships with co-manufacturers", ""); !strings.Contains(n, "Contract manufacturing counts as co-manufacturing.") {
		t.Errorf("prefix term did not match: %q", n)
	}

	var none *Catalog
	if n, ids := none.Notes("anything", ""); n != "" || ids != nil {
		t.Errorf("a nil catalog produced notes: %q", n)
	}
	if n, _ := c.Notes("Strong written communication", ""); n != "" {
		t.Errorf("a requirement nothing speaks to got notes: %q", n)
	}
}

// What the extraction stage turns the model's output into (FR-HF-02).
func TestDraft(t *testing.T) {
	c := testCatalog()
	prov := Provenance{PromptID: "jd_requirements", PromptFingerprint: "v5:abcd", Model: "m"}

	q := Draft(RequirementInput{
		LocalID: "r1", Text: "Bachelor's degree in engineering or equivalent experience", Category: "must", Weight: 2,
		Type: "credential", Alternatives: []string{"a Bachelor's degree in engineering", "equivalent experience"},
	}, c, prov)
	if q.Type != "credential" || q.Class != ClassRequired || q.Status != StatusDraft {
		t.Errorf("draft: type=%s class=%s status=%s", q.Type, q.Class, q.Status)
	}
	if q.Thresholds.CredentialLevel != "bachelor" || !q.Included || q.EvidenceStandardTemplate != "credential@"+TemplateVersion {
		t.Errorf("draft: thresholds=%+v included=%v template=%s", q.Thresholds, q.Included, q.EvidenceStandardTemplate)
	}
	if !strings.Contains(string(q.Extraction), `"type_source":"model"`) {
		t.Errorf("provenance does not say the model typed it: %s", q.Extraction)
	}

	// The heuristic stands in when the extractor gives no usable type.
	h := Draft(RequirementInput{LocalID: "r2", Text: "5+ years of PLC programming", Category: "must", Weight: 3, Type: "nonsense"}, c, prov)
	if h.Type != "experience" || !strings.Contains(string(h.Extraction), `"type_source":"`+TypeRule+`"`) {
		t.Errorf("fallback type = %s, provenance %s", h.Type, h.Extraction)
	}
	if h.Included {
		t.Error("a bare years item was included")
	}

	// Either reading of behavioral wins: the model must never judge a
	// working-style line (FR-HF-18).
	b := Draft(RequirementInput{LocalID: "r3", Text: "A collaborative team player", Category: "nice", Weight: 1, Type: "skill"}, c, prov)
	if b.Type != "behavioral" || b.Class != ClassPreferred {
		t.Errorf("behavioral by heuristic: type=%s class=%s", b.Type, b.Class)
	}
	m := Draft(RequirementInput{LocalID: "r4", Text: "Thrives in ambiguity", Category: "nice", Weight: 1, Type: "behavioral"}, c, prov)
	if m.Type != "behavioral" {
		t.Errorf("behavioral by the model: type=%s", m.Type)
	}

	// One "alternative" is the requirement restated, not a choice, and
	// a choice of wording is not an equivalency for the credential.
	one := Draft(RequirementInput{LocalID: "r5", Text: "PMP certification", Category: "nice", Weight: 1, Type: "credential", Alternatives: []string{"PMP certification"}}, c, prov)
	if len(one.Alternatives) != 0 || one.Included {
		t.Errorf("a single alternative: %v, included=%v", one.Alternatives, one.Included)
	}
	two := Draft(RequirementInput{LocalID: "r7", Text: "PMP or PRINCE2 certification", Category: "nice", Weight: 1, Type: "credential", Alternatives: []string{"PMP certification", "PRINCE2 certification"}}, c, prov)
	if len(two.Alternatives) != 2 || two.Included {
		t.Errorf("two certificates are a choice, not an equivalency: %v, included=%v", two.Alternatives, two.Included)
	}

	// Out-of-range values from the model are bounded, not refused.
	odd := Draft(RequirementInput{LocalID: "r6", Text: "Something", Category: "maybe", Weight: 9}, nil, prov)
	if odd.Category != "nice" || odd.Weight != 2 || odd.EquivalenceNotes != "" || odd.NamedParties == nil || odd.Alternatives == nil {
		t.Errorf("bounded draft = %+v", odd)
	}
}

// An edit carries over what it does not touch, derives again what was
// still derived, and keeps what a person set.
func TestApplyPatch(t *testing.T) {
	c := testCatalog()
	cur := Draft(RequirementInput{LocalID: "r1", Text: "10+ years of front-end development", Category: "must", Weight: 3, Type: "experience"}, c, Provenance{})
	cur.ID, cur.Status = "id-1", StatusActive
	if cur.Thresholds.MinYears == nil || cur.Included || cur.EquivalenceNotes == "" {
		t.Fatalf("fixture: %+v", cur)
	}

	// The capital-projects case: the text is corrected, and the
	// thresholds, the standard and the notes follow it.
	text := "Lead front-end loading of major capital investments: scope definition and basis of design"
	typ := "outcome"
	next, err := applyPatch(cur, RequirementPatch{Text: &text, Type: &typ}, c)
	if err != nil {
		t.Fatalf("applyPatch: %v", err)
	}
	if next.Text != text || next.Type != "outcome" {
		t.Errorf("text/type not applied: %+v", next)
	}
	if next.Thresholds.MinYears != nil {
		t.Errorf("the old wording's years threshold survived the rewording: %+v", next.Thresholds)
	}
	if next.EvidenceStandardTemplate != "outcome@"+TemplateVersion || !next.Included {
		t.Errorf("standard template %s, included %v; want the outcome template and included", next.EvidenceStandardTemplate, next.Included)
	}
	if !strings.Contains(next.EquivalenceNotes, "Front-end development (capital projects) counts as front-end loading (FEL)") {
		t.Errorf("notes did not follow the new text: %q", next.EquivalenceNotes)
	}

	// A threshold a person set is kept when the text changes later.
	handSet := cur
	handSet.Thresholds = Thresholds{MinYears: f64(8)}
	other := "Eight or so years of front-end development"
	kept, err := applyPatch(handSet, RequirementPatch{Text: &other}, c)
	if err != nil || kept.Thresholds.MinYears == nil || *kept.Thresholds.MinYears != 8 {
		t.Errorf("a hand-set threshold was not kept: %+v (err %v)", kept.Thresholds, err)
	}

	// A rewritten standard is marked as a person's and survives a later
	// change of type.
	std := "Met when the documents show a project taken through the definition phase."
	edited, err := applyPatch(cur, RequirementPatch{EvidenceStandard: &std}, c)
	if err != nil || edited.EvidenceStandard != std || edited.EvidenceStandardTemplate != EditedTemplatePrefix+"experience@"+TemplateVersion {
		t.Fatalf("edited standard: %q (%s), err %v", edited.EvidenceStandard, edited.EvidenceStandardTemplate, err)
	}
	retyped, err := applyPatch(edited, RequirementPatch{Type: &typ}, c)
	if err != nil || retyped.EvidenceStandard != std {
		t.Errorf("a person's standard was overwritten by the template on a type change: %q (err %v)", retyped.EvidenceStandard, err)
	}

	// Defining the equivalency is what brings a years item into the
	// numbers (FR-HF-23).
	eq := "A graduate engineering degree counts as two of the ten years."
	defined, err := applyPatch(cur, RequirementPatch{Equivalency: &eq}, c)
	if err != nil || !defined.Included {
		t.Errorf("defining an equivalency did not include the item: included=%v err=%v", defined.Included, err)
	}

	// Class and weight.
	ideal, w := ClassIdeal, 1
	classed, err := applyPatch(cur, RequirementPatch{Class: &ideal, Weight: &w}, c)
	if err != nil || classed.Class != ClassIdeal || classed.Weight != 1 || classed.Category != "must" {
		t.Errorf("class/weight: %+v (err %v); the posting's category must not change", classed, err)
	}

	// Refusals.
	same := cur.Text
	if _, err := applyPatch(cur, RequirementPatch{Text: &same}, c); !errors.Is(err, ErrNothingChanged) {
		t.Errorf("an edit that changes nothing: err = %v", err)
	}
	if _, err := applyPatch(cur, RequirementPatch{}, c); !errors.Is(err, ErrNothingChanged) {
		t.Errorf("an empty edit: err = %v", err)
	}
	empty, bad, badClass, zero := "  ", "wizardry", "essential", 0
	long := strings.Repeat("x", MaxRequirementText+1)
	for name, p := range map[string]RequirementPatch{
		"empty text":            {Text: &empty},
		"over-long text":        {Text: &long},
		"unknown type":          {Type: &bad},
		"unknown class":         {Class: &badClass},
		"weight 0":              {Weight: &zero},
		"invalid threshold":     {Thresholds: &Thresholds{MinYears: f64(-2)}},
		"too many alternatives": {Alternatives: &[]string{"a", "b", "c", "d", "e", "f", "g"}},
	} {
		if _, err := applyPatch(cur, p, c); !errors.Is(err, ErrInvalidRequirement) {
			t.Errorf("%s: err = %v, want ErrInvalidRequirement", name, err)
		}
	}
}

// Formula v4 (FR-HF-23): required at full weight, preferred at a
// fraction and always counted, ideal as bonus only, and an item with no
// defined equivalency left out.
func TestCoverageV4(t *testing.T) {
	near := func(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

	// The stimulus's shape: required items carry the record; an unmet
	// preferred item costs a quarter of its weight.
	c := CoverageV4([]Item{
		{Weight: 3, Class: ClassRequired, Determination: DetMet},
		{Weight: 2, Class: ClassRequired, Determination: DetMetEquivalent},
		{Weight: 2, Class: ClassRequired, Determination: DetUnresolved},
		{Weight: 2, Class: ClassPreferred, Determination: DetNotMet},
	}, 0.25)
	// denominator 3 + 2 + 2 + 0.5 = 7.5
	if !near(c.Denominator, 7.5) || !near(c.Evidenced, 5/7.5) || !near(c.Unresolved, 2/7.5) || !near(c.Unmet, 0.5/7.5) {
		t.Errorf("coverage = %+v", c)
	}
	if !near(c.EquivalentShare, 2.0/5.0) {
		t.Errorf("equivalent share = %v, want 0.4", c.EquivalentShare)
	}
	if !near(c.Evidenced+c.Unresolved+c.Unmet, 1) {
		t.Errorf("the three numbers sum to %v", c.Evidenced+c.Unresolved+c.Unmet)
	}

	// Ideal never counts against, and can only raise a figure.
	base := []Item{{Weight: 3, Class: ClassRequired, Determination: DetMet}, {Weight: 3, Class: ClassRequired, Determination: DetNotMet}}
	without := CoverageV4(base, 0.25)
	unmetIdeal := CoverageV4(append(append([]Item{}, base...), Item{Weight: 3, Class: ClassIdeal, Determination: DetNotMet}), 0.25)
	metIdeal := CoverageV4(append(append([]Item{}, base...), Item{Weight: 3, Class: ClassIdeal, Determination: DetMet}), 0.25)
	if !near(unmetIdeal.Evidenced, without.Evidenced) || !near(unmetIdeal.Denominator, without.Denominator) {
		t.Errorf("an unmet ideal item moved the numbers: %+v against %+v", unmetIdeal, without)
	}
	if !(metIdeal.Evidenced > without.Evidenced) {
		t.Errorf("an evidenced ideal item did not raise coverage: %v against %v", metIdeal.Evidenced, without.Evidenced)
	}

	// Excluded and weight 0 items are out of everything.
	ex := CoverageV4([]Item{
		{Weight: 3, Class: ClassRequired, Determination: DetMet},
		{Weight: 3, Class: ClassRequired, Determination: DetNotMet, Excluded: true},
		{Weight: 0, Class: ClassRequired, Determination: DetUnresolved},
	}, 0.25)
	if !near(ex.Evidenced, 1) || !near(ex.Denominator, 3) {
		t.Errorf("excluded items counted: %+v", ex)
	}

	// The class a row never got is the one its category implies, and a
	// fraction out of range is the default.
	implied := CoverageV4([]Item{{Weight: 2, Category: "must", Determination: DetMet}, {Weight: 2, Category: "nice", Determination: DetNotMet}}, 0)
	if !near(implied.Denominator, 2.5) || !near(implied.Evidenced, 2/2.5) {
		t.Errorf("implied classes at the default fraction: %+v", implied)
	}
	half := CoverageV4([]Item{{Weight: 2, Class: ClassRequired, Determination: DetMet}, {Weight: 2, Class: ClassPreferred, Determination: DetNotMet}}, 0.5)
	if !near(half.Denominator, 3) {
		t.Errorf("a fraction of one half: %+v", half)
	}
	if got := CoverageV4(nil, 0.25); got != (Coverage{}) {
		t.Errorf("no items: %+v", got)
	}
}

// The rules compare against the stored thresholds, so an edit to a
// threshold is what the rule reads, whatever the text still says.
func TestRulesReadStoredThresholds(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	met := Verdict{Determination: DetMet, Basis: BasisDirect}
	facts := ReportedFacts{StatedSpanYears: 6, AlternativeSatisfied: -1}
	text := "10+ years of controls engineering"

	// No row: the text decides, as before.
	v, adj := ApplyRules(RuleInput{Text: text, Type: "experience"}, met, facts, now)
	if v.Determination != DetNotMet || len(adj) != 1 || adj[0].Rule != RuleDuration {
		t.Errorf("text-derived threshold: %+v %+v", v, adj)
	}
	// The hiring manager lowered the threshold to 6: met stands.
	v, adj = ApplyRules(RuleInput{Text: text, Type: "experience", Thresholds: &Thresholds{MinYears: f64(6)}}, met, facts, now)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Errorf("edited threshold of 6 against 6 stated: %+v %+v", v, adj)
	}
	// And removed it: no comparison at all, though the text says ten.
	v, adj = ApplyRules(RuleInput{Text: text, Type: "experience", Thresholds: &Thresholds{}}, met, facts, now)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Errorf("removed threshold: %+v %+v", v, adj)
	}
	// A scope threshold in the vocabulary's unit compares with the
	// unit the judge reports.
	scoped := RuleInput{Text: "Lead a team", Type: "outcome", Thresholds: &Thresholds{MinScope: &Scope{Unit: "team_size", Value: 10}}}
	v, adj = ApplyRules(scoped, met, ReportedFacts{StatedScope: &Scope{Unit: "people", Value: 14}, AlternativeSatisfied: -1}, now)
	if v.Determination != DetMet || len(adj) != 0 {
		t.Errorf("team_size 10 against 14 people: %+v %+v", v, adj)
	}
	v, adj = ApplyRules(scoped, met, ReportedFacts{StatedScope: &Scope{Unit: "people", Value: 3}, AlternativeSatisfied: -1}, now)
	if v.Determination != DetNotMet || len(adj) != 1 || adj[0].Rule != RuleScope || adj[0].RuleVersion != RuleScopeVersion {
		t.Errorf("team_size 10 against 3 people: %+v %+v", v, adj)
	}
}
