package users

import (
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The curated dataset, as a file somebody can take away.
//
// The dataset is the deliverable (FSD §7b). A deliverable that can only
// be read through an admin console is not one, so this is the handoff to
// whatever does the analysis: R, a notebook, a spreadsheet, or the graph
// projection when it arrives (ADR 0030).
//
// Two properties this export has on purpose.
//
// **The columns come from the view, not from a list in this file.** The
// header and the body are both built by asking Postgres what
// v_dt_answers has, in its own order. A hand-written column list is a
// second definition of the dataset, and the first time somebody adds a
// column to the view and not to the list, the export quietly starts
// shipping something narrower than what the console shows while both
// still look right.
//
// **What the view omits stays omitted.** No name, no email address, no
// chosen_index, no digits. v_dt_answers was built that way because the
// answer key only has to escape once and this is the one function whose
// output is meant to leave the server. There is no filtering here that
// could be forgotten: there is nothing to filter, because the query is
// SELECT * from a view that does not have those columns.

// DTExport is a rendered dataset.
type DTExport struct {
	CSV      string
	Filename string
	Rows     int
	Sessions int
}

// DTExportCSV renders v_dt_answers as RFC 4180 CSV.
//
// Synthetic runs are excluded unless asked for. An agent run is useful
// for checking the instrument and ruinous averaged into a claim about
// people, so it is a decision the caller has to make rather than
// something that rides along.
func (r *Repo) DTExportCSV(ctx context.Context, includeSynthetic bool) (DTExport, error) {
	var out DTExport

	// pgx gives the column names back on the result, in the view's own
	// order, which is what makes this export follow the view rather
	// than a copy of it.
	q := `SELECT * FROM v_dt_answers`
	if !includeSynthetic {
		q += ` WHERE NOT is_synthetic`
	}
	q += ` ORDER BY started_at, session_key, position_overall`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return out, fmt.Errorf("decision test: export: %w", err)
	}
	defer rows.Close()

	var b strings.Builder
	w := csv.NewWriter(&b)

	header := make([]string, 0, len(rows.FieldDescriptions()))
	for _, f := range rows.FieldDescriptions() {
		header = append(header, string(f.Name))
	}
	if err := w.Write(header); err != nil {
		return out, fmt.Errorf("decision test: export header: %w", err)
	}

	sessions := map[string]struct{}{}
	keyCol := indexOf(header, "session_key")

	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return out, fmt.Errorf("decision test: export row: %w", err)
		}
		rec := make([]string, len(vals))
		for i, v := range vals {
			rec[i] = csvValue(v)
		}
		if keyCol >= 0 && keyCol < len(rec) {
			sessions[rec[keyCol]] = struct{}{}
		}
		if err := w.Write(rec); err != nil {
			return out, fmt.Errorf("decision test: export write: %w", err)
		}
		out.Rows++
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("decision test: export scan: %w", err)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return out, fmt.Errorf("decision test: export flush: %w", err)
	}

	out.CSV = b.String()
	out.Sessions = len(sessions)
	out.Filename = fmt.Sprintf("decision-test-%s.csv", time.Now().UTC().Format("2006-01-02"))
	return out, nil
}

// csvValue renders one cell.
//
// A NULL and an empty string both come out as an empty field, which
// loses the distinction between them. That is acceptable here rather
// than merely tolerated: the text columns on v_dt_answers are all NOT
// NULL with a ” default, so an empty text field is always a skipped
// answer and never a missing one. NULL occurs only in numeric columns
// (confidence and latency on an expired question, memory_failure on an
// expired recall, baseline_rt_ms where the tap check gave nothing), and
// an empty numeric field is exactly what R and Postgres both read as
// NA, which is what those nulls mean.
//
// Forcing a literal "" for the empty string is not available:
// csv.Writer escapes whatever it is given, so a two-quote string comes
// out as six quote characters. If the schema ever gains a nullable text
// column this needs a sentinel instead, and the distinction has to be
// made here rather than left to the reader.
func csvValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case time.Time:
		// RFC 3339 in UTC. Local offsets in a dataset are a day lost to
		// timezone archaeology later.
		return t.UTC().Format(time.RFC3339)
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

func indexOf(hay []string, needle string) int {
	for i, s := range hay {
		if s == needle {
			return i
		}
	}
	return -1
}
