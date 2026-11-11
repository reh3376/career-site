package hiring

import (
	"fmt"
	"strconv"
	"strings"
)

// Evidence-standard templates (05 §2): what counts as met, as
// unresolved and as not met for each type of requirement, in words a
// person outside the field can read. The extraction stage instantiates
// the template for each requirement and the hiring manager edits the
// instance. The judge is shown an instance a person has rewritten, and
// not the template's own wording: shown on every requirement, the
// template's "unresolved when" clauses turned "not met" into
// "unresolved" on weak requirements and moved a posting across the gate
// (docs/llm-tuning-log.md, 2026-11-10, step 5). The candidate never
// sees either.
//
// Templates are data in the sense that matters: versioned, one per
// type, and recorded on every requirement as type@version, so a
// template revision can be told from a person's edit and recurring
// edits of one type's instance are the signal to revise that type's
// template (05 §2.4). They are compiled in rather than rows because a
// template is rendered with the thresholds, which only code does.
//
// Each sentence names the uncertainty code that goes with its
// unresolved case, because the code is what later becomes a question to
// the candidate, and a person rewriting a standard should see which
// question their wording leads to.

// TemplateVersion is the version of this set. It changes when any
// template's wording does.
const TemplateVersion = "1"

// EditedTemplatePrefix marks an evidence standard a person rewrote; the
// template it started from follows the prefix.
const EditedTemplatePrefix = "edited:"

// RenderEvidenceStandard instantiates the template for a requirement
// and returns it with its reference (type@version). An unknown type
// gets the skill template, which is the reading the heuristic and the
// extractor both fall back to.
func RenderEvidenceStandard(typ string, t Thresholds, namedParties []string) (text, template string) {
	var s string
	switch typ {
	case "outcome":
		s = "Met when the documents state the candidate's part in producing a comparable result, with the result itself stated. " +
			"Unresolved when the activity is shown but no result or scale is stated (scope_unknown). " +
			"Not met when no related activity appears."
		if t.MinScope != nil {
			s += " The scale asked for is " + describeScope(*t.MinScope) + "; it is compared in code with the scale the documents state."
		}
	case "tool":
		s = "Met when the documents name this tool, or one the known equivalents accept in full, in a work context. " +
			"Unresolved when only a partly equivalent tool is shown (equivalence_unconfirmed), or the tool appears only in a list with no context (depth_unknown). " +
			"Not met when neither the tool nor an equivalent appears."
	case "domain":
		s = "Met when the documents show sustained work, at least one engagement, in this field or in one accepted as transferring in full. " +
			"Unresolved when the work is in a partly transferable field (equivalence_unconfirmed), or the field is mentioned without an engagement (depth_unknown). " +
			"Not met when no engagement in the field or a transferable one appears."
	case "credential":
		what := "the credential"
		switch {
		case t.CredentialLevel != "" && t.CredentialField != "":
			what = credentialPhrase(t.CredentialLevel) + " in " + t.CredentialField
		case t.CredentialLevel != "":
			what = credentialPhrase(t.CredentialLevel)
		}
		s = "Met when the documents state " + what + " as completed, or satisfy an alternative the requirement itself offers. " +
			"Unresolved when a credential is stated but its level or field is unclear (credential_unknown), or the work it certifies is shown and no accepted equivalent covers it (equivalence_unconfirmed). " +
			"Not met when no credential and no accepted alternative appears."
	case "experience":
		if t.MinYears != nil {
			years := strconv.FormatFloat(*t.MinYears, 'g', -1, 64)
			s = "Met when the documents state at least " + years + " years of the work asked for; the years are compared in code. " +
				"Unresolved when the work is shown but no years are stated for it (duration_unknown), or the years stated fall up to a quarter short. " +
				"Not met when the years stated fall further short, or no such work appears."
		} else {
			s = "Met when the documents show the kind of work asked for, in a job or project. " +
				"Unresolved when the work is shown but how long or how deeply is not stated (duration_unknown, depth_unknown). " +
				"Not met when no such work appears."
		}
		if t.RecencyYears != nil {
			s += " The work must fall within the last " + strconv.FormatFloat(*t.RecencyYears, 'g', -1, 64) + " years; the dates are compared in code."
		}
	case "relationship":
		who := "an organisation the requirement names"
		if len(namedParties) > 0 {
			who = strings.Join(namedParties, ", ")
		}
		s = "Met when the documents name " + who + " as an organisation the candidate worked with or for. " +
			"Unresolved when one is named but the kind of relationship is not stated (party_unknown). " +
			"Not met when none is named, or only its products are used: using a product is not a working relationship."
	case "constraint":
		s = "Met when the documents state the condition is satisfied. " +
			"Unresolved when they touch on it without settling it (evidence_ambiguous, or credential_unknown for a clearance or authorisation). " +
			"Not met when they state it is not satisfied or say nothing about it. A condition like this is usually settled by asking the candidate."
	case "behavioral":
		s = "Not judged from documents. A working-style requirement is recorded as not assessed, left to the people who interview, and does not count in the figures."
	default:
		typ = "skill"
		s = "Met when the documents show the candidate doing this in a job or project. " +
			"Unresolved when it is only named in a list with no context (depth_unknown), or a closely related skill is shown in its place (equivalence_unconfirmed). " +
			"Not met when nothing names or shows it, or an accepted equivalent."
	}
	if len(s) > MaxEvidenceStandard {
		s = s[:MaxEvidenceStandard]
	}
	return s, fmt.Sprintf("%s@%s", typ, TemplateVersion)
}

func credentialPhrase(level string) string {
	switch level {
	case "certificate":
		return "the certification"
	case "associate":
		return "an associate degree"
	case "bachelor":
		return "a bachelor's degree"
	case "master":
		return "a master's degree"
	case "doctorate":
		return "a doctorate"
	case "license":
		return "the licence"
	}
	return "the credential"
}
