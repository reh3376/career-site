package dtreport

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Answer is one question of a completed run, as v_dt_answers defines it.
type Answer struct {
	Position int // 1 to 30, in the order asked
	BlockNo  int
	Load     string
	ItemCode string
	Family   string
	// correct, lure, other or expired.
	Outcome string
	// -1 when no rating was given. Zero is never a rating: the scale
	// starts at 10.
	Confidence int
	LatencyMs  int

	// How many options the question offered, for the guessing figure.
	// Zero when unknown.
	Options int

	// The question and its answers. Read only for the run the report is
	// about, and printed only in the owner's copy.
	Prompt      string
	ChosenText  string
	CorrectText string
}

func (a Answer) correct() bool { return a.Outcome == "correct" }
func (a Answer) lure() bool    { return a.Outcome == "lure" }
func (a Answer) expired() bool { return a.Outcome == "expired" }
func (a Answer) rated() bool   { return a.Confidence >= 0 }

// Recall is the number a participant gave back at the end of a block.
type Recall struct {
	BlockNo   int
	Load      string
	Presented string
	Expected  string
	Response  string
	// exact, partial, untransformed, wrong_digits or expired.
	Outcome    string
	DigitsHeld int
}

// Run is one completed sitting.
type Run struct {
	// The session's public id. The first eight characters are the short
	// id printed on the report and used in its filename.
	Key      string
	Started  time.Time
	Finished time.Time

	Device       string // desktop, tablet, phone or unknown
	Audio        string // sound or visual
	TimerVisible bool

	InstrumentVersion string
	ItemSetVersion    string

	// Which sitting this was. Nil when no identity could place the run,
	// which is not the same statement as a first sitting.
	AttemptNo *int
	Synthetic bool

	// encode, defer, unsure, or empty when the question was never
	// answered.
	RecallStrategy string

	// Not set aside on review, at the level of the run or of any block.
	// Only read for peers: the run a report is about is reported
	// whatever the owner later decided about it.
	Usable bool

	Answers []Answer
	Recalls []Recall
}

// ShortID is the handle the admin console, the CSV export and the
// report all print for a run.
func (r Run) ShortID() string {
	if len(r.Key) >= 8 {
		return r.Key[:8]
	}
	return r.Key
}

var generationRE = regexp.MustCompile(`^v(\d+)`)

// Generation is the instrument generation a run was taken under: 4 for
// "v4-m6000-q25000-r20000". Zero when the version does not carry one.
func (r Run) Generation() int {
	m := generationRE.FindStringSubmatch(r.InstrumentVersion)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// Timings is the part of the instrument version that names the time
// limits. Two runs are comparable only when it matches (FR-DT-15).
func (r Run) Timings() string {
	if i := strings.Index(r.InstrumentVersion, "-"); i >= 0 {
		return r.InstrumentVersion[i+1:]
	}
	return ""
}

var timingRE = regexp.MustCompile(`m(\d+)-q(\d+)-r(\d+)`)

// limits reads the three time limits, in seconds, out of the version.
func (r Run) limits() (memorise, question, recall float64) {
	m := timingRE.FindStringSubmatch(r.InstrumentVersion)
	if m == nil {
		return 0, 0, 0
	}
	f := func(s string) float64 { n, _ := strconv.Atoi(s); return float64(n) / 1000 }
	return f(m[1]), f(m[2]), f(m[3])
}

// EmailFigures are the numbers and readings the results email states.
//
// They are handed in rather than worked out again. The report is
// attached to that email, so the two are read side by side, and a
// second derivation here would be a second definition waiting to
// disagree with the first in front of the participant.
type EmailFigures struct {
	EarlyAccuracy   int
	EarlyConfidence int
	HardAccuracy    int
	HardConfidence  int
	// no_decline, effect, overcorrected or calibrated.
	Reading     string
	FatigueHeld bool
}

// Input is everything the report is built from.
type Input struct {
	Run Run
	// Every other completed, non-synthetic run on the same question pool
	// and the same time limits: first sittings and repeats, usable or
	// set aside. The package decides who is comparable, so the rule is
	// in one place and is tested.
	Peers []Run

	Email EmailFigures

	// The confidence at or above which an answer counts as sure. Read
	// from dt_confident_threshold(), which is its one definition.
	ConfidentThreshold int

	// How many other first-time participants a ranking needs.
	MinPeers int

	// When the report was prepared.
	Now time.Time
}

// Why a report carries no ranking.
const (
	ReasonNone       = ""
	ReasonSmallGroup = "small_group"
	ReasonRepeat     = "repeat"
	ReasonUnplaced   = "unplaced"
	ReasonSynthetic  = "synthetic"
)

// era groups the instrument generations a participant would experience
// as the same test.
//
// The generation moves whenever the data should record that something
// changed, and not every change makes two runs different to sit. v3
// stopped printing the worked answer beside the number in blocks 3 and
// 4, which made those blocks harder: a real difference. v4 added a
// countdown for a random half and is otherwise v3. So the report pools
// every generation run under the same questions and time limits, says
// how many of the group sat a different form, and can restate a
// standing among those who sat the same one.
//
// A generation missing from this table is treated as a form of its own,
// which is the safe reading, and TestCurrentGenerationHasAnEra fails
// until somebody decides.
var eras = map[int]int{2: 1, 3: 2, 4: 2}

// eraNotes says, for a participant, what marked each form of the test.
var eraNotes = map[int]string{
	1: "the screen showed the worked answer for the number in blocks 3 and 4, which made the number easier to hold",
	2: "the worked answer for the number was no longer shown in blocks 3 and 4, which made the number harder to hold",
}

// generationNotes says what each generation changed, for the method
// note at the end of the report.
var generationNotes = map[int]string{
	3: "Version 3 stopped showing the worked answer for the number in blocks 3 and 4.",
	4: "Version 4 shows a countdown on each question to a random half of participants.",
}

// KnowsGeneration reports whether a generation has been placed in an
// era. The users package tests the current one against it, so bumping
// the instrument's generation without deciding how its runs compare
// with the earlier ones fails a test rather than a participant's report.
func KnowsGeneration(generation int) bool {
	_, ok := eras[generation]
	return ok
}

func eraOf(generation int) int {
	if e, ok := eras[generation]; ok {
		return e
	}
	// Unknown generations each stand alone, well clear of the known eras.
	return 1000 + generation
}
