package dtreport

import (
	"fmt"
	"math"
	"strings"
)

// Wording helpers. Small, and tested, because a report that prints
// "1 questions" or opens a sentence with a digit reads as generated.

var numberWords = []string{
	"zero", "one", "two", "three", "four", "five", "six", "seven", "eight",
	"nine", "ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen",
	"sixteen", "seventeen", "eighteen", "nineteen",
}

var tensWords = []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}

// word spells a count below a hundred, for use inside a sentence:
// "three", "thirty", "forty-two". A larger count stays in digits, where
// the words would be longer than they are worth.
func word(n int) string {
	switch {
	case n < 0 || n >= 100:
		return fmt.Sprintf("%d", n)
	case n < len(numberWords):
		return numberWords[n]
	case n%10 == 0:
		return tensWords[n/10]
	}
	return tensWords[n/10] + "-" + numberWords[n%10]
}

// Word spells a count with a capital, for the start of a sentence, so
// that no sentence in a report opens with a digit while the group is
// under a hundred.
func Word(n int) string {
	w := word(n)
	if w == "" {
		return w
	}
	return strings.ToUpper(w[:1]) + w[1:]
}

// ordinal renders 1 as 1st, 22 as 22nd, 113 as 113th.
func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

// plural picks the singular for exactly one.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// wasWere agrees a verb with a count.
func wasWere(n int) string { return plural(n, "was", "were") }

// nOf renders "3 of 10".
func nOf(k, n int) string { return fmt.Sprintf("%d of %d", k, n) }

// whole rounds half away from zero, the way a reader does.
func whole(v float64) int { return int(math.Round(v)) }

// num prints a figure that is usually whole, such as a median of
// scores, without a trailing ".0" and without hiding a real half.
func num(v float64) string {
	if math.Abs(v-math.Round(v)) < 0.05 {
		return fmt.Sprintf("%d", whole(v))
	}
	return fmt.Sprintf("%.1f", v)
}

// signed prints a whole number with its sign, and zero without one.
func signed(v float64) string {
	n := whole(v)
	if n == 0 {
		return "0"
	}
	return fmt.Sprintf("%+d", n)
}

// points prints a count of points: "1 point", "8 points".
func points(n int) string { return fmt.Sprintf("%d %s", n, plural(n, "point", "points")) }

// signedPoints prints a movement in points with its sign: "+1 point",
// "-3 points", "0 points".
func signedPoints(v float64) string {
	n := whole(v)
	if n < 0 {
		n = -n
	}
	return signed(v) + " " + plural(n, "point", "points")
}

// secs prints seconds to one decimal.
func secs(v float64) string { return fmt.Sprintf("%.1f", v) }

// esc makes text from the database safe inside the renderer's markup,
// which reads <b> as bold. Questions contain ampersands and inequality
// signs often enough that this is not hypothetical.
func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// bold wraps a lead-in.
func bold(s string) string { return "<b>" + s + "</b>" }

// joinAnd renders a list the way a sentence does: "a", "a and b",
// "a, b and c".
func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// capitalise raises the first letter of a sentence built from parts.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
