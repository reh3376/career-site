package hiring

import "strings"

// The consistency rules of 04 §2.3, enforced after decode. The schema
// bounds what the model can write; it cannot express "met implies
// direct", so that is checked here. A judgment that breaks a rule is
// sent back once with the rule named (the assessor does that); one
// that breaks it again is repaired deterministically, with the repair
// recorded as a RuleAdjustment, because losing a four-hour run to one
// mislabelled field is worse than correcting the label in the open.

// Violations lists the consistency rules a judgment breaks, each as a
// sentence the model can be shown. Empty means the judgment is
// consistent. codes is the vocabulary the judgment must draw from;
// the schema already constrains it under Ollama, and other providers
// may not, so unknown codes are violations too.
func Violations(v Verdict, facts ReportedFacts, evidenceCount int, codes VocabLists) []string {
	var out []string
	if !has(Codes(codes.Determination), v.Determination) {
		out = append(out, "determination "+quote(v.Determination)+" is not one of the allowed codes")
	}
	if !has(Codes(codes.Basis), v.Basis) {
		out = append(out, "basis "+quote(v.Basis)+" is not one of the allowed codes")
	}
	for _, u := range v.Uncertainty {
		if !has(Codes(codes.Uncertainty), u) {
			out = append(out, "uncertainty code "+quote(u)+" is not one of the allowed codes")
		}
	}
	switch v.Determination {
	case DetMet:
		if v.Basis != BasisDirect && facts.AlternativeSatisfied < 0 {
			out = append(out, "determination met requires basis direct, or alternative_satisfied naming the alternative the posting offered; an equivalency you reasoned to is met_equivalent")
		}
		if len(v.Uncertainty) > 0 {
			out = append(out, "determination met carries no uncertainty codes")
		}
	case DetMetEquivalent:
		if !IsEquivalencyBasis(v.Basis) {
			out = append(out, "determination met_equivalent requires an equivalency basis: "+strings.Join(EquivalencyBases, ", "))
		}
		if len(v.Uncertainty) > 0 {
			out = append(out, "determination met_equivalent carries no uncertainty codes")
		}
	case DetUnresolved:
		if len(v.Uncertainty) == 0 {
			out = append(out, "determination unresolved requires at least one uncertainty code naming the missing fact")
		}
	case DetNotMet:
		if len(v.Uncertainty) > 0 {
			out = append(out, "determination not_met carries no uncertainty codes; if a fact would change the answer the determination is unresolved")
		}
		if v.Basis == BasisNotAddressed && evidenceCount > 0 {
			out = append(out, "basis not_addressed means nothing in the evidence speaks to the requirement, so evidence_ids must be empty")
		}
		if v.Basis != BasisNotAddressed && v.Basis != "" && evidenceCount == 0 {
			out = append(out, "a not_met with basis "+v.Basis+" rests on evidence that falls short, so evidence_ids must cite it; with no evidence the basis is not_addressed")
		}
	}
	return out
}

// Repair makes a judgment consistent when the model would not. It
// changes as little as it can and prefers the reading that keeps the
// model's finding: a met with an equivalency basis becomes
// met_equivalent (the model found an equivalency and mislabelled it);
// a met or met_equivalent with a shortfall basis becomes the unresolved
// question that basis describes; one with an absence basis becomes
// not_met. The returned flag says whether evidence_ids must be cleared
// (a not_addressed cites nothing). The adjustment records what moved.
func Repair(v Verdict, facts ReportedFacts, evidenceCount int, codes VocabLists) (out Verdict, clearEvidence bool, adj *RuleAdjustment) {
	out = Verdict{Determination: v.Determination, Basis: v.Basis, Uncertainty: append([]string{}, v.Uncertainty...)}
	if !has(Codes(codes.Determination), out.Determination) {
		out.Determination = DetUnresolved
	}
	if !has(Codes(codes.Basis), out.Basis) {
		out.Basis = BasisNotAddressed
	}
	var kept []string
	for _, u := range out.Uncertainty {
		if has(Codes(codes.Uncertainty), u) {
			kept = append(kept, u)
		}
	}
	out.Uncertainty = kept

	switch out.Determination {
	case DetMet:
		if out.Basis != BasisDirect && facts.AlternativeSatisfied < 0 {
			switch {
			case IsEquivalencyBasis(out.Basis):
				out.Determination = DetMetEquivalent
			case ShortfallBases[out.Basis] != "":
				out.Determination = DetUnresolved
				out.Uncertainty = appendCode(out.Uncertainty, ShortfallBases[out.Basis])
			default:
				out.Determination = DetNotMet
			}
		}
	case DetMetEquivalent:
		if !IsEquivalencyBasis(out.Basis) {
			switch {
			case out.Basis == BasisDirect:
				out.Determination = DetMet
			case ShortfallBases[out.Basis] != "":
				out.Determination = DetUnresolved
				out.Uncertainty = appendCode(out.Uncertainty, ShortfallBases[out.Basis])
			default:
				out.Determination = DetNotMet
			}
		}
	}
	switch out.Determination {
	case DetMet, DetMetEquivalent:
		out.Uncertainty = []string{}
	case DetUnresolved:
		if len(out.Uncertainty) == 0 {
			if code := ShortfallBases[out.Basis]; code != "" {
				out.Uncertainty = []string{code}
			} else {
				out.Uncertainty = []string{UncEvidenceAmbiguous}
			}
		}
	case DetNotMet:
		out.Uncertainty = []string{}
		if out.Basis == BasisNotAddressed && evidenceCount > 0 {
			clearEvidence = true
		}
		if out.Basis != BasisNotAddressed && evidenceCount == 0 {
			out.Basis = BasisNotAddressed
		}
	}
	if out.Uncertainty == nil {
		out.Uncertainty = []string{}
	}
	if out.Determination == v.Determination && out.Basis == v.Basis && sameCodes(out.Uncertainty, v.Uncertainty) && !clearEvidence {
		return out, false, nil
	}
	added := []string{}
	for _, u := range out.Uncertainty {
		if !has(v.Uncertainty, u) {
			added = append(added, u)
		}
	}
	reason := "the model's answer broke a consistency rule twice: " + strings.Join(Violations(v, facts, evidenceCount, codes), "; ")
	if clearEvidence && out.Determination == v.Determination && out.Basis == v.Basis {
		reason = "a not_addressed judgment cites nothing; the cited evidence was dropped"
	}
	return out, clearEvidence, &RuleAdjustment{
		Rule: RuleConsistency, RuleVersion: RuleConsistencyVersion,
		FromDetermination: v.Determination, ToDetermination: out.Determination,
		FromBasis: v.Basis, ToBasis: out.Basis,
		AddedUncertainty: added,
		Reason:           reason,
	}
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sameCodes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		if !has(b, x) {
			return false
		}
	}
	return true
}

func quote(s string) string { return "\"" + s + "\"" }
