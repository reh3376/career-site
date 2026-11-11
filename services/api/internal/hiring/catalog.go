package hiring

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// The equivalence catalog as the extraction stage reads it (05 §1.3
// step 1): for each requirement, the active entries that speak to what
// it asks for are rendered into its equivalence notes, where the hiring
// manager sees them and can rewrite them, and the judge is shown them.
//
// An entry speaks to a requirement when the concept on its `to` side
// (either side of a symmetric entry) matches the requirement's text or
// source quote by one of the concept's match terms. Matching is
// deliberately plain: phrases on word boundaries. The catalog is
// expected to be wrong in places (05 status), and a matcher clever
// enough to hide that would be worse than one a person can predict.

// CatalogEntry is one active assertion with the labels and terms of
// the two concepts it relates.
type CatalogEntry struct {
	ID         string
	Kind       string
	Direction  string // one_way | symmetric
	Strength   string // full | partial | none
	Conditions []string
	Rationale  string
	FromLabel  string
	ToLabel    string
	FromTerms  []string
	ToTerms    []string
}

// Catalog is the active entries, loaded once per run.
type Catalog struct {
	Entries []CatalogEntry
}

// maxCatalogNotes caps how many entries are rendered into one
// requirement's notes; a requirement that matches more than this is
// matching on a term that is too common, and the first few are enough
// to show it.
const maxCatalogNotes = 4

// LoadCatalog reads the active entries of the shared base catalog and
// the tenant's own.
func (r *Repo) LoadCatalog(ctx context.Context) (*Catalog, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT e.id::text, e.kind, e.direction, e.strength, e.conditions, e.rationale,
           f.label, t.label, f.match_terms, t.match_terms
    FROM equivalence_catalog e
    JOIN concepts f ON f.id = e.from_concept
    JOIN concepts t ON t.id = e.to_concept
    WHERE e.status = 'active'
    ORDER BY e.kind, f.slug, t.slug`)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}
	defer rows.Close()
	c := &Catalog{}
	for rows.Next() {
		var e CatalogEntry
		if err := rows.Scan(&e.ID, &e.Kind, &e.Direction, &e.Strength, &e.Conditions, &e.Rationale,
			&e.FromLabel, &e.ToLabel, &e.FromTerms, &e.ToTerms); err != nil {
			return nil, fmt.Errorf("load catalog: scan: %w", err)
		}
		c.Entries = append(c.Entries, e)
	}
	return c, rows.Err()
}

// Notes renders the entries that speak to a requirement, one sentence
// each, and returns them with the ids of the entries used. Full
// entries first, then partial, then the ones that say two things are
// not equivalent, so the notes read from what counts to what does not.
// An empty catalog, or a requirement nothing speaks to, gives "".
func (c *Catalog) Notes(text, sourceQuote string) (notes string, entryIDs []string) {
	if c == nil || len(c.Entries) == 0 {
		return "", nil
	}
	hay := normaliseForMatch(text + " \n " + sourceQuote)
	type line struct {
		rank int
		text string
		id   string
	}
	var lines []line
	seen := map[string]bool{}
	for _, e := range c.Entries {
		// What the requirement asks for is the `to` side; a symmetric
		// entry may be read from either end.
		offered, asked := "", ""
		switch {
		case matchesAny(hay, e.ToTerms):
			offered, asked = e.FromLabel, e.ToLabel
		case e.Direction == "symmetric" && matchesAny(hay, e.FromTerms):
			offered, asked = e.ToLabel, e.FromLabel
		default:
			continue
		}
		var s string
		rank := 0
		switch e.Strength {
		case "full":
			s = fmt.Sprintf("%s counts as %s.", offered, lowerFirst(asked))
		case "partial":
			rank = 1
			s = fmt.Sprintf("%s may count toward %s; it is not confirmed, so treat it as a question.", offered, lowerFirst(asked))
		case "none":
			rank = 2
			s = fmt.Sprintf("%s is not %s.", offered, lowerFirst(asked))
		default:
			continue
		}
		if len(e.Conditions) > 0 {
			s += " " + strings.TrimRight(strings.Join(e.Conditions, "; "), ".") + "."
		} else if e.Strength == "none" && e.Rationale != "" {
			s += " " + strings.TrimRight(e.Rationale, ".") + "."
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		lines = append(lines, line{rank: rank, text: s, id: e.ID})
	}
	if len(lines) == 0 {
		return "", nil
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].rank < lines[j].rank })
	if len(lines) > maxCatalogNotes {
		lines = lines[:maxCatalogNotes]
	}
	var b strings.Builder
	for _, l := range lines {
		if b.Len()+len(l.text)+1 > MaxEquivalenceNotes {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(l.text)
		entryIDs = append(entryIDs, l.id)
	}
	return b.String(), entryIDs
}

// normaliseForMatch lower-cases and collapses whitespace, with a space
// at each end so a term can be tested on word boundaries by plain
// substring search.
func normaliseForMatch(s string) string {
	return " " + strings.ToLower(strings.Join(strings.Fields(s), " ")) + " "
}

// matchesAny reports whether any term occurs in the normalised text on
// word boundaries. A term ending in * matches as a prefix: the start of
// the term must be on a boundary and its end need not be.
func matchesAny(hay string, terms []string) bool {
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		prefix := strings.HasSuffix(term, "*")
		term = strings.TrimSuffix(term, "*")
		if term == "" {
			continue
		}
		from := 0
		for {
			i := strings.Index(hay[from:], term)
			if i < 0 {
				break
			}
			start := from + i
			end := start + len(term)
			if isBoundary(hay, start-1) && (prefix || isBoundary(hay, end)) {
				return true
			}
			from = start + 1
		}
	}
	return false
}

// isBoundary reports whether the byte at i is not part of a word. Out
// of range counts as a boundary.
func isBoundary(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return true
	}
	r := rune(s[i])
	return !(unicode.IsLetter(r) || unicode.IsDigit(r))
}

// lowerFirst lower-cases a label's first letter for use mid-sentence,
// leaving acronyms and proper names (a second capital, or a capital
// after the first word starts) alone.
func lowerFirst(s string) string {
	if len(s) < 2 {
		return s
	}
	r := []rune(s)
	if unicode.IsUpper(r[0]) && unicode.IsLower(r[1]) && !startsWithProperName(s) {
		r[0] = unicode.ToLower(r[0])
	}
	return string(r)
}

// startsWithProperName keeps vendor and product names capitalised.
func startsWithProperName(s string) bool {
	for _, p := range []string{"Siemens", "Rockwell", "Ignition", "Wonderware", "Grafana", "Power BI", "PostgreSQL", "Go", "Python", "Director", "AI"} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
