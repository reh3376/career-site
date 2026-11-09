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
// weight and category from the requirement, its effective
// determination from the judgment, and whether the judgment cites any
// candidate_response evidence.
type Item struct {
	Weight        int
	Category      string // must | nice
	Determination string // effective: override if set, else post-rule
	SelfAttested  bool
}

// Coverage is the match record's arithmetic (FR-HF-06): three numbers
// that sum to one over the denominator, and two shares of the evidenced
// part. Denominator is the weight the three numbers are fractions of,
// kept so a reader can tell a record with one requirement from one with
// twenty.
type Coverage struct {
	Evidenced         float64
	Unresolved        float64
	Unmet             float64
	EquivalentShare   float64 // of Evidenced, from met_equivalent
	SelfAttestedShare float64 // of Evidenced, resting on candidate_response evidence
	Denominator       int
}

// CoverageV3 is FormulaV3 over effective determinations. Must items
// are always in the denominator; nice items only when evidenced, the
// v2 rule, so a posting's wish list can add to a score and never take
// from it. Weight 0 items (behavioral, FR-HF-18) are excluded from
// everything. unresolved is reported as its own number and never
// scored as half of anything.
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
		Denominator: denom,
	}
	if evidenced > 0 {
		c.EquivalentShare = float64(equivalent) / float64(evidenced)
		c.SelfAttestedShare = float64(selfAttested) / float64(evidenced)
	}
	return c
}
