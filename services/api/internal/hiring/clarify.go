package hiring

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The clarification loop's rules, as pure functions (FSD FR-HF-13, 14,
// 24; 04 §4; Phase 0 step 7). An unresolved judgment names the one fact
// that would settle it. This file turns that into a question whose
// answer has a fixed shape, checks an answer against the shape, renders
// it as the statement a judge and a reader see, and, where the missing
// fact was a number a code rule needed, settles the judgment in code
// from the number the candidate stated.
//
// Two constraints run through all of it (03 §3.3, 04 §4.1). A question
// asks for evidence and never for confirmation: it does not repeat the
// figure a requirement is scored on. And every choice offered comes
// from the candidate's own corpus or from a list that is the same for
// every requirement, so choosing reveals nothing about the scoring.

// Answer types (vocabulary answer_type).
const (
	AnswerSpan            = "span"
	AnswerMagnitude       = "magnitude"
	AnswerInvolvement     = "involvement"
	AnswerCredential      = "credential"
	AnswerRelationship    = "relationship"
	AnswerEvidencePointer = "evidence_pointer"
	AnswerDisambiguation  = "disambiguation"
)

// Item and request states.
const (
	ItemPending       = "pending"
	ItemAnswered      = "answered"
	ItemNotApplicable = "not_applicable"
	ItemDeclined      = "declined"
	ItemWithdrawn     = "withdrawn"

	RequestSent     = "sent"
	RequestAnswered = "answered"
	RequestExpired  = "expired"

	ClosedAnswered   = "answered"
	ClosedExpired    = "expired"
	ClosedSuperseded = "superseded"

	AskedBySystem    = "system"
	AskedByCandidate = "candidate"

	// RunAwaitingCandidate is where a run and its posting rest while
	// the candidate has been asked something and the fit is withheld
	// (FR-HF-22).
	RunAwaitingCandidate = "awaiting_candidate"
)

// QuestionTemplateVersion names the question wording and the fields
// offered, stored on every item so a later wording does not rewrite
// what a candidate was actually asked.
const QuestionTemplateVersion = "q1"

// MaxAnswerFreeText bounds the free-text supplement (04 §4.2). It is
// evidence for the judge and is never parsed.
const MaxAnswerFreeText = 600

// RuleClarification is the rule adjustment recorded when a candidate's
// answer settles a judgment in code.
const (
	RuleClarification        = "clarification"
	RuleClarificationVersion = "v1"
)

// SourceCandidateResponse is the evidence kind of an answer.
const SourceCandidateResponse = "candidate_response"

// AnswerTypeFor maps an uncertainty code to the one answer type that
// resolves it (04 §4.1). A code this build does not know is asked as a
// pointer to the candidate's own work, the most general shape.
func AnswerTypeFor(code string) string {
	switch code {
	case UncDurationUnknown, UncRecencyUnknown:
		return AnswerSpan
	case UncScopeUnknown:
		return AnswerMagnitude
	case UncDepthUnknown:
		return AnswerInvolvement
	case UncCredentialUnknown:
		return AnswerCredential
	case UncPartyUnknown:
		return AnswerRelationship
	case UncEvidenceAmbiguous:
		return AnswerDisambiguation
	}
	return AnswerEvidencePointer
}

// uncertaintyOrder decides which code an item asks about when a
// judgment carries several: one question per judgment (FR-HF-13). The
// facts a code rule compares come first, because a number settles a
// row outright; the questions only a judge can weigh come last.
var uncertaintyOrder = []string{
	UncPartyUnknown, UncCredentialUnknown, UncDurationUnknown, UncRecencyUnknown, UncScopeUnknown,
	UncDepthUnknown, UncEquivalenceUnconfirmed, UncEvidenceAmbiguous,
}

// PrimaryUncertainty is the one code a judgment's question resolves. A
// judgment a person set to unresolved carries no code of its own
// (FR-HF-08 has a person set a determination and a basis), so the code
// is the one its basis implies: a shortfall basis names its fact, a
// lexical basis asks for the equivalence, a credential swap asks for
// the credential.
func PrimaryUncertainty(uncertainty []string, basis string) string {
	for _, want := range uncertaintyOrder {
		for _, got := range uncertainty {
			if got == want {
				return want
			}
		}
	}
	if len(uncertainty) > 0 {
		return uncertainty[0]
	}
	if code := ShortfallBases[basis]; code != "" {
		return code
	}
	switch basis {
	case BasisExperienceForEducation, BasisEducationForExperience:
		return UncCredentialUnknown
	case BasisVocabulary, BasisToolEquivalent, BasisAdjacentDomain:
		return UncEquivalenceUnconfirmed
	}
	return UncEvidenceAmbiguous
}

// AsksUnprompted reports whether an unresolved judgment on this basis
// produces a question without waiting to be asked (FR-HF-24): the
// lexical bases, which are the gap a keyword screen turns into a
// rejection, and an unproven relationship.
func AsksUnprompted(basis string) bool {
	if basis == BasisRelationshipUnproven {
		return true
	}
	for _, b := range LexicalBases {
		if b == basis {
			return true
		}
	}
	return false
}

// Engagement is one of the candidate's own roles or projects as the
// corpus holds it: what "your own corpus" means when a question offers
// a choice.
type Engagement struct {
	Ref          string // corpus_document:<id>
	Kind         string // role | project
	Title        string
	Organization string
	Start, End   int // years; 0 when the document does not say
}

// Label is the engagement as a choice reads.
func (e Engagement) Label() string {
	parts := []string{e.Title}
	if e.Organization != "" {
		parts = append(parts, e.Organization)
	}
	switch {
	case e.Start > 0 && e.End > 0 && e.End != e.Start:
		parts = append(parts, fmt.Sprintf("%d to %d", e.Start, e.End))
	case e.Start > 0:
		parts = append(parts, strconv.Itoa(e.Start))
	}
	if e.Kind == "project" {
		parts = append(parts, "project")
	}
	return strings.Join(parts, ", ")
}

// Field kinds.
const (
	FieldChoice = "choice"
	FieldMulti  = "multi"
	FieldNumber = "number"
	FieldText   = "text"

	OptionOwnCorpus   = "own_corpus"
	OptionGenericList = "generic_list"
	OptionPosting     = "posting"
)

// Option is one choice offered, with where it came from, stored on the
// item so the answer can be read later even after the corpus changes.
// Start and End ride on an engagement so a span can be worked out from
// what was offered.
type Option struct {
	Value  string `json:"value"`
	Label  string `json:"label"`
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
	Start  int    `json:"start,omitempty"`
	End    int    `json:"end,omitempty"`
}

// Field is one controlled input of a question.
type Field struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Kind     string   `json:"kind"`
	Required bool     `json:"required,omitempty"`
	Options  []Option `json:"options,omitempty"`
	// Max bounds a number; MaxLen a text.
	Max    float64 `json:"max,omitempty"`
	MaxLen int     `json:"max_len,omitempty"`
	Hint   string  `json:"hint,omitempty"`
}

// GenericLists are the requirement-independent lists a question may
// offer, read from the vocabulary registry.
type GenericLists struct {
	Involvement       []Code
	ScopeUnits        []Code
	CredentialLevels  []Code
	RelationshipKinds []Code
	ActivityClasses   []Code
}

// GenericListsFrom reads the lists from the registry, live codes only.
func GenericListsFrom(reg *Registry) GenericLists {
	live := func(vocab string, skip ...string) []Code {
		var out []Code
		for _, c := range reg.Codes(vocab) {
			if c.RetiredIn != "" {
				continue
			}
			skipped := false
			for _, s := range skip {
				if c.Code == s {
					skipped = true
				}
			}
			if !skipped {
				out = append(out, c)
			}
		}
		return out
	}
	return GenericLists{
		// "unknown" is what the evidence says of a depth nobody stated; a
		// candidate asked about their own work is not offered it.
		Involvement:       live(VocabInvolvementLevel, "unknown"),
		ScopeUnits:        live(VocabScopeUnit),
		CredentialLevels:  live(VocabCredentialLevel),
		RelationshipKinds: live(VocabRelationshipKind),
		ActivityClasses:   live(VocabActivityClass),
	}
}

// questionTemplates is keyed by (answer_type, requirement.type) with
// "" as the answer type's default (FR-HF-13). None of them repeats a
// figure, a level or a party the requirement is scored on: the
// candidate is shown the posting's own words above the question and
// asked what they did.
var questionTemplates = map[[2]string]string{
	{AnswerSpan, ""}:                   "In which of your roles did you do the work this line of the posting describes? Pick every role that applies. If it was only part of your time there, say roughly how many years it comes to in all.",
	{AnswerSpan, "tool"}:               "In which of your roles did you work with the tool or system this line names? Pick every role that applies. If it was only part of your time there, say roughly how many years it comes to in all.",
	{AnswerSpan, "domain"}:             "In which of your roles did you work in the industry or setting this line names? Pick every role that applies. If it was only part of your time there, say roughly how many years it comes to in all.",
	{AnswerMagnitude, ""}:              "How large was the work this line describes at its largest? Choose what was measured and give the figure.",
	{AnswerInvolvement, ""}:            "Which best describes your part in the work this line describes, and in which role or project?",
	{AnswerInvolvement, "tool"}:        "Which best describes how you worked with the tool or system this line names, and in which role or project?",
	{AnswerInvolvement, "outcome"}:     "Which best describes your part in delivering what this line describes, and in which role or project?",
	{AnswerCredential, ""}:             "What is the highest qualification you hold that bears on this line? Give its level, its field, where it is from and the year.",
	{AnswerRelationship, ""}:           "Which of the organisations this line names have you worked with, and in what capacity?",
	{AnswerEvidencePointer, ""}:        "Which of your roles or projects comes closest to the work this line describes? Pick them, and say in a few sentences what you did there.",
	{AnswerEvidencePointer, "tool"}:    "In which of your roles or projects did you use this tool, or one that does the same job? Pick them, and say in a few sentences what you used and for what.",
	{AnswerEvidencePointer, "domain"}:  "Which of your roles or projects were in this industry or setting, or one close to it? Pick them, and say in a few sentences what the setting was.",
	{AnswerEvidencePointer, "outcome"}: "In which of your roles or projects did you deliver something like this? Pick them, and say in a few sentences what you delivered.",
	// A qualification line is not "work this line describes": the
	// question asks where the qualification, or what stands in for it,
	// shows.
	{AnswerEvidencePointer, "credential"}: "Which of your roles or projects best shows what this line asks for, by the qualification itself or by what you did? Pick them, and say in a few sentences what you studied or did there.",
	{AnswerDisambiguation, ""}:            "This line can be read more than one way. Which of your roles or projects does it best fit, and what kind of work was it?",
}

// QuestionText renders the question for an answer type and a
// requirement type, falling back to the answer type's default wording.
func QuestionText(answerType, requirementType string) string {
	if q, ok := questionTemplates[[2]string{answerType, requirementType}]; ok {
		return q
	}
	return questionTemplates[[2]string{answerType, ""}]
}

func engagementOptions(list []Engagement, kinds ...string) []Option {
	var out []Option
	for _, e := range list {
		ok := len(kinds) == 0
		for _, k := range kinds {
			if e.Kind == k {
				ok = true
			}
		}
		if ok {
			out = append(out, Option{Value: e.Ref, Label: e.Label(), Source: OptionOwnCorpus, Ref: e.Ref, Start: e.Start, End: e.End})
		}
	}
	return out
}

func codeOptions(codes []Code, vocab string) []Option {
	out := make([]Option, 0, len(codes))
	for _, c := range codes {
		out = append(out, Option{Value: c.Code, Label: c.Label, Source: OptionGenericList, Ref: vocab})
	}
	return out
}

// QuestionFields are the controlled inputs of a question: choices from
// the candidate's own roles and projects, from the generic lists, and,
// for a relationship, from the organisations the posting itself names,
// which are public in the posting already.
func QuestionFields(answerType string, engagements []Engagement, lists GenericLists, namedParties []string) []Field {
	roles := engagementOptions(engagements, "role")
	all := engagementOptions(engagements)
	switch answerType {
	case AnswerSpan:
		return []Field{
			{Name: "engagements", Label: "Your roles", Kind: FieldMulti, Required: true, Options: roles},
			{Name: "years", Label: "Years in all, if less than the whole of those roles", Kind: FieldNumber, Max: 60,
				Hint: "Leave empty when it was the whole of the roles you picked."},
		}
	case AnswerMagnitude:
		return []Field{
			{Name: "unit", Label: "What was measured", Kind: FieldChoice, Required: true, Options: codeOptions(lists.ScopeUnits, VocabScopeUnit)},
			{Name: "value", Label: "The figure", Kind: FieldNumber, Required: true, Max: 1e12},
		}
	case AnswerInvolvement:
		return []Field{
			{Name: "level", Label: "Your involvement", Kind: FieldChoice, Required: true, Options: codeOptions(lists.Involvement, VocabInvolvementLevel)},
			{Name: "engagement", Label: "Which role or project", Kind: FieldChoice, Required: true, Options: all},
		}
	case AnswerCredential:
		return []Field{
			{Name: "level", Label: "Level", Kind: FieldChoice, Required: true, Options: codeOptions(lists.CredentialLevels, VocabCredentialLevel)},
			{Name: "field", Label: "Field", Kind: FieldText, Required: true, MaxLen: 120},
			{Name: "institution", Label: "Institution or issuing body", Kind: FieldText, MaxLen: 120},
			{Name: "year", Label: "Year", Kind: FieldNumber, Max: 2100},
			{Name: "status", Label: "Status", Kind: FieldChoice, Required: true, Options: []Option{
				{Value: "completed", Label: "Completed", Source: OptionGenericList, Ref: "credential_status"},
				{Value: "in_progress", Label: "In progress", Source: OptionGenericList, Ref: "credential_status"},
			}},
		}
	case AnswerRelationship:
		var orgs []Option
		for _, p := range namedParties {
			if p = strings.TrimSpace(p); p != "" {
				orgs = append(orgs, Option{Value: p, Label: p, Source: OptionPosting, Ref: "named_parties"})
			}
		}
		return []Field{
			{Name: "orgs", Label: "Organisations you have worked with", Kind: FieldMulti, Required: true, Options: orgs},
			{Name: "kind", Label: "In what capacity", Kind: FieldChoice, Required: true, Options: codeOptions(lists.RelationshipKinds, VocabRelationshipKind)},
		}
	case AnswerDisambiguation:
		return []Field{
			{Name: "engagement", Label: "Which role or project", Kind: FieldChoice, Required: true, Options: all},
			{Name: "activity", Label: "What kind of work it was", Kind: FieldChoice, Required: true, Options: codeOptions(lists.ActivityClasses, VocabActivityClass)},
		}
	}
	// evidence_pointer, and any type this build does not know.
	return []Field{
		{Name: "engagements", Label: "Your roles or projects", Kind: FieldMulti, Required: true, Options: all},
	}
}

// FreeTextRequired reports whether an answer of this type must carry
// the free-text supplement: a pointer to one's own work says nothing
// until it says what was done there.
func FreeTextRequired(answerType string) bool { return answerType == AnswerEvidencePointer }

// Answer is a candidate's answer to one item: what they chose in each
// field, and the free-text supplement. Status is answered, or one of
// the two ways out every item offers: not_applicable ("I have not done
// this") and declined ("prefer not to answer").
type Answer struct {
	Status   string         `json:"status"`
	Values   map[string]any `json:"values,omitempty"`
	FreeText string         `json:"free_text,omitempty"`
}

// ErrInvalidAnswer wraps the sentence that says what was wrong with an
// answer as given.
var ErrInvalidAnswer = errors.New("hiring: invalid answer")

func invalidAnswer(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidAnswer, fmt.Sprintf(format, args...))
}

func optionByValue(f Field, v string) (Option, bool) {
	for _, o := range f.Options {
		if o.Value == v {
			return o, true
		}
	}
	return Option{}, false
}

// NormaliseAnswer checks an answer against the fields its item offered
// and returns it in canonical form: every chosen value one that was
// offered, numbers in range, text trimmed and bounded. A value that
// was not offered is refused, never coerced: the options are the
// control (04 §4.1). not_applicable and declined carry no values.
func NormaliseAnswer(answerType string, fields []Field, a Answer) (Answer, error) {
	out := Answer{Status: a.Status, FreeText: strings.TrimSpace(a.FreeText)}
	if len([]rune(out.FreeText)) > MaxAnswerFreeText {
		return out, invalidAnswer("the note is longer than %d characters", MaxAnswerFreeText)
	}
	switch a.Status {
	case ItemNotApplicable, ItemDeclined:
		return out, nil
	case ItemAnswered:
	default:
		return out, invalidAnswer("an answer is answered, not applicable or declined")
	}
	out.Values = map[string]any{}
	for _, f := range fields {
		raw, present := a.Values[f.Name]
		// A value that arrives as a list for an input that takes one (the
		// wire carries every input as a list) is its first entry.
		if list, ok := raw.([]string); ok && f.Kind != FieldMulti {
			if len(list) > 0 {
				raw = list[0]
			} else {
				raw, present = nil, false
			}
		}
		switch f.Kind {
		case FieldChoice:
			v, _ := raw.(string)
			v = strings.TrimSpace(v)
			if v == "" {
				if f.Required {
					return out, invalidAnswer("%s: choose one", f.Label)
				}
				continue
			}
			if _, ok := optionByValue(f, v); !ok {
				return out, invalidAnswer("%s: that choice was not offered", f.Label)
			}
			out.Values[f.Name] = v
		case FieldMulti:
			var picked []string
			switch list := raw.(type) {
			case []string:
				picked = list
			case []any:
				for _, x := range list {
					if s, ok := x.(string); ok {
						picked = append(picked, s)
					}
				}
			}
			seen := map[string]bool{}
			var kept []string
			// Kept in the order offered, so the same choices are the same
			// answer however they were ticked.
			for _, o := range f.Options {
				for _, v := range picked {
					if strings.TrimSpace(v) == o.Value && !seen[o.Value] {
						seen[o.Value] = true
						kept = append(kept, o.Value)
					}
				}
			}
			for _, v := range picked {
				if v = strings.TrimSpace(v); v != "" && !seen[v] {
					return out, invalidAnswer("%s: a choice that was not offered", f.Label)
				}
			}
			if len(kept) == 0 {
				if f.Required {
					return out, invalidAnswer("%s: choose at least one", f.Label)
				}
				continue
			}
			out.Values[f.Name] = kept
		case FieldNumber:
			var n float64
			switch x := raw.(type) {
			case float64:
				n = x
			case int:
				n = float64(x)
			case string:
				if x = strings.TrimSpace(x); x != "" {
					parsed, err := strconv.ParseFloat(x, 64)
					if err != nil {
						return out, invalidAnswer("%s: a number is needed", f.Label)
					}
					n = parsed
				} else {
					present = false
				}
			default:
				present = false
			}
			if !present || n == 0 {
				if f.Required {
					return out, invalidAnswer("%s: a number is needed", f.Label)
				}
				continue
			}
			if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || (f.Max > 0 && n > f.Max) {
				return out, invalidAnswer("%s: out of range", f.Label)
			}
			out.Values[f.Name] = n
		case FieldText:
			v, _ := raw.(string)
			v = strings.Join(strings.Fields(v), " ")
			if v == "" {
				if f.Required {
					return out, invalidAnswer("%s: this is needed", f.Label)
				}
				continue
			}
			if f.MaxLen > 0 && len([]rune(v)) > f.MaxLen {
				return out, invalidAnswer("%s: longer than %d characters", f.Label, f.MaxLen)
			}
			out.Values[f.Name] = v
		}
	}
	if FreeTextRequired(answerType) && out.FreeText == "" {
		return out, invalidAnswer("say in a few sentences what you did there")
	}
	return out, nil
}

func (a Answer) str(name string) string {
	s, _ := a.Values[name].(string)
	return s
}

func (a Answer) num(name string) float64 {
	switch x := a.Values[name].(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return 0
}

func (a Answer) list(name string) []string {
	switch x := a.Values[name].(type) {
	case []string:
		return x
	case []any:
		var out []string
		for _, v := range x {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func fieldByName(fields []Field, name string) (Field, bool) {
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// AnswerLines renders an answer as labelled lines, each field with the
// label of what was chosen, in the order the fields were offered. It is
// what a reader of the record is shown and what the statement below is
// built from.
func AnswerLines(fields []Field, a Answer) []string {
	var out []string
	switch a.Status {
	case ItemNotApplicable:
		out = append(out, "The candidate states they have not done this.")
	case ItemDeclined:
		out = append(out, "The candidate preferred not to answer.")
	}
	for _, f := range fields {
		raw, ok := a.Values[f.Name]
		if !ok {
			continue
		}
		switch f.Kind {
		case FieldChoice:
			v, _ := raw.(string)
			if o, ok := optionByValue(f, v); ok {
				out = append(out, f.Label+": "+o.Label)
			}
		case FieldMulti:
			var labels []string
			for _, v := range a.list(f.Name) {
				if o, ok := optionByValue(f, v); ok {
					labels = append(labels, o.Label)
				}
			}
			if len(labels) > 0 {
				out = append(out, f.Label+": "+strings.Join(labels, "; "))
			}
		case FieldNumber:
			out = append(out, f.Label+": "+strconv.FormatFloat(a.num(f.Name), 'f', -1, 64))
		case FieldText:
			out = append(out, f.Label+": "+a.str(f.Name))
		}
	}
	if a.FreeText != "" {
		out = append(out, "The candidate adds: "+a.FreeText)
	}
	return out
}

// AnswerStatement is the answer as one passage of evidence: what the
// judge is shown and what the evidence item stores. It says whose
// statement it is and when it was made, because a judgment that cites
// it rests on self-attestation and has to be readable as such.
func AnswerStatement(fields []Field, a Answer, at time.Time) string {
	lines := AnswerLines(fields, a)
	for i, line := range lines {
		// Each line ends in one full stop, whatever the candidate typed.
		lines[i] = strings.TrimRight(strings.TrimSpace(line), ".")
	}
	return "The candidate's own answer to a follow-up question about this requirement, given " +
		at.UTC().Format("2006-01-02") + ". " + strings.Join(lines, ". ") + "."
}

// spanYears is the length of time the chosen engagements cover, in
// years, with overlapping roles counted once, and the latest year any
// of them reaches. An engagement with no end is still going.
func spanYears(f Field, picked []string, now time.Time) (years float64, latest int) {
	type span struct{ from, to int }
	var spans []span
	for _, v := range picked {
		o, ok := optionByValue(f, v)
		if !ok || o.Start <= 0 {
			continue
		}
		end := o.End
		if end <= 0 || end > now.Year() {
			end = now.Year()
		}
		if end < o.Start {
			continue
		}
		spans = append(spans, span{o.Start, end})
		if end > latest {
			latest = end
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].from < spans[j].from })
	reach := 0
	for _, s := range spans {
		from := s.from
		if from < reach {
			from = reach
		}
		if s.to > from {
			years += float64(s.to - from)
			reach = s.to
		}
	}
	return years, latest
}

// AnswerFacts puts what a structured answer states into the reported
// facts, where the code rules read it: a span and its latest year, a
// magnitude, a credential, the organisations worked with, a depth of
// involvement. The candidate's figure replaces the model's reading for
// the fact that was asked about, because the question existed for the
// reason that the documents did not state it. Answer types whose
// content only a judge can weigh leave the facts as they were.
func AnswerFacts(answerType string, fields []Field, a Answer, facts ReportedFacts, now time.Time) ReportedFacts {
	if a.Status != ItemAnswered {
		return facts
	}
	out := facts
	switch answerType {
	case AnswerSpan:
		f, _ := fieldByName(fields, "engagements")
		years, latest := spanYears(f, a.list("engagements"), now)
		if stated := a.num("years"); stated > 0 && (years == 0 || stated < years) {
			years = stated
		}
		if years > 0 {
			out.StatedSpanYears = years
		}
		if latest > 0 {
			out.MostRecentYear = latest
		}
	case AnswerMagnitude:
		if unit, value := a.str("unit"), a.num("value"); unit != "" && value > 0 {
			out.StatedScope = &Scope{Unit: unit, Value: value}
		}
	case AnswerCredential:
		if level := a.str("level"); level != "" {
			out.CredentialFound = &Credential{Level: level, Field: a.str("field"), Status: a.str("status")}
		}
	case AnswerRelationship:
		for _, org := range a.list("orgs") {
			known := false
			for _, got := range out.PartiesEvidenced {
				if PartyMatches(org, got) {
					known = true
				}
			}
			if !known {
				out.PartiesEvidenced = append(append([]string{}, out.PartiesEvidenced...), org)
			}
		}
	case AnswerInvolvement:
		if level := a.str("level"); level != "" {
			out.InvolvementLevel = level
		}
	}
	return out
}

// codeRules are the rules whose missing fact a structured answer
// supplies, with the answer type that supplies it.
var codeRules = map[string]string{
	RuleDuration:     AnswerSpan,
	RuleRecency:      AnswerSpan,
	RuleScope:        AnswerMagnitude,
	RuleCredential:   AnswerCredential,
	RuleRelationship: AnswerRelationship,
}

// Resettle settles in code a judgment that a code rule had left
// unresolved, now that the candidate has stated the fact the rule was
// missing. Such a judgment is one the model had found evidenced; the
// rule set it aside because a number, a credential or a named party was
// not in the documents. With the fact supplied the same rules are run
// again over the model's own verdict, and whatever they now say stands:
// met when the stated fact satisfies the requirement, a shortfall when
// it does not. No model is asked, because nothing remains to be judged.
//
// ok is false when the judgment was not left unresolved by a rule this
// answer speaks to (the model itself was unsure, or the answer is of a
// kind only a judge can weigh); the caller then has the judge look
// again with the answer as evidence.
func Resettle(req RuleInput, answerType string, adjustments []RuleAdjustment, basis string, facts ReportedFacts, now time.Time) (Verdict, []RuleAdjustment, bool) {
	at := -1
	for i, adj := range adjustments {
		if codeRules[adj.Rule] == answerType && adj.ToDetermination == DetUnresolved {
			at = i
		}
	}
	if at < 0 {
		return Verdict{}, nil, false
	}
	rule := adjustments[at]
	model := Verdict{Determination: rule.FromDetermination, Basis: rule.FromBasis, Uncertainty: []string{}}
	settled, again := ApplyRules(req, model, facts, now)
	if settled.Uncertainty == nil {
		settled.Uncertainty = []string{}
	}
	// What ran before the rule (the consistency repair, the evidence
	// rule) is kept; the rule's own entry is replaced by what the rules
	// say now, and the settling is recorded as its own entry.
	out := append([]RuleAdjustment{}, adjustments[:at]...)
	out = append(out, again...)
	out = append(out, RuleAdjustment{
		Rule: RuleClarification, RuleVersion: RuleClarificationVersion,
		FromDetermination: DetUnresolved, ToDetermination: settled.Determination,
		FromBasis: basis, ToBasis: settled.Basis,
		AddedUncertainty: []string{},
		Reason:           "the candidate stated the fact the " + rule.Rule + " rule was missing, and the rule was applied again to it",
	})
	return settled, out, true
}

// NotApplicableVerdict is what "I have not done this" resolves a
// judgment to (FR-HF-14): not met, not addressed, on the candidate's
// own statement.
func NotApplicableVerdict(basis string) (Verdict, RuleAdjustment) {
	return Verdict{Determination: DetNotMet, Basis: BasisNotAddressed, Uncertainty: []string{}},
		RuleAdjustment{
			Rule: RuleClarification, RuleVersion: RuleClarificationVersion,
			FromDetermination: DetUnresolved, ToDetermination: DetNotMet,
			FromBasis: basis, ToBasis: BasisNotAddressed,
			AddedUncertainty: []string{},
			Reason:           "the candidate states they have not done this",
		}
}
