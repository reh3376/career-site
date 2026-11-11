package hiring

// The determinations of 03 §2.1 and 04 §2.1. Two of the four are
// "met"; the arithmetic reads them the same and the record flags the
// second so an auditor can see how much rests on equivalency.
const (
	DetMet           = "met"
	DetMetEquivalent = "met_equivalent"
	DetUnresolved    = "unresolved"
	DetNotMet        = "not_met"
)

// The prototype's three-way verdict, which decision_log.output.verdict
// and the owner's graded series keep (FR-HF-07).
const (
	VerdictMet     = "met"
	VerdictPartial = "partial"
	VerdictUnmet   = "unmet"
)

// DeriveVerdict is FR-HF-07: the three-way verdict computed from a
// determination in code, so the 31 graded verdicts stay a comparable
// series across the change (13 §0 decision 9). An unknown
// determination derives unmet rather than partial, because partial
// would read as "the system does not know", and a code the system does
// not recognise is not that.
func DeriveVerdict(determination string) string {
	switch determination {
	case DetMet, DetMetEquivalent:
		return VerdictMet
	case DetUnresolved:
		return VerdictPartial
	default:
		return VerdictUnmet
	}
}

// Item is what the formula needs from one judged requirement: its
// weight, category and class from the requirement, whether it counts at
// all, its effective determination from the judgment, and whether the
// judgment cites any candidate_response evidence.
type Item struct {
	Weight   int
	Category string // must | nice, as the posting stated it (formula v3)
	// Class is required | preferred | ideal (formula v4, FR-HF-23).
	// Empty means the class the category implies.
	Class string
	// Excluded marks a years or credential item for which no
	// equivalency was defined (FR-HF-23): shown, judged, and left out of
	// the numbers. Read by formula v4 only.
	Excluded      bool
	Determination string // effective: override if set, else post-rule
	SelfAttested  bool
}

// Coverage is the match record's arithmetic (FR-HF-06): three numbers
// that sum to one over the denominator, and two shares of the evidenced
// part. Denominator is the weight the three numbers are fractions of,
// kept so a reader can tell a record with one requirement from one with
// twenty; under formula v4 it is fractional, because a preferred item
// counts at a fraction of its weight.
type Coverage struct {
	Evidenced         float64
	Unresolved        float64
	Unmet             float64
	EquivalentShare   float64 // of Evidenced, from met_equivalent
	SelfAttestedShare float64 // of Evidenced, resting on candidate_response evidence
	Denominator       float64
}

// DefaultPreferredFraction is the share of its weight a preferred item
// counts at unless the setting says otherwise (FR-HF-23).
const DefaultPreferredFraction = 0.25

// CoverageV4 is FormulaV4: the three numbers over the hiring manager's
// classes (FR-HF-23). A required item counts at its full weight and is
// always in the denominator. A preferred item counts at
// preferredFraction of its weight and is always in the denominator, so
// an unmet preference costs a little rather than nothing. An ideal
// item is bonus credit only: it joins the arithmetic, at the same
// fraction, when it is evidenced and never otherwise, so it can raise
// a figure and never lower one. An excluded item, and a weight 0 item
// (behavioral, FR-HF-18), is left out of everything. unresolved is its
// own number and never half of anything.
func CoverageV4(items []Item, preferredFraction float64) Coverage {
	if preferredFraction <= 0 || preferredFraction > 1 {
		preferredFraction = DefaultPreferredFraction
	}
	var denom, evidenced, unresolved, unmet, equivalent, selfAttested float64
	for _, it := range items {
		if it.Weight <= 0 || it.Excluded {
			continue
		}
		class := it.Class
		if class == "" {
			class = ClassFromCategory(it.Category)
		}
		isEvidenced := it.Determination == DetMet || it.Determination == DetMetEquivalent
		w := float64(it.Weight)
		switch class {
		case ClassRequired:
		case ClassIdeal:
			if !isEvidenced {
				continue
			}
			w *= preferredFraction
		default: // preferred, and any class this build does not know
			w *= preferredFraction
		}
		denom += w
		switch {
		case isEvidenced:
			evidenced += w
			if it.Determination == DetMetEquivalent {
				equivalent += w
			}
			if it.SelfAttested {
				selfAttested += w
			}
		case it.Determination == DetUnresolved:
			unresolved += w
		default:
			unmet += w
		}
	}
	if denom == 0 {
		return Coverage{}
	}
	c := Coverage{
		Evidenced:   evidenced / denom,
		Unresolved:  unresolved / denom,
		Unmet:       unmet / denom,
		Denominator: denom,
	}
	if evidenced > 0 {
		c.EquivalentShare = equivalent / evidenced
		c.SelfAttestedShare = selfAttested / evidenced
	}
	return c
}

// CoverageV3 is FormulaV3 over effective determinations. Must items
// are always in the denominator; nice items only when evidenced, the
// v2 rule, so a posting's wish list can add to a score and never take
// from it. Weight 0 items (behavioral, FR-HF-18) are excluded from
// everything. unresolved is reported as its own number and never
// scored as half of anything. Kept for records written under it; new
// records are written under CoverageV4.
func CoverageV3(items []Item) Coverage {
	var denom, evidenced, unresolved, unmet, equivalent, selfAttested int
	for _, it := range items {
		if it.Weight <= 0 {
			continue
		}
		isEvidenced := it.Determination == DetMet || it.Determination == DetMetEquivalent
		must := it.Category == "must"
		if !must && !isEvidenced {
			continue
		}
		denom += it.Weight
		switch {
		case isEvidenced:
			evidenced += it.Weight
			if it.Determination == DetMetEquivalent {
				equivalent += it.Weight
			}
			if it.SelfAttested {
				selfAttested += it.Weight
			}
		case it.Determination == DetUnresolved:
			unresolved += it.Weight
		default:
			unmet += it.Weight
		}
	}
	if denom == 0 {
		return Coverage{}
	}
	c := Coverage{
		Evidenced:   float64(evidenced) / float64(denom),
		Unresolved:  float64(unresolved) / float64(denom),
		Unmet:       float64(unmet) / float64(denom),
		Denominator: float64(denom),
	}
	if evidenced > 0 {
		c.EquivalentShare = float64(equivalent) / float64(evidenced)
		c.SelfAttestedShare = float64(selfAttested) / float64(evidenced)
	}
	return c
}
