package hiring

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// The requirement as an editable entity (FR-HF-01, 02, 03, 23; Phase 0
// step 5). A requirement is drafted by the extraction stage, edited by
// a person with the admin role, released, and judged; every edit is a
// new version, so a judgment's requirement_id names exactly what was
// judged.
//
// What the model contributes to a draft is the text, the quote, the
// posting's category and weight, the parties, the type and the
// alternatives the posting itself offers. Everything else here is
// computed in code from those: thresholds are read out of the text by
// the same narrow parsers the rules have always used, the evidence
// standard is rendered from the type's template, the equivalence notes
// from the catalog, the class from the category, and inclusion from
// the rule below. Numbers are decided in code, as everywhere else.

// Limits, from 04 §1.3. The text limit is the schema's; the extractor
// is asked for 200 and an editor may use the rest.
const (
	MaxRequirementText  = 500
	MaxSourceQuote      = 1000
	MaxEvidenceStandard = 500
	MaxEquivalenceNotes = 1000
	MaxEquivalency      = 500
	MaxAlternatives     = 6
	MaxAlternativeText  = 300
	MaxNamedParties     = 8
	MaxNamedPartyText   = 120
	MaxEditReason       = 500
	MaxCredentialField  = 120
	MaxThresholdYears   = 60
)

// Requirement statuses (04 §1.1). A requirement has at most one active
// version, the one judging reads, and at most one draft, the pending
// edit; everything else is retired.
const (
	StatusDraft   = "draft"
	StatusActive  = "active"
	StatusRetired = "retired"
)

// ThresholdsRule names the parsers that read thresholds out of a
// requirement's text, recorded in each draft's provenance so a row
// whose thresholds were derived can be told from one a person set.
const ThresholdsRule = "thresholds-v1"

// InclusionRule names the rule that decides whether a requirement
// counts in the numbers (FR-HF-23), recorded on the match record.
const InclusionRule = "incl-v1"

// Thresholds are the structured values the code rules compare against
// (04 §1.2), so a shortfall is arithmetic. All optional; present only
// when the requirement states one.
type Thresholds struct {
	MinYears         *float64 `json:"min_years,omitempty"`
	RecencyYears     *float64 `json:"recency_years,omitempty"`
	MinScope         *Scope   `json:"min_scope,omitempty"`
	CredentialLevel  string   `json:"credential_level,omitempty"`
	CredentialField  string   `json:"credential_field,omitempty"`
	RelationshipKind string   `json:"relationship_kind,omitempty"`
}

// IsZero reports whether no threshold is set.
func (t Thresholds) IsZero() bool {
	return t.MinYears == nil && t.RecencyYears == nil && t.MinScope == nil &&
		t.CredentialLevel == "" && t.CredentialField == "" && t.RelationshipKind == ""
}

// Equal compares two sets of thresholds by value.
func (t Thresholds) Equal(o Thresholds) bool {
	a, _ := json.Marshal(t)
	b, _ := json.Marshal(o)
	return string(a) == string(b)
}

// JSON is the stored form: {} when nothing is set.
func (t Thresholds) JSON() json.RawMessage {
	b, err := json.Marshal(t)
	if err != nil || string(b) == "null" {
		return json.RawMessage(`{}`)
	}
	return b
}

// ParseThresholds reads the stored form, tolerating an empty value.
func ParseThresholds(raw []byte) Thresholds {
	var t Thresholds
	if len(raw) == 0 {
		return t
	}
	_ = json.Unmarshal(raw, &t)
	return t
}

// Validate checks every threshold against its range or vocabulary.
func (t Thresholds) Validate() error {
	if t.MinYears != nil && (*t.MinYears < 0 || *t.MinYears > MaxThresholdYears) {
		return fmt.Errorf("min_years must be between 0 and %d", MaxThresholdYears)
	}
	if t.RecencyYears != nil && (*t.RecencyYears < 0 || *t.RecencyYears > MaxThresholdYears) {
		return fmt.Errorf("recency_years must be between 0 and %d", MaxThresholdYears)
	}
	if t.MinScope != nil {
		if !slices.Contains(ScopeUnits, t.MinScope.Unit) {
			return fmt.Errorf("min_scope unit must be one of %s", strings.Join(ScopeUnits, ", "))
		}
		if t.MinScope.Value <= 0 {
			return errors.New("min_scope value must be greater than 0")
		}
	}
	if t.CredentialLevel != "" && !slices.Contains(CredentialLevels, t.CredentialLevel) {
		return fmt.Errorf("credential_level must be one of %s", strings.Join(CredentialLevels, ", "))
	}
	if len(t.CredentialField) > MaxCredentialField {
		return fmt.Errorf("credential_field is longer than %d characters", MaxCredentialField)
	}
	if t.RelationshipKind != "" && !slices.Contains(RelationshipKinds, t.RelationshipKind) {
		return fmt.Errorf("relationship_kind must be one of %s", strings.Join(RelationshipKinds, ", "))
	}
	return nil
}

// scopeUnitFor maps the three units the text parser reads to the
// vocabulary's names (04 §1.2).
func scopeUnitFor(parsed string) string {
	switch parsed {
	case "people":
		return "team_size"
	case "usd":
		return "budget_usd"
	case "sites":
		return "sites"
	}
	return "other"
}

// Credential levels as postings write them. Order matters: the higher
// credential is tested first, so "Master's or Bachelor's with five
// years" reads as a master's requirement with an alternative, which is
// how the posting meant it.
var credentialLevelMarkers = []struct {
	level   string
	markers []string
}{
	{"doctorate", []string{"phd", "ph.d", "doctorate", "doctoral"}},
	{"master", []string{"master's", "masters", "master of", " m.s.", " ms in", " msc", "m.sc", " mba", " m.eng", "graduate degree"}},
	{"bachelor", []string{"bachelor", " b.s.", " bs in", " bs/", " bsc", "b.sc", " b.a.", " ba in", " b.eng", "undergraduate degree", "four-year degree", "4-year degree", "college degree", "university degree"}},
	{"associate", []string{"associate's", "associates degree", "associate degree", "two-year degree", "2-year degree"}},
	{"license", []string{"licens", "licenc", " pe ", "professional engineer", "p.eng", "chartered"}},
	{"certificate", []string{"certif"}},
}

// credentialFieldRe reads the field of study after a degree word: "in
// Electrical Engineering", "in engineering or a related field". Narrow
// on purpose; a field it cannot read is left empty for a person to set.
var credentialFieldRe = regexp.MustCompile(`(?i)\b(?:degree|bachelor(?:'s)?|master(?:'s)?|b\.?s\.?|m\.?s\.?|phd|ph\.d\.?|doctorate)\s+(?:degree\s+)?in\s+([a-z][a-z &/,-]{2,80}?)(?:\s+or\s+|\s+with\s+|\s+and\s+\d|[.;(]|$)`)

// DeriveThresholds reads the thresholds a requirement states out of
// its text: the years it asks for, a recency window, a scale, and for a
// credential its level and field. The same parsers the code rules read
// the text with before the thresholds were stored, so a requirement
// nobody has edited is compared against exactly what it always was. A
// relationship requirement gets the widest kind; a person narrows it.
func DeriveThresholds(text, typ string, namedParties []string) Thresholds {
	var t Thresholds
	if n, ok := RequiredYears(text); ok {
		v := float64(n)
		t.MinYears = &v
	}
	if n, ok := RequiredRecencyYears(text); ok {
		v := float64(n)
		t.RecencyYears = &v
	}
	if s, ok := RequiredScope(text); ok {
		t.MinScope = &Scope{Unit: scopeUnitFor(s.Unit), Value: s.Value}
	}
	if typ == "credential" {
		lower := " " + strings.ToLower(text) + " "
		for _, c := range credentialLevelMarkers {
			if containsAny(lower, c.markers) {
				t.CredentialLevel = c.level
				break
			}
		}
		if m := credentialFieldRe.FindStringSubmatch(text); m != nil {
			field := strings.TrimSpace(strings.Trim(m[1], " ,-/&"))
			if len(field) > MaxCredentialField {
				field = field[:MaxCredentialField]
			}
			t.CredentialField = field
		}
	}
	if typ == "relationship" || len(namedParties) > 0 {
		t.RelationshipKind = "any_working"
	}
	return t
}

// offersEquivalentRe reads the ways a posting itself says something
// may stand in for the years or the credential it names: "or
// equivalent experience", "or an equivalent combination of education
// and experience", "in lieu of a degree". Deliberately only in-lieu
// language. "Or a related field" widens the field and still asks for
// the degree, and "Allen-Bradley or Siemens" is a choice of platform,
// not something accepted in place of the years.
var offersEquivalentRe = regexp.MustCompile(`(?i)\bor\s+(?:an?\s+|the\s+)?(?:equivalent|comparable|commensurate)\b|\bequivalent\s+(?:combination|experience|work|practical|professional|military|training)\b|\bcombination\s+of\s+education\s+and\s+experience\b|\bin\s+lieu\s+of\b|\bin\s+place\s+of\s+(?:a|the)\s+degree\b`)

// PostingOffersEquivalent reports whether a requirement's own wording
// accepts something in place of the years or credential it names.
func PostingOffersEquivalent(text string) bool {
	return offersEquivalentRe.MatchString(text)
}

// Inclusion decides whether a requirement counts in the numbers
// (FR-HF-23): a years or credential item counts only when what is
// equivalent to it has been defined, by the hiring manager's
// equivalency or by the posting's own wording ("or equivalent
// experience"). Without one it is shown on the record and left out of
// the arithmetic, because a bare years or credential line is the
// filter this project exists to stop applying blindly. Everything else
// counts.
//
// A years item is one that states a number of years, whatever its
// type: "5+ years of PLC programming" is a years item typed tool or
// experience alike, and an "experience" line that states no years is
// not one. Reading the threshold rather than the type keeps a mistyped
// requirement from being left out, or counted, by its label. The
// reason is a sentence a reader of the record can act on.
func Inclusion(typ string, t Thresholds, text, equivalency string) (included bool, reason string) {
	years := t.MinYears != nil && *t.MinYears > 0
	credential := typ == "credential"
	if !years && !credential {
		return true, ""
	}
	kind := "years"
	if credential {
		kind = "credential"
	}
	if strings.TrimSpace(equivalency) != "" {
		return true, "Included: an equivalency is defined for this " + kind + " item."
	}
	if PostingOffersEquivalent(text) {
		return true, "Included: the posting itself accepts an equivalent in place of this " + kind + " item."
	}
	return false, "Not included in the evaluation: no equivalency has been defined for this " + kind + " item. It is shown and judged, and counts once what is accepted in its place is written down."
}

// Requirement is one version of a requirement as stored: one row of
// requirements (04 §1.1 plus the class, equivalency and release
// columns of migration 00074).
type Requirement struct {
	ID                       string
	RoleID                   string
	LocalID                  string
	Version                  int
	Supersedes               string // "" for the first version
	SchemaVersion            string
	VocabVersion             string
	Text                     string
	SourceQuote              string
	Type                     string
	Category                 string // must | nice, as the posting stated it
	Class                    string // required | preferred | ideal
	Weight                   int
	EvidenceStandard         string
	EvidenceStandardTemplate string // type@template_version, "edited:" prefixed when a person rewrote it
	Alternatives             []string
	EquivalenceNotes         string
	Equivalency              string
	Thresholds               Thresholds
	NamedParties             []string
	Included                 bool
	Extraction               json.RawMessage
	EditedBy                 int64 // 0 for the untouched extraction
	EditedByDisplay          string
	EditedAt                 *time.Time
	EditReason               string
	Status                   string
	ReleasedAt               *time.Time
	ReleasedBy               int64
	CreatedAt                time.Time
}

// InclusionReason is the sentence that goes with Included.
//
// Included is stored when a version is written and is what the
// arithmetic of every record that judged the version read; the sentence
// is worked out from the rule as it stands now. When the rule has
// changed since the version was written the two can part, and a record
// must not say "not included" beside a reason for including. The stored
// value is the fact, so the sentence follows it and says why it differs.
func (q Requirement) InclusionReason() string {
	included, reason := Inclusion(q.Type, q.Thresholds, q.Text, q.Equivalency)
	if included == q.Included {
		return reason
	}
	if q.Included {
		return "Included: counted under the inclusion rule in force when this version was written."
	}
	return "Not included in the evaluation: this version was written under an earlier inclusion rule and was left out of the figures. Under the rule as it stands it would count; saving the requirement again applies it."
}

// RequirementPatch is an edit: only the fields set are changed
// (FR-HF-03 names text, type, weight, thresholds, evidence standard and
// equivalence notes; FR-HF-23 adds class and the equivalency). The
// category and the source quote are the posting's and are not
// editable.
type RequirementPatch struct {
	Text             *string
	Type             *string
	Class            *string
	Weight           *int
	Thresholds       *Thresholds
	EvidenceStandard *string
	EquivalenceNotes *string
	Equivalency      *string
	Alternatives     *[]string
	NamedParties     *[]string
}

// ErrInvalidRequirement wraps every validation failure of an edit, so
// a handler can answer invalid_argument with the sentence.
var ErrInvalidRequirement = errors.New("invalid requirement")

// ErrNothingChanged is returned for an edit that changes nothing: a
// version that differs from its predecessor only in its number would
// be a record of nothing.
var ErrNothingChanged = errors.New("the edit changes nothing")

// ErrNotEditable is returned for an edit of a version that is not the
// current one for its requirement.
var ErrNotEditable = errors.New("this version cannot be edited")

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRequirement, fmt.Sprintf(format, a...))
}

// cleanList trims, drops empties and duplicates, and caps a list.
func cleanList(in []string, maxItems, maxLen int, what string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[strings.ToLower(s)] {
			continue
		}
		if len(s) > maxLen {
			return nil, invalid("%s entry is longer than %d characters", what, maxLen)
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	if len(out) > maxItems {
		return nil, invalid("at most %d %s", maxItems, what)
	}
	return out, nil
}

// applyPatch returns the requirement as the edit leaves it. Fields the
// edit does not set are carried over, except three that are derived:
// when the text or the type changes and the thresholds, the evidence
// standard or the equivalence notes were still the derived ones, they
// are derived again from the new values, so correcting the wording of
// a requirement does not leave it compared against the numbers of the
// old wording. A value a person set stays set.
func applyPatch(cur Requirement, p RequirementPatch, catalog *Catalog) (Requirement, error) {
	next := cur
	if p.Text != nil {
		next.Text = strings.TrimSpace(*p.Text)
		if next.Text == "" {
			return next, invalid("text is required")
		}
		if len(next.Text) > MaxRequirementText {
			return next, invalid("text is longer than %d characters", MaxRequirementText)
		}
	}
	if p.Type != nil {
		if !slices.Contains(RequirementTypes, *p.Type) {
			return next, invalid("type must be one of %s", strings.Join(RequirementTypes, ", "))
		}
		next.Type = *p.Type
	}
	if p.Class != nil {
		if !slices.Contains(RequirementClasses, *p.Class) {
			return next, invalid("class must be one of %s", strings.Join(RequirementClasses, ", "))
		}
		next.Class = *p.Class
	}
	if p.Weight != nil {
		if *p.Weight < 1 || *p.Weight > 3 {
			return next, invalid("weight must be 1, 2 or 3")
		}
		next.Weight = *p.Weight
	}
	if p.Alternatives != nil {
		list, err := cleanList(*p.Alternatives, MaxAlternatives, MaxAlternativeText, "alternatives")
		if err != nil {
			return next, err
		}
		next.Alternatives = list
	}
	if p.NamedParties != nil {
		list, err := cleanList(*p.NamedParties, MaxNamedParties, MaxNamedPartyText, "named parties")
		if err != nil {
			return next, err
		}
		next.NamedParties = list
	}
	if p.Equivalency != nil {
		next.Equivalency = strings.TrimSpace(*p.Equivalency)
		if len(next.Equivalency) > MaxEquivalency {
			return next, invalid("equivalency is longer than %d characters", MaxEquivalency)
		}
	}

	reworded := next.Text != cur.Text || next.Type != cur.Type || !slices.Equal(next.NamedParties, cur.NamedParties)

	// Thresholds: set by the edit, or derived again when they were
	// still the derived ones.
	switch {
	case p.Thresholds != nil:
		if err := p.Thresholds.Validate(); err != nil {
			return next, invalid("%s", err.Error())
		}
		next.Thresholds = *p.Thresholds
	case reworded && cur.Thresholds.Equal(DeriveThresholds(cur.Text, cur.Type, cur.NamedParties)):
		next.Thresholds = DeriveThresholds(next.Text, next.Type, next.NamedParties)
	}

	// Equivalence notes: the same rule against the catalog's rendering.
	switch {
	case p.EquivalenceNotes != nil:
		next.EquivalenceNotes = strings.TrimSpace(*p.EquivalenceNotes)
		if len(next.EquivalenceNotes) > MaxEquivalenceNotes {
			return next, invalid("equivalence notes are longer than %d characters", MaxEquivalenceNotes)
		}
	case reworded && catalog != nil:
		if was, _ := catalog.Notes(cur.Text, cur.SourceQuote); was == cur.EquivalenceNotes {
			next.EquivalenceNotes, _ = catalog.Notes(next.Text, next.SourceQuote)
		}
	}

	// Evidence standard: a person's rewrite is marked as one and keeps
	// the template it started from, so recurring edits of one type's
	// standard can be counted (05 §2.4); an untouched one is rendered
	// again from whatever changed.
	switch {
	case p.EvidenceStandard != nil:
		next.EvidenceStandard = strings.TrimSpace(*p.EvidenceStandard)
		if len(next.EvidenceStandard) > MaxEvidenceStandard {
			return next, invalid("evidence standard is longer than %d characters", MaxEvidenceStandard)
		}
		if next.EvidenceStandard != cur.EvidenceStandard && !strings.HasPrefix(cur.EvidenceStandardTemplate, EditedTemplatePrefix) {
			next.EvidenceStandardTemplate = EditedTemplatePrefix + cur.EvidenceStandardTemplate
		}
	case !strings.HasPrefix(cur.EvidenceStandardTemplate, EditedTemplatePrefix):
		next.EvidenceStandard, next.EvidenceStandardTemplate = RenderEvidenceStandard(next.Type, next.Thresholds, next.NamedParties)
	}

	next.Included, _ = Inclusion(next.Type, next.Thresholds, next.Text, next.Equivalency)
	if sameJudged(cur, next) {
		return next, ErrNothingChanged
	}
	return next, nil
}

// judgedKey is everything about a version that the judge sees or the
// arithmetic reads. Two versions with the same key are the same
// version; anything that differs is a new one, so a judgment's
// requirement_id is enough to say exactly what it was judged against.
func judgedKey(q Requirement) string {
	parties := slices.Clone(q.NamedParties)
	if parties == nil {
		parties = []string{}
	}
	slices.Sort(parties)
	alts := q.Alternatives
	if alts == nil {
		alts = []string{}
	}
	b, _ := json.Marshal([]any{
		q.Text, q.SourceQuote, q.Category, q.Class, q.Weight, q.Type, parties, alts,
		q.Thresholds, q.EvidenceStandard, q.EquivalenceNotes, q.Equivalency, q.Included,
	})
	return string(b)
}

func sameJudged(a, b Requirement) bool { return judgedKey(a) == judgedKey(b) }
