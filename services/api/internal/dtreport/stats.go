package dtreport

import (
	"math"
	"sort"
)

// The few statistics the report states. Every one is a count, a mean or
// a median over at most a few hundred numbers, so they are written out
// rather than imported: the definitions are the interesting part, and a
// reader checking a percentile should find it in ten lines.

// nan marks a figure that does not exist for a person, such as the
// average confidence of somebody who rated nothing. It is kept as NaN
// rather than zero because zero is a real rating-scale value and every
// consumer downstream would believe it.
var nan = math.NaN()

func isNaN(v float64) bool { return math.IsNaN(v) }

// finite drops the figures that do not exist.
func finite(vs []float64) []float64 {
	out := make([]float64, 0, len(vs))
	for _, v := range vs {
		if !math.IsNaN(v) {
			out = append(out, v)
		}
	}
	return out
}

func mean(vs []float64) float64 {
	vs = finite(vs)
	if len(vs) == 0 {
		return nan
	}
	var s float64
	for _, v := range vs {
		s += v
	}
	return s / float64(len(vs))
}

// median is the middle value, or the mean of the middle two.
func median(vs []float64) float64 {
	vs = finite(vs)
	if len(vs) == 0 {
		return nan
	}
	s := append([]float64(nil), vs...)
	sort.Float64s(s)
	m := len(s) / 2
	if len(s)%2 == 1 {
		return s[m]
	}
	return (s[m-1] + s[m]) / 2
}

func minMax(vs []float64) (lo, hi float64) {
	vs = finite(vs)
	if len(vs) == 0 {
		return nan, nan
	}
	lo, hi = vs[0], vs[0]
	for _, v := range vs[1:] {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	return lo, hi
}

// same compares two figures the way the report prints them. Averages
// arrive as floats, and two people who both averaged 62.5 must be level
// rather than separated by the last bit of a division.
func same(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// countWhere counts the figures a predicate accepts, skipping the ones
// that do not exist.
func countWhere(vs []float64, keep func(float64) bool) int {
	n := 0
	for _, v := range vs {
		if !math.IsNaN(v) && keep(v) {
			n++
		}
	}
	return n
}

// placing is where one figure sits among a group that includes it.
type placing struct {
	// Rank counts from 1, ties sharing the better place: two people
	// level for second are both 2nd and the next is 4th.
	Rank int
	// Tied is how many others are level.
	Tied int
	// Of is the size of the group the rank is out of, which can be
	// smaller than the comparison group when a figure does not exist
	// for everybody.
	Of int
}

// place ranks mine among others. higherFirst says which end is first.
func place(mine float64, others []float64, higherFirst bool) placing {
	others = finite(others)
	p := placing{Rank: 1, Of: len(others) + 1}
	for _, v := range others {
		switch {
		case same(v, mine):
			p.Tied++
		case higherFirst && v > mine, !higherFirst && v < mine:
			p.Rank++
		}
	}
	return p
}

// percentileRank is the share of the group a score is above, counting
// the person and anyone level with them as half a place each. It is the
// definition the report prints beside the number, so that 5th of 16 and
// the 72nd percentile can be reconciled by hand.
func percentileRank(mine float64, others []float64) float64 {
	others = finite(others)
	below, equal := 0, 0
	for _, v := range others {
		switch {
		case same(v, mine):
			equal++
		case v < mine:
			below++
		}
	}
	return (float64(below) + 0.5*float64(equal+1)) / float64(len(others)+1) * 100
}

// niceAxis picks limits and ticks for a strip of dots, so a chart of
// twelve people and a chart of four hundred both end on a round number.
// floor and ceil pin an end the measure itself fixes (a score cannot be
// below zero or above thirty); pass NaN to leave an end to the data.
//
// counts says the measure is a whole number of things, so a tick never
// lands on two and a half answers.
func niceAxis(vs []float64, floor, ceil float64, counts bool) (lo, hi float64, ticks []float64) {
	dlo, dhi := minMax(vs)
	if isNaN(dlo) {
		dlo, dhi = 0, 1
	}
	if !isNaN(floor) {
		dlo = math.Min(dlo, floor)
	}
	if !isNaN(ceil) {
		dhi = math.Max(dhi, ceil)
	}
	if dhi-dlo < 1e-9 {
		dhi = dlo + 1
	}
	// A step from 1, 2, 2.5, 5 times a power of ten that gives about
	// four intervals.
	raw := (dhi - dlo) / 4
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	step := mag
	if counts && mag < 1 {
		mag = 1
	}
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if counts && m*mag != math.Trunc(m*mag) {
			continue
		}
		if step = m * mag; step >= raw-1e-9 {
			break
		}
	}
	lo = math.Floor(dlo/step+1e-9) * step
	hi = math.Ceil(dhi/step-1e-9) * step
	if !isNaN(floor) && lo < floor {
		lo = floor
	}
	if !isNaN(ceil) && hi > ceil {
		hi = ceil
	}
	for t := lo; t <= hi+1e-9; t += step {
		ticks = append(ticks, math.Round(t*1000)/1000)
	}
	return lo, hi, ticks
}
