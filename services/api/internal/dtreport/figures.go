package dtreport

import (
	"math"
	"sort"
	"time"
)

// The five ratings a participant can give, and their words.
var confidenceLevels = []int{10, 30, 50, 70, 90}

var confidenceWords = map[int]string{
	10: "Guessing", 30: "Doubtful", 50: "Unsure", 70: "Fairly sure", 90: "Certain",
}

// The order the kinds of question are shown in, most numerous first. A
// family the bank gains later is appended rather than dropped.
var familyOrder = []string{"syllogism", "arithmetic", "conjunction", "base_rate"}

// person is one sitting reduced to the figures the report compares.
type person struct {
	key        string
	generation int
	started    time.Time

	total, correct, lure, other, expired int
	rated                                int

	accuracy  float64 // percent of all questions asked
	meanConf  float64 // over rated answers; NaN when none
	gap       float64 // meanConf minus accuracy
	confRight float64 // average confidence on answers that were right
	confWrong float64 // and on ones that were not
	disc      float64 // the difference between those two

	wrongSure   int // wrong, and rated at or above the threshold
	usedCertain int // ratings at the top of the scale
	lat         float64
	brier       float64

	blockCorrect [6]int     // index 1 to 5
	blockAsked   [6]int     //
	blockConf    [6]float64 // NaN when the block has no rating
	// A trap answer given while sure, in block 1, the lightest load.
	luredSureBlock1 bool

	recallExact [6]bool
	recallKnown [6]bool
}

// summarise reduces a run. threshold is the confidence at or above
// which an answer counts as sure.
func summarise(r Run, threshold int) person {
	p := person{key: r.Key, generation: r.Generation(), started: r.Started}
	var conf, confRight, confWrong, lat, brier []float64
	blockConf := map[int][]float64{}
	top := confidenceLevels[len(confidenceLevels)-1]
	for _, a := range r.Answers {
		p.total++
		if a.BlockNo >= 1 && a.BlockNo <= 5 {
			p.blockAsked[a.BlockNo]++
		}
		switch a.Outcome {
		case "correct":
			p.correct++
			if a.BlockNo >= 1 && a.BlockNo <= 5 {
				p.blockCorrect[a.BlockNo]++
			}
		case "lure":
			p.lure++
		case "expired":
			p.expired++
		default:
			p.other++
		}
		if !a.expired() {
			lat = append(lat, float64(a.LatencyMs)/1000)
		}
		if !a.rated() {
			continue
		}
		p.rated++
		c := float64(a.Confidence)
		conf = append(conf, c)
		blockConf[a.BlockNo] = append(blockConf[a.BlockNo], c)
		hit := 0.0
		if a.correct() {
			hit = 1
			confRight = append(confRight, c)
		} else {
			confWrong = append(confWrong, c)
		}
		// Rounded to four places per answer, as v_dt_answers.brier is, so
		// the mean here and the mean of that column are the same number.
		brier = append(brier, math.Round(math.Pow(c/100-hit, 2)*1e4)/1e4)
		if a.Confidence >= threshold && !a.correct() && !a.expired() {
			p.wrongSure++
		}
		if a.Confidence >= threshold && a.lure() && a.BlockNo == 1 {
			p.luredSureBlock1 = true
		}
		if a.Confidence >= top {
			p.usedCertain++
		}
	}
	if p.total > 0 {
		p.accuracy = float64(p.correct) / float64(p.total) * 100
	}
	p.meanConf = mean(conf)
	p.gap = p.meanConf - p.accuracy
	p.confRight, p.confWrong = mean(confRight), mean(confWrong)
	p.disc = p.confRight - p.confWrong
	p.lat = mean(lat)
	p.brier = mean(brier)
	for b := 1; b <= 5; b++ {
		p.blockConf[b] = mean(blockConf[b])
	}
	for _, rc := range r.Recalls {
		if rc.BlockNo >= 1 && rc.BlockNo <= 5 {
			p.recallKnown[rc.BlockNo] = true
			p.recallExact[rc.BlockNo] = rc.Outcome == "exact"
		}
	}
	return p
}

// blockAccuracy is the share of a block's questions answered correctly.
func (p person) blockAccuracy(b int) float64 {
	if p.blockAsked[b] == 0 {
		return nan
	}
	return float64(p.blockCorrect[b]) / float64(p.blockAsked[b]) * 100
}

// BlockFigures is one block of the participant's run.
type BlockFigures struct {
	No      int
	Load    string
	Asked   int
	Correct int
	Lure    int
	Other   int
	Expired int
	Conf    float64 // NaN when nothing in the block was rated
	// Averages across the others, one person one vote.
	OthersCorrect float64
	OthersConf    float64
}

// FamilyFigures is one kind of question.
type FamilyFigures struct {
	Family  string
	Asked   int
	Correct int
	Lure    int
	Other   int
	Expired int
	Acc     float64
	Conf    float64
	Lat     float64
	// Every answer the others gave to this kind, pooled.
	OthersAcc float64
}

// LevelFigures is one of the five ratings.
type LevelFigures struct {
	Level       int
	N           int
	Right       int
	Share       float64 // percent of the participant's rated answers
	Acc         float64 // NaN when the rating was never used
	OthersShare float64
	OthersAcc   float64
}

// MemoryFigures is one block's number, as given back.
type MemoryFigures struct {
	Recall
	OthersExact int
	OthersN     int
}

// QuestionFigures is one answer, with how the others did on the same
// question among those who were given it.
type QuestionFigures struct {
	Answer
	OthersN     int
	OthersRight int
}

// Figures is every number the report states.
type Figures struct {
	// Whether the report ranks the participant, and why not when it
	// does not.
	Compared bool
	Reason   string
	// The comparison group: N includes the participant, Others does not.
	// Others is filled even when the group is too small to rank against,
	// because the report then says how many there are.
	N      int
	Others int

	me     person
	others []person

	QuestionLimitS float64
	MemoriseS      float64
	RecallS        float64
	Minutes        int
	Guess          float64
	Threshold      int

	// Score.
	Score             int
	Total             int
	ScorePlace        placing
	ScoreAbove        int
	ScoreBelow        int
	Percentile        float64
	OthersScoreMedian float64
	OthersScoreMean   float64
	OthersScoreMin    int
	OthersScoreMax    int
	PlaceIfOneMore    placing
	PlaceIfOneFewer   placing

	// Other ways of drawing the group, for the note on who was left out.
	FirstFrom, FirstTo time.Time
	RepeatSittings     int
	AllSittings        int
	AllRank            int
	OtherForm          int // others who sat a different form of the test
	OtherFormEra       int // which form those were, when they agree
	OtherFormEarlier   bool
	SameFormN          int // the participant and those who sat the same form
	SameFormRank       int
	Generations        map[int]int // the others, by generation

	// Confidence.
	Rated               int
	UnratedAnswered     int
	OthersConfMedian    float64
	ConfLow             placing
	ConfHigh            placing
	OthersGapMedian     float64
	GapClosest          placing
	OthersOverconfident int // more than ten points more sure than right
	OthersBelowDiagonal int // more sure than right at all
	OthersWrongSure     struct{ Median, Max float64 }
	WrongSureFewest     placing
	WrongSureMost       placing
	WrongAtLevel        map[int]int
	OthersUsedCertain   int
	OthersCertainMost   bool // Certain was the others' most used rating
	OthersDiscMedian    float64
	OthersLuredBlock1   int
	OthersBrierMedian   float64
	BrierBest           placing

	// Time.
	OthersLatMedian    float64
	LatSlow            placing
	LatFast            placing
	OthersExpiredMed   float64
	ExpiredFewest      placing
	ExpiredMost        placing
	QuickS             float64
	LongS              float64
	Answered           int
	QuickN             int
	QuickCorrect       int
	QuickLure          int
	QuickFamilies      []string
	ConsideredN        int
	ConsideredCorrect  int
	LongN              int
	TimesByOutcome     map[string][]float64
	OthersLatByOutcome map[string]float64

	// Blocks.
	Blocks             []BlockFigures
	AccChange          float64 // block 4 minus block 1, in points
	ConfChange         float64
	OthersAccFell      int
	OthersAccHeld      int
	OthersAccRose      int
	OthersConfChange   float64 // median
	OthersConfFell10   int
	OthersConfChangeN  int
	LightCorrect       int // blocks 1 and 2
	LightAsked         int
	LoadedCorrect      int // blocks 3 and 4
	LoadedAsked        int
	OthersLightCorrect float64
	OthersLoadedCorr   float64
	LoadedRecallExact  int

	Families []FamilyFigures
	Levels   []LevelFigures
	Memory   []MemoryFigures

	// The draw.
	PoolSeen     int
	SameDraw     bool    // everybody in the group was asked the same questions
	DrawExpected float64 // what the others' success rates on these questions add up to
	DrawSeenMin  int
	DrawSeenMax  int
	Questions    []QuestionFigures
}

// comparable says whether a peer belongs in the comparison group: a
// first sitting, not set aside on review, with every question recorded.
func comparable(r Run, total int) bool {
	return !r.Synthetic && r.Usable && r.AttemptNo != nil && *r.AttemptNo == 1 && len(r.Answers) == total
}

// Compute works out every figure.
func Compute(in Input) *Figures {
	run := in.Run
	threshold := in.ConfidentThreshold
	if threshold <= 0 {
		threshold = 70
	}
	f := &Figures{Threshold: threshold, Total: len(run.Answers)}
	f.MemoriseS, f.QuestionLimitS, f.RecallS = run.limits()
	if f.QuestionLimitS <= 0 {
		f.QuestionLimitS = 25
	}
	f.Minutes = whole(run.Finished.Sub(run.Started).Minutes())
	f.me = summarise(run, threshold)
	f.Score = f.me.correct
	f.Rated = f.me.rated
	f.Answered = f.me.total - f.me.expired

	// Who the participant is compared with.
	var cohort []Run
	for _, p := range in.Peers {
		if p.Key == run.Key {
			continue
		}
		if comparable(p, f.Total) {
			cohort = append(cohort, p)
		}
		if !p.Synthetic {
			f.AllSittings++
			if p.AttemptNo != nil && *p.AttemptNo > 1 {
				f.RepeatSittings++
			}
		}
	}
	sort.Slice(cohort, func(i, j int) bool { return cohort[i].Key < cohort[j].Key })
	f.Others = len(cohort)
	f.N = f.Others + 1
	if !run.Synthetic {
		f.AllSittings++
	}
	switch {
	case run.Synthetic:
		f.Reason = ReasonSynthetic
	case run.AttemptNo == nil:
		f.Reason = ReasonUnplaced
	case *run.AttemptNo > 1:
		f.Reason = ReasonRepeat
	case f.Others < in.MinPeers:
		f.Reason = ReasonSmallGroup
	default:
		f.Compared = true
	}

	for _, a := range run.Answers {
		if a.Options > 0 {
			f.Guess += 1 / float64(a.Options)
		}
	}

	f.computeOwn(run)
	if f.Compared {
		for _, p := range cohort {
			f.others = append(f.others, summarise(p, threshold))
		}
		f.computeCompared(run, cohort, in.Peers)
	} else {
		// The tables keep their shape without a group; the columns that
		// describe the others are simply not drawn.
		for _, a := range run.Answers {
			f.Questions = append(f.Questions, QuestionFigures{Answer: a})
		}
	}
	return f
}

// computeOwn fills what the participant's own run supports, with or
// without a group to set it against.
func (f *Figures) computeOwn(run Run) {
	me := f.me
	f.UnratedAnswered = f.Answered - f.Rated
	f.WrongAtLevel = map[int]int{}
	for _, a := range run.Answers {
		if a.rated() && !a.correct() && !a.expired() {
			f.WrongAtLevel[a.Confidence]++
		}
	}

	// Blocks.
	loads := map[int]string{}
	type tally struct{ lure, other, expired int }
	t := map[int]*tally{}
	for _, a := range run.Answers {
		loads[a.BlockNo] = a.Load
		if t[a.BlockNo] == nil {
			t[a.BlockNo] = &tally{}
		}
		switch a.Outcome {
		case "lure":
			t[a.BlockNo].lure++
		case "expired":
			t[a.BlockNo].expired++
		case "correct":
		default:
			t[a.BlockNo].other++
		}
	}
	for b := 1; b <= 5; b++ {
		bt := t[b]
		if bt == nil {
			bt = &tally{}
		}
		f.Blocks = append(f.Blocks, BlockFigures{
			No: b, Load: loads[b], Asked: me.blockAsked[b], Correct: me.blockCorrect[b],
			Lure: bt.lure, Other: bt.other, Expired: bt.expired, Conf: me.blockConf[b],
			OthersCorrect: nan, OthersConf: nan,
		})
	}
	f.AccChange = me.blockAccuracy(4) - me.blockAccuracy(1)
	f.ConfChange = me.blockConf[4] - me.blockConf[1]
	f.LightCorrect, f.LightAsked = me.blockCorrect[1]+me.blockCorrect[2], me.blockAsked[1]+me.blockAsked[2]
	f.LoadedCorrect, f.LoadedAsked = me.blockCorrect[3]+me.blockCorrect[4], me.blockAsked[3]+me.blockAsked[4]
	for _, b := range []int{3, 4} {
		if me.recallExact[b] {
			f.LoadedRecallExact++
		}
	}

	// Kinds of question.
	seen := map[string]bool{}
	order := append([]string(nil), familyOrder...)
	for _, k := range familyOrder {
		seen[k] = true
	}
	for _, a := range run.Answers {
		if !seen[a.Family] {
			seen[a.Family] = true
			order = append(order, a.Family)
		}
	}
	for _, fam := range order {
		ff := FamilyFigures{Family: fam, OthersAcc: nan}
		var conf, lat []float64
		for _, a := range run.Answers {
			if a.Family != fam {
				continue
			}
			ff.Asked++
			switch a.Outcome {
			case "correct":
				ff.Correct++
			case "lure":
				ff.Lure++
			case "expired":
				ff.Expired++
			default:
				ff.Other++
			}
			if a.rated() {
				conf = append(conf, float64(a.Confidence))
			}
			if !a.expired() {
				lat = append(lat, float64(a.LatencyMs)/1000)
			}
		}
		if ff.Asked == 0 {
			continue
		}
		ff.Acc = float64(ff.Correct) / float64(ff.Asked) * 100
		ff.Conf, ff.Lat = mean(conf), mean(lat)
		f.Families = append(f.Families, ff)
	}

	// Ratings.
	for _, level := range confidenceLevels {
		lf := LevelFigures{Level: level, Acc: nan, OthersShare: nan, OthersAcc: nan}
		for _, a := range run.Answers {
			if a.Confidence == level {
				lf.N++
				if a.correct() {
					lf.Right++
				}
			}
		}
		if f.Rated > 0 {
			lf.Share = float64(lf.N) / float64(f.Rated) * 100
		}
		if lf.N > 0 {
			lf.Acc = float64(lf.Right) / float64(lf.N) * 100
		}
		f.Levels = append(f.Levels, lf)
	}

	// Time. Quick and long are fixed shares of the limit, so the words
	// keep their meaning if the owner moves it: under two fifths of the
	// clock is an answer given before there was time to check it.
	f.QuickS = math.Round(f.QuestionLimitS * 0.4)
	f.LongS = math.Round(f.QuestionLimitS * 0.8)
	f.TimesByOutcome = map[string][]float64{}
	quickFam := map[string]bool{}
	for _, a := range run.Answers {
		if a.expired() {
			continue
		}
		s := float64(a.LatencyMs) / 1000
		f.TimesByOutcome[a.Outcome] = append(f.TimesByOutcome[a.Outcome], s)
		switch {
		case s < f.QuickS:
			f.QuickN++
			quickFam[a.Family] = true
			if a.correct() {
				f.QuickCorrect++
			}
			if a.lure() {
				f.QuickLure++
			}
		default:
			f.ConsideredN++
			if a.correct() {
				f.ConsideredCorrect++
			}
		}
		if s >= f.LongS {
			f.LongN++
		}
	}
	for _, fam := range order {
		if quickFam[fam] {
			f.QuickFamilies = append(f.QuickFamilies, fam)
		}
	}
	for k := range f.TimesByOutcome {
		sort.Float64s(f.TimesByOutcome[k])
	}

	// The number.
	recalls := append([]Recall(nil), run.Recalls...)
	sort.Slice(recalls, func(i, j int) bool { return recalls[i].BlockNo < recalls[j].BlockNo })
	for _, rc := range recalls {
		f.Memory = append(f.Memory, MemoryFigures{Recall: rc})
	}
}

// computeCompared fills everything that needs the group.
func (f *Figures) computeCompared(run Run, cohort, peers []Run) {
	me, others := f.me, f.others
	col := func(get func(person) float64) []float64 {
		out := make([]float64, len(others))
		for i, p := range others {
			out[i] = get(p)
		}
		return out
	}
	scores := col(func(p person) float64 { return float64(p.correct) })
	mine := float64(me.correct)

	f.ScorePlace = place(mine, scores, true)
	f.ScoreAbove = countWhere(scores, func(v float64) bool { return v > mine })
	f.ScoreBelow = countWhere(scores, func(v float64) bool { return v < mine })
	f.Percentile = percentileRank(mine, scores)
	f.OthersScoreMedian, f.OthersScoreMean = median(scores), mean(scores)
	lo, hi := minMax(scores)
	f.OthersScoreMin, f.OthersScoreMax = whole(lo), whole(hi)
	f.PlaceIfOneMore = place(mine+1, scores, true)
	f.PlaceIfOneFewer = place(mine-1, scores, true)

	f.FirstFrom, f.FirstTo = run.Started, run.Started
	for _, p := range cohort {
		if p.Started.Before(f.FirstFrom) {
			f.FirstFrom = p.Started
		}
		if p.Started.After(f.FirstTo) {
			f.FirstTo = p.Started
		}
	}

	// The same score among every finished sitting, repeats included.
	f.AllRank = 1
	for _, p := range peers {
		if p.Key == run.Key || p.Synthetic {
			continue
		}
		n := 0
		for _, a := range p.Answers {
			if a.correct() {
				n++
			}
		}
		if n > me.correct {
			f.AllRank++
		}
	}

	// And among those who sat the same form of the test.
	f.Generations = map[int]int{}
	myEra := eraOf(me.generation)
	f.SameFormN, f.SameFormRank = 1, 1
	otherEras := map[int]int{}
	for _, p := range others {
		f.Generations[p.generation]++
		if e := eraOf(p.generation); e != myEra {
			f.OtherForm++
			otherEras[e]++
			continue
		}
		f.SameFormN++
		if p.correct > me.correct {
			f.SameFormRank++
		}
	}
	if len(otherEras) == 1 {
		for e := range otherEras {
			f.OtherFormEra, f.OtherFormEarlier = e, e < myEra
		}
	}

	// Confidence.
	conf := col(func(p person) float64 { return p.meanConf })
	f.OthersConfMedian = median(conf)
	f.ConfLow, f.ConfHigh = place(me.meanConf, conf, false), place(me.meanConf, conf, true)
	gaps := col(func(p person) float64 { return p.gap })
	f.OthersGapMedian = median(gaps)
	absGaps := col(func(p person) float64 { return math.Abs(p.gap) })
	f.GapClosest = place(math.Abs(me.gap), absGaps, false)
	f.OthersOverconfident = countWhere(gaps, func(v float64) bool { return v > 10 })
	f.OthersBelowDiagonal = countWhere(gaps, func(v float64) bool { return v > 0 })
	wrongSure := col(func(p person) float64 { return float64(p.wrongSure) })
	f.OthersWrongSure.Median = median(wrongSure)
	_, f.OthersWrongSure.Max = minMax(wrongSure)
	f.WrongSureFewest = place(float64(me.wrongSure), wrongSure, false)
	f.WrongSureMost = place(float64(me.wrongSure), wrongSure, true)
	f.OthersUsedCertain = countWhere(col(func(p person) float64 { return float64(p.usedCertain) }),
		func(v float64) bool { return v > 0 })
	f.OthersDiscMedian = median(col(func(p person) float64 { return p.disc }))
	for _, p := range others {
		if p.luredSureBlock1 {
			f.OthersLuredBlock1++
		}
	}
	briers := col(func(p person) float64 { return p.brier })
	f.OthersBrierMedian = median(briers)
	f.BrierBest = place(me.brier, briers, false)

	// Time.
	lats := col(func(p person) float64 { return p.lat })
	f.OthersLatMedian = median(lats)
	f.LatSlow, f.LatFast = place(me.lat, lats, true), place(me.lat, lats, false)
	expired := col(func(p person) float64 { return float64(p.expired) })
	f.OthersExpiredMed = median(expired)
	f.ExpiredFewest = place(float64(me.expired), expired, false)
	f.ExpiredMost = place(float64(me.expired), expired, true)

	// Blocks: one person one vote, so a single long run cannot stand in
	// for the group.
	for i := range f.Blocks {
		b := f.Blocks[i].No
		f.Blocks[i].OthersCorrect = mean(col(func(p person) float64 { return float64(p.blockCorrect[b]) }))
		f.Blocks[i].OthersConf = mean(col(func(p person) float64 { return p.blockConf[b] }))
	}
	var confChanges []float64
	for _, p := range others {
		d := p.blockAccuracy(4) - p.blockAccuracy(1)
		switch {
		case isNaN(d):
		case d < 0:
			f.OthersAccFell++
		case d > 0:
			f.OthersAccRose++
		default:
			f.OthersAccHeld++
		}
		if c := p.blockConf[4] - p.blockConf[1]; !isNaN(c) {
			confChanges = append(confChanges, c)
			if c <= -10 {
				f.OthersConfFell10++
			}
		}
	}
	f.OthersConfChange, f.OthersConfChangeN = median(confChanges), len(confChanges)
	f.OthersLightCorrect = mean(col(func(p person) float64 { return float64(p.blockCorrect[1] + p.blockCorrect[2]) }))
	f.OthersLoadedCorr = mean(col(func(p person) float64 { return float64(p.blockCorrect[3] + p.blockCorrect[4]) }))

	// Pooled over every answer the others gave: kinds, ratings, and each
	// question the participant was asked.
	type tally struct{ n, right int }
	byFamily, byLevel, byItem := map[string]*tally{}, map[int]*tally{}, map[string]*tally{}
	latByOutcome := map[string][]float64{}
	codes := map[string]bool{}
	ratedOthers := 0
	add := func(m map[string]*tally, k string, right bool) {
		if m[k] == nil {
			m[k] = &tally{}
		}
		m[k].n++
		if right {
			m[k].right++
		}
	}
	for _, p := range cohort {
		for _, a := range p.Answers {
			codes[a.ItemCode] = true
			add(byFamily, a.Family, a.correct())
			add(byItem, a.ItemCode, a.correct())
			if !a.expired() {
				latByOutcome[a.Outcome] = append(latByOutcome[a.Outcome], float64(a.LatencyMs)/1000)
			}
			if a.rated() {
				ratedOthers++
				if byLevel[a.Confidence] == nil {
					byLevel[a.Confidence] = &tally{}
				}
				byLevel[a.Confidence].n++
				if a.correct() {
					byLevel[a.Confidence].right++
				}
			}
		}
	}
	f.OthersLatByOutcome = map[string]float64{}
	for k, v := range latByOutcome {
		f.OthersLatByOutcome[k] = mean(v)
	}
	for i := range f.Families {
		if t := byFamily[f.Families[i].Family]; t != nil && t.n > 0 {
			f.Families[i].OthersAcc = float64(t.right) / float64(t.n) * 100
		}
	}
	most, mostN := 0, -1
	for i := range f.Levels {
		t := byLevel[f.Levels[i].Level]
		if t == nil {
			t = &tally{}
		}
		if ratedOthers > 0 {
			f.Levels[i].OthersShare = float64(t.n) / float64(ratedOthers) * 100
		}
		if t.n > 0 {
			f.Levels[i].OthersAcc = float64(t.right) / float64(t.n) * 100
		}
		if t.n > mostN {
			most, mostN = f.Levels[i].Level, t.n
		}
	}
	f.OthersCertainMost = most == confidenceLevels[len(confidenceLevels)-1]

	// The draw. A question nobody else was given is counted at the
	// others' rate for its kind, so an unseen question neither helps nor
	// hurts the comparison.
	othersCodes := len(codes)
	f.DrawSeenMin, f.DrawSeenMax = -1, 0
	for _, a := range run.Answers {
		codes[a.ItemCode] = true
		q := QuestionFigures{Answer: a}
		if t := byItem[a.ItemCode]; t != nil {
			q.OthersN, q.OthersRight = t.n, t.right
		}
		switch {
		case q.OthersN > 0:
			f.DrawExpected += float64(q.OthersRight) / float64(q.OthersN)
		case byFamily[a.Family] != nil && byFamily[a.Family].n > 0:
			f.DrawExpected += float64(byFamily[a.Family].right) / float64(byFamily[a.Family].n)
		}
		if f.DrawSeenMin < 0 || q.OthersN < f.DrawSeenMin {
			f.DrawSeenMin = q.OthersN
		}
		if q.OthersN > f.DrawSeenMax {
			f.DrawSeenMax = q.OthersN
		}
		f.Questions = append(f.Questions, q)
	}
	f.PoolSeen = len(codes)
	// Everybody was asked the same questions when the group has seen no
	// more of them than one sitting holds.
	f.SameDraw = len(codes) <= f.Total && othersCodes <= f.Total

	// The number, block by block.
	for i := range f.Memory {
		b := f.Memory[i].BlockNo
		for _, p := range others {
			if b >= 1 && b <= 5 && p.recallKnown[b] {
				f.Memory[i].OthersN++
				if p.recallExact[b] {
					f.Memory[i].OthersExact++
				}
			}
		}
	}
}
