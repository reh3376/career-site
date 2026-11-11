package hiring

import "sort"

// The vocabulary 1.0 codes the judge emits and the rules read, as Go
// constants so a misspelling is a compile error rather than a row the
// registry rejects at runtime. The registry in vocab_codes is the
// source of record for what the codes mean; the descriptions here are
// the seed's, kept identical so a prompt rendered from this file and
// one rendered from the database are the same bytes and carry the same
// fingerprint.

// Basis codes (03 §2.2).
const (
	BasisDirect                 = "direct"
	BasisVocabulary             = "vocabulary"
	BasisToolEquivalent         = "tool_equivalent"
	BasisExperienceForEducation = "experience_for_education"
	BasisEducationForExperience = "education_for_experience"
	BasisAdjacentDomain         = "adjacent_domain"
	BasisScopeShortfall         = "scope_shortfall"
	BasisDurationShortfall      = "duration_shortfall"
	BasisRecency                = "recency"
	BasisRelationshipUnproven   = "relationship_unproven"
	BasisNotAddressed           = "not_addressed"
	BasisContradicted           = "contradicted"
)

// Uncertainty codes (03 §2.3): each names the one fact that would
// settle an unresolved judgment, which is what makes a clarification
// question possible.
const (
	UncScopeUnknown           = "scope_unknown"
	UncDurationUnknown        = "duration_unknown"
	UncRecencyUnknown         = "recency_unknown"
	UncDepthUnknown           = "depth_unknown"
	UncEquivalenceUnconfirmed = "equivalence_unconfirmed"
	UncCredentialUnknown      = "credential_unknown"
	UncPartyUnknown           = "party_unknown"
	UncEvidenceAmbiguous      = "evidence_ambiguous"
)

// CodeDoc is one code with the sentence the prompt shows the model.
type CodeDoc struct {
	Code        string
	Description string
}

// VocabLists is what the judge prompt renders its allowed codes from:
// the four vocabularies the model emits. Built from the registry at
// boot (VocabFromRegistry) and from this file when the registry is
// unavailable (DefaultVocab); the two agree at vocabulary 1.0 and a
// test holds them to it.
type VocabLists struct {
	Determination    []CodeDoc
	Basis            []CodeDoc
	Uncertainty      []CodeDoc
	InvolvementLevel []CodeDoc
}

// DefaultVocab is vocabulary 1.0 as seeded by migration 00066, in seed
// order with the seed's descriptions.
func DefaultVocab() VocabLists {
	return VocabLists{
		Determination: []CodeDoc{
			{DetMet, "The evidence satisfies the requirement as written or via one of its own stated alternatives. Full weight."},
			{DetMetEquivalent, "The evidence satisfies the requirement through an equivalency the system or a reviewer asserted. Full weight, flagged for audit."},
			{DetUnresolved, "The evidence is suggestive but a specific, nameable fact is missing. Reported as unresolved weight, never scored as half."},
			{DetNotMet, "The evidence addresses the requirement and falls short, or does not address it and there is no reason to believe it could. Zero."},
		},
		Basis: []CodeDoc{
			{BasisDirect, "Same thing, same words or trivially the same."},
			{BasisVocabulary, "Same capability, different vocabulary, title or framing. The lexical gap the project exists for."},
			{BasisToolEquivalent, "Different named tool, platform or vendor, same function."},
			{BasisExperienceForEducation, "The requirement names a credential; the evidence shows the work the credential certifies."},
			{BasisEducationForExperience, "The requirement names years or exposure; the evidence shows formal training or a credential covering it."},
			{BasisAdjacentDomain, "Same skill demonstrated in a neighbouring industry or context; transferability is the question."},
			{BasisScopeShortfall, "Right work at a smaller scale, team, budget or system size than stated."},
			{BasisDurationShortfall, "Right work, fewer years than stated."},
			{BasisRecency, "Right work, but old relative to what the requirement implies."},
			{BasisRelationshipUnproven, "The requirement names organisations; the evidence uses their products but shows no working relationship."},
			{BasisNotAddressed, "Nothing in the evidence speaks to it."},
			{BasisContradicted, "The evidence affirmatively indicates the requirement is not satisfied."},
		},
		Uncertainty: []CodeDoc{
			{UncScopeUnknown, "Scale not stated in the evidence."},
			{UncDurationUnknown, "Years not stated."},
			{UncRecencyUnknown, "Dates not stated."},
			{UncDepthUnknown, "Used, owned or designed is ambiguous."},
			{UncEquivalenceUnconfirmed, "A vocabulary or tool equivalence is plausible but asserted by no catalog entry or reviewer."},
			{UncCredentialUnknown, "The degree or certificate is neither confirmed nor denied by the evidence."},
			{UncPartyUnknown, "An organisation named in the requirement is not confirmed as a working relationship."},
			{UncEvidenceAmbiguous, "The text can be read more than one way."},
		},
		InvolvementLevel: []CodeDoc{
			{"aware", "Knows of it."},
			{"used", "Operated it as a user."},
			{"configured", "Set it up or tuned it."},
			{"designed", "Specified or architected it."},
			{"owned", "Accountable for it end to end."},
			{"taught", "Trained others in it."},
			{"unknown", "The evidence does not say."},
		},
	}
}

// VocabFromRegistry renders the four lists from the loaded registry,
// live codes only, in the registry's sort order. This is what makes a
// vocabulary change an INSERT and a new prompt fingerprint (14 §5).
func VocabFromRegistry(reg *Registry) VocabLists {
	pick := func(vocab string) []CodeDoc {
		codes := reg.Codes(vocab)
		sort.SliceStable(codes, func(i, j int) bool { return codes[i].SortOrder < codes[j].SortOrder })
		out := make([]CodeDoc, 0, len(codes))
		for _, c := range codes {
			if c.RetiredIn != "" {
				continue
			}
			out = append(out, CodeDoc{Code: c.Code, Description: c.Description})
		}
		return out
	}
	return VocabLists{
		Determination:    pick(VocabDetermination),
		Basis:            pick(VocabBasis),
		Uncertainty:      pick(VocabUncertainty),
		InvolvementLevel: pick(VocabInvolvementLevel),
	}
}

// Codes lists the codes of a list, for schema enums and tests.
func Codes(docs []CodeDoc) []string {
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.Code)
	}
	return out
}

// The bases a met_equivalent may carry (03 §2.2): the equivalency
// categories, as opposed to direct, the shortfalls and the absences.
var EquivalencyBases = []string{
	BasisVocabulary, BasisToolEquivalent, BasisExperienceForEducation, BasisEducationForExperience, BasisAdjacentDomain,
}

// ShortfallBases maps each shortfall basis to the uncertainty code
// that names the fact settling it, for the consistency repair that
// turns a mislabelled "met" with a shortfall basis into the question
// it describes.
var ShortfallBases = map[string]string{
	BasisScopeShortfall:       UncScopeUnknown,
	BasisDurationShortfall:    UncDurationUnknown,
	BasisRecency:              UncRecencyUnknown,
	BasisRelationshipUnproven: UncPartyUnknown,
}

// IsEquivalencyBasis reports whether a basis is one of the equivalency
// categories.
func IsEquivalencyBasis(basis string) bool {
	for _, b := range EquivalencyBases {
		if b == basis {
			return true
		}
	}
	return false
}

// LexicalBases are the bases FR-HF-24 names: an unresolved judgment on
// one of these, or on relationship_unproven, produces a clarification
// item without waiting to be asked (step 7).
var LexicalBases = []string{BasisVocabulary, BasisToolEquivalent, BasisAdjacentDomain}
