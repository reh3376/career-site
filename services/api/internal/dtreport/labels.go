package dtreport

import (
	"fmt"
	"sort"
	"strings"
)

// The words for things the database names with a code.

func familyLabel(f string) string {
	switch f {
	case "syllogism":
		return "Syllogism"
	case "arithmetic":
		return "Arithmetic"
	case "conjunction":
		return "Conjunction"
	case "base_rate":
		return "Base rate"
	}
	return capitalise(strings.ReplaceAll(f, "_", " "))
}

// familyPlural names a kind of question in the middle of a sentence.
func familyPlural(f string) string {
	switch f {
	case "syllogism":
		return "syllogisms"
	case "arithmetic":
		return "arithmetic questions"
	case "conjunction":
		return "conjunction questions"
	case "base_rate":
		return "base-rate questions"
	}
	return strings.ReplaceAll(f, "_", " ") + " questions"
}

// familyCount is "ten syllogisms" or "one base-rate question".
func familyCount(f string, n int) string {
	if f == "syllogism" {
		return fmt.Sprintf("%s %s (does the conclusion follow?)", word(n), plural(n, "syllogism", "syllogisms"))
	}
	name := familyPlural(f)
	if n == 1 {
		name = strings.TrimSuffix(name, "s")
	}
	return word(n) + " " + name
}

// What each block asked the participant to hold, three ways: the size
// of the number, what had to come back, and the short form under a
// chart.
type loadWords struct{ number, giveBack, task, short string }

func loadText(load string) loadWords {
	switch load {
	case "d3":
		return loadWords{"3 digits", "The digits as shown", "3 digits, as shown", "3 digits"}
	case "d4":
		return loadWords{"4 digits", "The digits as shown", "4 digits, as shown", "4 digits"}
	case "d4_plus1":
		return loadWords{"4 digits", "Each digit with 1 added", "4 digits, add 1 to each", "4 digits, add 1"}
	case "d4_plus3":
		return loadWords{"4 digits", "Each digit with 3 added", "4 digits, add 3 to each", "4 digits, add 3"}
	case "d3_control":
		return loadWords{"3 digits", "The digits as shown. Same difficulty as block 1, to check for tiring",
			"3 digits, as shown", "3 digits (control)"}
	}
	return loadWords{load, "", load, load}
}

func outcomeLabel(o string) string {
	switch o {
	case "correct":
		return "Correct"
	case "lure":
		return "Intuitive trap"
	case "expired":
		return "Ran out of time"
	}
	return "Other wrong"
}

// recallResult describes a number as given back.
func recallResult(r Recall) string {
	switch r.Outcome {
	case "exact":
		return "Exact"
	case "partial":
		return fmt.Sprintf("Partly right, %d of %d digits in place", r.DigitsHeld, len(r.Expected))
	case "untransformed":
		return "Number held, rule not applied"
	case "expired":
		return "Ran out of time"
	}
	if rightDigitsWrongOrder(r) {
		return "Right digits, wrong order"
	}
	return "Wrong digits"
}

// rightDigitsWrongOrder is the one failure worth telling apart from a
// lost number: every digit came back and the positions did not.
func rightDigitsWrongOrder(r Recall) bool {
	if r.Outcome == "exact" || r.Response == "" || len(r.Response) != len(r.Expected) {
		return false
	}
	a, b := []byte(r.Response), []byte(r.Expected)
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })
	return string(a) == string(b)
}

// sureWords names the ratings at or above the threshold: "Fairly sure
// or Certain".
func sureWords(threshold int) string {
	var names []string
	for _, l := range confidenceLevels {
		if l >= threshold {
			names = append(names, confidenceWords[l])
		}
	}
	if len(names) == 0 {
		return "Certain"
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// rankText is "5th" or "joint 5th".
func rankText(p placing) string {
	if p.Tied > 0 {
		return "joint " + ordinal(p.Rank)
	}
	return ordinal(p.Rank)
}

// placingText is a placing as a line of its own: "5th highest of 16",
// "Joint 3rd fewest of 16", "Joint fewest, with 5 others".
func placingText(p placing, end string) string {
	switch {
	case p.Rank == 1 && p.Tied > 0:
		return fmt.Sprintf("Joint %s, with %d %s", end, p.Tied, plural(p.Tied, "other", "others"))
	case p.Rank == 1:
		return fmt.Sprintf("The %s of %d", end, p.Of)
	case p.Tied > 0:
		return fmt.Sprintf("Joint %s %s of %d", ordinal(p.Rank), end, p.Of)
	}
	return fmt.Sprintf("%s %s of %d", ordinal(p.Rank), end, p.Of)
}

// rankPhrase is a placing inside a sentence: "the 2nd slowest of 16",
// "the slowest of 16".
func rankPhrase(p placing, end string) string {
	joint := ""
	if p.Tied > 0 {
		joint = "joint "
	}
	if p.Rank == 1 {
		return fmt.Sprintf("the %s%s of %d", joint, end, p.Of)
	}
	return fmt.Sprintf("the %s%s %s of %d", joint, ordinal(p.Rank), end, p.Of)
}

// nearer words a placing from whichever end of the group it is closer
// to. Somebody with the second most confident mistakes of twelve is
// "2nd most", not "11th fewest".
func nearer(low placing, lowEnd string, high placing, highEnd string) (placing, string) {
	if high.Rank < low.Rank {
		return high, highEnd
	}
	return low, lowEnd
}

// othersWho counts some of the others at the start of a sentence: "Eight
// of the 11 others", "All 11 others", "None of the 11 others".
func othersWho(n, of int) string {
	switch {
	case of > 0 && n == of:
		return fmt.Sprintf("All %d others", of)
	case n == 0:
		return fmt.Sprintf("None of the %d others", of)
	}
	return fmt.Sprintf("%s of the %d others", Word(n), of)
}

// minutes is "12 minutes" or "1 minute".
func minutes(n int) string { return fmt.Sprintf("%d %s", n, plural(n, "minute", "minutes")) }

// times is "once", "twice" or "5 times".
func times(n int) string {
	switch n {
	case 1:
		return "once"
	case 2:
		return "twice"
	}
	return fmt.Sprintf("%d times", n)
}
