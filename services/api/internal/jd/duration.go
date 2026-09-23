package jd

import (
	"regexp"
	"strconv"
	"strings"
)

// Duration requirements are decided in code, not by the judge.
//
// The reason is the same one that keeps the score out of the model: a
// comparison between two numbers is arithmetic, and arithmetic belongs
// where it can be read and tested. Three prompt versions tried to teach
// a 4B model to check a span before answering "met" and all three
// failed, the last one producing a verdict identical to the previous
// run down to the wording of the rationale. The model is good at
// finding what the evidence says. It is not good at policing its own
// inference. So it reports the span it found, and this file decides.
//
// What the model supplies: stated_span_years, the number of years the
// evidence explicitly states for the work in question, 0 when none.
// What this file supplies: how many years the requirement demands, read
// from the requirement's own text, and the comparison.

// requiredYearsRe matches the ways postings ask for a length of service.
// Deliberately narrow: a number next to a time word. Anything cleverer
// starts matching salaries, team sizes and version numbers.
var requiredYearsRe = regexp.MustCompile(`(?i)(\d{1,2})\s*\+?\s*(?:or more\s*)?(?:years?|yrs?)`)

// writtenYears covers the small numbers postings spell out. Beyond ten
// they are written as digits in practice, and a longer list would be
// more surface than it is worth.
var writtenYears = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

var writtenYearsRe = regexp.MustCompile(`(?i)\b(one|two|three|four|five|six|seven|eight|nine|ten)\s+(?:or more\s+)?(?:years?|yrs?)`)

// RequiredYears reports how many years a requirement demands, and
// whether it demands any at all.
//
// When a requirement names more than one span ("10+ years of software
// engineering with 3+ years leading teams") the largest is returned.
// That is the requirement's headline ask; the lesser one is a detail
// inside it, and taking the smaller number would let a candidate clear
// the requirement on the easier half.
func RequiredYears(text string) (int, bool) {
	best := 0
	for _, m := range requiredYearsRe.FindAllStringSubmatch(text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > best && n <= 60 {
			best = n
		}
	}
	for _, m := range writtenYearsRe.FindAllStringSubmatch(text, -1) {
		if n, ok := writtenYears[strings.ToLower(m[1])]; ok && n > best {
			best = n
		}
	}
	return best, best > 0
}

// applyDurationRule downgrades a verdict the evidence does not support.
//
// It only ever weakens a verdict. A judge that says "unmet" is left
// alone: this rule exists to stop a span being invented, not to argue
// a candidate up. And it downgrades to "partial" rather than "unmet",
// because the work itself was evidenced; what is missing is proof of
// how long, which is a lesser claim rather than no claim.
//
// Returns the verdict to store and a note when it changed, so the
// adjustment is visible in the derivation rather than silent.
func applyDurationRule(requirementText, verdict string, statedSpanYears float64) (string, string) {
	if verdict != "met" {
		return verdict, ""
	}
	need, asks := RequiredYears(requirementText)
	if !asks {
		return verdict, ""
	}
	if statedSpanYears >= float64(need) {
		return verdict, ""
	}
	if statedSpanYears <= 0 {
		return "partial", "asks for " + strconv.Itoa(need) +
			" years; the evidence shows the work but states no span for it"
	}
	return "partial", "asks for " + strconv.Itoa(need) + " years; the evidence states " +
		strconv.FormatFloat(statedSpanYears, 'g', -1, 64)
}
