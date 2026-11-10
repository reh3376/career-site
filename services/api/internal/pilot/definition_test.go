package pilot

import (
	"encoding/json"
	"strings"
	"testing"
)

// The instrument that ships has to be the one in
// docs/framework-research/06 §3 with the change-log additions: twenty
// problems in C1, the four [WF] items, the two pilot-only closers, and
// a route for every A1 option. A definition that loads but is missing
// one of those would field a different survey from the one the owner
// approved, and nobody would notice until the data came back.
func TestCurrentInstrumentLoadsAndMatchesTheDesign(t *testing.T) {
	in, raw, err := LoadInstrument(CurrentInstrumentVersion)
	if err != nil {
		t.Fatalf("LoadInstrument: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatal("raw definition is not valid JSON")
	}
	c1, _ := in.Item("C1")
	if c1 == nil || len(c1.Options) != 20 {
		t.Fatalf("C1 should list the twenty problems, has %d", len(c1.Options))
	}
	if c1.MaxSelect != 5 {
		t.Fatalf("C1 max_select = %d, want 5", c1.MaxSelect)
	}
	for _, id := range []string{"D4", "D8"} {
		it, _ := in.Item(id)
		if it == nil || !it.Required {
			t.Fatalf("%s must exist and be required ([WF] additions)", id)
		}
	}
	if d6, _ := in.Item("D6"); d6 == nil || d6.Branch != BranchHM {
		t.Fatal("D6 is the hiring-manager item")
	}
	if d7, _ := in.Item("D7"); d7 == nil || d7.Branch != BranchRecruiter {
		t.Fatal("D7 is the recruiter item")
	}
	p := in.Section("P")
	if p == nil || !p.PilotOnly || len(p.Items) != 2 {
		t.Fatal("section P holds the two pilot-only closing items")
	}
	for _, s := range in.Sections {
		if s.Intro == "" {
			t.Errorf("section %s has no intro", s.ID)
		}
	}
	e := in.Section("E")
	if e == nil || !strings.Contains(e.Intro, "breaks the job description into individual requirements") {
		t.Fatal("section E's intro is the concept paragraph the survey measures against")
	}
}

func TestBranchingCoversEveryA1Option(t *testing.T) {
	in, _, err := LoadInstrument(CurrentInstrumentVersion)
	if err != nil {
		t.Fatal(err)
	}
	a1, _ := in.Item("A1")
	for _, opt := range a1.Options {
		b := in.BranchFor(mustJSON(opt))
		if b == "" {
			t.Errorf("A1 option %q routes nowhere", opt)
		}
	}
	if in.BranchFor(mustJSON("Both recruiting and hiring-manager duties")) != BranchBoth {
		t.Error("both duties should route to both")
	}
	if in.BranchFor(mustJSON("None of these")) != BranchEnd {
		t.Error("none of these should end the survey")
	}
	if got := in.SectionOrder(BranchEnd); len(got) != 1 || got[0] != "A" {
		t.Errorf("end branch should see only A, got %v", got)
	}
	// A hiring manager never sees the recruiter-only item and vice
	// versa; "both" sees everything.
	d := in.Section("D")
	ids := func(items []Item) string {
		var b strings.Builder
		for _, it := range items {
			b.WriteString(it.ID + " ")
		}
		return b.String()
	}
	if s := ids(d.VisibleItems(BranchHM)); strings.Contains(s, "D7") || !strings.Contains(s, "D6") {
		t.Errorf("hm sees %s", s)
	}
	if s := ids(d.VisibleItems(BranchRecruiter)); strings.Contains(s, "D6 ") || !strings.Contains(s, "D7 ") {
		t.Errorf("recruiter sees %s", s)
	}
	if s := ids(d.VisibleItems(BranchBoth)); !strings.Contains(s, "D6 ") || !strings.Contains(s, "D7 ") {
		t.Errorf("both sees %s", s)
	}
}

func TestAnswerValidationByType(t *testing.T) {
	in, _, err := LoadInstrument(CurrentInstrumentVersion)
	if err != nil {
		t.Fatal(err)
	}
	c1, _ := in.Item("C1")
	two := mustJSON([]string{c1.Options[0], c1.Options[3]})
	if err := c1.ValidateAnswer(two, c1.ResolveOptions(nil)); err != nil {
		t.Errorf("two valid C1 picks refused: %v", err)
	}
	six := mustJSON(c1.Options[:6])
	if err := c1.ValidateAnswer(six, c1.Options); err == nil {
		t.Error("six C1 picks accepted; max is five")
	}
	if err := c1.ValidateAnswer(mustJSON([]string{"not an option"}), c1.Options); err == nil {
		t.Error("an unknown C1 option was accepted")
	}

	answers := map[string]string{"C1": two}
	c2, _ := in.Item("C2")
	opts := c2.ResolveOptions(answers)
	if len(opts) != 2 {
		t.Fatalf("C2 should offer the two C1 picks, got %v", opts)
	}
	if err := c2.ValidateAnswer(mustJSON(c1.Options[0]), opts); err != nil {
		t.Errorf("C2 pick from C1 refused: %v", err)
	}
	if err := c2.ValidateAnswer(mustJSON(c1.Options[5]), opts); err == nil {
		t.Error("C2 accepted a problem that was not selected in C1")
	}

	c3, _ := in.Item("C3")
	good := map[string]map[string]string{
		c1.Options[0]: {"frequency": "Most roles", "cost": "Major"},
		c1.Options[3]: {"frequency": "Rarely", "cost": "Minor"},
	}
	if err := c3.ValidateAnswer(mustJSON(good), opts); err != nil {
		t.Errorf("complete C3 grid refused: %v", err)
	}
	delete(good[c1.Options[3]], "cost")
	if err := c3.ValidateAnswer(mustJSON(good), opts); err == nil {
		t.Error("C3 with a missing cost was accepted")
	}

	d7, _ := in.Item("D7")
	if err := d7.ValidateAnswer("3", d7.Options); err != nil {
		t.Errorf("scale 3 refused: %v", err)
	}
	if err := d7.ValidateAnswer("6", d7.Options); err == nil {
		t.Error("scale 6 accepted on a 1 to 5 scale")
	}
	if err := d7.ValidateAnswer("2.5", d7.Options); err == nil {
		t.Error("scale 2.5 accepted")
	}

	p1, _ := in.Item("P1")
	if err := p1.ValidateAnswer("12", nil); err != nil {
		t.Errorf("minutes 12 refused: %v", err)
	}
	if !p1.IsEmptyAnswer("") || !p1.IsEmptyAnswer("null") || p1.IsEmptyAnswer("0") {
		t.Error("IsEmptyAnswer misjudges blanks")
	}
}

func TestRecordAndWorkflowLoad(t *testing.T) {
	r, err := LoadRecord(CurrentRecordVersion)
	if err != nil {
		t.Fatalf("LoadRecord: %v", err)
	}
	if len(r.RowIDs) != 10 || !r.HasRow("r6") || r.HasRow("r11") {
		t.Fatalf("record rows: %v", r.RowIDs)
	}
	w, err := LoadWorkflow(CurrentWorkflowVersion)
	if err != nil {
		t.Fatalf("LoadWorkflow: %v", err)
	}
	if len(w.Nodes) != 26 || !w.HasNode("C-01") || !w.HasNode("C-26") || w.HasNode("K-01") {
		t.Fatalf("workflow nodes: %d", len(w.Nodes))
	}
	if len(w.Prompts) != 0 || len(w.Phases) != 5 || len(w.Raw) == 0 {
		t.Fatalf("workflow v3 carries no prompts, five phases and its raw file: %d prompts, %d phases, %d bytes", len(w.Prompts), len(w.Phases), len(w.Raw))
	}
	if !w.HasPhase("B") || w.HasPhase("F") || !w.IsDecision("C-04") || w.IsDecision("C-01") {
		t.Fatal("phase and decision lookups wrong")
	}
	decisions := 0
	for _, n := range w.Nodes {
		if n.Decision {
			decisions++
		}
	}
	if decisions != 6 {
		t.Fatalf("six decision points, have %d", decisions)
	}
	// No em dash reaches a participant from these files (AGENTS.md).
	for _, raw := range [][]byte{r.Raw, w.Raw} {
		if strings.Contains(string(raw), "—") {
			t.Error("the stimulus contains an em dash")
		}
	}
	in, raw, _ := LoadInstrument(CurrentInstrumentVersion)
	if strings.Contains(string(raw), "—") {
		t.Error("the instrument contains an em dash")
	}
	_ = in
}

func TestParticipantCode(t *testing.T) {
	if got := ParticipantCode(3); got != "P-03" {
		t.Errorf("ParticipantCode(3) = %q", got)
	}
	if got := ParticipantCode(12); got != "P-12" {
		t.Errorf("ParticipantCode(12) = %q", got)
	}
	if got := MeasuredMinutes(1000, 1000+11*60+40); got != 12 {
		t.Errorf("MeasuredMinutes = %d, want 12", got)
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
