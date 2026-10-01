package users

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// The admin query list: what Ask Roger can read out of the database.
//
// Roger asked for the assistant to be able to query the database, then
// for the admin to choose the query from a dropdown, then for the
// result to come back as a table. All three instructions pull the same
// way, and together they describe something simpler and safer than what
// "let the assistant query the database" first suggests.
//
//   - **The model never writes SQL.** Every statement here is fixed and
//     takes no argument from anywhere. There is no string being built,
//     so there is nothing to inject into.
//   - **The model never chooses.** A person picks from the list, which
//     removes the classification step. On this hardware that step would
//     have cost a second model call, about ten seconds, and been the
//     least reliable part of the feature.
//   - **No model is involved at all.** A fixed statement runs and the
//     rows come back exact, in milliseconds.
//
// # Rows and columns, not sentences
//
// The first version returned one concatenated sentence per query,
// "5 total, 4 active, 0 awaiting approval". It worked and it was the
// wrong shape: a count per line is read by scanning a column, and a
// sentence has to be parsed by eye every time. Each query now returns
// whatever columns it naturally has, and the surface draws a table.
//
// # Counts, never rows about a person
//
// Nothing here selects a name, an address or a message body. Results
// are rendered into a surface and may be quoted into a prompt, prompts
// are written to decision_log, and that is exported as training data,
// so a person's details must not be reachable from here.

// AdminQuery is one named, fixed query.
type AdminQuery struct {
	// ID is the stable name the client sends back. Matched against this
	// list, never interpolated into SQL.
	ID string
	// Label is what the dropdown shows.
	Label string
	// Detail says what the numbers mean.
	Detail string
	sql    string
}

// AdminQueryResult is a small table.
type AdminQueryResult struct {
	Columns []string
	Rows    [][]string
}

// adminQueryRowCap bounds what one query may return. Every statement
// here aggregates, so a result longer than this means a GROUP BY went
// wider than expected, and truncating is better than pasting a thousand
// rows into a panel.
const adminQueryRowCap = 100

// adminQueries is the whole allowlist. Adding one is an edit here and
// nowhere else: what the assistant can read is a list a person
// maintains, not a capability it has.
var adminQueries = []AdminQuery{
	{
		ID: "members", Label: "Members",
		Detail: "Registered accounts by status.",
		sql: `SELECT status::text AS status, count(*) AS accounts
		        FROM users GROUP BY status ORDER BY count(*) DESC`,
	},
	{
		ID: "jd_submissions", Label: "JD submissions",
		Detail: "Job descriptions submitted to the reviewer, by day, most recent first.",
		sql: `SELECT created_at::date::text AS day, count(*) AS submissions
		        FROM jd_submissions
		       WHERE created_at > now() - interval '30 days'
		       GROUP BY 1 ORDER BY 1 DESC`,
	},
	{
		ID: "ask_roger", Label: "Ask Roger usage",
		Detail: "Messages by who wrote them, plus conversations opened.",
		sql: `SELECT role AS wrote, count(*) AS messages
		        FROM chat_messages GROUP BY role
		       UNION ALL
		      SELECT 'conversations', count(*)
		        FROM chat_conversations WHERE deleted_at IS NULL
		       ORDER BY 2 DESC`,
	},
	{
		ID: "answer_paths", Label: "How answers were produced",
		Detail: "The path each answer took, and what it cost. This is where to look to see whether the Q&A bank is firing.",
		sql: `SELECT input->>'path' AS path,
		             count(*) AS answers,
		             round(avg((input->'timings'->>'total_ms')::numeric)/1000, 1) AS avg_seconds,
		             round(avg(completion_tokens)) AS avg_out_tokens
		        FROM decision_log
		       WHERE kind = 'chat_answer'
		       GROUP BY 1 ORDER BY 2 DESC`,
	},
	{
		ID: "qa_bank", Label: "Q&A bank",
		Detail: "Entries and their phrasings. Anything not embedded cannot match.",
		sql: `SELECT CASE WHEN e.enabled THEN 'approved' ELSE 'not approved' END AS state,
		             count(DISTINCT e.id) AS entries,
		             count(p.id) AS phrasings,
		             count(p.id) FILTER (WHERE p.embedding IS NULL) AS not_embedded
		        FROM qa_entries e LEFT JOIN qa_phrasings p ON p.entry_id = e.id
		       GROUP BY 1 ORDER BY 1`,
	},
	{
		ID: "corpus", Label: "Corpus",
		Detail: "What the assistant and the reviewer retrieve from, by visibility.",
		sql: `SELECT d.visibility::text AS visibility,
		             count(DISTINCT d.id) AS documents,
		             count(c.id) AS chunks,
		             count(c.id) FILTER (WHERE c.embedding IS NULL) AS not_embedded
		        FROM corpus_documents d LEFT JOIN corpus_chunks c ON c.document_id = d.id
		       GROUP BY 1 ORDER BY 1`,
	},
	{
		ID: "corpus_by_kind", Label: "Corpus by document kind",
		Detail: "Which kinds of document dominate retrieval. A kind with far more chunks than the rest wins questions it should not.",
		sql: `SELECT d.source_kind AS kind,
		             count(DISTINCT d.id) AS documents,
		             count(c.id) AS chunks
		        FROM corpus_documents d LEFT JOIN corpus_chunks c ON c.document_id = d.id
		       GROUP BY 1 ORDER BY 3 DESC`,
	},
	{
		ID: "meetings", Label: "Meetings",
		Detail: "Bookings on the calendar.",
		sql: `SELECT CASE
		               WHEN cancelled_at IS NOT NULL THEN 'cancelled'
		               WHEN held && tstzrange(now(), 'infinity') THEN 'upcoming'
		               ELSE 'past'
		             END AS state,
		             count(*) AS bookings
		        FROM meeting_bookings GROUP BY 1 ORDER BY 2 DESC`,
	},
	{
		ID: "model_usage", Label: "Model usage, last 7 days",
		Detail: "Calls, tokens and failures by what asked for them.",
		sql: `SELECT kind AS asked_by,
		             count(*) AS calls,
		             coalesce(sum(prompt_tokens), 0) AS prompt_tokens,
		             coalesce(sum(completion_tokens), 0) AS output_tokens,
		             count(*) FILTER (WHERE NOT ok) AS failed
		        FROM llm_usage
		       WHERE created_at > now() - interval '7 days'
		       GROUP BY 1 ORDER BY 2 DESC`,
	},
	{
		ID: "review_queue", Label: "Decisions to review",
		Detail: "The training set. A graded row can be counted; one with a correction can be learned from.",
		sql: `SELECT kind,
		             count(*) AS total,
		             count(*) FILTER (WHERE reviewed_at IS NOT NULL) AS graded,
		             count(*) FILTER (WHERE human_answer <> '') AS with_correction
		        FROM decision_log GROUP BY 1 ORDER BY 2 DESC`,
	},
}

// ErrNoSuchAdminQuery is returned for an id outside the list.
var ErrNoSuchAdminQuery = errors.New("no such query")

// ListAdminQueries returns the list for the dropdown. The SQL stays in
// this package; a caller gets names and descriptions only.
func ListAdminQueries() []AdminQuery {
	out := make([]AdminQuery, 0, len(adminQueries))
	for _, q := range adminQueries {
		out = append(out, AdminQuery{ID: q.ID, Label: q.Label, Detail: q.Detail})
	}
	return out
}

// RunAdminQuery runs the named query and returns its rows.
//
// Column names come from the statement rather than from a declaration
// alongside it, so a query edited here cannot drift out of step with a
// header written somewhere else.
func (r *Repo) RunAdminQuery(ctx context.Context, id string) (AdminQuery, AdminQueryResult, error) {
	var found *AdminQuery
	for i := range adminQueries {
		if adminQueries[i].ID == id {
			found = &adminQueries[i]
			break
		}
	}
	if found == nil {
		return AdminQuery{}, AdminQueryResult{}, ErrNoSuchAdminQuery
	}

	// Bounded hard: these aggregate over small tables, so anything slow
	// enough to notice means something is wrong and the caller should
	// be told rather than left waiting.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, found.sql)
	if err != nil {
		return AdminQuery{}, AdminQueryResult{}, err
	}
	defer rows.Close()

	out := AdminQueryResult{}
	for _, fd := range rows.FieldDescriptions() {
		out.Columns = append(out.Columns, string(fd.Name))
	}
	for rows.Next() {
		if len(out.Rows) >= adminQueryRowCap {
			break
		}
		vals, err := rows.Values()
		if err != nil {
			return AdminQuery{}, AdminQueryResult{}, err
		}
		row := make([]string, 0, len(vals))
		for _, v := range vals {
			row = append(row, formatCell(v))
		}
		out.Rows = append(out.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return AdminQuery{}, AdminQueryResult{}, err
	}

	return AdminQuery{ID: found.ID, Label: found.Label, Detail: found.Detail}, out, nil
}

// formatCell renders one value for a table.
//
// A NULL becomes an empty cell rather than the word "null", because a
// table of counts reads better with a gap than with a keyword, and
// because "null" in a column of numbers invites being read as a value.
func formatCell(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case time.Time:
		return t.UTC().Format("2006-01-02 15:04")
	default:
		return fmt.Sprintf("%v", t)
	}
}
