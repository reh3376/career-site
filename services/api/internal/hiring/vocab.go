package hiring

import (
	"context"
	"fmt"
	"sort"
)

// Code is one row of vocab_codes: what a determination, basis,
// uncertainty, requirement type or other controlled value may be, with
// the version that introduced it and, if it is gone, the one that
// retired it (04 §0 rule 5).
type Code struct {
	Vocab        string
	Code         string
	Label        string
	Description  string
	IntroducedIn string
	RetiredIn    string // "" while live
	ReplacedBy   string // "" unless retired in favour of another
	SortOrder    int
}

// Vocabularies this build knows. A writer refuses a code outside its
// vocabulary; a reader keeps the raw value and treats it as Other.
const (
	VocabDetermination    = "determination"
	VocabBasis            = "basis"
	VocabUncertainty      = "uncertainty"
	VocabRequirementType  = "requirement_type"
	VocabAnswerType       = "answer_type"
	VocabSourceKind       = "source_kind"
	VocabOverrideRole     = "override_role"
	VocabInvolvementLevel = "involvement_level"
	VocabCatalogKind      = "catalog_kind"
	VocabCatalogStrength  = "catalog_strength"
	// Vocabulary 1.1 (migration 00074).
	VocabRequirementClass = "requirement_class"
	VocabScopeUnit        = "scope_unit"
	VocabCredentialLevel  = "credential_level"
	VocabRelationshipKind = "relationship_kind"

	// Other is what an unknown code resolves to for logic. The raw
	// value is kept beside it, never replaced (04 §0 rule 3).
	Other = "other"
)

// Registry is the vocabulary as loaded from vocab_codes. It is data,
// not code: the judge prompt renders its allowed codes from here, so a
// vocabulary change is one INSERT and a new prompt fingerprint, never
// a migration or a deploy of a new enum.
type Registry struct {
	codes map[string]map[string]Code
}

// NewRegistry builds a registry from rows, for tests and for the
// loader. Rows with an empty vocab or code are ignored rather than
// trusted.
func NewRegistry(rows []Code) *Registry {
	r := &Registry{codes: map[string]map[string]Code{}}
	for _, c := range rows {
		if c.Vocab == "" || c.Code == "" {
			continue
		}
		if r.codes[c.Vocab] == nil {
			r.codes[c.Vocab] = map[string]Code{}
		}
		r.codes[c.Vocab][c.Code] = c
	}
	return r
}

// Known reports whether the code exists in the vocabulary at all,
// retired or not. Writers use it: a retired code may still be written
// by a reader upcasting old rows, but a code that never existed is a
// defect.
func (r *Registry) Known(vocab, code string) bool {
	if r == nil {
		return false
	}
	_, ok := r.codes[vocab][code]
	return ok
}

// Resolve returns the code to use for logic at vocabulary version
// atVersion. A code the registry knows and that is live at that
// version is returned as itself with true. Anything else returns Other
// and false: an unknown code, one introduced after atVersion, or one
// retired at or before it. Callers keep the raw string beside the
// resolved one.
func (r *Registry) Resolve(vocab, code, atVersion string) (string, bool) {
	if r == nil {
		return Other, false
	}
	c, ok := r.codes[vocab][code]
	if !ok {
		return Other, false
	}
	if !versionLE(c.IntroducedIn, atVersion) {
		return Other, false
	}
	if c.RetiredIn != "" && versionLE(c.RetiredIn, atVersion) {
		return Other, false
	}
	return code, true
}

// Codes lists a vocabulary's live codes in sort order, for rendering
// into a prompt or a form.
func (r *Registry) Codes(vocab string) []Code {
	if r == nil {
		return nil
	}
	out := make([]Code, 0, len(r.codes[vocab]))
	for _, c := range r.codes[vocab] {
		if c.RetiredIn == "" {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// LoadVocab reads the registry from vocab_codes.
func (r *Repo) LoadVocab(ctx context.Context) (*Registry, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT vocab, code, label, description, introduced_in,
           COALESCE(retired_in, ''), COALESCE(replaced_by, ''), sort_order
    FROM vocab_codes`)
	if err != nil {
		return nil, fmt.Errorf("load vocab: %w", err)
	}
	defer rows.Close()
	var out []Code
	for rows.Next() {
		var c Code
		if err := rows.Scan(&c.Vocab, &c.Code, &c.Label, &c.Description, &c.IntroducedIn, &c.RetiredIn, &c.ReplacedBy, &c.SortOrder); err != nil {
			return nil, fmt.Errorf("scan vocab: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return NewRegistry(out), nil
}

// RequirementTypes is vocabulary 1.0's requirement_type list, compiled
// in so the type heuristic (classify.go) can be checked against it
// without a database. The registry in the database is authoritative;
// TestWrittenCodesAreRegistered reads what was written against it.
var RequirementTypes = []string{
	"skill", "outcome", "tool", "domain", "credential",
	"experience", "relationship", "constraint", "behavioral",
}

// The requirement classes of FR-HF-23, and the three vocabularies
// thresholds are written in (04 §1.2), compiled in for the same reason:
// validation and the formula must not need a database. Vocabulary 1.1.
const (
	ClassRequired  = "required"
	ClassPreferred = "preferred"
	ClassIdeal     = "ideal"
)

var (
	RequirementClasses = []string{ClassRequired, ClassPreferred, ClassIdeal}
	ScopeUnits         = []string{"team_size", "budget_usd", "users", "sites", "io_points", "systems", "revenue_usd", "other"}
	CredentialLevels   = []string{"certificate", "associate", "bachelor", "master", "doctorate", "license"}
	RelationshipKinds  = []string{"employed_by", "contracted_to", "customer_of", "partner_of", "any_working"}
)

// ClassFromCategory is the class a requirement starts with: what the
// posting required is required, what it preferred is preferred. Ideal
// is only ever set by a person.
func ClassFromCategory(category string) string {
	if category == "must" {
		return ClassRequired
	}
	return ClassPreferred
}
