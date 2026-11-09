package hiring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
)

// Writer is the dual write of Phase 0 step 3 (14 §4): after the JD
// pipeline has stored its jsonb assessment, the same facts are written
// as rows in the framework's tables, and the jsonb stays authoritative
// until step 4 proves the rows. Judgment rows are not written here; a
// judgment cannot be faked from a three-way verdict without inventing
// a basis.
type Writer struct {
	repo *Repo
	log  *slog.Logger
}

// NewWriter builds a writer over the repository.
func NewWriter(log *slog.Logger, repo *Repo) *Writer {
	return &Writer{repo: repo, log: log}
}

// Chunk is one corpus chunk as offered to the judge: id and text, the
// two things the evidence items and the keyword baseline need.
type Chunk struct {
	ID   int64
	Text string
}

// JudgmentInput is what the prototype's judgment contributes to the
// rows: which requirement, the three-way verdict, and the chunks it
// cited. The verdict is not stored anywhere new; it decides nothing
// here beyond which chunks become evidence items.
type JudgmentInput struct {
	LocalID          string
	Verdict          string
	EvidenceChunkIDs []int64
}

// RunInput is one finished pipeline run as the scorer saw it.
type RunInput struct {
	SubmissionID int64
	Title        string
	RunID        string
	Attempt      int
	Score        float64
	Model        string
	// Prompts is the fingerprint per prompt id, as jd_runs.prompts holds
	// it ("v2:9f1c2e7a"); the extractor's is copied into each
	// requirement's provenance.
	Prompts           map[string]string
	ExtractorPromptID string
	Embedder          string
	CorpusFingerprint string
	Requirements      []RequirementInput
	Judgments         []JudgmentInput
	// Offered lists, per requirement local id, the chunks the judge was
	// shown, cited or not; the keyword baseline runs over all of them,
	// because the comparison is "would a filter over the same evidence
	// have found it", not "over the evidence the judge chose".
	Offered map[string][]Chunk
}

// Record writes the run as rows. Errors are returned for the caller to
// log; nothing here can fail the member's review, which was persisted
// before this was called.
func (w *Writer) Record(ctx context.Context, in RunInput) error {
	if w == nil || w.repo == nil {
		return nil
	}
	if in.RunID == "" {
		return fmt.Errorf("hiring: record: run id is empty")
	}
	roleID, err := w.repo.EnsureRole(ctx, in.SubmissionID, trimTitle(in.Title))
	if err != nil {
		return err
	}

	// Requirements, typed by the heuristic until the extractor types
	// them (step 5), with the provenance saying so.
	extractorFP := in.Prompts[in.ExtractorPromptID]
	reqs := make([]RequirementInput, 0, len(in.Requirements))
	for _, r := range in.Requirements {
		if r.LocalID == "" {
			continue
		}
		if r.Type == "" {
			r.Type = InferType(r.Text, r.NamedParties)
		}
		if len(r.Extraction) == 0 {
			prov, _ := json.Marshal(map[string]any{
				"prompt_id":          in.ExtractorPromptID,
				"prompt_fingerprint": extractorFP,
				"model":              in.Model,
				"type_rule":          TypeRule,
			})
			r.Extraction = prov
		}
		reqs = append(reqs, r)
	}
	rows, err := w.repo.SyncRequirementVersions(ctx, roleID, reqs)
	if err != nil {
		return err
	}
	reqIDs := make([]string, 0, len(rows))
	for _, q := range rows {
		reqIDs = append(reqIDs, q.ID)
	}

	// Evidence items for every chunk a judgment cited, lazily.
	var cited []int64
	for _, j := range in.Judgments {
		for _, id := range j.EvidenceChunkIDs {
			if !slices.Contains(cited, id) {
				cited = append(cited, id)
			}
		}
	}
	if _, err := w.repo.EnsureChunkEvidence(ctx, cited); err != nil {
		return err
	}

	// The keyword baseline over the offered evidence (FR-HF-12).
	items := map[string]any{}
	for _, r := range reqs {
		var texts []string
		for _, c := range in.Offered[r.LocalID] {
			texts = append(texts, c.Text)
		}
		found, terms := KeywordBaseline(r.Text, texts)
		if terms == nil {
			terms = []string{}
		}
		items[r.LocalID] = map[string]any{"found": found, "terms": terms}
	}
	kb, _ := json.Marshal(map[string]any{"rule": KeywordRule, "items": items})

	prev, err := w.repo.PreviousRunID(ctx, in.SubmissionID, in.Attempt)
	if err != nil {
		return err
	}
	versions, _ := json.Marshal(map[string]any{
		"requirement_schema": RequirementSchema,
		"judgment_schema":    JudgmentSchema,
		"vocab":              VocabVersion,
		"formula":            FormulaV2,
		"rules":              "",
		"keyword_rule":       KeywordRule,
		"type_rule":          TypeRule,
		"prompts":            sortedPrompts(in.Prompts),
		"model":              in.Model,
		"embedder":           in.Embedder,
		"corpus_fingerprint": in.CorpusFingerprint,
	})
	score := in.Score
	return w.repo.UpsertMatchRecord(ctx, MatchRecord{
		RunID:           in.RunID,
		RoleID:          roleID,
		CandidateID:     OwnerCandidateID,
		Attempt:         in.Attempt,
		SupersedesRunID: prev,
		RequirementIDs:  reqIDs,
		FormulaVersion:  FormulaV2,
		Evidenced:       &score,
		KeywordBaseline: kb,
		Versions:        versions,
	})
}

func trimTitle(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}

func sortedPrompts(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(m))
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}
