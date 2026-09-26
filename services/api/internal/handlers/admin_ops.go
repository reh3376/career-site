package handlers

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"syscall"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/jobs"
)

// GetOpsStatus answers "what is the box doing", which until now was a
// question with no answer short of ssh.
//
// The job runner holds jobs in memory, so recreating the api container
// kills anything running. That happened twice on 2026-09-24, the second
// time losing an eight-posting evaluation at seven. Whether a deploy is
// safe depends entirely on whether something is running, and that was
// visible nowhere: not in the console, not to the deploy script, not to
// the person about to press the button.
func (a *Admin) GetOpsStatus(
	ctx context.Context,
	req *connect.Request[v1.GetOpsStatusRequest],
) (*connect.Response[v1.GetOpsStatusResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	if a.jobs == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("job runner not wired"))
	}
	out := &v1.GetOpsStatusResponse{}

	for _, j := range a.jobs.Recent(25) {
		row := &v1.JobRow{
			Id:       j.ID,
			Kind:     j.Kind,
			Status:   string(j.Status),
			Progress: j.Progress,
			Summary:  j.Summary,
		}
		if !j.StartedAt.IsZero() {
			row.StartedAt = timestamppb.New(j.StartedAt)
		}
		if j.FinishedAt != nil {
			row.FinishedAt = timestamppb.New(*j.FinishedAt)
		} else {
			// Unfinished means it is still holding the box.
			out.Busy = true
		}
		out.Jobs = append(out.Jobs, row)
	}

	// The pipeline, from the table rather than from memory, so it
	// survives the restart that empties the runner.
	if counts, err := a.users.PipelineCounts(ctx); err == nil {
		for _, c := range counts {
			pc := &v1.PipelineCount{Status: c.Status, Count: int32(c.Count)}
			if c.Oldest != nil {
				pc.Oldest = timestamppb.New(*c.Oldest)
			}
			out.Pipeline = append(out.Pipeline, pc)
		}
	} else {
		a.log.Warn("ops: pipeline counts unavailable", slog.String("error", err.Error()))
		out.Warnings = append(out.Warnings, "submissions by status could not be read: "+err.Error())
	}

	if calls, failures, avgSeconds, err := a.users.LLMActivity(ctx); err == nil {
		out.CallsLastHour, out.CallFailuresLastHour, out.AvgCallSeconds = int32(calls), int32(failures), avgSeconds
	} else {
		a.log.Warn("ops: llm activity unavailable", slog.String("error", err.Error()))
		out.Warnings = append(out.Warnings, "model activity could not be read: "+err.Error())
	}

	if runs, err := a.users.ListEvalRuns(ctx, 1); err != nil {
		out.Warnings = append(out.Warnings, "the latest evaluation could not be read: "+err.Error())
	} else if len(runs) > 0 {
		e := runs[0]
		out.LatestEval = &v1.EvalRun{
			Id: e.ID, Note: e.Note, Status: e.Status,
			GateCorrect: int32(e.GateCorrect), Scored: int32(e.Scored),
			OrderViolations: int32(e.OrderViolations), Margin: e.Margin,
			Total: int32(e.Total),
		}
	}

	load1, load5 := hostLoad()
	out.Load_1, out.Load_5 = load1, load5
	out.MemTotalMb, out.MemAvailableMb = hostMemoryMB()
	out.DiskFreeGb, out.DiskTotalGb = diskGB("/")

	return connect.NewResponse(out), nil
}

// GetJobDetail returns one job with every progress report it made.
//
// The list on /admin/ops shows a job's latest summary, which for a run
// measured in hours is the least useful line: an evaluation says
// "posting 4 of 9: scoring" having already discarded that the first
// three took 28, 31 and 26 minutes. The pace is what separates a
// healthy run from one that is merely alive.
func (a *Admin) GetJobDetail(
	ctx context.Context,
	req *connect.Request[v1.GetJobDetailRequest],
) (*connect.Response[v1.GetJobDetailResponse], error) {
	if _, err := requireAdmin(a, ctx, req); err != nil {
		return nil, err
	}
	if a.jobs == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("job runner not wired"))
	}
	j, ok := a.jobs.Get(strings.TrimSpace(req.Msg.JobId))
	if !ok {
		// The runner forgets finished jobs after a day and everything
		// when the api restarts, so a missing id is ordinary rather than
		// a client error worth alarming about.
		return nil, connect.NewError(connect.CodeNotFound,
			errors.New("no such job; the runner forgets finished jobs after a day and all jobs when the api restarts"))
	}

	row := &v1.JobRow{
		Id:       j.ID,
		Kind:     j.Kind,
		Status:   string(j.Status),
		Progress: j.Progress,
		Summary:  j.Summary,
	}
	if !j.StartedAt.IsZero() {
		row.StartedAt = timestamppb.New(j.StartedAt)
	}
	if j.FinishedAt != nil {
		row.FinishedAt = timestamppb.New(*j.FinishedAt)
	}

	out := &v1.GetJobDetailResponse{
		Job: row,
		// At the cap the runner drops the oldest, so a full slice means
		// the timeline starts mid-run. Saying so beats letting a reader
		// conclude the job began when its first visible event did.
		Truncated: len(j.Events) >= jobs.MaxEvents,
		Events:    make([]*v1.JobEvent, 0, len(j.Events)),
	}
	for _, e := range j.Events {
		out.Events = append(out.Events, &v1.JobEvent{
			At:       timestamppb.New(e.At),
			Progress: e.Progress,
			Summary:  e.Summary,
		})
	}
	return connect.NewResponse(out), nil
}

// hostLoad reads /proc/loadavg. In a container this reports the host's
// load, not the container's, which is what we want: the question is
// whether the machine is saturated, and Ollama is a different container.
func hostLoad() (one, five float64) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0
	}
	f := strings.Fields(string(b))
	if len(f) < 2 {
		return 0, 0
	}
	one, _ = strconv.ParseFloat(f[0], 64)
	five, _ = strconv.ParseFloat(f[1], 64)
	return one, five
}

// hostMemoryMB reads /proc/meminfo, which is likewise the host's.
// MemAvailable is the number that matters: MemFree excludes cache the
// kernel would hand back under pressure, and reading it instead makes a
// healthy box look full.
func hostMemoryMB() (total, available int32) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total = int32(kb / 1024)
		case "MemAvailable:":
			available = int32(kb / 1024)
		}
	}
	return total, available
}

// diskGB reports the filesystem the api is running from. On this
// deployment that is the host disk through the overlay, which is the
// one that fills up with old image tags.
func diskGB(path string) (free, total int32) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	bs := uint64(st.Bsize)
	const gb = 1024 * 1024 * 1024
	return int32(st.Bavail * bs / gb), int32(st.Blocks * bs / gb)
}
