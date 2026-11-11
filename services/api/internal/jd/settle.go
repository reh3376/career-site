package jd

import (
	"strconv"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Settling a judgment: everything that happens to the model's answer
// after it decodes and before it is scored. Pure, so it can be tested
// without a model or a database.

// settleInput is one requirement's answer and what it was judged with.
type settleInput struct {
	req      prompts.Requirement
	raw      rawJudgment
	offered  map[int64]bool    // chunk ids the judge was shown for this requirement
	evidence []users.CorpusHit // the same chunks, for the rationale check
	now      time.Time
	vocab    hiring.VocabLists
	// repair is set when the answer broke a consistency rule twice; the
	// judgment is then made consistent in code and the repair recorded.
	repair bool
}

// judgeOrDefault is the judge this assessor sends, falling back to the
// compiled-in rendering for an assessor built without NewAssessor
// (tests).
func (a *Assessor) judgeOrDefault() prompts.Prompt {
	if a == nil || a.judge.ID == "" {
		return prompts.RequirementJudge
	}
	return a.judge
}

// consistencyViolations lists the rules a batch's answers break, for
// the retry message. Evidence is counted as the model cited it; the
// offered-id filter runs later, in settleJudgment.
func (a *Assessor) consistencyViolations(raws []rawJudgment, offered map[string][]int64) []string {
	vocab := a.vocab
	if len(vocab.Determination) == 0 {
		vocab = hiring.DefaultVocab()
	}
	var out []string
	for _, j := range raws {
		valid := 0
		allowed := map[int64]bool{}
		for _, id := range offered[j.RequirementID] {
			allowed[id] = true
		}
		for _, s := range j.EvidenceIDs {
			if id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil && allowed[id] {
				valid++
			}
		}
		v := hiring.Verdict{Determination: j.Determination, Basis: j.Basis, Uncertainty: j.Uncertainty}
		out = append(out, hiring.Violations(v, j.ReportedFacts, valid, vocab)...)
	}
	return out
}

// settleJudgment turns the model's raw answer into the judgment the
// record keeps: cited ids filtered to what was offered, the
// consistency repair when it was earned, the evidence rule, the code
// rules of FR-HF-05, the derived verdict of FR-HF-07, and the rationale
// check. Every change the code makes is in RuleAdjustments.
func settleJudgment(in settleInput) Judgment {
	vocab := in.vocab
	if len(vocab.Determination) == 0 {
		vocab = hiring.DefaultVocab()
	}
	var ids []int64
	for _, s := range in.raw.EvidenceIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err == nil && in.offered[id] {
			ids = append(ids, id)
		}
	}
	facts := in.raw.ReportedFacts
	verdict := hiring.Verdict{Determination: in.raw.Determination, Basis: in.raw.Basis, Uncertainty: append([]string{}, in.raw.Uncertainty...)}
	var adjustments []hiring.RuleAdjustment

	// A second consistency failure is repaired here, in the open.
	if in.repair {
		fixed, clear, adj := hiring.Repair(verdict, facts, len(ids), vocab)
		if adj != nil {
			adjustments = append(adjustments, *adj)
			verdict = fixed
			if clear {
				ids = nil
			}
		}
	} else if len(hiring.Violations(verdict, facts, len(ids), vocab)) > 0 {
		// The retry passed on the model's own ids and the offered-id
		// filter has since removed some: repair quietly rather than
		// store an inconsistent row.
		fixed, clear, adj := hiring.Repair(verdict, facts, len(ids), vocab)
		if adj != nil {
			adjustments = append(adjustments, *adj)
			verdict = fixed
			if clear {
				ids = nil
			}
		}
	}

	// A judgment that cites nothing it was shown rests on nothing.
	if v, adj := hiring.EvidenceRule(verdict, len(ids)); adj != nil {
		adjustments = append(adjustments, *adj)
		verdict = v
		ids = nil
	}

	// Duration, recency, scope, credential and named parties: compared
	// in code from the reported facts (FR-HF-05).
	verdict, ruleAdjs := hiring.ApplyRules(hiring.RuleInput{Text: in.req.Text, Type: in.req.Type, NamedParties: in.req.NamedParties}, verdict, facts, in.now)
	adjustments = append(adjustments, ruleAdjs...)
	if verdict.Determination == hiring.DetNotMet && verdict.Basis == hiring.BasisNotAddressed {
		ids = nil
	}
	if verdict.Uncertainty == nil {
		verdict.Uncertainty = []string{}
	}

	// Does the sentence agree with the record it was written from?
	// Nothing here changes the verdict. The model's own determination is
	// what the check reads, because the disjunction arm asks whether the
	// model contradicted itself, not whether code disagreed with it.
	var cited []string
	for _, h := range in.evidence {
		for _, id := range ids {
			if h.Chunk.ID == id {
				cited = append(cited, h.Chunk.Text)
			}
		}
	}
	issues := checkRationale(in.raw.Rationale, in.req.Text, in.req.SourceQuote,
		hiring.DeriveVerdict(in.raw.Determination), cited, facts.StatedSpanYears)

	var reasons []string
	for _, adj := range adjustments {
		reasons = append(reasons, adj.Reason)
	}
	factsCopy := facts
	return Judgment{
		RequirementID:    in.req.ID,
		Verdict:          hiring.DeriveVerdict(verdict.Determination),
		Determination:    verdict.Determination,
		Basis:            verdict.Basis,
		Uncertainty:      verdict.Uncertainty,
		Type:             in.req.Type,
		EvidenceIDs:      ids,
		Rationale:        truncErr(strings.TrimSpace(in.raw.Rationale)),
		ReportedFacts:    &factsCopy,
		Confidence:       in.raw.Confidence,
		StatedSpanYears:  facts.StatedSpanYears,
		PartiesEvidenced: facts.PartiesEvidenced,
		RuleAdjustments:  adjustments,
		Adjusted:         strings.Join(reasons, "; "),
		RationaleIssues:  issues,
	}
}

// notAssessedJudgment is the record of a behavioral requirement
// (FR-HF-18): unresolved with evidence_ambiguous, no evidence, no
// model call, weight 0 in every number, shown as "not assessed".
func notAssessedJudgment(r prompts.Requirement) Judgment {
	adj := hiring.RuleAdjustment{
		Rule: hiring.RuleBehavioral, RuleVersion: hiring.RuleBehavioralVersion,
		FromDetermination: "", ToDetermination: hiring.DetUnresolved,
		FromBasis: "", ToBasis: hiring.BasisNotAddressed,
		AddedUncertainty: []string{hiring.UncEvidenceAmbiguous},
		Reason:           "a working-style requirement is recorded and shown as not assessed; it is not judged from documents and weighs nothing in the numbers",
	}
	return Judgment{
		RequirementID:   r.ID,
		Verdict:         hiring.DeriveVerdict(hiring.DetUnresolved),
		Determination:   hiring.DetUnresolved,
		Basis:           hiring.BasisNotAddressed,
		Uncertainty:     []string{hiring.UncEvidenceAmbiguous},
		Type:            r.Type,
		EvidenceIDs:     nil,
		Rationale:       "Not assessed: a working-style requirement is recorded, not judged from documents.",
		RuleAdjustments: []hiring.RuleAdjustment{adj},
		Adjusted:        adj.Reason,
		NotAssessed:     true,
	}
}

// The party and span helpers moved to hiring with the rules; entail.go
// and rationale.go read them under their old names.
func normaliseParty(s string) string { return hiring.NormaliseParty(s) }
func acronymOf(s string) string      { return hiring.AcronymOf(s) }

var (
	writtenYears   = hiring.WrittenYears
	writtenYearsRe = hiring.WrittenYearsRe
)
