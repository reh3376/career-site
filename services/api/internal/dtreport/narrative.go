package dtreport

import (
	"fmt"
	"math"
	"strings"
)

// The findings on the first page, and the sentences elsewhere that
// depend on what a run shows.
//
// Every sentence here is chosen by a rule, and the rule is the claim.
// A lead such as "You took your time" is printed only when the run sits
// in the slowest third of the group, so it is true of every report that
// carries it. Where a run supports nothing in particular the report
// states the figures and stops: telling a volunteer a story the data
// does not support is worse than telling them nothing, because they
// cannot check it and have no reason to doubt it.

// finding is a bold lead and the sentences behind it.
func finding(lead, body string) string { return bold(lead) + " " + body }

// findings are the numbered points under "What stands out".
func (b *builder) findings() []string {
	out := []string{b.findScore(), b.findGap(), b.findWrongSure(), b.findPace(), b.findLoad(), b.findTiring()}
	kept := out[:0]
	for _, s := range out {
		if s != "" {
			kept = append(kept, s)
		}
	}
	return kept
}

const trickQuestions = "these are trick questions, built so that the first answer to come to mind is wrong"

func (b *builder) findScore() string {
	f := b.f
	if !f.Compared {
		body := fmt.Sprintf("That is %d%% correct", whole(f.me.accuracy))
		if f.Guess > 0 {
			body += fmt.Sprintf(", where guessing alone would average about %d", whole(f.Guess))
		}
		body += ". " + capitalise(trickQuestions) + ", so a low score is the normal result."
		return finding(fmt.Sprintf("You got %d of %d right.", f.Score, f.Total), body)
	}
	lead := "You scored the same as the typical participant."
	switch {
	case float64(f.Score) > f.OthersScoreMedian:
		lead = "You scored above the typical participant."
	case float64(f.Score) < f.OthersScoreMedian:
		lead = "You scored below the typical participant."
	}
	// Judged on the percentile as printed, so "67th percentile" is never
	// followed by "the middle third" for the sake of a rounding the
	// reader cannot see.
	third := "in the middle third"
	switch pct := whole(f.Percentile); {
	case pct >= 67:
		third = "in the top third"
	case pct <= 33:
		third = "in the bottom third"
	}
	body := fmt.Sprintf("You got %d of %d right. The median for the %d other people taking the test for the first time "+
		"was %s, and their scores ran from %d to %d. That puts you %s of %d: the %s percentile, %s. ",
		f.Score, f.Total, f.Others, num(f.OthersScoreMedian), f.OthersScoreMin, f.OthersScoreMax,
		rankText(f.ScorePlace), f.N, ordinal(whole(f.Percentile)), third)
	// "Low for everyone" is a claim about the group, so it is made only
	// when the group bears it out.
	if f.OthersScoreMedian <= float64(f.Total)/2 {
		body += "Scores are low for everyone by design: " + trickQuestions + "."
	} else {
		body += capitalise(trickQuestions) + "."
	}
	return finding(lead, body)
}

func (b *builder) findGap() string {
	f := b.f
	if f.Rated == 0 {
		return finding("You did not rate your confidence.",
			"No answer carried a rating, so this report cannot set how sure you were against how you did.")
	}
	gap := whole(f.me.gap)
	lead := "Your confidence was close to your results."
	switch {
	case gap > 10:
		lead = "You were more sure than right."
	case gap < -10:
		lead = "You were less sure than right."
	}
	body := fmt.Sprintf("You rated yourself %d out of 100 on average and got %d%% right, ", whole(f.me.meanConf), whole(f.me.accuracy))
	switch {
	case gap == 0:
		body += "an exact match."
	case gap > 0:
		body += fmt.Sprintf("a gap of %d %s.", gap, plural(gap, "point", "points"))
	default:
		body += fmt.Sprintf("which is %d %s less sure than right.", -gap, plural(-gap, "point", "points"))
	}
	if !f.Compared {
		return finding(lead, body)
	}
	if f.OthersGapMedian > 10 && f.OthersOverconfident*2 > f.Others {
		far := ""
		if f.OthersGapMedian >= 25 {
			far = "far "
		}
		body += fmt.Sprintf(" Most participants were %smore sure than right: the others' median gap was %s, and %d of the %d "+
			"were more than 10 points overconfident.", far, points(whole(f.OthersGapMedian)), f.OthersOverconfident, f.Others)
	} else {
		body += fmt.Sprintf(" The others' median gap was %s.", signedPoints(f.OthersGapMedian))
	}
	switch closer := f.GapClosest.Rank - 1; {
	case closer == 0:
		body += " Nobody came closer to zero than you."
	case closer*3 <= f.Others:
		body += fmt.Sprintf(" Only %s came closer to zero than you.", word(closer))
	case closer == f.Others:
		body += fmt.Sprintf(" All %d came closer to zero than you.", f.Others)
	default:
		body += fmt.Sprintf(" %s of the %d came closer to zero than you.", Word(closer), f.Others)
	}
	return finding(lead, body)
}

// wrongSureSplit says how many wrong answers sat at each sure rating:
// "Three of your wrong answers were rated Fairly sure, and none Certain."
func (b *builder) wrongSureSplit() string {
	f := b.f
	if f.me.wrongSure == 0 {
		return fmt.Sprintf("None of your wrong answers was rated %s.", sureWords(f.Threshold))
	}
	var s string
	var none []string
	for _, l := range confidenceLevels {
		if l < f.Threshold {
			continue
		}
		n := f.WrongAtLevel[l]
		switch {
		case n == 0:
			none = append(none, confidenceWords[l])
		case s == "":
			s = fmt.Sprintf("%s of your wrong answers %s rated %s", Word(n), wasWere(n), confidenceWords[l])
		default:
			s += fmt.Sprintf(" and %s %s", word(n), confidenceWords[l])
		}
	}
	if len(none) > 0 {
		s += ", and none " + strings.Join(none, " or ")
	}
	return s + "."
}

func (b *builder) findWrongSure() string {
	f := b.f
	if f.Rated == 0 {
		return ""
	}
	ws := f.me.wrongSure
	top := confidenceWords[confidenceLevels[len(confidenceLevels)-1]]
	topShare := f.Levels[len(f.Levels)-1].OthersShare

	lead := "You were never wrong and sure."
	if ws > 0 {
		lead = fmt.Sprintf("You were wrong and sure %s.", times(ws))
	}
	body := b.wrongSureSplit()
	if f.Compared {
		med := f.OthersWrongSure.Median
		switch {
		case ws == 0:
		case float64(ws) <= med/2:
			lead = "You were rarely wrong and sure."
		case float64(ws) < med:
			lead = "You were wrong and sure less often than most."
		case same(float64(ws), med):
			lead = "You were wrong and sure about as often as most."
		default:
			lead = "You were wrong and sure more often than most."
		}
		body += fmt.Sprintf(" The others' median was %s wrong %s at %s, and the highest was %d.",
			num(med), plural(whole(med), "answer", "answers"), sureWords(f.Threshold), whole(f.OthersWrongSure.Max))
	}
	switch {
	case f.me.usedCertain == 0 && f.Compared && !isNaN(topShare) && whole(topShare) > 0:
		body += fmt.Sprintf(" You never chose %s at all; the others chose it for %d%% of their answers.", top, whole(topShare))
	case f.me.usedCertain == 0:
		body += fmt.Sprintf(" You never chose %s at all.", top)
	case f.Compared && !isNaN(topShare):
		body += fmt.Sprintf(" You chose %s for %s rated answers; the others chose it for %d%% of theirs.",
			top, nOf(f.me.usedCertain, f.Rated), whole(topShare))
	default:
		body += fmt.Sprintf(" You chose %s for %s rated answers.", top, nOf(f.me.usedCertain, f.Rated))
	}
	return finding(lead, body)
}

// quickWereRisky is the rule behind "your quick answers were the risky
// ones": at least three answers given before there was time to check,
// and most of them the trap.
func (b *builder) quickWereRisky() bool {
	return b.f.QuickN >= 3 && b.f.QuickLure*2 > b.f.QuickN
}

// paceRank is the participant's speed as a placing, from whichever end
// is nearer.
func (b *builder) paceRank() string {
	f := b.f
	if f.LatSlow.Rank <= f.LatFast.Rank {
		return rankPhrase(f.LatSlow, "slowest")
	}
	return rankPhrase(f.LatFast, "fastest")
}

func (b *builder) expiredPhrase() string {
	switch n := b.f.me.expired; n {
	case 0:
		return "no question ran out of time"
	case 1:
		return "only one question ran out of time"
	default:
		return fmt.Sprintf("%s questions ran out of time", word(n))
	}
}

// quickSentence says what became of the answers given quickly.
func (b *builder) quickSentence() string {
	f := b.f
	q := whole(f.QuickS)
	switch {
	case f.QuickN == 0:
		return fmt.Sprintf("None of your answers came in under %d seconds.", q)
	case f.QuickN == 1:
		was := "wrong"
		switch {
		case f.QuickCorrect == 1:
			was = "right"
		case f.QuickLure == 1:
			was = "the intuitive trap"
		}
		return fmt.Sprintf("One answer came in under %d seconds, and it was %s.", q, was)
	case f.QuickLure == 0:
		return fmt.Sprintf("%s answers came in under %d seconds; none of those was the intuitive trap.", Word(f.QuickN), q)
	case f.QuickLure == f.QuickN:
		return fmt.Sprintf("%s answers came in under %d seconds; every one of those was the intuitive trap.", Word(f.QuickN), q)
	}
	return fmt.Sprintf("%s answers came in under %d seconds; %s of those %s the intuitive trap.",
		Word(f.QuickN), q, word(f.QuickLure), wasWere(f.QuickLure))
}

func (b *builder) findPace() string {
	f := b.f
	if f.Answered == 0 {
		return ""
	}
	risky := "."
	if b.quickWereRisky() {
		risky = ", and your quick answers were the risky ones."
	}
	if !f.Compared {
		return finding(fmt.Sprintf("You averaged %s seconds an answer%s", secs(f.me.lat), risky),
			fmt.Sprintf("The limit was %d seconds, and %s. %s", whole(f.QuestionLimitS), b.expiredPhrase(), b.quickSentence()))
	}
	third := int(math.Ceil(float64(f.N) / 3))
	lead := "Your pace was close to the group's"
	switch {
	case f.LatSlow.Rank <= third:
		lead = "You took your time"
	case f.LatFast.Rank <= third:
		lead = "You answered quickly"
	}
	return finding(lead+risky, fmt.Sprintf("You averaged %s seconds an answer, %s, and %s. %s",
		secs(f.me.lat), b.paceRank(), b.expiredPhrase(), b.quickSentence()))
}

// loadShift is how the two hardest blocks compared with the two before
// them, in questions. A single question either way is noise on twelve.
func (b *builder) loadShift() int { return b.f.LoadedCorrect - b.f.LightCorrect }

func (b *builder) findLoad() string {
	f := b.f
	if f.LightAsked == 0 || f.LoadedAsked == 0 || f.LightAsked != f.LoadedAsked {
		return ""
	}
	d, exact := b.loadShift(), f.LoadedRecallExact
	held := d >= -1
	var lead string
	switch {
	case held && exact == 0:
		lead = "Holding the number did not cost you on the questions. It cost you the number."
	case held && exact == 2:
		lead = "The harder numbers cost you nothing."
	case held:
		lead = "Your accuracy held up when the number got harder."
	case exact == 2:
		lead = "You kept the number, and the questions paid for it."
	case exact == 1:
		lead = "Your accuracy dropped when the number got harder."
	default:
		lead = "The harder number cost you on both counts."
	}
	than := "about the same as"
	switch {
	case d >= 2:
		than = "more than"
	case d <= -2:
		than = "fewer than"
	case d == 0:
		than = "the same as"
	}
	body := fmt.Sprintf("In the two hardest blocks you got %s right, %s in the two blocks before them (%s).",
		nOf(f.LoadedCorrect, f.LoadedAsked), than, nOf(f.LightCorrect, f.LightAsked))
	if f.Compared {
		body += fmt.Sprintf(" The others averaged %s of %d in the hardest blocks and %s in the two before.",
			secs(f.OthersLoadedCorr), f.LoadedAsked, secs(f.OthersLightCorrect))
	}
	switch exact {
	case 0:
		body += " In neither of those blocks did you give the number back exactly."
	case 1:
		body += " You gave the number back exactly in one of those two blocks."
	default:
		body += " You gave the number back exactly in both."
	}
	return finding(lead, body)
}

// findTiring reads block 5 against block 1. Whether the last block held
// is the email's FatigueHeld and nothing else, so the report and the
// message it is attached to cannot disagree.
func (b *builder) findTiring() string {
	f := b.f
	if len(f.Blocks) < 5 || f.Blocks[0].Asked == 0 || f.Blocks[4].Asked == 0 {
		return ""
	}
	b1, b5 := f.Blocks[0], f.Blocks[4]
	const repeats = "Block 5 repeats the difficulty of block 1 at the end of the test."
	if b.in.Email.FatigueHeld {
		scored := fmt.Sprintf("%s in block 1 and %d in block 5", nOf(b1.Correct, b1.Asked), b5.Correct)
		if b1.Correct == b5.Correct {
			scored = nOf(b1.Correct, b1.Asked) + " in both"
		}
		return finding("No sign of tiring.", fmt.Sprintf("%s You scored %s.", repeats, scored))
	}
	return finding("The last block fell away.", fmt.Sprintf("%s You scored %s in block 1 and %d in block 5, so some of what "+
		"happened in between may be tiredness as much as the number.", repeats, nOf(b1.Correct, b1.Asked), b5.Correct))
}

// blockReading is the paragraph under the block chart. It is built on
// the email's reading and the email's figures: early blocks against the
// hardest, with the same four outcomes.
func (b *builder) blockReading() string {
	f, e := b.f, b.in.Email
	s := "The usual pattern on this test is that the trap works more often as the number gets harder to hold, while " +
		"confidence stays where it was."
	accDrop, confDrop := e.EarlyAccuracy-e.HardAccuracy, e.EarlyConfidence-e.HardConfidence
	span := fmt.Sprintf("from %d%% in blocks 1 and 2 to %d%% in block 4, the hardest", e.EarlyAccuracy, e.HardAccuracy)
	confSpan := fmt.Sprintf("from %d in blocks 1 and 2 to %d in block 4", e.EarlyConfidence, e.HardConfidence)
	rated := f.Rated > 0 && !isNaN(f.me.blockConf[4])
	switch e.Reading {
	case "no_decline":
		s += fmt.Sprintf(" Your accuracy did not fall: %d%% in blocks 1 and 2 and %d%% in block 4, the hardest.",
			e.EarlyAccuracy, e.HardAccuracy)
		switch {
		case !rated:
		case confDrop >= 10:
			s += fmt.Sprintf(" Your confidence came down all the same, %s.", confSpan)
		case confDrop <= -10:
			s += fmt.Sprintf(" Your confidence rose as well, %s.", confSpan)
		default:
			s += fmt.Sprintf(" Your confidence barely moved, %s.", confSpan)
		}
	case "effect":
		s += fmt.Sprintf(" Yours followed it: accuracy fell %s, %s", points(accDrop), span)
		switch {
		case !rated:
			s += "."
		case confDrop > 0:
			s += fmt.Sprintf(", while confidence fell only %s, %s.", points(confDrop), confSpan)
		case confDrop == 0:
			s += fmt.Sprintf(", while confidence did not move from %d.", e.EarlyConfidence)
		default:
			s += fmt.Sprintf(", while confidence rose %s, %s.", points(-confDrop), confSpan)
		}
	default:
		s += fmt.Sprintf(" Your accuracy fell %s, %s.", points(accDrop), span)
		if rated {
			s += " " + movement("Your confidence", confDrop) + ", " + confSpan + "."
			if e.Reading == "overcorrected" {
				s += " It fell further than your accuracy did."
			} else {
				s += " It tracked your accuracy closely."
			}
		}
	}
	if f.Compared {
		s += fmt.Sprintf(" Among the others, accuracy from block 1 to block 4 fell for %d, held for %d and rose for %d",
			f.OthersAccFell, f.OthersAccHeld, f.OthersAccRose)
		switch c := whole(f.OthersConfChange); {
		case f.OthersConfChangeN == 0:
			s += "."
		case c == 0:
			s += ", and their confidence did not move (the median change was 0)."
		case c < 0:
			s += fmt.Sprintf(", and their confidence fell by a median of %s.", points(-c))
		default:
			s += fmt.Sprintf(", and their confidence rose by a median of %s.", points(c))
		}
	}
	return s
}

// movement words a change the way the results email does, so "fell 6
// points" in one is "fell 6 points" in the other.
func movement(subject string, drop int) string {
	switch {
	case drop > 0:
		return fmt.Sprintf("%s fell %s", subject, points(drop))
	case drop < 0:
		return fmt.Sprintf("%s rose %s", subject, points(-drop))
	default:
		return subject + " did not move"
	}
}

// memoryReading is the paragraph under the table of numbers.
func (b *builder) memoryReading() []Block {
	f := b.f
	if len(f.Memory) == 0 {
		return nil
	}
	exact := 0
	var notable []string
	for _, m := range f.Memory {
		switch {
		case m.Outcome == "exact":
			exact++
		case m.Outcome == "expired":
			notable = append(notable, fmt.Sprintf("In block %d the time to give the number back ran out.", m.BlockNo))
		case m.Outcome == "untransformed":
			notable = append(notable, fmt.Sprintf("In block %d you gave back %s, the number as shown, without the rule applied.",
				m.BlockNo, esc(m.Response)))
		case rightDigitsWrongOrder(m.Recall):
			notable = append(notable, fmt.Sprintf("In block %d you gave %s where %s was expected: every digit was right and "+
				"the order was not, and the test scores digits by position.", m.BlockNo, esc(m.Response), esc(m.Expected)))
		case m.Outcome == "partial":
			notable = append(notable, fmt.Sprintf("In block %d you had %s digits in place.", m.BlockNo, nOf(m.DigitsHeld, len(m.Expected))))
		default:
			notable = append(notable, fmt.Sprintf("In block %d the number was lost.", m.BlockNo))
		}
	}
	var s string
	switch {
	case exact == len(f.Memory):
		s = fmt.Sprintf("You gave the number back exactly in all %s blocks.", word(len(f.Memory)))
	case exact == 0:
		s = fmt.Sprintf("You gave the number back exactly in none of the %s blocks.", word(len(f.Memory)))
	default:
		s = fmt.Sprintf("You gave the number back exactly in %s of the %s blocks.", word(exact), word(len(f.Memory)))
	}
	if len(notable) > 0 {
		s += " " + strings.Join(notable, " ")
	}
	switch b.in.Run.RecallStrategy {
	case "encode":
		s += " After the test you said that you did the adding straight away and held the result."
	case "defer":
		s += " After the test you said that you held the number as shown and did the adding at the end."
	case "unsure":
		s += " After the test you said that you used a bit of both ways of holding the number, or did not remember."
	}
	out := []Block{para(s)}

	// The trade, named only when both halves of it are in the data.
	const hedge = ", so treat this as a reading, not a finding."
	if f.LightAsked > 0 && f.LightAsked == f.LoadedAsked {
		switch d := b.loadShift(); {
		case d >= -1 && f.LoadedRecallExact == 0:
			out = append(out, para("One reading is that your attention went to the questions and the number paid for it. "+
				"The test cannot see whether you were still rehearsing the number while you answered"+hedge))
		case d <= -2 && f.LoadedRecallExact == 2:
			out = append(out, para("One reading is that your attention went to the number and the questions paid for it. "+
				"The test cannot see how you divided your attention"+hedge))
		}
	}
	return out
}

// confidenceBullets sorts what can be said about a participant's
// confidence into what they share with the group and what sets them
// apart. Each returns whether it is a likeness, and its text.
func (b *builder) confidenceBullets() (like, unlike []string) {
	f := b.f
	add := func(isLike bool, s string) {
		if s == "" {
			return
		}
		if isLike {
			like = append(like, s)
		} else {
			unlike = append(unlike, s)
		}
	}
	sure := sureWords(f.Threshold)
	top := confidenceWords[confidenceLevels[len(confidenceLevels)-1]]

	// How much the ratings separated right from wrong.
	if d := f.me.disc; !isNaN(d) {
		s := b.discSentence() + fmt.Sprintf(" The others' median was %d.", whole(f.OthersDiscMedian))
		if f.OthersDiscMedian <= 12 {
			s += " On these questions confidence is a weak signal for nearly everyone."
		}
		add(isNaN(f.OthersDiscMedian) || math.Abs(d-f.OthersDiscMedian) <= 5, s)
	}

	// The trap, taken while sure, in the lightest block.
	if f.me.luredSureBlock1 {
		add(f.OthersLuredBlock1*2 >= f.Others, fmt.Sprintf("%s In block 1, the lightest load, you gave a trap answer rated %s. "+
			"%s did the same.", bold("The trap caught you early, while sure."), sure, othersWho(f.OthersLuredBlock1, f.Others)))
	} else {
		add(f.OthersLuredBlock1*2 < f.Others, fmt.Sprintf("%s In block 1, the lightest load, you gave no trap answer rated %s. "+
			"%s gave at least one.", bold("The trap did not catch you early while sure."), sure,
			othersWho(f.OthersLuredBlock1, f.Others)))
	}

	// How sure overall.
	mine, med := whole(f.me.meanConf), whole(f.OthersConfMedian)
	switch diff := f.me.meanConf - f.OthersConfMedian; {
	case math.Abs(diff) <= 5:
		add(true, fmt.Sprintf("%s %d against a median of %d.", bold("About as confident overall:"), mine, med))
	case diff < 0:
		add(false, fmt.Sprintf("%s %d against a median of %d. %s", bold("Lower overall:"), mine, med,
			fewOthers(f.ConfLow.Rank-1, f.Others, "less confident")))
	default:
		add(false, fmt.Sprintf("%s %d against a median of %d. %s", bold("Higher overall:"), mine, med,
			fewOthers(f.ConfHigh.Rank-1, f.Others, "more confident")))
	}

	// The top of the scale.
	most := f.OthersUsedCertain*2 >= f.Others
	switch {
	case f.me.usedCertain == 0 && most:
		s := fmt.Sprintf("%s %s used it", bold("Never "+top+"."), othersWho(f.OthersUsedCertain, f.Others))
		if f.OthersCertainMost {
			s += ", and it was their most used rating"
		}
		add(false, s+".")
	case f.me.usedCertain == 0:
		add(true, fmt.Sprintf("%s Only %s of the %d others used it.", bold("Never "+top+", like most."), word(f.OthersUsedCertain), f.Others))
	case most:
		add(true, fmt.Sprintf("%s %s used it too.", bold("You used "+top+" "+times(f.me.usedCertain)+"."),
			othersWho(f.OthersUsedCertain, f.Others)))
	default:
		add(false, fmt.Sprintf("%s Only %s of the %d others used it at all.", bold("You used "+top+" "+times(f.me.usedCertain)+"."),
			word(f.OthersUsedCertain), f.Others))
	}

	// A rating leaned on far more than the group leans on it.
	if l, ok := b.defaultRating(); ok && !isNaN(l.OthersShare) && l.Share-l.OthersShare >= 20 {
		add(false, fmt.Sprintf("%s %s ratings (%d%%), against %d%% for the others.", bold(confidenceWords[l.Level]+" was your default:"),
			nOf(l.N, f.Rated), whole(l.Share), whole(l.OthersShare)))
	}

	// Whether it moved when the number got harder.
	if c := f.ConfChange; !isNaN(c) {
		switch {
		case c <= -10:
			only := ""
			if f.OthersConfFell10*3 <= f.OthersConfChangeN {
				only = "only "
			}
			add(f.OthersConfFell10*2 >= f.OthersConfChangeN, fmt.Sprintf("%s Yours fell %d points from block 1 to block 4; %s%d of the "+
				"%d others with ratings in both blocks fell by 10 or more.", bold("It moved with difficulty."), whole(-c), only,
				f.OthersConfFell10, f.OthersConfChangeN))
		case c >= 10:
			add(f.OthersConfChange >= 10, fmt.Sprintf("%s Yours rose %d points from block 1 to block 4; the others' median change was %s.",
				bold("It went up as the number got harder."), whole(c), signed(f.OthersConfChange)))
		default:
			add(math.Abs(f.OthersConfChange) < 10, fmt.Sprintf("%s Yours changed by %s from block 1 to block 4; the others' "+
				"median change was %s.", bold("It did not move with difficulty."), signedPoints(c), signed(f.OthersConfChange)))
		}
	}
	return like, unlike
}

// ownConfidenceBullets is the same ground without a group.
func (b *builder) ownConfidenceBullets() []string {
	f := b.f
	var out []string
	top := confidenceWords[confidenceLevels[len(confidenceLevels)-1]]
	if !isNaN(f.me.disc) {
		out = append(out, b.discSentence())
	}
	if l, ok := b.defaultRating(); ok {
		out = append(out, fmt.Sprintf("%s %s ratings (%d%%).", bold("Your most used rating was "+confidenceWords[l.Level]+":"),
			nOf(l.N, f.Rated), whole(l.Share)))
	}
	if f.me.usedCertain == 0 {
		out = append(out, bold("You never chose "+top+"."))
	} else {
		out = append(out, fmt.Sprintf("%s That is %s rated answers.", bold("You chose "+top+" "+times(f.me.usedCertain)+"."),
			nOf(f.me.usedCertain, f.Rated)))
	}
	if c := f.ConfChange; !isNaN(c) {
		switch {
		case c <= -10:
			out = append(out, fmt.Sprintf("%s It fell %d points from block 1 to block 4.", bold("It moved with difficulty."), whole(-c)))
		case c >= 10:
			out = append(out, fmt.Sprintf("%s It rose %d points from block 1 to block 4.", bold("It went up as the number got harder."), whole(c)))
		default:
			out = append(out, fmt.Sprintf("%s It changed by %s from block 1 to block 4.", bold("It did not move with difficulty."), signedPoints(c)))
		}
	}
	return out
}

// discSentence says how far apart the ratings on right and wrong
// answers were.
func (b *builder) discSentence() string {
	f := b.f
	d := f.me.disc
	lead := "Your confidence told you something."
	switch {
	case d <= -3:
		lead = "Your confidence pointed the wrong way."
	case d < 3:
		lead = "Your confidence did not tell right from wrong."
	case d < 10:
		lead = "Your confidence told you a little."
	}
	apart := whole(math.Abs(d))
	return fmt.Sprintf("%s It averaged %d on answers that turned out right and %d on ones that turned out wrong, %d %s apart.",
		bold(lead), whole(f.me.confRight), whole(f.me.confWrong), apart, plural(apart, "point", "points"))
}

// defaultRating is the rating a participant used most, when one stands
// clear of the rest.
func (b *builder) defaultRating() (LevelFigures, bool) {
	var best LevelFigures
	tie := false
	for _, l := range b.f.Levels {
		switch {
		case l.N > best.N:
			best, tie = l, false
		case l.N == best.N && l.N > 0:
			tie = true
		}
	}
	return best, best.N > 0 && !tie && best.Share >= 40
}

// fewOthers says how many of the others were further along: "Only three
// people were less confident."
func fewOthers(n, of int, what string) string {
	switch {
	case n == 0:
		return fmt.Sprintf("Nobody was %s.", what)
	case n*3 <= of:
		return fmt.Sprintf("Only %s %s %s %s.", word(n), plural(n, "person", "people"), wasWere(n), what)
	}
	return fmt.Sprintf("%s were %s.", othersWho(n, of), what)
}
