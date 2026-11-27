package dtreport

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// The text width the renderer lays out on, in inches. Column widths
// below add up to it.
const textWidth = 7.0

type builder struct {
	in Input
	f  *Figures
}

// Build works out the figures and writes the report.
func Build(in Input) (*Document, *Figures) {
	if in.Now.IsZero() {
		in.Now = time.Now()
	}
	b := &builder{in: in, f: Compute(in)}
	id := in.Run.ShortID()
	doc := &Document{
		Schema:    SchemaVersion,
		Template:  TemplateVersion,
		Title:     "Decision test results, run " + id,
		Subject:   "Individual results",
		Footer:    fmt.Sprintf("Decision test  ·  run %s  ·  prepared %s", id, day(in.Now)),
		OwnerFlag: "OWNER'S COPY  ·  CONTAINS THE ANSWER KEY  ·  NOT FOR PARTICIPANTS",
	}
	if b.f.Compared {
		doc.Subject = "Individual results with comparison to other participants"
	}
	add := func(blocks ...Block) { doc.Blocks = append(doc.Blocks, blocks...) }

	add(b.frontPage()...)
	add(pageBreak())
	add(b.aboutTheTest()...)
	add(pageBreak())
	if b.f.Compared {
		add(b.standing()...)
		add(pageBreak())
	}
	add(b.blockByBlock()...)
	add(pageBreak())
	add(b.confidence()...)
	add(pageBreak())
	add(b.kindsAndTime()...)
	add(pageBreak())
	add(b.thirtyAnswers()...)
	add(b.method()...)
	add(b.appendix()...)
	return doc, b.f
}

func day(t time.Time) string   { return t.UTC().Format("2 January 2006") }
func clock(t time.Time) string { return t.UTC().Format("15:04") }

// ---------------------------------------------------------------- page 1

func (b *builder) frontPage() []Block {
	f, run := b.f, b.in.Run
	title := "Your results"
	if f.Compared {
		title = "Your results, and how they compare"
	}
	out := []Block{
		{Type: "kicker", Text: "DECISION TEST  ·  INDIVIDUAL RESULTS"},
		{Type: "title", Text: title},
		{Type: "meta", Text: b.metaLine()},
		owner(note(bold("Owner's copy.") + " The appendix at the end prints the questions with the answers given and the " +
			"correct answers. The questions are reused for every participant, so this copy is not for the participant or " +
			"anyone who may take the test. A second copy without the appendix was written beside this one.")),
		owner(spacer(10)),
		{Type: "tiles", Tiles: b.tiles()},
		h2("What stands out"),
		numbered(b.findings()...),
		spacer(4),
	}
	closing := "nothing here measures intelligence or how well anyone does their job."
	switch {
	case f.Compared:
		out = append(out, note(fmt.Sprintf("This is one sitting of %s, compared with %d other people. Read the rankings as "+
			"rough positions: a single question either way changes them, and %s", minutes(f.Minutes), f.Others, closing)))
	case run.Synthetic:
		out = append(out, note("This run was made by software to test the system. It is not a person's result, it is not "+
			"ranked against participants, and it is never counted in anyone else's comparison."))
	default:
		out = append(out, note(fmt.Sprintf("This is one sitting of %s. %s", minutes(f.Minutes), capitalise(closing))))
	}
	return out
}

// metaLine is the line under the title: which run, when, and under what
// conditions.
func (b *builder) metaLine() string {
	f, run := b.f, b.in.Run
	parts := []string{
		"Run " + run.ShortID(),
		fmt.Sprintf("taken %s, %s to %s UTC (%s)", day(run.Started), clock(run.Started), clock(run.Finished), minutes(f.Minutes)),
	}
	switch {
	case run.Synthetic:
		parts = append(parts, "software test run")
	case run.AttemptNo == nil:
	case *run.AttemptNo == 1:
		parts = append(parts, "first sitting")
	default:
		parts = append(parts, ordinal(*run.AttemptNo)+" sitting")
	}
	var how []string
	switch run.Device {
	case "phone":
		how = append(how, "on a phone")
	case "tablet":
		how = append(how, "on a tablet")
	case "desktop":
		how = append(how, "on a computer")
	}
	if run.Audio == "visual" {
		how = append(how, "sound off")
	} else {
		how = append(how, "sound on")
	}
	// The countdown is a condition only from the generation that
	// randomised it. Before that nobody saw one, and saying "hidden"
	// would describe a choice that was never made.
	if run.Generation() >= 4 {
		if run.TimerVisible {
			how = append(how, "countdown shown")
		} else {
			how = append(how, "countdown hidden")
		}
	}
	return strings.Join(append(parts, strings.Join(how, ", ")), "  ·  ")
}

func (b *builder) tiles() []Tile {
	f := b.f
	score := Tile{Label: "Score", Value: nOf(f.Score, f.Total), Detail: fmt.Sprintf("%d%% correct.", whole(f.me.accuracy))}
	if f.Guess > 0 {
		score.Detail += fmt.Sprintf(" Guessing alone would average about %d.", whole(f.Guess))
	}
	conf := Tile{Label: "Average confidence", Value: "Not rated", Detail: "No answer carried a confidence rating."}
	gap := Tile{Label: "Confidence minus accuracy", Value: "None", Detail: "It needs at least one rated answer."}
	if f.Rated > 0 {
		conf.Value = fmt.Sprintf("%d of 100", whole(f.me.meanConf))
		conf.Detail = fmt.Sprintf("Across the %d answers you rated.", f.Rated)
		gap.Value = signed(f.me.gap) + " " + plural(whole(math.Abs(f.me.gap)), "point", "points")
		gap.Detail = "Zero would mean your confidence matched your results."
	}
	if !f.Compared {
		pace := Tile{Label: "Time per answer", Value: secs(f.me.lat) + " s",
			Detail: fmt.Sprintf("The limit was %d seconds. %s.", whole(f.QuestionLimitS), capitalise(b.expiredPhrase()))}
		if f.Answered == 0 {
			pace.Value = "None"
		}
		return []Tile{score, conf, gap, pace}
	}
	standing := Tile{Label: "Standing", Value: fmt.Sprintf("%s of %d", ordinal(f.ScorePlace.Rank), f.N)}
	if f.ScorePlace.Tied > 0 {
		standing.Detail = fmt.Sprintf("Level with %d %s. ", f.ScorePlace.Tied, plural(f.ScorePlace.Tied, "other", "others"))
	}
	standing.Detail += fmt.Sprintf("Above %d of the %d other first-time participants. %s percentile.",
		f.ScoreBelow, f.Others, ordinal(whole(f.Percentile)))
	if f.Rated > 0 {
		conf.Detail = fmt.Sprintf("The others' median was %d. %s.", whole(f.OthersConfMedian), b.confidencePlacing())
		gap.Detail = fmt.Sprintf("The others' median was %s. %s.", signed(f.OthersGapMedian), placingText(f.GapClosest, "closest to zero"))
	}
	return []Tile{score, standing, conf, gap}
}

// confidencePlacing counts from whichever end the participant is nearer.
func (b *builder) confidencePlacing() string {
	f := b.f
	if f.ConfLow.Rank <= f.ConfHigh.Rank {
		return placingText(f.ConfLow, "lowest")
	}
	return placingText(f.ConfHigh, "highest")
}

// ---------------------------------------------------------------- page 2

func (b *builder) aboutTheTest() []Block {
	f := b.f
	var rows [][]Cell
	for _, bl := range f.Blocks {
		w := loadText(bl.Load)
		rows = append(rows, row(fmt.Sprintf("%d", bl.No), w.number, w.giveBack))
	}
	var kinds []string
	for _, fam := range f.Families {
		kinds = append(kinds, familyCount(fam.Family, fam.Asked))
	}
	perBlock := 0
	if len(f.Blocks) > 0 {
		perBlock = f.Blocks[0].Asked
	}
	out := []Block{
		h2("What the test was doing"),
		para(fmt.Sprintf("The test has %s blocks of %s questions. At the start of each block you were shown a number to hold in "+
			"mind while you answered, and asked to give it back at the end. The number got harder to hold as the test went on, "+
			"then eased off:", word(len(f.Blocks)), word(perBlock))),
		{Type: "table", Table: &Table{Style: "data", BoldFirst: true, Rows: rows, Columns: []Column{
			{Title: "Block", Width: 0.7}, {Title: "The number", Width: 1.3}, {Title: "What you had to give back", Width: textWidth - 2.0},
		}}},
		spacer(9),
		para(fmt.Sprintf("Each question had %d seconds, including a rating of how sure you were: Guessing (10), Doubtful (30), "+
			"Unsure (50), Fairly sure (70) or Certain (90). There were %s, mixed through the blocks.",
			whole(f.QuestionLimitS), joinAnd(kinds))),
		para("Every question was built around a well-documented habit: an answer comes to mind quickly and feels right, and " +
			"checking it takes deliberate effort. This report calls that quick wrong answer the " + bold("intuitive trap") + ". " +
			"Holding a number uses the same limited attention that checking does. The test looks at whether the trap works " +
			"more often as the number gets harder to hold, and whether confidence comes down when it does. So each answer is " +
			"one of four things: correct, the intuitive trap, some other wrong answer, or out of time."),
		owner(para("The body of this report describes answers by kind, never by content. The questions and their answers are " +
			"in the appendix of this copy only.")),
		participant(para(bold("Why this report shows no questions.") + " Everyone is asked from the same pool, so printing a " +
			"question or its answer would spoil the test for the people who take it next. Your answers are described here by " +
			"kind, never by content.")),
	}
	if f.Compared {
		return append(out, b.comparedWith()...)
	}
	return append(out, b.notCompared()...)
}

// comparedWith says who the group is, who was left out, and what the
// comparison is worth.
func (b *builder) comparedWith() []Block {
	f := b.f
	from, to := day(f.FirstFrom), day(f.FirstTo)
	if f.FirstFrom.Year() == f.FirstTo.Year() {
		from = f.FirstFrom.UTC().Format("2 January")
	}
	out := []Block{
		h2("Who you are compared with"),
		para(fmt.Sprintf("The comparison group is the %d people, you and %d others, who finished the test for the first time between "+
			"%s and %s, on the same pool of questions and the same time limits. Throughout this report, \"the others\" means those %d.",
			f.N, f.Others, from, to, f.Others)),
	}
	left := bold("Left out:") + " "
	if f.RepeatSittings > 0 {
		left += fmt.Sprintf("%s %s by people who had taken the test before", word(f.RepeatSittings), plural(f.RepeatSittings, "sitting", "sittings"))
	} else {
		left += "sittings by people who had taken the test before"
	}
	left += ", because the questions are easier the second time; runs made by software to test the system; and runs that were " +
		"not finished or were set aside on review."
	items := []string{left}

	if f.SameDraw {
		items = append(items, bold("Everyone saw the same questions.")+" The group was asked the same "+word(f.Total)+
			", so the scores are directly comparable.")
	} else {
		draw := "about as hard as a typical draw"
		switch d := f.DrawExpected - f.OthersScoreMean; {
		case d >= 1:
			draw = "easier than a typical draw"
		case d <= -1:
			draw = "harder than a typical draw"
		}
		s := fmt.Sprintf("%s Each person's are drawn from a larger pool, always in the same mix of kinds; this group has seen %d "+
			"different questions between them. The %s you happened to get were %s: the others' success rates on those same "+
			"questions add up to %s correct, where the others averaged %s on their own draws. You got %d.",
			bold("Not everyone saw the same "+word(f.Total)+" questions."), f.PoolSeen, word(f.Total), draw,
			secs(f.DrawExpected), secs(f.OthersScoreMean), f.Score)
		if f.DrawSeenMax > 0 {
			s += fmt.Sprintf(" Each question had been seen by only %d to %d of the others, which makes this a rough check.",
				f.DrawSeenMin, f.DrawSeenMax)
		}
		items = append(items, s)
	}

	if f.OtherForm > 0 {
		s := bold("The test has changed while this group was taking it.") + " "
		if note, ok := eraNotes[f.OtherFormEra]; ok && f.OtherFormEra != 0 {
			when := "a later"
			if f.OtherFormEarlier {
				when = "an earlier"
			}
			s += fmt.Sprintf("%s of the others took %s form of it, in which %s. ", Word(f.OtherForm), when, note)
		} else {
			s += fmt.Sprintf("%s of the others took a different form of it. ", Word(f.OtherForm))
		}
		s += "Questions and time limits were the same."
		if f.SameFormN >= 5 {
			s += fmt.Sprintf(" Among the %d who took the same form as you, you placed %s.", f.SameFormN, ordinal(f.SameFormRank))
		}
		if f.AllSittings > f.N {
			s += fmt.Sprintf(" Counting all %d finished sittings, repeats included, you placed %s.", f.AllSittings, ordinal(f.AllRank))
		}
		items = append(items, s)
	} else if f.AllSittings > f.N {
		items = append(items, fmt.Sprintf("%s Counting all %d finished sittings, repeats included, you placed %s.",
			bold("The standing barely depends on who is left out."), f.AllSittings, ordinal(f.AllRank)))
	}
	return append(out, bullets(items...))
}

// notCompared says, once and plainly, why the report has no ranking.
func (b *builder) notCompared() []Block {
	f, run := b.f, b.in.Run
	var why string
	switch f.Reason {
	case ReasonRepeat:
		why = fmt.Sprintf("This was your %s sitting. The questions are easier once seen, so a repeat sitting cannot fairly be "+
			"ranked against people taking the test for the first time.", ordinal(*run.AttemptNo))
	case ReasonUnplaced:
		why = "The test could not tell whether this was a first sitting, and a ranking against first-time participants would " +
			"claim that it was."
	case ReasonSynthetic:
		why = "This run was made by software to test the system, so it is not ranked against participants and is never " +
			"counted in anyone else's comparison."
	default:
		so := fmt.Sprintf("So far there %s %s.", plural(f.Others, "is", "are"), word(f.Others))
		if f.Others == 0 {
			so = "So far there are none."
		}
		why = fmt.Sprintf("A ranking needs at least %s other people who have finished the test for the first time on the same "+
			"questions and the same time limits. %s", word(b.in.MinPeers), so)
	}
	return []Block{h2("Why this report has no ranking"), para(why + " This report covers your own results only.")}
}

// ---------------------------------------------------------------- page 3

func (b *builder) standing() []Block {
	f := b.f
	col := func(get func(person) float64) []float64 {
		out := make([]float64, 0, len(f.others))
		for _, p := range f.others {
			if v := get(p); !isNaN(v) {
				out = append(out, v)
			}
		}
		return out
	}
	strip := func(title, unit, line, placing string, mine float64, others []float64, floor, ceil float64, counts bool) standingRow {
		lo, hi, ticks := niceAxis(append(append([]float64(nil), others...), mine), floor, ceil, counts)
		return standingRow{Title: title, Unit: unit, Line: line, Placing: placing, Mine: mine, Others: others,
			Median: median(others), Lo: lo, Hi: hi, Ticks: ticks}
	}
	line := func(mine, med string) string { return fmt.Sprintf("You %s  ·  their median %s", mine, med) }

	scores := col(func(p person) float64 { return float64(p.correct) })
	rows := []standingRow{
		strip("Score", fmt.Sprintf("questions correct, of %d", f.Total),
			line(fmt.Sprintf("%d", f.Score), num(f.OthersScoreMedian)), placingText(f.ScorePlace, "highest"),
			float64(f.Score), scores, 0, float64(f.Total), true),
	}
	if f.Rated > 0 {
		rows = append(rows,
			strip("Average confidence", "0 to 100",
				line(fmt.Sprintf("%d", whole(f.me.meanConf)), fmt.Sprintf("%d", whole(f.OthersConfMedian))), b.confidencePlacing(),
				f.me.meanConf, col(func(p person) float64 { return p.meanConf }), 0, 100, false),
			strip("Confidence minus accuracy", "points; 0 is a perfect match",
				line(signed(f.me.gap), signed(f.OthersGapMedian)), placingText(f.GapClosest, "closest to zero"),
				f.me.gap, col(func(p person) float64 { return p.gap }), nan, nan, false),
			strip("Wrong and sure", "wrong answers rated "+sureWords(f.Threshold),
				line(fmt.Sprintf("%d", f.me.wrongSure), num(f.OthersWrongSure.Median)), placingText(nearer(f.WrongSureFewest, "fewest", f.WrongSureMost, "most")),
				float64(f.me.wrongSure), col(func(p person) float64 { return float64(p.wrongSure) }), 0, nan, true))
	}
	if f.Answered > 0 {
		pace := placingText(f.LatFast, "fastest")
		if f.LatSlow.Rank <= f.LatFast.Rank {
			pace = placingText(f.LatSlow, "slowest")
		}
		rows = append(rows, strip("Time per answer", fmt.Sprintf("average seconds; the limit is %d", whole(f.QuestionLimitS)),
			line(secs(f.me.lat)+" s", secs(f.OthersLatMedian)+" s"), pace,
			f.me.lat, col(func(p person) float64 { return p.lat }), nan, f.QuestionLimitS, false))
	}
	rows = append(rows, strip("Ran out of time", fmt.Sprintf("questions, of %d", f.Total),
		line(fmt.Sprintf("%d", f.me.expired), num(f.OthersExpiredMed)), placingText(nearer(f.ExpiredFewest, "fewest", f.ExpiredMost, "most")),
		float64(f.me.expired), col(func(p person) float64 { return float64(p.expired) }), 0, nan, true))

	out := []Block{
		h2("Where you sit"),
		para(fmt.Sprintf("Each dot is one of the %d first-time participants. You are the blue dot, and the short line marks the "+
			"others' median.", f.N)),
		chartBlock("standing", 0, standingChart{
			Legend: fmt.Sprintf("The %d other first-time participants", f.Others), Rows: rows}),
		caption("Each row has its own scale. Dots at the same value are stacked."),
		h3("How to read the standing"),
		para(b.standingReading()),
	}
	if s := b.profile(); s != "" {
		out = append(out, para(s))
	}
	if f.Rated > 0 && !isNaN(f.me.brier) {
		out = append(out, para(b.brierReading()))
	}
	return out
}

func (b *builder) standingReading() string {
	f := b.f
	higher := "nobody scored higher"
	if f.ScoreAbove > 0 {
		higher = fmt.Sprintf("%d %s scored higher", f.ScoreAbove, plural(f.ScoreAbove, "person", "people"))
	}
	s := fmt.Sprintf("A score of %d placed you %s of %d: %s and %d scored lower", f.Score, rankText(f.ScorePlace), f.N, higher, f.ScoreBelow)
	half := "counting yourself as half a place"
	if f.ScorePlace.Tied > 0 {
		s += fmt.Sprintf(", with %d level with you", f.ScorePlace.Tied)
		half = "counting yourself and anyone level with you as half a place each"
	}
	worth := whole(100 / float64(f.N))
	s += fmt.Sprintf(". As a percentile that is the %s, %s. With %d people, each place is worth about %d percentile %s.",
		ordinal(whole(f.Percentile)), half, f.N, worth, plural(worth, "point", "points"))
	more, fewer := f.Score < f.Total, f.Score > 0
	switch {
	case more && fewer:
		s += fmt.Sprintf(" One more question right would have put you %s; one fewer, %s.", rankText(f.PlaceIfOneMore), rankText(f.PlaceIfOneFewer))
	case more:
		s += fmt.Sprintf(" One more question right would have put you %s.", rankText(f.PlaceIfOneMore))
	case fewer:
		s += fmt.Sprintf(" One fewer right would have put you %s.", rankText(f.PlaceIfOneFewer))
	}
	return s
}

// profile lines the other rows up against the group's medians. The
// closing sentence is kept for the two patterns that have a plain
// reading, and is left off everything in between.
func (b *builder) profile() string {
	f := b.f
	if f.Rated == 0 || f.Answered == 0 {
		return ""
	}
	side := func(d, margin float64, below, level, above string) (string, int) {
		switch {
		case d < -margin:
			return below, -1
		case d > margin:
			return above, 1
		}
		return level, 0
	}
	conf, c := side(f.me.meanConf-f.OthersConfMedian, 5, "less confident", "about as confident", "more confident")
	gap, g := side(math.Abs(f.me.gap)-math.Abs(f.OthersGapMedian), 5, "closer to your real results", "about as close to your real results",
		"further from your real results")
	sure, w := side(float64(f.me.wrongSure)-f.OthersWrongSure.Median, 0.5, "fewer confident mistakes", "as many confident mistakes",
		"more confident mistakes")
	pace, p := side(f.me.lat-f.OthersLatMedian, 1, "faster", "about as fast", "slower")
	out, _ := side(float64(f.me.expired)-f.OthersExpiredMed, 0.5, "fewer questions left unanswered", "as many questions left unanswered",
		"more questions left unanswered")
	s := fmt.Sprintf("Against the others' medians you were %s, %s, made %s, were %s, and had %s.", conf, gap, sure, pace, out)
	switch {
	case c < 0 && g < 0 && w < 0 && p > 0:
		s += " Taken together that is the profile of someone who treated the questions as hard, which they are, when most people did not."
	case c > 0 && g > 0 && w > 0 && p < 0:
		s += " Taken together that is the most common profile on this test, and the one the questions are built to draw out."
	}
	return s
}

func (b *builder) brierReading() string {
	f := b.f
	joint := ""
	if f.BrierBest.Tied > 0 {
		joint = "joint "
	}
	s := fmt.Sprintf("One more summary figure, for completeness: your Brier score was %.2f (%s%s of %d, where 1st is best; the "+
		"others' median was %.2f). It combines being right with being appropriately sure, and lower is better. Rating every "+
		"answer Unsure would score exactly 0.25, ", f.me.brier, joint, ordinal(f.BrierBest.Rank), f.BrierBest.Of, f.OthersBrierMedian)
	switch d := f.me.brier - 0.25; {
	case d < -0.02:
		s += "so your ratings did better than steady caution would have."
	case d > 0.02:
		s += "so steady caution would have scored better than your ratings did."
	default:
		s += "so your ratings did about as well as steady caution would have."
	}
	switch {
	case f.me.brier < f.OthersBrierMedian-0.005:
		s += " Most people's ratings did worse."
	case f.me.brier > f.OthersBrierMedian+0.005:
		s += " Most people's ratings did better."
	}
	return s
}

// ---------------------------------------------------------------- blocks

func (b *builder) blockByBlock() []Block {
	f := b.f
	chart := blocksChart{Compare: f.Compared}
	for _, bl := range f.Blocks {
		chart.Blocks = append(chart.Blocks, blockPoint{
			Label:   fmt.Sprintf("Block %d\n%s", bl.No, loadText(bl.Load).short),
			Correct: bl.Correct, Lure: bl.Lure, Other: bl.Other, Expired: bl.Expired,
			Conf: opt(bl.Conf), OthersCorrect: opt(bl.OthersCorrect), OthersConf: opt(bl.OthersConf),
		})
	}
	perBlock := "six"
	if len(f.Blocks) > 0 {
		perBlock = word(f.Blocks[0].Asked)
	}
	cap := fmt.Sprintf("Top: your %s answers in each block. Bottom: your average confidence in each block.", perBlock)
	if f.Compared {
		cap = fmt.Sprintf("Top: your %s answers in each block, with the others' average number correct. Bottom: your average "+
			"confidence against theirs.", perBlock)
	}
	out := []Block{h2("Block by block"), chartBlock("blocks", 0, chart), caption(cap), para(b.blockReading())}
	if len(f.Memory) == 0 {
		return out
	}

	cols := []Column{{Title: "Block", Width: 0.5}, {Title: "Task", Width: 1.7}, {Title: "Shown", Width: 0.8},
		{Title: "Expected", Width: 0.9}, {Title: "You gave", Width: 0.9}, {Title: "Result", Width: 2.2}}
	if f.Compared {
		cols = []Column{{Title: "Block", Width: 0.5}, {Title: "Task", Width: 1.55}, {Title: "Shown", Width: 0.62},
			{Title: "Expected", Width: 0.72}, {Title: "You gave", Width: 0.72}, {Title: "Result", Width: 1.99},
			{Title: "Others exact", Width: 0.9}}
	}
	var rows [][]Cell
	for _, m := range f.Memory {
		gave := esc(m.Response)
		if gave == "" {
			gave = "No answer"
		}
		r := row(fmt.Sprintf("%d", m.BlockNo), loadText(m.Load).task, esc(m.Presented), esc(m.Expected), gave, recallResult(m.Recall))
		if f.Compared {
			r = append(r, text(nOf(m.OthersExact, m.OthersN)))
		}
		rows = append(rows, r)
	}
	out = append(out, h3("The number you were holding"),
		Block{Type: "table", Table: &Table{Style: "data", BoldFirst: true, Columns: cols, Rows: rows}}, spacer(8))
	return append(out, b.memoryReading()...)
}

// ---------------------------------------------------------------- confidence

func (b *builder) confidence() []Block {
	f := b.f
	out := []Block{h2("Confidence")}
	if f.Rated == 0 {
		return append(out, para(fmt.Sprintf("None of your %d answers carried a confidence rating, so there is nothing to set "+
			"against your results. The rating is part of each question's time limit, and it is the half of the test that looks "+
			"at whether a person can tell when they are wrong.", f.Total)))
	}
	intro := fmt.Sprintf("You rated %d of your %d answers.", f.Rated, f.Total)
	e, u := f.me.expired, f.UnratedAnswered
	it := func(n int) string { return plural(n, "it", "them") }
	switch {
	case e > 0 && u > 0:
		intro += fmt.Sprintf(" %s %s ran out of time, and %s %s answered with no time left to rate %s.",
			Word(e), plural(e, "question", "questions"), word(u), wasWere(u), it(u))
	case e > 0:
		intro += fmt.Sprintf(" %s %s ran out of time.", Word(e), plural(e, "question", "questions"))
	case u > 0:
		intro += fmt.Sprintf(" %s %s answered with no time left to rate %s.", Word(u), wasWere(u), it(u))
	}
	intro += " The ratings were Guessing (10), Doubtful (30), Unsure (50), Fairly sure (70) and Certain (90)."

	levels := levelsChart{Compare: f.Compared}
	for _, l := range f.Levels {
		levels.Levels = append(levels.Levels, levelPoint{Level: l.Level, Label: confidenceWords[l.Level], N: l.N, Right: l.Right,
			Share: l.Share, Acc: opt(l.Acc), OthersShare: opt(l.OthersShare), OthersAcc: opt(l.OthersAcc)})
	}
	half := textWidth/2 - 0.08
	out = append(out, para(intro), Block{Type: "columns", LeftWidth: textWidth / 2,
		Left: []Block{chartBlock("confidence_use", half, levels),
			caption(bold("The ratings you used.") + " Share of rated answers at each of the five ratings.")},
		Right: []Block{chartBlock("calibration", half, levels),
			caption(bold("How often each rating was right.") + " On the diagonal, a rating means what it says. Your counts are small.")},
	}, spacer(7), para(b.calibrationReading()))

	if !f.Compared {
		if items := b.ownConfidenceBullets(); len(items) > 0 {
			out = append(out, h3("How your confidence behaved"), bullets(items...))
		}
		return out
	}

	scatter := scatterChart{You: scatterPoint{Conf: f.me.meanConf, Acc: f.me.accuracy}, Others: []scatterPoint{}}
	for _, p := range f.others {
		if !isNaN(p.meanConf) {
			scatter.Others = append(scatter.Others, scatterPoint{Conf: p.meanConf, Acc: p.accuracy})
		}
	}
	cap := "One dot per participant: how sure on average, against how many right. Below the diagonal a person was " +
		"more sure than right. " + othersWho(f.OthersBelowDiagonal, f.Others) + " are below it"
	switch g := whole(f.me.gap); {
	case g > 0:
		cap += ", as you are"
	case g < 0:
		cap += "; you are above it"
	default:
		cap += "; you are on it"
	}
	switch closer := f.GapClosest.Rank - 1; {
	case closer == 0:
		cap += "; nobody sits closer to it than you"
	case closer*3 <= f.Others:
		cap += fmt.Sprintf("; only %s %s closer to it than you", word(closer), plural(closer, "sits", "sit"))
	}
	var right []Block
	like, unlike := b.confidenceBullets()
	if len(like) > 0 {
		right = append(right, h3("Like the others"), bullets(like...))
	}
	if len(unlike) > 0 {
		right = append(right, h3("Unlike the others"), bullets(unlike...))
	}
	return append(out, h3("How your confidence compares"), Block{Type: "columns", LeftWidth: 3.1,
		Left: []Block{chartBlock("scatter", 2.95, scatter), caption(cap + ".")}, Right: right})
}

// calibrationReading states how often each rating the participant used
// turned out right, and leaves the judgement to the chart beside it.
func (b *builder) calibrationReading() string {
	f := b.f
	var parts []string
	for _, l := range f.Levels {
		if l.N == 0 {
			continue
		}
		if len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("At %s you were right on %s", confidenceWords[l.Level], nOf(l.Right, l.N)))
		} else {
			parts = append(parts, fmt.Sprintf("at %s %s", confidenceWords[l.Level], nOf(l.Right, l.N)))
		}
	}
	s := "A rating is well judged when it matches how often you turn out right: Certain should be right about nine times in " +
		"ten, Unsure about half the time. " + joinAnd(parts) + "."
	if f.Compared {
		var theirs []string
		for _, l := range f.Levels {
			if l.Level >= f.Threshold && !isNaN(l.OthersAcc) {
				theirs = append(theirs, fmt.Sprintf("%d%% of the time at %s", whole(l.OthersAcc), confidenceWords[l.Level]))
			}
		}
		if len(theirs) > 0 {
			s += " The others were right " + joinAnd(theirs) + "."
		}
	}
	return s + " With so few answers at each rating, read these as rough."
}

// ---------------------------------------------------------------- kinds and time

func (b *builder) kindsAndTime() []Block {
	f := b.f
	chart := familiesChart{Compare: f.Compared}
	var rows [][]Cell
	for _, fam := range f.Families {
		chart.Rows = append(chart.Rows, familyPoint{Label: familyLabel(fam.Family), Correct: fam.Correct, Asked: fam.Asked,
			Acc: fam.Acc, OthersAcc: opt(fam.OthersAcc)})
		rows = append(rows, row(familyLabel(fam.Family), fmt.Sprintf("%d", fam.Asked), fmt.Sprintf("%d", fam.Correct),
			fmt.Sprintf("%d", fam.Lure), fmt.Sprintf("%d", fam.Other), fmt.Sprintf("%d", fam.Expired), orDash(fam.Conf, "%.0f"),
			orDash(fam.Lat, "%.1f s")))
	}
	rows = append(rows, row("All "+word(f.Total), fmt.Sprintf("%d", f.Total), fmt.Sprintf("%d", f.Score), fmt.Sprintf("%d", f.me.lure),
		fmt.Sprintf("%d", f.me.other), fmt.Sprintf("%d", f.me.expired), orDash(f.me.meanConf, "%.0f"), orDash(f.me.lat, "%.1f s")))

	out := []Block{
		h2("Kinds of question"),
		chartBlock("families", 0, chart),
		para(b.kindsReading()),
		spacer(2),
		{Type: "table", Table: &Table{Style: "data", BoldFirst: true, Rows: rows, Breaks: []int{len(rows) - 1}, Columns: []Column{
			{Title: "Kind", Width: 1.15}, {Title: "Asked", Width: 0.6, Right: true}, {Title: "Correct", Width: 0.7, Right: true},
			{Title: "Intuitive trap", Width: 0.95, Right: true}, {Title: "Other wrong", Width: 0.9, Right: true},
			{Title: "Out of time", Width: 0.85, Right: true}, {Title: "Average confidence", Width: 1.05, Right: true},
			{Title: "Average time", Width: 0.8, Right: true},
		}}},
		h2("Time"),
	}
	if f.Answered == 0 {
		return append(out, para("Every question ran out of time, so there are no answer times to show."))
	}

	// The participant's copy groups the times by how each answer ended.
	// The owner's shows them in the order asked, which says which answer
	// was which and so stays in the copy that already holds the key.
	byOutcome := timeByOutcomeChart{LimitS: f.QuestionLimitS, MarkS: f.QuickS}
	for _, o := range []string{"correct", "lure", "other"} {
		// Never nil: an outcome with no answers is an empty row, and a
		// nil slice would reach the renderer as null in place of a list.
		ts := append([]float64{}, f.TimesByOutcome[o]...)
		label := outcomeLabel(o)
		if o == "other" {
			label = "Other wrong answer"
		}
		detail := "none"
		switch {
		case len(ts) == 1:
			detail = fmt.Sprintf("1 answer, %s s", secs(ts[0]))
		case len(ts) > 1:
			detail = fmt.Sprintf("%d answers, average %s s", len(ts), secs(mean(ts)))
		}
		byOutcome.Rows = append(byOutcome.Rows, timeOutcomeRow{Outcome: o, Label: label, Detail: detail, Times: ts})
	}
	inOrder := timelineChart{LimitS: f.QuestionLimitS, OthersTypical: nil}
	if f.Compared {
		inOrder.OthersTypical = opt(f.OthersLatMedian)
	}
	for _, a := range b.in.Run.Answers {
		pt := timelinePoint{Position: a.Position, BlockNo: a.BlockNo, Outcome: a.Outcome}
		if !a.expired() {
			pt.Seconds = opt(float64(a.LatencyMs) / 1000)
		}
		inOrder.Points = append(inOrder.Points, pt)
	}
	return append(out,
		participant(chartBlock("time_by_outcome", 0, byOutcome)),
		participant(caption(fmt.Sprintf("Each mark is one of your answers, placed by how long it took and grouped by how it turned "+
			"out. The darker line marks %d seconds.", whole(f.QuickS)))),
		owner(chartBlock("timeline", 0, inOrder)),
		para(b.timeReading()))
}

func orDash(v float64, format string) string {
	if isNaN(v) {
		return "none"
	}
	return fmt.Sprintf(format, v)
}

// kindsReading names the strongest kind and, with a group, where the
// participant was furthest ahead of it and furthest behind.
func (b *builder) kindsReading() string {
	f := b.f
	if len(f.Families) == 0 {
		return ""
	}
	best := f.Families[0].Acc
	for _, fam := range f.Families {
		best = math.Max(best, fam.Acc)
	}
	var strongest []FamilyFigures
	for _, fam := range f.Families {
		if same(fam.Acc, best) {
			strongest = append(strongest, fam)
		}
	}
	vs := func(fam FamilyFigures) string {
		if f.Compared && !isNaN(fam.OthersAcc) {
			return fmt.Sprintf("%s against %d%% for the others", nOf(fam.Correct, fam.Asked), whole(fam.OthersAcc))
		}
		return nOf(fam.Correct, fam.Asked)
	}
	var s string
	switch {
	case len(strongest) == len(f.Families) && len(f.Families) > 1:
		s = "No kind of question stood out: your share correct was the same in each."
	case len(strongest) == 1:
		s = fmt.Sprintf("%s were your strongest kind, %s.", capitalise(familyPlural(strongest[0].Family)), vs(strongest[0]))
	default:
		var names, scores []string
		for _, fam := range strongest {
			names = append(names, familyPlural(fam.Family))
			scores = append(scores, nOf(fam.Correct, fam.Asked))
		}
		s = fmt.Sprintf("%s were your strongest kinds (%s).", capitalise(joinAnd(names)), joinAnd(scores))
	}
	if f.Compared {
		var ahead, behind, hardest *FamilyFigures
		for i := range f.Families {
			fam := &f.Families[i]
			if isNaN(fam.OthersAcc) {
				continue
			}
			// Ahead or behind means by at least one question of that
			// kind. Three of ten against a group rate of 35% is the same
			// result, and saying otherwise would read noise as a gap.
			d, one := fam.Acc-fam.OthersAcc, 100/float64(fam.Asked)
			if d >= one && (ahead == nil || d > ahead.Acc-ahead.OthersAcc) {
				ahead = fam
			}
			if d <= -one && (behind == nil || d < behind.Acc-behind.OthersAcc) {
				behind = fam
			}
			if hardest == nil || fam.OthersAcc < hardest.OthersAcc {
				hardest = fam
			}
		}
		// When the strongest kind is also the one furthest ahead of the
		// group, the first sentence has already given its figures.
		same := ahead != nil && len(strongest) == 1 && strongest[0].Family == ahead.Family
		switch {
		case same && behind != nil:
			s += fmt.Sprintf(" That is also where you were furthest ahead of the others; you were furthest behind on %s (%s).",
				familyPlural(behind.Family), vs(*behind))
		case same:
			s += " That is also where you were furthest ahead of the others, and you were behind on none by as much as one question."
		case ahead != nil && behind != nil:
			s += fmt.Sprintf(" Against the others you were furthest ahead on %s (%s) and furthest behind on %s (%s).",
				familyPlural(ahead.Family), vs(*ahead), familyPlural(behind.Family), vs(*behind))
		case ahead != nil:
			s += fmt.Sprintf(" Against the others you were furthest ahead on %s (%s), and behind on none by as much as one question.",
				familyPlural(ahead.Family), vs(*ahead))
		case behind != nil:
			s += fmt.Sprintf(" Against the others you were furthest behind on %s (%s), and ahead on none by as much as one question.",
				familyPlural(behind.Family), vs(*behind))
		default:
			s += " In every kind you were within one question of the others' rate."
		}
		if hardest != nil {
			s += fmt.Sprintf(" %s were the hardest for the others, who got %d%% right.",
				capitalise(familyPlural(hardest.Family)), whole(hardest.OthersAcc))
		}
	}
	return s + " With five or ten questions of a kind, one answer moves these figures a lot."
}

// timeReading sets the quick answers against the rest.
func (b *builder) timeReading() string {
	f := b.f
	long, quick := whole(f.LongS), whole(f.QuickS)
	s := fmt.Sprintf("%s of your %d answers took %d seconds or more, and %s took under %d. ",
		Word(f.LongN), f.Answered, long, word(f.QuickN), quick)
	if f.Compared {
		s += fmt.Sprintf("Your average of %s seconds was %s.", secs(f.me.lat), b.paceRank())
	} else {
		s += fmt.Sprintf("Your average was %s seconds, against a limit of %d.", secs(f.me.lat), whole(f.QuestionLimitS))
	}
	if f.QuickN == 0 || f.ConsideredN == 0 {
		return s
	}
	of := ","
	if len(f.QuickFamilies) == 1 && f.QuickN >= 2 {
		of = ", all of them " + familyPlural(f.QuickFamilies[0]) + ","
	}
	rest := fmt.Sprintf("of the %d slower ones, %s %s right", f.ConsideredN, word(f.ConsideredCorrect), wasWere(f.ConsideredCorrect))
	if f.ConsideredCorrect == 0 {
		rest = fmt.Sprintf("of the %d slower ones, none was right", f.ConsideredN)
	}
	if f.QuickN == 1 {
		was := "wrong"
		switch {
		case f.QuickCorrect == 1:
			was = "right"
		case f.QuickLure == 1:
			was = "the intuitive trap"
		}
		s += fmt.Sprintf(" The one quick answer was %s; %s.", was, rest)
	} else {
		got := fmt.Sprintf("%s %s right", word(f.QuickCorrect), wasWere(f.QuickCorrect))
		if f.QuickCorrect == 0 {
			got = "none was right"
		}
		if f.QuickLure > 0 {
			got += fmt.Sprintf(" and %s %s the intuitive trap", word(f.QuickLure), wasWere(f.QuickLure))
		}
		s += fmt.Sprintf(" Of the %s quick answers%s %s; %s.", word(f.QuickN), of, got, rest)
	}
	if f.QuickN < 3 {
		return s
	}
	quickAcc := float64(f.QuickCorrect) / float64(f.QuickN)
	restAcc := float64(f.ConsideredCorrect) / float64(f.ConsideredN)
	switch {
	case b.quickWereRisky() && quickAcc < restAcc:
		s += fmt.Sprintf(" That is what the test is built to show: the tempting answer arrives quickly, and time spent checking "+
			"is what beats it. %s quick answers is a small number, so take it as a pointer.", Word(f.QuickN))
	case quickAcc >= restAcc:
		s += " Your quick answers were no less accurate than your slower ones, which is not the usual pattern on this test."
	}
	return s
}

// ---------------------------------------------------------------- owner's pages

// thirtyAnswers is the answer-by-answer table. It says which answers
// were right, question by question, so it is in the owner's copy only.
func (b *builder) thirtyAnswers() []Block {
	f := b.f
	cols := []Column{{Title: "No.", Width: 0.5}, {Title: "Block", Width: 0.6}, {Title: "Kind", Width: 1.2}, {Title: "", Width: 0.22},
		{Title: "Result", Width: 1.6}, {Title: "How sure", Width: 1.6}, {Title: "Time", Width: 1.28, Right: true}}
	intro := "In the order asked. This table says which answers were right, question by question, so it is in this copy only."
	if f.Compared {
		cols = []Column{{Title: "No.", Width: 0.42}, {Title: "Block", Width: 0.5}, {Title: "Kind", Width: 0.95}, {Title: "", Width: 0.22},
			{Title: "Result", Width: 1.2}, {Title: "How sure", Width: 1.2}, {Title: "Time", Width: 0.66, Right: true},
			{Title: "Others on the same question", Width: 1.85}}
		intro = "In the order asked. The last column shows how the others did on the same question, among those who were given " +
			"it. This table says which answers were right, question by question, so it is in this copy only."
	}
	var rows [][]Cell
	var breaks []int
	for i, q := range f.Questions {
		if i > 0 && q.BlockNo != f.Questions[i-1].BlockNo {
			breaks = append(breaks, i)
		}
		sure, took := "", ""
		if !q.expired() {
			took = secs(float64(q.LatencyMs)/1000) + " s"
			sure = "Not rated"
			if q.rated() {
				sure = fmt.Sprintf("%s (%d)", confidenceWords[q.Confidence], q.Confidence)
				if confidenceWords[q.Confidence] == "" {
					sure = fmt.Sprintf("%d", q.Confidence)
				}
			}
		}
		r := []Cell{text(fmt.Sprintf("%d", q.Position)), text(fmt.Sprintf("%d", q.BlockNo)), text(familyLabel(q.Family)),
			marker(q.Outcome), text(outcomeLabel(q.Outcome)), text(sure), text(took)}
		if f.Compared {
			others := "Seen by no one else"
			if q.OthersN > 0 {
				others = nOf(q.OthersRight, q.OthersN) + " correct"
			}
			r = append(r, text(others))
		}
		rows = append(rows, r)
	}
	return []Block{
		owner(h2("The " + word(f.Total) + " answers")),
		owner(para(intro)),
		owner(Block{Type: "table", Table: &Table{Style: "data", Columns: cols, Rows: rows, Breaks: breaks}}),
		owner(pageBreak()),
	}
}

// appendix prints the questions and their answers. Owner's copy only:
// it is the answer key for the questions this run drew.
func (b *builder) appendix() []Block {
	f := b.f
	var rows [][]Cell
	var breaks []int
	for i, q := range f.Questions {
		if i > 0 && q.BlockNo != f.Questions[i-1].BlockNo {
			breaks = append(breaks, i)
		}
		chosen := esc(q.ChosenText)
		if q.expired() || chosen == "" {
			chosen = "No answer"
		}
		rows = append(rows, []Cell{
			{Text: fmt.Sprintf("%d", q.Position), Bold: true},
			text(esc(q.ItemCode) + "<br/>" + familyLabel(q.Family)),
			text(esc(q.Prompt)), text(chosen), text(esc(q.CorrectText)),
			marker(q.Outcome), text(outcomeLabel(q.Outcome)),
		})
	}
	return []Block{
		owner(pageBreak()),
		owner(h2("Appendix: the questions and their answers")),
		owner(note(bold("Owner's copy only.") + fmt.Sprintf(" This is the answer key for %s questions of the pool. A participant is "+
			"never shown the correct answer to a question (FSD FR-DT-04): anyone who reads this page and then takes the test has "+
			"seen part of it. Confidence and time for each question are in the table “The %s answers”.",
			word(f.Total), word(f.Total)))),
		owner(spacer(8)),
		owner(Block{Type: "table", Table: &Table{Style: "small", Rows: rows, Breaks: breaks, Columns: []Column{
			{Title: "No.", Width: 0.3}, {Title: "Item", Width: 0.74}, {Title: "Question", Width: 2.78}, {Title: "Answer given", Width: 1.1},
			{Title: "Correct answer", Width: 1.1}, {Title: "", Width: 0.18}, {Title: "Result", Width: 0.8},
		}}}),
	}
}

// ---------------------------------------------------------------- method

func (b *builder) method() []Block {
	f, run := b.f, b.in.Run
	sure := sureWords(f.Threshold)
	items := []string{fmt.Sprintf("%s Run %s (%s answers, %s blocks)%s, read on %s. No name or address was used, and none appears here.",
		bold("Source."), run.ShortID(), word(f.Total), word(len(f.Blocks)),
		map[bool]string{true: " and the test's own records of every other finished sitting", false: ""}[f.Compared], day(b.in.Now))}

	if f.Compared {
		items = append(items, b.groupNote(),
			bold("First sitting")+" means no sign of an earlier one from the same account, address or browser. It can miss a "+
				"repeat taken on another device.")
		pct := fmt.Sprintf("%s The share of the group you scored above, counting yourself as half a place: (%d + 0.5) / %d = %.2f, "+
			"the %s percentile.", bold("Percentile."), f.ScoreBelow, f.N, f.Percentile/100, ordinal(whole(f.Percentile)))
		if t := f.ScorePlace.Tied; t > 0 {
			pct = fmt.Sprintf("%s The share of the group you scored above, counting yourself and the %d level with you as half a "+
				"place each: (%d + %.1f) / %d = %.2f, the %s percentile.", bold("Percentile."), t, f.ScoreBelow, 0.5*float64(t+1),
				f.N, f.Percentile/100, ordinal(whole(f.Percentile)))
		}
		items = append(items, pct)
	}
	if f.Guess > 0 {
		items = append(items, fmt.Sprintf("%s Each question offers three or four answers. Picking at random on the %s you were asked "+
			"would average about %d.", bold("Guessing."), word(f.Total), whole(f.Guess)))
	}
	if f.Rated > 0 {
		items = append(items, fmt.Sprintf("%s is over the answers that were rated (%d of your %d). %s is that average minus the "+
			"percentage of all %s questions answered correctly.", bold("Average confidence"), f.Rated, f.Total,
			bold("Confidence minus accuracy"), word(f.Total)),
			fmt.Sprintf("%s counts wrong answers rated %s (%d or more). A question that ran out of time counts as wrong and as unrated.",
				bold("Wrong and sure"), sure, f.Threshold))
	}
	items = append(items, bold("Time per answer")+" is from the question appearing to the confidence rating being given, over "+
		"answered questions only.")
	if f.Compared {
		items = append(items, bold("The others' figures")+" are medians across people where the report says median, and averages "+
			"across people in the block charts. The chart by kind of question and the ratings charts pool all of the others' answers.")
		if f.Rated > 0 {
			items = append(items, bold("Brier score.")+" For each answer, the square of the distance between the confidence given "+
				"(as a probability) and what happened (1 for correct, 0 for not), averaged.")
		}
	}

	where := ""
	switch run.Device {
	case "phone":
		where = " on a phone"
	case "tablet":
		where = " on a tablet"
	case "desktop":
		where = " on a computer"
	}
	took := Word(f.Minutes) + " " + plural(f.Minutes, "minute", "minutes")
	limits := []string{fmt.Sprintf("%s %s%s. A second sitting would not give the same numbers, and it would not be "+
		"comparable, because the questions are easier once seen.", bold("It is one sitting."), took, where)}
	if f.Reason == ReasonRepeat {
		limits[0] = fmt.Sprintf("%s %s%s. The questions are easier once seen, so these numbers are not comparable with a "+
			"first sitting, yours or anyone's.", bold("It is a repeat sitting."), took, where)
	}
	if f.Compared && f.Others < 30 {
		limits = append(limits, fmt.Sprintf("%s %s other people. Every ranking here would shift with a few more, and differences "+
			"of a place or two mean nothing.", bold("The group is small."), Word(f.Others)))
	}
	limits = append(limits, bold("The questions are traps.")+" A low score is the normal result, and the test says nothing about "+
		"ability at arithmetic or logic when there is time to check, or about judgement in work a person is trained for.")
	strict := bold("The number task is scored strictly.") + " Digits in the wrong position count as not held"
	for _, m := range f.Memory {
		if rightDigitsWrongOrder(m.Recall) {
			strict += fmt.Sprintf(", which is why block %d reads as a complete miss when the digits themselves were right", m.BlockNo)
			break
		}
	}
	limits = append(limits, strict+".")

	return []Block{h2("How these numbers were made"), bullets(items...),
		h2("What this report cannot tell you"), bullets(limits...)}
}

// groupNote describes the comparison group for somebody checking it:
// the pool, the limits, and which versions of the test are in it.
func (b *builder) groupNote() string {
	f, run := b.f, b.in.Run
	s := fmt.Sprintf("%s %s first sittings on question pool %s", bold("Comparison group."), Word(f.N),
		esc(strings.TrimPrefix(run.ItemSetVersion, "pool-")))
	if f.MemoriseS > 0 {
		s += fmt.Sprintf(" with %s seconds to memorise, %s seconds a question and %s seconds to recall",
			num(f.MemoriseS), num(f.QuestionLimitS), num(f.RecallS))
	}
	s += "."
	mine := run.Generation()
	present := map[int]bool{mine: true}
	var gens []int
	for g := range f.Generations {
		present[g] = true
		gens = append(gens, g)
	}
	sortInts(gens)
	if len(present) == 1 {
		if mine > 0 {
			s += fmt.Sprintf(" Everyone in the group took version %d of the test.", mine)
		}
	} else {
		var counts []string
		for i, g := range gens {
			c := fmt.Sprintf("%d version %d", f.Generations[g], g)
			if i == 0 {
				c = fmt.Sprintf("%d took version %d", f.Generations[g], g)
			}
			counts = append(counts, c)
		}
		s += fmt.Sprintf(" Of the %d others, %s. Yours was version %d.", f.Others, joinAnd(counts), mine)
	}
	// What each version changed, for the versions somebody in the group
	// sat. The first of them changed nothing relative to the group.
	lowest := mine
	for g := range present {
		if g < lowest {
			lowest = g
		}
	}
	var all []int
	for g := range present {
		all = append(all, g)
	}
	sortInts(all)
	countdown := false
	for _, g := range all {
		if g == lowest {
			continue
		}
		if n := generationNotes[g]; n != "" {
			s += " " + n
			countdown = countdown || g == 4
		}
	}
	// The countdown is a condition of this run from version 4 on, so it
	// is stated even when the whole group sat that version and nothing
	// above had cause to mention it.
	if mine >= 4 {
		if !countdown {
			s += " " + generationNotes[4]
		}
		saw := "did not see it"
		if run.TimerVisible {
			saw = "saw it"
		}
		s += fmt.Sprintf(" You were in the half that %s.", saw)
	}
	return s
}

func sortInts(v []int) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
