package hiring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// The clarification loop's rows (FSD FR-HF-13, 14, 22, 24; Phase 0
// step 7): the questionnaire generated from a run's unresolved
// judgments, the answers a candidate gives, and the evidence those
// answers become. The rules of what is asked and how an answer is read
// are in clarify.go; this file stores them.

// DecisionKindClarification is the decision_log kind of a question
// asked and of an answer given (03 §3.3: every question is logged like
// a decision).
const DecisionKindClarification = "hf_clarification"

// Errors of the loop. Each wraps the sentence for the person acting.
var (
	// ErrNothingToAsk: the run has no unresolved judgment that is not
	// already asked about or answered.
	ErrNothingToAsk = errors.New("hiring: nothing to ask")
	// ErrClarificationClosed: the request is no longer open, so it takes
	// no question and no answer.
	ErrClarificationClosed = errors.New("hiring: the questions are closed")
)

// ClarificationItem is one question and, once given, its answer.
type ClarificationItem struct {
	ID              string
	RequestID       string
	JudgmentID      string
	RequirementID   string
	LocalID         string
	UncertaintyCode string
	AnswerType      string
	QuestionText    string
	TemplateVersion string
	Fields          []Field
	// Answer.Status is pending until the candidate answers.
	Answer         Answer
	AnsweredAt     *time.Time
	WithdrawnAt    *time.Time
	EvidenceItemID string
	RejudgeRunID   string
	AskedBy        string
	// SourceQuote is the posting's own wording for the requirement, which
	// is what the candidate is shown above the question; RequirementText
	// stands in when extraction kept no verified quote. Both are public
	// in the posting.
	SourceQuote     string
	RequirementText string
	RequirementType string
}

// ClarificationRequest is one questionnaire, from one run.
type ClarificationRequest struct {
	ID           string
	RunID        string
	RoleID       string
	SubmissionID int64
	Attempt      int
	Round        int
	Status       string
	// Held is whether the run carries no fit while this is open
	// (FR-HF-22).
	Held         bool
	AskedBy      string
	GeneratedAt  time.Time
	ExpiresAt    *time.Time
	RemindedAt   *time.Time
	AnsweredAt   *time.Time
	ClosedAt     *time.Time
	ClosedReason string
	ClosingRunID string
	RoleTitle    string
	Employer     string
	// RetrievalScope is what the run that asked was allowed to read
	// ("public" or "all"); the run that closes the request reads the
	// same.
	RetrievalScope string
	Items          []ClarificationItem
}

// Open reports whether the request still takes answers.
func (q ClarificationRequest) Open() bool { return q.Status == RequestSent }

// Pending counts the items not yet answered one way or another.
func (q ClarificationRequest) Pending() int {
	n := 0
	for _, it := range q.Items {
		if it.Answer.Status == ItemPending {
			n++
		}
	}
	return n
}

// Engagements lists the candidate's own roles and projects as the
// corpus holds them, roles first in their stated order. Organisation
// and years come from the document's front matter, kept in meta at
// ingest; a document ingested before that carries only its title, and
// is offered by title.
func (r *Repo) Engagements(ctx context.Context) ([]Engagement, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id, source_kind, title,
           COALESCE(meta->>'organization', ''), COALESCE(meta->>'start', ''), COALESCE(meta->>'end', '')
    FROM corpus_documents
    WHERE source_kind IN ('role', 'project') AND candidate_id = $1::uuid
    ORDER BY source_kind DESC, COALESCE(NULLIF(regexp_replace(meta->>'order', '\D', '', 'g'), '')::int, 100000), source_path, id`,
		OwnerCandidateID)
	if err != nil {
		return nil, fmt.Errorf("engagements: %w", err)
	}
	defer rows.Close()
	var out []Engagement
	for rows.Next() {
		var id int64
		var e Engagement
		var start, end string
		if err := rows.Scan(&id, &e.Kind, &e.Title, &e.Organization, &start, &end); err != nil {
			return nil, fmt.Errorf("engagements: %w", err)
		}
		e.Ref = "corpus_document:" + strconv.FormatInt(id, 10)
		e.Start, _ = strconv.Atoi(start)
		e.End, _ = strconv.Atoi(end)
		out = append(out, e)
	}
	return out, rows.Err()
}

// GenerateInput says which of a run's unresolved judgments to ask
// about and on what terms.
type GenerateInput struct {
	RunID string
	// All asks about every unresolved judgment, which is the candidate's
	// own request (FR-HF-13). Without it only those FR-HF-24 asks
	// unprompted are asked: the lexical bases and an unproven
	// relationship.
	All bool
	// Held withholds the run's fit while the request is open.
	Held      bool
	ExpiresAt *time.Time
	AskedBy   string
}

// GenerateClarification writes the questionnaire for a run: one item
// per unresolved judgment asked about, each with the question and the
// fields it offers, stored as offered. A requirement version that was
// already answered, set aside or declined on any run is not asked
// again, and neither is one with a question still open: the earlier
// answer is evidence wherever that version is judged. A question that
// closed unanswered is asked again only at the candidate's request
// (All), never unprompted. One request per
// run (one round in Phase 0); a later call adds to it while it is
// open. It returns the request and how many items this call added.
func (r *Repo) GenerateClarification(ctx context.Context, in GenerateInput) (ClarificationRequest, int, error) {
	tid := tenant.FromContext(ctx).Int64()
	var submissionID int64
	var roleID, candidateID string
	err := r.pool.QueryRow(ctx, `
    SELECT run.submission_id, m.role_id::text, m.candidate_id::text
    FROM jd_runs run JOIN match_records m ON m.run_id = run.run_id
    WHERE run.run_id = $1::uuid AND m.tenant_id = $2`, in.RunID, tid).Scan(&submissionID, &roleID, &candidateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClarificationRequest{}, 0, ErrNotFound
	}
	if err != nil {
		return ClarificationRequest{}, 0, fmt.Errorf("clarification: run: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
    SELECT j.id::text, j.requirement_id::text, j.requirement_local_id, j.uncertainty, j.effective_basis,
           q.type, q.named_parties,
           EXISTS (
             SELECT 1 FROM clarification_items i
             JOIN clarification_requests cr ON cr.id = i.request_id
             WHERE i.requirement_id = j.requirement_id AND i.tenant_id = j.tenant_id
               AND i.answer_status = 'pending' AND cr.closed_reason = 'expired') AS lapsed
    FROM judgments j
    JOIN requirements q ON q.id = j.requirement_id
    WHERE j.run_id = $1::uuid AND j.tenant_id = $2
      AND j.effective_determination = 'unresolved' AND q.type <> 'behavioral'
      AND NOT EXISTS (
        SELECT 1 FROM clarification_items i
        JOIN clarification_requests cr ON cr.id = i.request_id
        WHERE i.requirement_id = j.requirement_id AND i.tenant_id = j.tenant_id
          AND (i.answer_status IN ('answered', 'not_applicable', 'declined')
               OR (i.answer_status = 'pending' AND cr.status = 'sent')))`, in.RunID, tid)
	if err != nil {
		return ClarificationRequest{}, 0, fmt.Errorf("clarification: judgments: %w", err)
	}
	type ask struct {
		judgmentID, requirementID, localID, basis, typ string
		uncertainty, parties                           []string
		lapsed                                         bool
	}
	var asks []ask
	for rows.Next() {
		var a ask
		if err := rows.Scan(&a.judgmentID, &a.requirementID, &a.localID, &a.uncertainty, &a.basis, &a.typ, &a.parties, &a.lapsed); err != nil {
			rows.Close()
			return ClarificationRequest{}, 0, fmt.Errorf("clarification: judgments: %w", err)
		}
		// A question the candidate let lapse, by letting its time run
		// out, is not put to them again unprompted: a re-score would
		// otherwise withhold the fit for the same unanswered question
		// every time. It is asked again when the candidate asks. A
		// question that a new run closed before it could be answered did
		// not lapse, and the new run asks it again.
		if in.All || (AsksUnprompted(a.basis) && !a.lapsed) {
			asks = append(asks, a)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return ClarificationRequest{}, 0, fmt.Errorf("clarification: judgments: %w", err)
	}
	if len(asks) == 0 {
		return ClarificationRequest{}, 0, ErrNothingToAsk
	}
	sort.SliceStable(asks, func(a, b int) bool { return localNumber(asks[a].localID) < localNumber(asks[b].localID) })

	engagements, err := r.Engagements(ctx)
	if err != nil {
		return ClarificationRequest{}, 0, err
	}
	reg, err := r.LoadVocab(ctx)
	if err != nil {
		return ClarificationRequest{}, 0, fmt.Errorf("clarification: vocabulary: %w", err)
	}
	lists := GenericListsFrom(reg)
	askedBy := in.AskedBy
	if askedBy == "" {
		askedBy = AskedBySystem
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ClarificationRequest{}, 0, fmt.Errorf("clarification: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var requestID, status string
	err = tx.QueryRow(ctx, `
    INSERT INTO clarification_requests
      (tenant_id, run_id, role_id, candidate_id, submission_id, round, status, asked_by, held, sent_at, expires_at)
    VALUES ($1, $2::uuid, $3::uuid, $4::uuid, $5, 1, 'sent', $6, $7, now(), $8)
    ON CONFLICT (run_id, round) DO UPDATE SET updated_at = now()
    RETURNING id::text, status`,
		tid, in.RunID, roleID, candidateID, submissionID, askedBy, in.Held, in.ExpiresAt).Scan(&requestID, &status)
	if err != nil {
		return ClarificationRequest{}, 0, fmt.Errorf("clarification: request: %w", err)
	}
	if status != RequestSent {
		return ClarificationRequest{}, 0, fmt.Errorf("%w: this run's questions were closed and one round is asked", ErrClarificationClosed)
	}
	added := 0
	for _, a := range asks {
		code := PrimaryUncertainty(a.uncertainty, a.basis)
		answerType := AnswerTypeFor(code)
		// A relationship question offers the organisations the posting
		// names. With none named there is nothing to choose from, and the
		// question becomes a pointer to the candidate's own work.
		if answerType == AnswerRelationship && len(a.parties) == 0 {
			answerType = AnswerEvidencePointer
		}
		fields := QuestionFields(answerType, engagements, lists, a.parties)
		question := QuestionText(answerType, a.typ)
		options, _ := json.Marshal(fields)
		var itemID string
		err := tx.QueryRow(ctx, `
      INSERT INTO clarification_items
        (tenant_id, request_id, judgment_id, requirement_id, requirement_local_id, uncertainty_code, answer_type,
         question_text, template_version, options, asked_by, schema_version, vocab_version)
      VALUES ($1, $2::uuid, $3::uuid, $4::uuid, $5, $6, $7, $8, $9, $10, $11, $12, $13)
      ON CONFLICT (request_id, judgment_id) DO NOTHING
      RETURNING id::text`,
			tid, requestID, a.judgmentID, a.requirementID, a.localID, code, answerType,
			question, QuestionTemplateVersion, options, askedBy, ClarificationSchema, VocabVersion).Scan(&itemID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return ClarificationRequest{}, 0, fmt.Errorf("clarification: item %s: %w", a.localID, err)
		}
		added++
		input, _ := json.Marshal(map[string]any{
			"item_id": itemID, "request_id": requestID, "judgment_id": a.judgmentID, "requirement_id": a.requirementID,
			"uncertainty": a.uncertainty, "basis": a.basis, "requirement_type": a.typ,
		})
		output, _ := json.Marshal(map[string]any{
			"action": "asked", "uncertainty_code": code, "answer_type": answerType,
			"question": question, "template_version": QuestionTemplateVersion, "asked_by": askedBy,
		})
		if err := logClarification(ctx, tx, tid, in.RunID, submissionID, a.localID, "code", input, output); err != nil {
			return ClarificationRequest{}, 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ClarificationRequest{}, 0, fmt.Errorf("clarification: commit: %w", err)
	}
	req, err := r.GetClarificationRequest(ctx, requestID)
	return req, added, err
}

// logClarification writes one decision_log row for a question asked or
// an answer given. It is entered as reviewed: neither is a model's
// decision waiting on a person's grade.
func logClarification(ctx context.Context, tx pgx.Tx, tid int64, runID string, submissionID int64, localID, who string, input, output []byte) error {
	if _, err := tx.Exec(ctx, `
    INSERT INTO decision_log
      (tenant_id, run_id, kind, ref_kind, ref_id, key, model, input, output, reviewed_at)
    VALUES ($1, $2::uuid, $3, 'jd_submission', $4, $5, $6, $7, $8, now())`,
		tid, runID, DecisionKindClarification, submissionID, localID, who, input, output); err != nil {
		return fmt.Errorf("clarification: log: %w", err)
	}
	return nil
}

const clarificationRequestColumns = `
  cr.id::text, cr.run_id::text, cr.role_id::text, COALESCE(cr.submission_id, 0), run.attempt, cr.round, cr.status,
  cr.held, cr.asked_by, cr.generated_at, cr.expires_at, cr.reminded_at, cr.answered_at, cr.closed_at,
  cr.closed_reason, COALESCE(cr.closing_run_id::text, ''),
  COALESCE(NULLIF(s.role_hint, ''), ro.title, ''), COALESCE(s.employer_hint, ''), run.retrieval_scope`

const clarificationRequestFrom = `
  FROM clarification_requests cr
  JOIN jd_runs run ON run.run_id = cr.run_id
  LEFT JOIN roles ro ON ro.id = cr.role_id
  LEFT JOIN jd_submissions s ON s.id = cr.submission_id `

func scanClarificationRequest(row rowScanner) (ClarificationRequest, error) {
	var q ClarificationRequest
	err := row.Scan(&q.ID, &q.RunID, &q.RoleID, &q.SubmissionID, &q.Attempt, &q.Round, &q.Status,
		&q.Held, &q.AskedBy, &q.GeneratedAt, &q.ExpiresAt, &q.RemindedAt, &q.AnsweredAt, &q.ClosedAt,
		&q.ClosedReason, &q.ClosingRunID, &q.RoleTitle, &q.Employer, &q.RetrievalScope)
	return q, err
}

const clarificationItemColumns = `
  i.id::text, i.request_id::text, i.judgment_id::text, i.requirement_id::text, i.requirement_local_id,
  i.uncertainty_code, i.answer_type, i.question_text, i.template_version, i.options,
  i.answer, i.answer_free_text, i.answer_status, i.answered_at, i.withdrawn_at,
  COALESCE(i.evidence_item_id::text, ''), COALESCE(i.rejudge_run_id::text, ''), i.asked_by,
  q.source_quote, q.text, q.type`

const clarificationItemFrom = `
  FROM clarification_items i
  JOIN requirements q ON q.id = i.requirement_id `

func scanClarificationItem(row rowScanner) (ClarificationItem, error) {
	var it ClarificationItem
	var options, answer []byte
	var free, status string
	if err := row.Scan(&it.ID, &it.RequestID, &it.JudgmentID, &it.RequirementID, &it.LocalID,
		&it.UncertaintyCode, &it.AnswerType, &it.QuestionText, &it.TemplateVersion, &options,
		&answer, &free, &status, &it.AnsweredAt, &it.WithdrawnAt,
		&it.EvidenceItemID, &it.RejudgeRunID, &it.AskedBy,
		&it.SourceQuote, &it.RequirementText, &it.RequirementType); err != nil {
		return it, err
	}
	_ = json.Unmarshal(options, &it.Fields)
	it.Answer = Answer{Status: status, FreeText: free}
	if len(answer) > 0 {
		_ = json.Unmarshal(answer, &it.Answer.Values)
	}
	return it, nil
}

func (r *Repo) clarificationItems(ctx context.Context, where string, args ...any) ([]ClarificationItem, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+clarificationItemColumns+clarificationItemFrom+where, args...)
	if err != nil {
		return nil, fmt.Errorf("clarification items: %w", err)
	}
	defer rows.Close()
	var out []ClarificationItem
	for rows.Next() {
		it, err := scanClarificationItem(rows)
		if err != nil {
			return nil, fmt.Errorf("clarification items: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("clarification items: %w", err)
	}
	sort.SliceStable(out, func(a, b int) bool { return localNumber(out[a].LocalID) < localNumber(out[b].LocalID) })
	return out, nil
}

// GetClarificationRequest reads one questionnaire with its items in
// reading order.
func (r *Repo) GetClarificationRequest(ctx context.Context, id string) (ClarificationRequest, error) {
	tid := tenant.FromContext(ctx).Int64()
	q, err := scanClarificationRequest(r.pool.QueryRow(ctx,
		`SELECT `+clarificationRequestColumns+clarificationRequestFrom+`WHERE cr.id = $1::uuid AND cr.tenant_id = $2`, id, tid))
	if errors.Is(err, pgx.ErrNoRows) {
		return ClarificationRequest{}, ErrNotFound
	}
	if err != nil {
		return ClarificationRequest{}, fmt.Errorf("clarification request: %w", err)
	}
	q.Items, err = r.clarificationItems(ctx, `WHERE i.request_id = $1::uuid AND i.tenant_id = $2`, id, tid)
	return q, err
}

// ClarificationForRun reads the questionnaire generated from a run; ok
// is false when the run has none.
func (r *Repo) ClarificationForRun(ctx context.Context, runID string) (ClarificationRequest, bool, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
    SELECT id::text FROM clarification_requests WHERE run_id = $1::uuid AND tenant_id = $2 ORDER BY round DESC LIMIT 1`,
		runID, tenant.FromContext(ctx).Int64()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClarificationRequest{}, false, nil
	}
	if err != nil {
		return ClarificationRequest{}, false, fmt.Errorf("clarification for run: %w", err)
	}
	q, err := r.GetClarificationRequest(ctx, id)
	return q, err == nil, err
}

// ListClarificationRequests lists the candidate's questionnaires,
// open ones first and newest first within each, without their items
// but with how many are pending.
func (r *Repo) ListClarificationRequests(ctx context.Context, openOnly bool, limit int) ([]ClarificationRequest, map[string][2]int, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	where := `WHERE cr.tenant_id = $1 AND cr.candidate_id = $2::uuid`
	if openOnly {
		where += ` AND cr.status = 'sent'`
	}
	rows, err := r.pool.Query(ctx, `SELECT `+clarificationRequestColumns+`,
      (SELECT count(*) FROM clarification_items i WHERE i.request_id = cr.id),
      (SELECT count(*) FROM clarification_items i WHERE i.request_id = cr.id AND i.answer_status = 'pending')`+
		clarificationRequestFrom+where+`
    ORDER BY (cr.status = 'sent') DESC, cr.generated_at DESC LIMIT $3`,
		tenant.FromContext(ctx).Int64(), OwnerCandidateID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("list clarification requests: %w", err)
	}
	defer rows.Close()
	var out []ClarificationRequest
	counts := map[string][2]int{}
	for rows.Next() {
		var q ClarificationRequest
		var total, pending int
		if err := rows.Scan(&q.ID, &q.RunID, &q.RoleID, &q.SubmissionID, &q.Attempt, &q.Round, &q.Status,
			&q.Held, &q.AskedBy, &q.GeneratedAt, &q.ExpiresAt, &q.RemindedAt, &q.AnsweredAt, &q.ClosedAt,
			&q.ClosedReason, &q.ClosingRunID, &q.RoleTitle, &q.Employer, &q.RetrievalScope, &total, &pending); err != nil {
			return nil, nil, fmt.Errorf("list clarification requests: %w", err)
		}
		out = append(out, q)
		counts[q.ID] = [2]int{total, pending}
	}
	return out, counts, rows.Err()
}

// SaveClarificationAnswer records a candidate's answer to one item of
// an open request, checked against the fields the item offered, and
// makes it evidence (FR-HF-14): an answer, or "I have not done this",
// becomes a candidate_response evidence item tied to the item, with the
// answer rendered as the statement a judge and a reader see. Saving
// again replaces the answer and its evidence. A declined item is
// recorded and carries no evidence.
func (r *Repo) SaveClarificationAnswer(ctx context.Context, itemID string, a Answer, now time.Time) (ClarificationItem, error) {
	tid := tenant.FromContext(ctx).Int64()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ClarificationItem{}, fmt.Errorf("answer: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	it, err := scanClarificationItem(tx.QueryRow(ctx,
		`SELECT `+clarificationItemColumns+clarificationItemFrom+`WHERE i.id = $1::uuid AND i.tenant_id = $2 FOR UPDATE OF i`, itemID, tid))
	if errors.Is(err, pgx.ErrNoRows) {
		return ClarificationItem{}, ErrNotFound
	}
	if err != nil {
		return ClarificationItem{}, fmt.Errorf("answer: item: %w", err)
	}
	var status, runID, candidateID string
	var submissionID int64
	if err := tx.QueryRow(ctx, `
    SELECT status, run_id::text, candidate_id::text, COALESCE(submission_id, 0)
    FROM clarification_requests WHERE id = $1::uuid FOR UPDATE`, it.RequestID).Scan(&status, &runID, &candidateID, &submissionID); err != nil {
		return ClarificationItem{}, fmt.Errorf("answer: request: %w", err)
	}
	if status != RequestSent {
		return ClarificationItem{}, fmt.Errorf("%w: these questions are no longer open", ErrClarificationClosed)
	}
	norm, err := NormaliseAnswer(it.AnswerType, it.Fields, a)
	if err != nil {
		return ClarificationItem{}, err
	}
	values, _ := json.Marshal(norm.Values)
	if norm.Values == nil {
		values = nil
	}
	evidenceID := ""
	switch norm.Status {
	case ItemAnswered, ItemNotApplicable:
		statement := AnswerStatement(it.Fields, norm, now)
		// employer_scoped: the statement is shown to the employer whose
		// posting it answers (owner's decision, 2026-11-15) and to no
		// other reader.
		if err := tx.QueryRow(ctx, `
      INSERT INTO evidence_items
        (tenant_id, candidate_id, source_kind, clarification_item_id, text, visibility, attested_at, schema_version)
      VALUES ($1, $2::uuid, $3, $4::uuid, $5, 'employer_scoped', $6, $7)
      ON CONFLICT (clarification_item_id) WHERE clarification_item_id IS NOT NULL DO UPDATE
        SET text = EXCLUDED.text, attested_at = EXCLUDED.attested_at, withdrawn_at = NULL
      RETURNING id::text`,
			tid, candidateID, SourceCandidateResponse, itemID, statement, now, EvidenceSchema).Scan(&evidenceID); err != nil {
			return ClarificationItem{}, fmt.Errorf("answer: evidence: %w", err)
		}
	default:
		// Declined: an earlier answer's evidence is withdrawn with it.
		if _, err := tx.Exec(ctx, `
      UPDATE evidence_items SET withdrawn_at = $2 WHERE clarification_item_id = $1::uuid AND withdrawn_at IS NULL`, itemID, now); err != nil {
			return ClarificationItem{}, fmt.Errorf("answer: evidence: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
    UPDATE clarification_items
    SET answer = $2, answer_free_text = $3, answer_status = $4, answered_at = $5, withdrawn_at = NULL,
        evidence_item_id = NULLIF($6, '')::uuid
    WHERE id = $1::uuid`, itemID, values, norm.FreeText, norm.Status, now, evidenceID); err != nil {
		return ClarificationItem{}, fmt.Errorf("answer: write: %w", err)
	}
	input, _ := json.Marshal(map[string]any{
		"item_id": itemID, "request_id": it.RequestID, "judgment_id": it.JudgmentID,
		"uncertainty_code": it.UncertaintyCode, "answer_type": it.AnswerType, "question": it.QuestionText,
	})
	output, _ := json.Marshal(map[string]any{
		"action": norm.Status, "values": norm.Values, "has_note": norm.FreeText != "", "evidence_item_id": evidenceID,
	})
	if err := logClarification(ctx, tx, tid, runID, submissionID, it.LocalID, "candidate", input, output); err != nil {
		return ClarificationItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ClarificationItem{}, fmt.Errorf("answer: commit: %w", err)
	}
	it.Answer, it.AnsweredAt, it.WithdrawnAt, it.EvidenceItemID = norm, &now, nil, evidenceID
	return it, nil
}

// SubmitClarificationRequest closes an open questionnaire to further
// answers once every item has been answered, set aside or declined,
// which is what starts the run that judges the affected requirements
// again. It refuses while any item is still pending: each question is
// answered or explicitly left, never skipped.
func (r *Repo) SubmitClarificationRequest(ctx context.Context, id string, now time.Time) (ClarificationRequest, error) {
	tid := tenant.FromContext(ctx).Int64()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ClarificationRequest{}, fmt.Errorf("submit answers: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM clarification_requests WHERE id = $1::uuid AND tenant_id = $2 FOR UPDATE`, id, tid).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClarificationRequest{}, ErrNotFound
	}
	if err != nil {
		return ClarificationRequest{}, fmt.Errorf("submit answers: %w", err)
	}
	if status != RequestSent {
		return ClarificationRequest{}, fmt.Errorf("%w: these questions are no longer open", ErrClarificationClosed)
	}
	var pending int
	if err := tx.QueryRow(ctx, `
    SELECT count(*) FROM clarification_items WHERE request_id = $1::uuid AND answer_status = 'pending'`, id).Scan(&pending); err != nil {
		return ClarificationRequest{}, fmt.Errorf("submit answers: %w", err)
	}
	if pending > 0 {
		noun := "questions still need"
		if pending == 1 {
			noun = "question still needs"
		}
		return ClarificationRequest{}, invalidAnswer("%d %s an answer, \"I have not done this\" or \"prefer not to answer\"", pending, noun)
	}
	if _, err := tx.Exec(ctx, `
    UPDATE clarification_requests SET status = 'answered', answered_at = $2 WHERE id = $1::uuid`, id, now); err != nil {
		return ClarificationRequest{}, fmt.Errorf("submit answers: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ClarificationRequest{}, fmt.Errorf("submit answers: commit: %w", err)
	}
	return r.GetClarificationRequest(ctx, id)
}

// WithdrawClarificationAnswer takes an answer back (03 §3.3: the
// candidate can withdraw). The item is marked withdrawn, its evidence
// item is soft-deleted, and every judgment that cites it is marked
// stale: it was made on a statement that no longer stands. Nothing is
// re-judged here; the record says the verdict is stale until a run
// judges the requirement again.
func (r *Repo) WithdrawClarificationAnswer(ctx context.Context, itemID string, now time.Time) (ClarificationItem, int64, error) {
	tid := tenant.FromContext(ctx).Int64()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ClarificationItem{}, 0, fmt.Errorf("withdraw answer: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	it, err := scanClarificationItem(tx.QueryRow(ctx,
		`SELECT `+clarificationItemColumns+clarificationItemFrom+`WHERE i.id = $1::uuid AND i.tenant_id = $2 FOR UPDATE OF i`, itemID, tid))
	if errors.Is(err, pgx.ErrNoRows) {
		return ClarificationItem{}, 0, ErrNotFound
	}
	if err != nil {
		return ClarificationItem{}, 0, fmt.Errorf("withdraw answer: %w", err)
	}
	if it.Answer.Status != ItemAnswered && it.Answer.Status != ItemNotApplicable {
		return ClarificationItem{}, 0, invalidAnswer("there is no answer on this question to withdraw")
	}
	var runID string
	var submissionID int64
	if err := tx.QueryRow(ctx, `
    SELECT run_id::text, COALESCE(submission_id, 0) FROM clarification_requests WHERE id = $1::uuid`, it.RequestID).Scan(&runID, &submissionID); err != nil {
		return ClarificationItem{}, 0, fmt.Errorf("withdraw answer: %w", err)
	}
	if _, err := tx.Exec(ctx, `
    UPDATE clarification_items SET answer_status = 'withdrawn', withdrawn_at = $2 WHERE id = $1::uuid`, itemID, now); err != nil {
		return ClarificationItem{}, 0, fmt.Errorf("withdraw answer: %w", err)
	}
	var stale int64
	if it.EvidenceItemID != "" {
		if _, err := tx.Exec(ctx, `UPDATE evidence_items SET withdrawn_at = $2 WHERE id = $1::uuid`, it.EvidenceItemID, now); err != nil {
			return ClarificationItem{}, 0, fmt.Errorf("withdraw answer: evidence: %w", err)
		}
		tag, err := tx.Exec(ctx, `
      UPDATE judgments SET stale = true WHERE tenant_id = $2 AND $1::uuid = ANY (evidence_ids) AND NOT stale`, it.EvidenceItemID, tid)
		if err != nil {
			return ClarificationItem{}, 0, fmt.Errorf("withdraw answer: judgments: %w", err)
		}
		stale = tag.RowsAffected()
	}
	input, _ := json.Marshal(map[string]any{"item_id": itemID, "request_id": it.RequestID, "evidence_item_id": it.EvidenceItemID})
	output, _ := json.Marshal(map[string]any{"action": "withdrawn", "judgments_marked_stale": stale})
	if err := logClarification(ctx, tx, tid, runID, submissionID, it.LocalID, "candidate", input, output); err != nil {
		return ClarificationItem{}, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ClarificationItem{}, 0, fmt.Errorf("withdraw answer: commit: %w", err)
	}
	it.Answer.Status, it.WithdrawnAt = ItemWithdrawn, &now
	return it, stale, nil
}

// ExpireClarificationRequests closes every held request whose time
// limit has passed (owner's decision, 2026-11-15: a recruiter waits at
// most the limit; after it the fit is issued from what is evidenced).
// Answers already saved stand as evidence; questions left pending are
// simply never answered. It returns the requests it closed, for the
// caller to start the run that issues each fit.
func (r *Repo) ExpireClarificationRequests(ctx context.Context, now time.Time) ([]ClarificationRequest, error) {
	rows, err := r.pool.Query(ctx, `
    UPDATE clarification_requests
    SET status = 'expired', closed_reason = 'expired'
    WHERE status = 'sent' AND held AND expires_at IS NOT NULL AND expires_at <= $1 AND tenant_id = $2
    RETURNING id::text, run_id::text, COALESCE(submission_id, 0)`, now, tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("expire clarification requests: %w", err)
	}
	defer rows.Close()
	var out []ClarificationRequest
	for rows.Next() {
		var q ClarificationRequest
		if err := rows.Scan(&q.ID, &q.RunID, &q.SubmissionID); err != nil {
			return nil, fmt.Errorf("expire clarification requests: %w", err)
		}
		q.Status, q.ClosedReason, q.Held = RequestExpired, ClosedExpired, true
		out = append(out, q)
	}
	return out, rows.Err()
}

// ClarificationReminders marks and returns the held requests whose
// limit falls within lead and whose candidate has not been reminded. A
// request whose whole time limit is no longer than the lead gets none:
// the reminder would arrive with the questions.
func (r *Repo) ClarificationReminders(ctx context.Context, now time.Time, lead time.Duration) ([]ClarificationRequest, error) {
	rows, err := r.pool.Query(ctx, `
    UPDATE clarification_requests cr
    SET reminded_at = $1
    WHERE cr.status = 'sent' AND cr.held AND cr.reminded_at IS NULL AND cr.tenant_id = $3
      AND cr.expires_at IS NOT NULL AND cr.expires_at > $1 AND cr.expires_at <= $2
      AND cr.expires_at - cr.generated_at > ($2::timestamptz - $1::timestamptz)
      AND EXISTS (SELECT 1 FROM clarification_items i WHERE i.request_id = cr.id AND i.answer_status = 'pending')
    RETURNING cr.id::text`, now, now.Add(lead), tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("clarification reminders: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("clarification reminders: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("clarification reminders: %w", err)
	}
	out := make([]ClarificationRequest, 0, len(ids))
	for _, id := range ids {
		q, err := r.GetClarificationRequest(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

// SupersedeClarifications closes the requests of a posting that a new
// full run is about to replace: their run's record is about to be
// history, and the new run asks its own questions. Answers already
// given stand as evidence on the requirement versions they were about.
func (r *Repo) SupersedeClarifications(ctx context.Context, submissionID int64, now time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
    UPDATE clarification_requests
    SET status = 'expired', closed_reason = 'superseded', closed_at = $2
    WHERE submission_id = $1 AND tenant_id = $3 AND closed_at IS NULL AND status IN ('sent', 'answered')`,
		submissionID, now, tenant.FromContext(ctx).Int64())
	if err != nil {
		return 0, fmt.Errorf("supersede clarification requests: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CloseClarificationRequest records the run that consumed a request:
// when it closed, why, and on each item that was answered, the run that
// judged its requirement again. A request that is already closed is
// left exactly as it is.
func (r *Repo) CloseClarificationRequest(ctx context.Context, id, reason, closingRunID string, now time.Time) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("close clarification request: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
    UPDATE clarification_requests
    SET closed_at = $2, closed_reason = $3, closing_run_id = NULLIF($4, '')::uuid,
        status = CASE WHEN status = 'sent' THEN 'expired' ELSE status END
    WHERE id = $1::uuid AND closed_at IS NULL`, id, now, reason, closingRunID)
	if err != nil {
		return fmt.Errorf("close clarification request: %w", err)
	}
	// Closed once. A second closing changes nothing, the items included:
	// they keep the run that judged them.
	if tag.RowsAffected() == 0 {
		return nil
	}
	if closingRunID != "" {
		if _, err := tx.Exec(ctx, `
      UPDATE clarification_items SET rejudge_run_id = $2::uuid
      WHERE request_id = $1::uuid AND answer_status IN ('answered', 'not_applicable')`, id, closingRunID); err != nil {
			return fmt.Errorf("close clarification request: items: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// ClarificationsOwedARun lists the requests that are answered or
// expired and not yet closed by a run: the run that judges the answered
// requirements again and issues the fit was never started, or was cut
// short. A request whose closing run has already been started maxRuns
// times is left out: something is wrong that a fourth attempt will not
// fix, and a person's re-score closes it.
func (r *Repo) ClarificationsOwedARun(ctx context.Context, maxRuns int) ([]ClarificationRequest, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT cr.id::text
    FROM clarification_requests cr
    WHERE cr.tenant_id = $1 AND cr.closed_at IS NULL AND cr.status IN ('answered', 'expired')
      AND cr.submission_id IS NOT NULL
      AND (SELECT count(*) FROM jd_runs x
           WHERE x.submission_id = cr.submission_id AND x.trigger IN ('clarification', 'expiry')
             AND x.started_at >= cr.generated_at) < $2
    ORDER BY cr.generated_at`, tenant.FromContext(ctx).Int64(), maxRuns)
	if err != nil {
		return nil, fmt.Errorf("clarifications owed a run: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("clarifications owed a run: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("clarifications owed a run: %w", err)
	}
	out := make([]ClarificationRequest, 0, len(ids))
	for _, id := range ids {
		q, err := r.GetClarificationRequest(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

// StoredAnswer is a candidate's standing answer about one requirement
// version: what a run needs to judge that version with the answer on
// the table.
type StoredAnswer struct {
	ItemID          string
	RequirementID   string
	UncertaintyCode string
	AnswerType      string
	Fields          []Field
	Answer          Answer
	EvidenceItemID  string
	Statement       string
	AnsweredAt      time.Time
}

// StandingAnswers returns, per requirement version, the candidate's
// newest answer that has not been withdrawn: an answer or "I have not
// done this". An answer is evidence about a requirement version, not
// about a run, so every later run that judges the version has it. A
// declined item is not an answer and is not returned.
func (r *Repo) StandingAnswers(ctx context.Context, requirementIDs []string) (map[string]StoredAnswer, error) {
	out := map[string]StoredAnswer{}
	if len(requirementIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
    SELECT DISTINCT ON (i.requirement_id)
           i.id::text, i.requirement_id::text, i.uncertainty_code, i.answer_type, i.options,
           i.answer, i.answer_free_text, i.answer_status, i.answered_at, e.id::text, e.text
    FROM clarification_items i
    JOIN evidence_items e ON e.id = i.evidence_item_id AND e.withdrawn_at IS NULL
    WHERE i.requirement_id = ANY($1::uuid[]) AND i.tenant_id = $2
      AND i.answer_status IN ('answered', 'not_applicable')
    ORDER BY i.requirement_id, i.answered_at DESC`, requirementIDs, tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("standing answers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a StoredAnswer
		var options, answer []byte
		var free, status string
		var at *time.Time
		if err := rows.Scan(&a.ItemID, &a.RequirementID, &a.UncertaintyCode, &a.AnswerType, &options,
			&answer, &free, &status, &at, &a.EvidenceItemID, &a.Statement); err != nil {
			return nil, fmt.Errorf("standing answers: %w", err)
		}
		_ = json.Unmarshal(options, &a.Fields)
		a.Answer = Answer{Status: status, FreeText: free}
		if len(answer) > 0 {
			_ = json.Unmarshal(answer, &a.Answer.Values)
		}
		if at != nil {
			a.AnsweredAt = *at
		}
		out[a.RequirementID] = a
	}
	return out, rows.Err()
}
