// Package hiring holds the generalised hiring entities of the framework
// (FSD §5.15; docs/framework-research/14-schema-rendering.md §5): the
// versioned requirement, the judgment vocabulary, the three-number
// formula, the keyword baseline and the writers for their tables. It
// sits beside jd, which it generalises, so the new rows do not land
// inside the package that is being replaced. Phase 0 step 3 writes
// roles, requirements, evidence items and match records from the
// existing pipeline (the dual write of 14 §4); judgments begin at step
// 4, when the judge emits what their rows need.
package hiring

import (
	"encoding/json"
	"fmt"
)

// Schema versions, one per entity, stored on every row at the version
// it was written and never rewritten (04 §0). MAJOR.MINOR: adding an
// optional field is a minor bump; removing, renaming or retyping one
// is a major bump.
const (
	RequirementSchema   = "1.0"
	JudgmentSchema      = "1.0"
	EvidenceSchema      = "1.0"
	ClarificationSchema = "1.0"
	MatchRecordSchema   = "1.0"

	// VocabVersion is the vocabulary every row written today names.
	// The codes themselves are rows in vocab_codes (vocab.go).
	VocabVersion = "1.0"

	// FormulaV2 is the prototype's arithmetic, which the dual write
	// carries until step 4; FormulaV3 is the three numbers (formula.go).
	FormulaV2 = "v2-nice-bonus"
	FormulaV3 = "v3-three-numbers"

	// OwnerCandidateID is the one candidate of Phase 0 (FR-HF-19),
	// seeded by 00066 so corpus_documents can default to it.
	OwnerCandidateID = "00000000-0000-0000-0000-000000000001"
)

// current maps an entity to the schema version this build writes and
// reads; Upcast walks a stored row from its version to this one.
var current = map[string]string{
	"requirement":   RequirementSchema,
	"judgment":      JudgmentSchema,
	"evidence":      EvidenceSchema,
	"clarification": ClarificationSchema,
	"match_record":  MatchRecordSchema,
}

// Upcaster is one pure step from one minor version to the next. It
// takes the row as stored and returns the row as the next version
// would have written it.
type Upcaster func(raw json.RawMessage) (json.RawMessage, error)

// upcasts is keyed "entity from->to". No upcasts exist at 1.0; the
// registry is the convention, present from the first row, so the first
// schema change registers a function rather than inventing a place.
var upcasts = map[string]Upcaster{}

// RegisterUpcast adds one step. Called from init() in the file that
// introduces the version it upcasts to, so a reader of that file sees
// the change and its migration together.
func RegisterUpcast(entity, from, to string, fn Upcaster) {
	upcasts[entity+" "+from+"->"+to] = fn
}

// Upcast brings a row stored at `from` to the current version of its
// entity, one registered step at a time. A row already current is
// returned unchanged; a version with no registered path is an error,
// because silently reading it as current is exactly the failure the
// versioning policy exists to prevent.
func Upcast(entity, from string, raw json.RawMessage) (json.RawMessage, error) {
	target, ok := current[entity]
	if !ok {
		return nil, fmt.Errorf("hiring: unknown entity %q", entity)
	}
	at := from
	for at != target {
		next, err := nextMinor(at)
		if err != nil {
			return nil, fmt.Errorf("hiring: upcast %s from %s: %w", entity, from, err)
		}
		fn, ok := upcasts[entity+" "+at+"->"+next]
		if !ok {
			return nil, fmt.Errorf("hiring: no upcast registered for %s %s->%s", entity, at, next)
		}
		out, err := fn(raw)
		if err != nil {
			return nil, fmt.Errorf("hiring: upcast %s %s->%s: %w", entity, at, next, err)
		}
		raw, at = out, next
	}
	return raw, nil
}

// nextMinor is "1.2" -> "1.3". Major bumps are not walked: a major
// change is a new reader, not an upcast chain.
func nextMinor(v string) (string, error) {
	major, minor, err := parseVersion(v)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d", major, minor+1), nil
}

func parseVersion(v string) (major, minor int, err error) {
	if _, err = fmt.Sscanf(v, "%d.%d", &major, &minor); err != nil {
		return 0, 0, fmt.Errorf("version %q is not MAJOR.MINOR", v)
	}
	return major, minor, nil
}

// versionLE reports a <= b for MAJOR.MINOR strings; an unparseable
// side compares false, so an odd version never passes a check by
// accident.
func versionLE(a, b string) bool {
	am, an, err := parseVersion(a)
	if err != nil {
		return false
	}
	bm, bn, err := parseVersion(b)
	if err != nil {
		return false
	}
	return am < bm || (am == bm && an <= bn)
}
