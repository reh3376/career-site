package users

import (
	"context"
	"errors"
	"strings"
	"time"
)

// The admin query list: what Ask Roger can read out of the database.
//
// Roger asked for the assistant to be able to query the database, and
// then for the admin to choose the query from a dropdown. The second
// instruction is what makes this safe, and it is also what makes it
// fast.
//
//   - **The model never writes SQL.** Every statement here is fixed in
//     this file and takes no argument from anywhere. There is no string
//     being built, so there is nothing to inject into.
//   - **The model never chooses.** A person picks from the list. That
//     removes the classification step entirely, which on this hardware
//     would have cost a second model call, about ten seconds, and would
//     have been the least reliable part of the feature.
//   - **Admin only**, enforced at the RPC.
//
// # Counts, never rows
//
// Nothing here selects a name, an address or a message body. Results
// are rendered into a surface and may end up quoted into a prompt, and
// prompts are written to decision_log, which is exported as training
// data. A count answers "how is the site doing" without putting a
// person into that file.

// AdminQuery is one named, fixed query.
type AdminQuery struct {
	// ID is the stable name the client sends back. Never interpolated
	// into SQL; it selects a statement from this list and nothing else.
	ID string
	// Label is what the dropdown shows.
	Label string
	// Detail says what the number means, for the admin reading it and
	// for anyone later wondering what was being counted.
	Detail string
	sql    string
}

// adminQueries is the whole allowlist. Adding one is an edit here and
// nowhere else: the set of things the assistant can read is a list a
// person maintains, not a capability it has.
var adminQueries = []AdminQuery{
	{
		ID: "members", Label: "Members",
		Detail: "Registered accounts by status.",
		sql: `SELECT count(*) || ' total, '
		          || count(*) FILTER (WHERE status = 'active')  || ' active, '
		          || count(*) FILTER (WHERE status = 'pending_approval') || ' awaiting approval'
		        FROM users`,
	},
	{
		ID: "jd_submissions", Label: "JD submissions",
		Detail: "Job descriptions submitted to the reviewer.",
		sql: `SELECT count(*) || ' total, '
		          || count(*) FILTER (WHERE created_at > now() - interval '7 days') || ' in the last 7 days'
		        FROM jd_submissions`,
	},
	{
		ID: "ask_roger", Label: "Ask Roger usage",
		Detail: "Conversations, questions asked and answers given.",
		sql: `SELECT (SELECT count(*) FROM chat_conversations WHERE deleted_at IS NULL) || ' conversations, '
		          || (SELECT count(*) FROM chat_messages WHERE role = 'user') || ' questions, '
		          || (SELECT count(*) FROM chat_messages WHERE role = 'assistant') || ' answers'`,
	},
	{
		ID: "answer_paths", Label: "How answers were produced",
		Detail: "Which path each answer took: the bank, the model, a refusal, or nothing found.",
		sql: `SELECT coalesce(string_agg(p || ': ' || n, ', ' ORDER BY n DESC), 'no answers yet')
		        FROM (SELECT input->>'path' AS p, count(*) AS n
		                FROM decision_log WHERE kind = 'chat_answer' GROUP BY 1) t`,
	},
	{
		ID: "qa_bank", Label: "Q&A bank",
		Detail: "Entries written, approved, and phrasings still waiting to be embedded.",
		sql: `SELECT (SELECT count(*) FROM qa_entries) || ' entries, '
		          || (SELECT count(*) FROM qa_entries WHERE enabled) || ' approved, '
		          || (SELECT count(*) FROM qa_phrasings WHERE embedding IS NULL) || ' awaiting embedding'`,
	},
	{
		ID: "corpus", Label: "Corpus",
		Detail: "Documents and chunks the assistant and the reviewer retrieve from.",
		sql: `SELECT (SELECT count(*) FROM corpus_documents) || ' documents, '
		          || (SELECT count(*) FROM corpus_chunks) || ' chunks, '
		          || (SELECT count(*) FROM corpus_chunks WHERE embedding IS NULL) || ' not embedded, '
		          || (SELECT count(*) FROM corpus_documents WHERE visibility = 'public') || ' public'`,
	},
	{
		ID: "meetings", Label: "Meetings",
		Detail: "Bookings on the calendar, excluding cancellations.",
		sql: `SELECT count(*) FILTER (WHERE held && tstzrange(now(), 'infinity')) || ' upcoming, '
		          || count(*) || ' booked in total'
		        FROM meeting_bookings WHERE cancelled_at IS NULL`,
	},
	{
		ID: "model_usage", Label: "Model usage, last 7 days",
		Detail: "Calls, tokens and failures across the reviewer and the assistant.",
		sql: `SELECT coalesce(count(*) || ' calls, '
		          || sum(prompt_tokens + completion_tokens) || ' tokens, '
		          || count(*) FILTER (WHERE NOT ok) || ' failed', 'no calls in the last 7 days')
		        FROM llm_usage WHERE created_at > now() - interval '7 days'`,
	},
	{
		ID: "review_queue", Label: "Decisions to review",
		Detail: "The training set: what is graded, and how much carries a correction.",
		sql: `SELECT count(*) FILTER (WHERE reviewed_at IS NULL) || ' ungraded, '
		          || count(*) FILTER (WHERE reviewed_at IS NOT NULL) || ' graded, '
		          || count(*) FILTER (WHERE human_answer <> '') || ' with a correction written'
		        FROM decision_log`,
	},
}

// ErrNoSuchAdminQuery is returned for an id outside the list. The id is
// matched, never interpolated, so an unknown one is simply refused.
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

// RunAdminQuery runs the named query and returns its single row.
//
// Bounded hard: these are aggregates over small tables, so anything
// slow enough to notice means something is wrong and the caller should
// be told that rather than left waiting.
func (r *Repo) RunAdminQuery(ctx context.Context, id string) (AdminQuery, string, error) {
	var found *AdminQuery
	for i := range adminQueries {
		if adminQueries[i].ID == id {
			found = &adminQueries[i]
			break
		}
	}
	if found == nil {
		return AdminQuery{}, "", ErrNoSuchAdminQuery
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var value string
	if err := r.pool.QueryRow(ctx, found.sql).Scan(&value); err != nil {
		return AdminQuery{}, "", err
	}
	return AdminQuery{ID: found.ID, Label: found.Label, Detail: found.Detail},
		strings.TrimSpace(value), nil
}
