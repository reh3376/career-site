package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/jd"
	"github.com/reh3376/career-site/services/api/internal/jobs"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Long-running admin operations run as jobs (see internal/jobs): the
// RPC returns an id at once and the console polls GetJob. On the
// production box a private reindex or a full embed sweep runs for
// minutes, past what the proxy lets a synchronous response wait.

// sweepBatch is how many chunks one EmbedSweep call handles inside the
// sweep job; small so progress reports are frequent and a cancelled
// job stops within a batch.
const sweepBatch = 32

// jobTimeout is how long a job of this kind may take before the runner
// cancels it. Zero means the runner's default of two hours.
//
// An evaluation is the exception. It scores every golden posting
// through the real pipeline with one judge call per requirement, and on
// this box one posting takes roughly fifty minutes, so a set of five is
// already a four-hour job and the set is meant to grow. The default
// ceiling cancelled a run three postings in on 2026-09-23 after two
// hours of work. Eight hours is not a promise that a run is healthy, it
// is a ceiling loose enough that hitting it means something is wrong
// rather than merely slow.
func jobTimeout(kind v1.JobKind) time.Duration {
	if kind == v1.JobKind_JOB_KIND_EVAL_QUICK {
		return 8 * time.Hour
	}
	return 0
}

// RunJob starts a job and returns its id.
func (a *Admin) RunJob(
	ctx context.Context,
	req *connect.Request[v1.RunJobRequest],
) (*connect.Response[v1.RunJobResponse], error) {
	admin, err := requireAdmin(a, ctx, req)
	if err != nil {
		return nil, err
	}
	if a.jobs == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("job runner not wired"))
	}
	kind := req.Msg.GetKind()
	sourceKind := strings.TrimSpace(req.Msg.GetSourceKind())
	// source_kind names a corpus kind for the reindex jobs. The
	// evaluation job reuses the field as a free note, so it is only
	// validated where it means what it says.
	isCorpusJob := kind == v1.JobKind_JOB_KIND_CORPUS_REINDEX_PUBLIC ||
		kind == v1.JobKind_JOB_KIND_CORPUS_REINDEX_PRIVATE
	if isCorpusJob && sourceKind != "" && !ingest.IsKnownKind(sourceKind) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("unknown source_kind %q; known: %s", sourceKind, strings.Join(ingest.KnownKinds, ", ")))
	}

	var fn jobs.Fn
	switch kind {
	case v1.JobKind_JOB_KIND_CORPUS_REINDEX_PUBLIC, v1.JobKind_JOB_KIND_CORPUS_REINDEX_PRIVATE:
		if a.ingest == nil {
			return nil, connect.NewError(connect.CodeUnavailable, errors.New("corpus ingester not wired"))
		}
		scope := "public"
		if kind == v1.JobKind_JOB_KIND_CORPUS_REINDEX_PRIVATE {
			scope = "private"
		}
		// Configuration errors surface now, not inside the job.
		if scope == "public" && a.corpus.Public == "" {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("CORPUS_ROOT is not configured"))
		}
		if scope == "private" && a.corpus.Private == "" {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("CORPUS_PRIVATE_ROOT is not configured"))
		}
		fn = func(jctx context.Context, report jobs.Report) (string, error) {
			report(5, "walking "+scope+" corpus")
			res, kinds, err := a.reindexCorpus(jctx, scope, sourceKind)
			if err != nil {
				return "", err
			}
			return summarizeWalk(scope, res, kinds), nil
		}
	case v1.JobKind_JOB_KIND_EVAL_QUICK:
		if a.evaluator == nil {
			return nil, connect.NewError(connect.CodeUnavailable,
				errors.New("the scoring pipeline is not wired, so there is nothing to evaluate"))
		}
		// An evaluation scores the whole set through the real pipeline,
		// which on the production box is tens of minutes. It is a job
		// with progress for that reason, and only one runs at a time,
		// which the runner already enforces per kind.
		note := strings.TrimSpace(req.Msg.GetSourceKind()) // reused as the free note
		fn = func(jctx context.Context, report jobs.Report) (string, error) {
			return a.evaluator.Run(jctx, note, admin.ID, jd.Report(report))
		}
	case v1.JobKind_JOB_KIND_EMBED_SWEEP:
		if a.ingest == nil {
			return nil, connect.NewError(connect.CodeUnavailable, errors.New("corpus ingester not wired"))
		}
		fn = func(jctx context.Context, report jobs.Report) (string, error) {
			var total, failed int
			model := ""
			for round := 0; ; round++ {
				res, err := a.ingest.EmbedSweep(jctx, ingest.SweepOptions{MaxChunks: sweepBatch})
				if err != nil {
					return "", fmt.Errorf("sweep round %d: %w", round+1, err)
				}
				total += res.Embedded
				failed += res.Failed
				model = res.Model
				done := total + failed
				pct := int32(0)
				if done+int(res.Remaining) > 0 {
					pct = int32(100 * done / (done + int(res.Remaining)))
				}
				report(pct, fmt.Sprintf("embedded %d, failed %d, remaining %d (%s)", total, failed, res.Remaining, model))
				if res.Remaining == 0 || res.Considered == 0 {
					break
				}
				if res.Embedded == 0 && res.Failed > 0 {
					return "", fmt.Errorf("sweep stalled: %d chunks failed to embed, %d remaining", res.Failed, res.Remaining)
				}
			}
			return fmt.Sprintf("embedded %d chunks, %d failed, 0 remaining (%s)", total, failed, model), nil
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("job kind %s is not runnable here", kind.String()))
	}

	j, err := a.jobs.StartWithin(kind.String(), jobTimeout(kind), fn)
	if err != nil {
		if errors.Is(err, jobs.ErrAlreadyRunning) {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not start job"))
	}
	a.log.Info("job started", slog.String("job", j.ID), slog.String("kind", j.Kind), slog.Int64("admin_id", admin.ID))
	return connect.NewResponse(&v1.RunJobResponse{JobId: j.ID}), nil
}

// GetJob reports a job's state.
func (a *Admin) GetJob(
	ctx context.Context,
	req *connect.Request[v1.GetJobRequest],
) (*connect.Response[v1.GetJobResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	if a.jobs == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("job runner not wired"))
	}
	j, ok := a.jobs.Get(strings.TrimSpace(req.Msg.GetJobId()))
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("job not found (the api may have restarted; start it again)"))
	}
	out := &v1.GetJobResponse{
		JobId:       j.ID,
		Kind:        v1.JobKind(v1.JobKind_value[j.Kind]),
		Status:      jobStatusToProto(j.Status),
		ProgressPct: j.Progress,
		StartedAt:   timestamppb.New(j.StartedAt),
		Summary:     j.Summary,
	}
	if j.FinishedAt != nil {
		out.FinishedAt = timestamppb.New(*j.FinishedAt)
	}
	return connect.NewResponse(out), nil
}

func jobStatusToProto(s jobs.Status) v1.JobStatus {
	switch s {
	case jobs.StatusQueued:
		return v1.JobStatus_JOB_STATUS_QUEUED
	case jobs.StatusRunning:
		return v1.JobStatus_JOB_STATUS_RUNNING
	case jobs.StatusSucceeded:
		return v1.JobStatus_JOB_STATUS_SUCCEEDED
	case jobs.StatusFailed:
		return v1.JobStatus_JOB_STATUS_FAILED
	}
	return v1.JobStatus_JOB_STATUS_UNSPECIFIED
}

// reindexCorpus is the shared body of ReindexCorpus and the reindex
// jobs: walk one scope (optionally one kind) and aggregate.
func (a *Admin) reindexCorpus(ctx context.Context, scope, kind string) (*ingest.WalkResult, []string, error) {
	switch scope {
	case "", "public":
		if a.corpus.Public == "" {
			return nil, nil, errors.New("CORPUS_ROOT is not configured")
		}
		res := &ingest.WalkResult{Root: a.corpus.Public, Visibility: users.VisibilityPublic}
		var kinds []string
		for k, subdir := range publicCorpusSubdirs {
			if kind != "" && k != kind {
				continue
			}
			one, werr := a.ingest.WalkDirectory(ctx, ingest.WalkOptions{
				Root:       a.corpus.Public + "/" + subdir,
				SourceKind: k,
				Visibility: users.VisibilityPublic,
			})
			if werr != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", k, werr.Error()))
				continue
			}
			res.Add(one)
			kinds = append(kinds, k)
		}
		if kind != "" && len(kinds) == 0 && len(res.Errors) == 0 {
			return nil, nil, fmt.Errorf("source_kind %q has no public content directory", kind)
		}
		return res, kinds, nil
	case "private":
		if a.corpus.Private == "" {
			return nil, nil, errors.New("CORPUS_PRIVATE_ROOT is not configured")
		}
		res, kinds, err := a.ingest.WalkKinds(ctx, a.corpus.Private, users.VisibilityCorpusOnly, kind)
		if err != nil {
			return nil, nil, err
		}
		return res, kinds, nil
	}
	return nil, nil, fmt.Errorf("scope %q must be public or private", scope)
}

func summarizeWalk(scope string, res *ingest.WalkResult, kinds []string) string {
	s := fmt.Sprintf("%s: %d files, %d docs ingested (%d unchanged), %d chunks inserted, %d embedded",
		scope, res.FilesScanned, res.DocsIngested, res.DocsSkipped, res.ChunksInserted, res.ChunksEmbedded)
	if len(kinds) > 0 {
		s += ", kinds " + strings.Join(kinds, ", ")
	}
	if len(res.Errors) > 0 {
		n := len(res.Errors)
		show := res.Errors
		if n > 5 {
			show = show[:5]
		}
		s += fmt.Sprintf("; %d error(s): %s", n, strings.Join(show, " | "))
	}
	return s
}
