package dtreport

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Fixtures are generated, never copied from real runs: this repository
// is public and a participant's answers are not test data.

var fixtureFamilies = []string{"arithmetic", "arithmetic", "base_rate", "conjunction", "syllogism", "syllogism"}
var fixtureLoads = []string{"", "d3", "d4", "d4_plus1", "d4_plus3", "d3_control"}

type shape struct {
	pCorrect, pLure, pExpired float64
	pRated                    float64
	levels                    []int
	slow                      bool
}

var typical = shape{pCorrect: 0.35, pLure: 0.35, pExpired: 0.05, pRated: 0.97, levels: []int{10, 30, 50, 70, 90}}

func fixtureRun(key string, seed int64, sh shape) Run {
	rng := rand.New(rand.NewSource(seed))
	one := 1
	start := time.Date(2026, 10, 8, 14, 0, 0, 0, time.UTC).Add(time.Duration(seed%40) * 24 * time.Hour)
	run := Run{Key: key, Started: start, Finished: start.Add(12 * time.Minute), Device: "phone", Audio: "sound",
		TimerVisible: seed%2 == 0, InstrumentVersion: "v4-m6000-q25000-r20000", ItemSetVersion: "pool-2026-10-07",
		AttemptNo: &one, Usable: true, RecallStrategy: []string{"encode", "defer", "unsure", ""}[seed%4]}
	pos := 0
	for b := 1; b <= 5; b++ {
		for q := 0; q < 6; q++ {
			pos++
			fam := fixtureFamilies[q]
			a := Answer{Position: pos, BlockNo: b, Load: fixtureLoads[b], Family: fam,
				ItemCode: fmt.Sprintf("%s%02d", strings.ToUpper(fam[:1]), rng.Intn(18)+1), Options: 3 + rng.Intn(2),
				Prompt: fmt.Sprintf("PROMPT-%s-%d", key, pos), ChosenText: fmt.Sprintf("CHOSEN-%s-%d", key, pos),
				CorrectText: fmt.Sprintf("CORRECT-%s-%d", key, pos), Confidence: -1}
			switch r := rng.Float64(); {
			case r < sh.pExpired:
				a.Outcome = "expired"
			case r < sh.pExpired+sh.pCorrect:
				a.Outcome = "correct"
			case r < sh.pExpired+sh.pCorrect+sh.pLure:
				a.Outcome = "lure"
			default:
				a.Outcome = "other"
			}
			if a.Outcome != "expired" {
				a.LatencyMs = 4000 + rng.Intn(20000)
				if sh.slow {
					a.LatencyMs = 18000 + rng.Intn(6000)
				}
				if rng.Float64() < sh.pRated && len(sh.levels) > 0 {
					a.Confidence = sh.levels[rng.Intn(len(sh.levels))]
				}
			}
			run.Answers = append(run.Answers, a)
		}
		rc := Recall{BlockNo: b, Load: fixtureLoads[b], Presented: "7478", Expected: "7478", Response: "7478", Outcome: "exact", DigitsHeld: 4}
		switch rng.Intn(6) {
		case 1:
			rc.Response, rc.Outcome, rc.DigitsHeld = "7482", "partial", 2
		case 2:
			rc.Response, rc.Outcome, rc.DigitsHeld = "4778", "wrong_digits", 2
		case 3:
			rc.Response, rc.Outcome, rc.DigitsHeld = "0582", "wrong_digits", 0
		case 4:
			rc.Response, rc.Outcome, rc.DigitsHeld = "", "expired", 0
		case 5:
			rc.Expected, rc.Response, rc.Outcome, rc.DigitsHeld = "8589", "7478", "untransformed", 4
		}
		run.Recalls = append(run.Recalls, rc)
	}
	return run
}

func fixtureInput(seed int64, peers int) Input {
	in := Input{Run: fixtureRun("aaaaaaaa-0000-4000-8000-000000000000", seed, typical), ConfidentThreshold: 70, MinPeers: 10,
		Now:   time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC),
		Email: EmailFigures{EarlyAccuracy: 42, EarlyConfidence: 60, HardAccuracy: 33, HardConfidence: 58, Reading: "calibrated", FatigueHeld: true}}
	for i := 0; i < peers; i++ {
		in.Peers = append(in.Peers, fixtureRun(fmt.Sprintf("%08x-1111-4000-8000-000000000000", i+1), seed*1000+int64(i)+1, typical))
	}
	return in
}

// visit walks every block, including the ones inside columns.
func visit(blocks []Block, inherited string, fn func(b Block, only string)) {
	for _, b := range blocks {
		only := b.Only
		if only == "" {
			only = inherited
		}
		fn(b, only)
		visit(b.Left, only, fn)
		visit(b.Right, only, fn)
	}
}

// prose collects the sentences of one copy: paragraphs, bullets,
// findings, notes and captions. Tiles, tables and chart payloads are
// left out; they are figures and labels rather than sentences.
func prose(doc *Document, copyName string) []string {
	var out []string
	visit(doc.Blocks, "", func(b Block, only string) {
		if only != "" && only != copyName {
			return
		}
		switch b.Type {
		case "p", "caption", "note", "h2", "h3", "title":
			out = append(out, b.Text)
		case "bullets", "numbered":
			out = append(out, b.Items...)
		}
	})
	return out
}

func charts(doc *Document, copyName string) map[string]bool {
	out := map[string]bool{}
	visit(doc.Blocks, "", func(b Block, only string) {
		if b.Chart != nil && (only == "" || only == copyName) {
			out[b.Chart.Kind] = true
		}
	})
	return out
}

func TestStatistics(t *testing.T) {
	// A made-up group of sixteen in which twelve places fifth: four
	// above, eleven below, nobody level, the 72nd percentile.
	others := []float64{27, 15, 13, 13, 11, 10, 10, 9, 9, 8, 8, 7, 6, 5, 2}
	if p := place(12, others, true); p.Rank != 5 || p.Tied != 0 || p.Of != 16 {
		t.Fatalf("place: %+v", p)
	}
	if pct := percentileRank(12, others); math.Abs(pct-71.875) > 1e-9 {
		t.Fatalf("percentile: %v", pct)
	}
	// Ties share the better place and count half each in the percentile.
	if p := place(13, others, true); p.Rank != 3 || p.Tied != 2 {
		t.Fatalf("tied place: %+v", p)
	}
	if pct := percentileRank(13, others); math.Abs(pct-(11+1.5)/16*100) > 1e-9 {
		t.Fatalf("tied percentile: %v", pct)
	}
	if m := median([]float64{3, 1, nan, 2, 10}); m != 2.5 {
		t.Fatalf("median skips what does not exist: %v", m)
	}
	if !isNaN(median(nil)) || !isNaN(mean([]float64{nan})) {
		t.Fatal("an empty group has no median and no mean")
	}
	// A figure that does not exist for somebody is not ranked.
	if p := place(50, []float64{40, nan, 60}, false); p.Rank != 2 || p.Of != 3 {
		t.Fatalf("place among those who have the figure: %+v", p)
	}
}

func TestNiceAxis(t *testing.T) {
	lo, hi, ticks := niceAxis([]float64{3, 24, 12}, 0, 30, true)
	if lo != 0 || hi != 30 || fmt.Sprint(ticks) != "[0 10 20 30]" {
		t.Fatalf("score axis: %v %v %v", lo, hi, ticks)
	}
	lo, hi, ticks = niceAxis([]float64{-17, 66, 8}, nan, nan, false)
	if lo != -25 || hi != 75 || len(ticks) != 5 {
		t.Fatalf("gap axis: %v %v %v", lo, hi, ticks)
	}
	// A count never gets a tick at two and a half.
	_, _, ticks = niceAxis([]float64{0, 9, 3}, 0, nan, true)
	for _, tk := range ticks {
		if tk != math.Trunc(tk) {
			t.Fatalf("fractional tick on a count: %v", ticks)
		}
	}
	// Everybody on one value still gets an axis.
	if lo, hi, _ = niceAxis([]float64{5, 5, 5}, nan, nan, true); hi <= lo {
		t.Fatalf("degenerate axis: %v %v", lo, hi)
	}
}

func TestWords(t *testing.T) {
	cases := map[int]string{0: "zero", 1: "one", 13: "thirteen", 20: "twenty", 21: "twenty-one", 30: "thirty", 99: "ninety-nine", 100: "100"}
	for n, want := range cases {
		if got := word(n); got != want {
			t.Errorf("word(%d) = %q, want %q", n, got, want)
		}
	}
	for n, want := range map[int]string{1: "1st", 2: "2nd", 3: "3rd", 4: "4th", 11: "11th", 12: "12th", 13: "13th", 21: "21st", 72: "72nd", 113: "113th"} {
		if got := ordinal(n); got != want {
			t.Errorf("ordinal(%d) = %q, want %q", n, got, want)
		}
	}
	if signed(0.2) != "0" || signed(7.9) != "+8" || signed(-3.4) != "-3" {
		t.Error("signed")
	}
	if num(10) != "10" || num(10.5) != "10.5" {
		t.Error("num")
	}
	if got := esc(`A < B & "C"`); got != `A &lt; B &amp; "C"` {
		t.Errorf("esc: %q", got)
	}
	if joinAnd([]string{"a", "b", "c"}) != "a, b and c" || joinAnd([]string{"a", "b"}) != "a and b" {
		t.Error("joinAnd")
	}
}

func TestComparedReport(t *testing.T) {
	in := fixtureInput(7, 14)
	doc, f := Build(in)
	if !f.Compared || f.Others != 14 || f.N != 15 || f.Reason != "" {
		t.Fatalf("expected a ranking against 14: %+v", f.Summary())
	}
	if doc.Schema != SchemaVersion || doc.Template != TemplateVersion {
		t.Fatalf("contract versions: %d %q", doc.Schema, doc.Template)
	}
	got := charts(doc, OnlyParticipant)
	for _, want := range []string{"standing", "blocks", "confidence_use", "calibration", "scatter", "families", "time_by_outcome"} {
		if !got[want] {
			t.Errorf("participant's copy is missing the %s chart", want)
		}
	}
	if got["timeline"] {
		t.Error("the per-question timeline is in the participant's copy")
	}
	if !charts(doc, OnlyOwner)["timeline"] || charts(doc, OnlyOwner)["time_by_outcome"] {
		t.Error("the owner's copy should show the timeline in place of the grouped times")
	}
	s := f.Summary()
	if s.Rank < 1 || s.Rank > 15 || s.GroupSize != 15 || s.Percentile <= 0 || s.Percentile >= 100 {
		t.Fatalf("summary: %+v", s)
	}
	if !strings.Contains(doc.Footer, "run aaaaaaaa") || !strings.Contains(doc.Footer, "20 November 2026") {
		t.Fatalf("footer: %q", doc.Footer)
	}
}

func TestWhoIsComparable(t *testing.T) {
	in := fixtureInput(3, 12)
	two := 2
	// A repeat, a run set aside on review, a run with no placeable
	// sitting, an agent's run and a run closed early are all left out.
	in.Peers[0].AttemptNo = &two
	in.Peers[1].Usable = false
	in.Peers[2].AttemptNo = nil
	in.Peers[3].Synthetic = true
	in.Peers[4].Answers = in.Peers[4].Answers[:24]
	f := Compute(in)
	if f.Others != 7 || f.Compared {
		t.Fatalf("7 comparable of 12 is under the minimum of 10: others=%d compared=%v", f.Others, f.Compared)
	}
	if f.Reason != ReasonSmallGroup {
		t.Fatalf("reason: %q", f.Reason)
	}
	if f.RepeatSittings != 1 {
		t.Fatalf("repeat sittings counted: %d", f.RepeatSittings)
	}
	in.MinPeers = 7
	if f = Compute(in); !f.Compared || f.N != 8 {
		t.Fatalf("at the minimum the report ranks: %+v", f.Summary())
	}
	// The run the report is about is never its own peer.
	in.Peers = append(in.Peers, in.Run)
	if g := Compute(in); g.Others != f.Others {
		t.Fatalf("the run was counted among its own peers: %d", g.Others)
	}
}

func TestNoRanking(t *testing.T) {
	two := 2
	cases := []struct {
		name   string
		mutate func(*Input)
		reason string
		says   string
	}{
		{"small group", func(in *Input) { in.Peers = in.Peers[:4] }, ReasonSmallGroup, "A ranking needs at least ten other people"},
		{"nobody yet", func(in *Input) { in.Peers = nil }, ReasonSmallGroup, "So far there are none."},
		{"repeat", func(in *Input) { in.Run.AttemptNo = &two }, ReasonRepeat, "This was your 2nd sitting."},
		{"unplaced", func(in *Input) { in.Run.AttemptNo = nil }, ReasonUnplaced, "could not tell whether this was a first sitting"},
		{"synthetic", func(in *Input) { in.Run.Synthetic = true }, ReasonSynthetic, "made by software to test the system"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := fixtureInput(11, 14)
			c.mutate(&in)
			doc, f := Build(in)
			if f.Compared || f.Reason != c.reason {
				t.Fatalf("compared=%v reason=%q", f.Compared, f.Reason)
			}
			if s := f.Summary(); s.Rank != 0 || s.Percentile != 0 || s.GroupSize != 0 {
				t.Fatalf("a report without a ranking stored one: %+v", s)
			}
			for _, kind := range []string{"standing", "scatter"} {
				if charts(doc, OnlyParticipant)[kind] {
					t.Errorf("the %s chart needs a group", kind)
				}
			}
			all := strings.Join(prose(doc, OnlyParticipant), "\n")
			if !strings.Contains(all, c.says) {
				t.Errorf("the report does not say why: want %q", c.says)
			}
			for _, banned := range []string{"percentile", "the others", "their median", "Standing"} {
				if strings.Contains(all, banned) {
					t.Errorf("a report with no group mentions %q", banned)
				}
			}
			for _, tile := range tilesOf(doc) {
				if tile.Label == "Standing" {
					t.Error("a standing tile with no ranking")
				}
			}
		})
	}
}

func tilesOf(doc *Document) []Tile {
	var out []Tile
	visit(doc.Blocks, "", func(b Block, _ string) { out = append(out, b.Tiles...) })
	return out
}

// The participant's copy must not carry any part of the answer key
// (FSD FR-DT-04): no question, no option, no item code, and nothing
// that says which numbered answer was right.
func TestParticipantCopyCarriesNoKey(t *testing.T) {
	for _, peers := range []int{0, 14} {
		in := fixtureInput(5, peers)
		doc, _ := Build(in)
		var participant, owner []Block
		visit(doc.Blocks, "", func(b Block, only string) {
			flat := b
			flat.Left, flat.Right = nil, nil
			if only == OnlyOwner {
				owner = append(owner, flat)
			} else {
				participant = append(participant, flat)
			}
		})
		raw, err := json.Marshal(participant)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range in.Run.Answers {
			for _, secret := range []string{a.Prompt, a.ChosenText, a.CorrectText} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("peers=%d: the participant's copy contains %q", peers, secret)
				}
			}
		}
		if strings.Contains(string(raw), "PROMPT-") || strings.Contains(string(raw), "CORRECT-") {
			t.Fatalf("peers=%d: question text reached the participant's copy", peers)
		}
		// Item codes are two-digit here (A07); none may appear.
		if m := regexp.MustCompile(`"[ABCS]\d\d[<"]`).FindString(string(raw)); m != "" {
			t.Fatalf("peers=%d: an item code reached the participant's copy: %s", peers, m)
		}
		// Nothing shared between the copies lists answers in the order
		// asked: a chart or table with a position column says which was
		// right.
		if strings.Contains(string(raw), `"position"`) {
			t.Fatalf("peers=%d: per-question positions reached the participant's copy", peers)
		}
		ownerRaw, _ := json.Marshal(owner)
		if !strings.Contains(string(ownerRaw), in.Run.Answers[0].Prompt) || !strings.Contains(string(ownerRaw), in.Run.Answers[0].CorrectText) {
			t.Fatalf("peers=%d: the owner's copy is missing the questions and their answers", peers)
		}
	}
}

var (
	tagRE        = regexp.MustCompile(`</?b>|<br/>`)
	digitStartRE = regexp.MustCompile(`(^|[.!?] )\d`)
	badPluralRE  = regexp.MustCompile(`(?i)\b(1|one) (questions|answers|points|people|others|sittings|blocks|digits|times)\b`)
	zeroOfRE     = regexp.MustCompile(`\b0 of 0\b`)
)

// Every sentence the builder can write, checked over a spread of runs
// that includes the awkward ones: nothing rated, everything right,
// everything timed out, a group of one hundred and forty.
func TestSentencesAreWellFormed(t *testing.T) {
	shapes := []shape{
		typical,
		{pCorrect: 1, pRated: 1, levels: []int{90}},
		{pCorrect: 0, pLure: 1, pRated: 1, levels: []int{90}},
		{pExpired: 1},
		{pCorrect: 0.3, pLure: 0.3, pRated: 0, levels: nil},
		{pCorrect: 0.5, pLure: 0.2, pExpired: 0.3, pRated: 0.5, levels: []int{10}},
		{pCorrect: 0.4, pLure: 0.4, pRated: 1, levels: []int{50}, slow: true},
	}
	readings := []string{"no_decline", "effect", "overcorrected", "calibrated"}
	two := 2
	n := 0
	for seed := int64(1); seed <= 60; seed++ {
		for si, sh := range shapes {
			for _, peers := range []int{0, 3, 10, 25, 140} {
				if peers == 140 && seed > 3 {
					continue
				}
				in := fixtureInput(seed, peers)
				in.Run = fixtureRun(in.Run.Key, seed*31+int64(si), sh)
				in.Email.Reading = readings[(int(seed)+si)%4]
				in.Email.FatigueHeld = seed%3 != 0
				in.Email.HardConfidence = []int{58, 20, 80}[seed%3]
				if seed%7 == 0 {
					in.Run.AttemptNo = &two
				}
				if seed%5 == 0 && peers > 0 {
					// A group taken partly under an earlier form.
					for i := range in.Peers {
						if i%2 == 0 {
							in.Peers[i].InstrumentVersion = "v2-m6000-q25000-r20000"
						}
					}
				}
				doc, f := Build(in)
				n++
				raw, err := json.Marshal(doc)
				if err != nil {
					t.Fatalf("seed %d shape %d peers %d: the document does not encode: %v", seed, si, peers, err)
				}
				for _, bad := range []string{"—", "–", "NaN", "%!", "+Inf", "<nil>"} {
					if strings.Contains(string(raw), bad) {
						t.Fatalf("seed %d shape %d peers %d: the document contains %q", seed, si, peers, bad)
					}
				}
				for _, copyName := range []string{OnlyParticipant, OnlyOwner} {
					for _, s := range prose(doc, copyName) {
						plain := tagRE.ReplaceAllString(s, "")
						where := fmt.Sprintf("seed %d shape %d peers %d (%s, compared=%v)", seed, si, peers, copyName, f.Compared)
						if strings.Count(s, "<b>") != strings.Count(s, "</b>") {
							t.Fatalf("%s: unbalanced bold: %q", where, s)
						}
						if strings.TrimSpace(plain) == "" {
							t.Fatalf("%s: an empty sentence", where)
						}
						if strings.Contains(plain, "  ") && !strings.Contains(plain, "  ·  ") {
							t.Fatalf("%s: a double space: %q", where, plain)
						}
						if f.Others < 100 && digitStartRE.MatchString(plain) {
							t.Fatalf("%s: a sentence opens with a digit: %q", where, plain)
						}
						if m := badPluralRE.FindString(plain); m != "" {
							t.Fatalf("%s: %q in %q", where, m, plain)
						}
						if zeroOfRE.MatchString(plain) {
							t.Fatalf("%s: a share of nothing: %q", where, plain)
						}
						if strings.Contains(plain, " .") || strings.Contains(plain, "..") || strings.Contains(plain, " ,") {
							t.Fatalf("%s: stray punctuation: %q", where, plain)
						}
					}
				}
			}
		}
	}
	if n < 1000 {
		t.Fatalf("only %d reports were checked", n)
	}
}

// The report is attached to the results email, so where they speak of
// the same thing they must say the same thing.
func TestAgreesWithTheEmail(t *testing.T) {
	text := func(in Input) string {
		doc, _ := Build(in)
		return strings.Join(prose(doc, OnlyParticipant), "\n")
	}
	in := fixtureInput(2, 12)
	in.Email.FatigueHeld = true
	if s := text(in); !strings.Contains(s, "No sign of tiring.") || strings.Contains(s, "The last block fell away.") {
		t.Error("the email says the last block held and the report does not")
	}
	in.Email.FatigueHeld = false
	if s := text(in); !strings.Contains(s, "The last block fell away.") || strings.Contains(s, "No sign of tiring.") {
		t.Error("the email says the last block fell away and the report does not")
	}
	for reading, want := range map[string]string{
		"no_decline":    "Your accuracy did not fall: 42% in blocks 1 and 2 and 33% in block 4",
		"effect":        "Yours followed it: accuracy fell 9 points",
		"overcorrected": "It fell further than your accuracy did.",
		"calibrated":    "It tracked your accuracy closely.",
	} {
		in.Email.Reading = reading
		if s := text(in); !strings.Contains(s, want) {
			t.Errorf("reading %s: the report does not say %q", reading, want)
		}
	}
	// The same words the email uses for a movement.
	if movement("Your confidence", 6) != "Your confidence fell 6 points" || movement("Your confidence", -1) != "Your confidence rose 1 point" {
		t.Error("movement wording has drifted from the email's")
	}
}

func TestVersionsOfTheTest(t *testing.T) {
	in := fixtureInput(4, 12)
	for i := range in.Peers[:7] {
		in.Peers[i].InstrumentVersion = "v2-m6000-q25000-r20000"
	}
	doc, f := Build(in)
	if f.OtherForm != 7 || !f.OtherFormEarlier || f.SameFormN != 6 {
		t.Fatalf("forms: other=%d earlier=%v same=%d", f.OtherForm, f.OtherFormEarlier, f.SameFormN)
	}
	all := strings.Join(prose(doc, OnlyParticipant), "\n")
	for _, want := range []string{
		"Seven of the others took an earlier form of it",
		"Among the 6 who took the same form as you",
		"7 took version 2 and 5 version 4. Yours was version 4.",
		"Version 4 shows a countdown",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q", want)
		}
	}
	// Seen from the other side: an earlier run among later ones.
	in.Run.InstrumentVersion = "v2-m6000-q25000-r20000"
	doc, f = Build(in)
	if f.OtherFormEarlier || f.OtherForm != 5 {
		t.Fatalf("an earlier run should see the later form as later: %+v %d", f.OtherFormEarlier, f.OtherForm)
	}
	all = strings.Join(prose(doc, OnlyParticipant), "\n")
	if !strings.Contains(all, "Five of the others took a later form of it") {
		t.Error("the later form is not described as later")
	}
	if strings.Contains(strings.Join(prose(doc, OnlyParticipant), "\n"), "countdown shown") || strings.Contains(metaOf(doc), "countdown") {
		t.Error("a run from before the countdown existed is described as having one")
	}
	// Different timings are a different instrument and never reach here
	// as peers; a generation nobody has placed stands alone.
	if eraOf(2) == eraOf(3) || eraOf(3) != eraOf(4) || eraOf(9) == eraOf(4) || KnowsGeneration(9) {
		t.Error("eras")
	}
}

// The renderer iterates what the document calls a list. A Go slice that
// was never appended to encodes as null, and the first real run with no
// trap answers stopped the renderer on exactly that: found by the first
// local run, not by a test, because the generated fixtures always had
// at least one of everything. So every null in every document is now
// accounted for: a figure that does not exist may be null, a list may
// not.
func TestNoListReachesTheRendererAsNull(t *testing.T) {
	optional := map[string]bool{
		"conf": true, "others_correct": true, "others_conf": true, "acc": true, "others_share": true,
		"others_acc": true, "seconds": true, "others_typical": true,
	}
	var walk func(path string, v any, t *testing.T)
	walk = func(path string, v any, t *testing.T) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if child == nil && !optional[k] {
					t.Errorf("%s.%s is null", path, k)
				}
				walk(path+"."+k, child, t)
			}
		case []any:
			for i, child := range x {
				walk(fmt.Sprintf("%s[%d]", path, i), child, t)
			}
		}
	}
	shapes := map[string]shape{
		"typical":         typical,
		"all correct":     {pCorrect: 1, pRated: 1, levels: []int{90}},
		"all trapped":     {pLure: 1, pRated: 1, levels: []int{70}},
		"all out of time": {pExpired: 1},
		"nothing rated":   {pCorrect: 0.4, pLure: 0.3},
	}
	for name, sh := range shapes {
		for _, peers := range []int{0, 12} {
			in := fixtureInput(9, peers)
			in.Run = fixtureRun(in.Run.Key, 9, sh)
			if name == "all out of time" {
				// And nothing given back either: the emptiest run there is.
				in.Run.Recalls = nil
			}
			for i := range in.Peers {
				if i%3 == 0 {
					in.Peers[i] = fixtureRun(in.Peers[i].Key, int64(i), sh)
				}
			}
			doc, _ := Build(in)
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			var tree any
			if err := json.Unmarshal(raw, &tree); err != nil {
				t.Fatal(err)
			}
			t.Run(fmt.Sprintf("%s, %d peers", name, peers), func(t *testing.T) { walk("doc", tree, t) })
		}
	}
}

func metaOf(doc *Document) string {
	for _, b := range doc.Blocks {
		if b.Type == "meta" {
			return b.Text
		}
	}
	return ""
}

func TestRunAccessors(t *testing.T) {
	// An obviously made-up session id. A realistic-looking one beside the
	// word "key" reads as a credential to the secret scanner, and it is
	// right to be suspicious.
	r := Run{Key: "aaaaaaaa-1111-4000-8000-000000000000", InstrumentVersion: "v4-m6000-q25000-r20000"}
	if r.ShortID() != "aaaaaaaa" || r.Generation() != 4 || r.Timings() != "m6000-q25000-r20000" {
		t.Fatalf("%q %d %q", r.ShortID(), r.Generation(), r.Timings())
	}
	if m, q, rc := r.limits(); m != 6 || q != 25 || rc != 20 {
		t.Fatalf("limits %v %v %v", m, q, rc)
	}
	if (Run{InstrumentVersion: "legacy"}).Generation() != 0 || (Run{InstrumentVersion: "legacy"}).Timings() != "" {
		t.Fatal("a version without a generation")
	}
}

func TestRecallWords(t *testing.T) {
	if !rightDigitsWrongOrder(Recall{Expected: "3355", Response: "5533", Outcome: "wrong_digits"}) {
		t.Error("5533 for 3355 is the right digits in the wrong order")
	}
	if rightDigitsWrongOrder(Recall{Expected: "2476", Response: "0582", Outcome: "wrong_digits"}) {
		t.Error("0582 for 2476 is not")
	}
	if rightDigitsWrongOrder(Recall{Expected: "088", Response: "088", Outcome: "exact"}) {
		t.Error("an exact recall is not a wrong order")
	}
	if got := recallResult(Recall{Expected: "7478", Response: "7482", Outcome: "partial", DigitsHeld: 2}); got != "Partly right, 2 of 4 digits in place" {
		t.Error(got)
	}
}
