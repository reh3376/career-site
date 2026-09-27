"use server";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export type JobEvent = {
  at?: string;
  progress?: number;
  summary?: string;
  // The record this report is about, when the job named one. An
  // evaluation writes "eval:<run id>:<golden id>". Empty for jobs that
  // have nothing to point at, and those events are not clickable.
  ref?: string;
};

// What a timeline event resolves to when it carries a ref.
export type EventTarget =
  | {
      kind: "eval-item";
      runId: string;
      runNote: string;
      runStatus: string;
      threshold?: number;
      goldenName: string;
      expectedGate: string;
      gateSide: string;
      matchScore?: number;
      fit: string;
      passed: boolean;
      error: string;
      submissionId?: string;
    }
  | {
      kind: "eval-run";
      runId: string;
      runNote: string;
      runStatus: string;
      threshold?: number;
      scored: number;
      total: number;
      gateCorrect: number;
      orderViolations: number;
      margin?: number;
      errors: number;
    }
  | { kind: "pending"; goldenName: string; runId: string }
  | { kind: "none" };

export type EventDetail =
  | { ok: true; target: EventTarget }
  | { ok: false; error: string };

export type JobDetail =
  | {
      ok: true;
      job: {
        id?: string;
        kind?: string;
        status?: string;
        progress?: number;
        summary?: string;
        startedAt?: string;
        finishedAt?: string;
      };
      events: JobEvent[];
      truncated: boolean;
    }
  | { ok: false; error: string };

// Fetched on click rather than carried in the polled list. /admin/ops
// refreshes while a job runs, and a few hundred events per job in that
// payload would make the page heavier the longer the run goes, which is
// backwards for a page you open because something is running.
export async function getJobDetailAction(jobId: string): Promise<JobDetail> {
  const id = jobId.trim();
  if (!id) return { ok: false, error: "No job id." };

  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in." };

  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetJobDetail",
    body: { jobId: id },
    cookie,
  });
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep the status */
    }
    return { ok: false, error: msg };
  }

  // Both spellings, because the gateway emits camelCase and a proxy in
  // front of it has been seen to pass the proto's snake_case through.
  const raw = (await resp.json()) as {
    job?: {
      id?: string;
      kind?: string;
      status?: string;
      progress?: number;
      summary?: string;
      startedAt?: string;
      started_at?: string;
      finishedAt?: string;
      finished_at?: string;
    };
    events?: JobEvent[];
    truncated?: boolean;
  };

  return {
    ok: true,
    job: {
      id: raw.job?.id,
      kind: raw.job?.kind,
      status: raw.job?.status,
      progress: raw.job?.progress,
      summary: raw.job?.summary,
      startedAt: raw.job?.startedAt ?? raw.job?.started_at,
      finishedAt: raw.job?.finishedAt ?? raw.job?.finished_at,
    },
    events: raw.events ?? [],
    truncated: raw.truncated ?? false,
  };
}

// Both spellings again: the gateway emits camelCase and a proxy in
// front of it has been seen to pass the proto's snake_case through.
type RawEvalItem = {
  golden_id?: string | number;
  goldenId?: string | number;
  golden_name?: string;
  goldenName?: string;
  expected_gate?: string;
  expectedGate?: string;
  gate_side?: string;
  gateSide?: string;
  match_score?: number;
  matchScore?: number;
  submission_id?: string | number;
  submissionId?: string | number;
  fit?: string;
  passed?: boolean;
  error?: string;
};

type RawEvalRun = {
  id?: string | number;
  status?: string;
  note?: string;
  threshold?: number;
  total?: number;
  scored?: number;
  gate_correct?: number;
  gateCorrect?: number;
  order_violations?: number;
  orderViolations?: number;
  margin?: number;
  errors?: number;
  items?: RawEvalItem[];
};

function n(v: number | undefined): number {
  return typeof v === "number" ? v : 0;
}

// Resolve a timeline event's ref to the record it names.
//
// The ref is written by the job, not parsed out of its summary. A
// progress line is prose for a person to read, and the moment a regex
// over it becomes load-bearing the wording can no longer be improved.
//
// An evaluation reports the ref before it scores the posting, because
// the report is what marks the start of the work and the gap to the
// next one is how long it took. So a reader who opens an event while
// that posting is still being scored finds no item, which is reported
// as "pending" rather than as an error: nothing is wrong, the answer
// does not exist yet.
export async function getEventDetailAction(ref: string): Promise<EventDetail> {
  const parts = ref.trim().split(":");
  if (parts.length !== 3 || parts[0] !== "eval") {
    return { ok: false, error: `Unrecognised reference "${ref}".` };
  }
  const runId = parts[1];
  const goldenId = parts[2];
  if (!/^\d+$/.test(runId) || !/^\d+$/.test(goldenId)) {
    return { ok: false, error: `Unrecognised reference "${ref}".` };
  }

  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in." };

  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetEvalRun",
    body: { id: runId },
    cookie,
  });
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep the status */
    }
    return { ok: false, error: msg };
  }

  const run = ((await resp.json()) as { run?: RawEvalRun }).run;
  if (!run) return { ok: false, error: "The evaluation is no longer there." };

  const runNote = run.note ?? "";
  const runStatus = run.status ?? "";

  // Golden id 0 is no posting: the ref points at the run itself.
  if (goldenId === "0") {
    return {
      ok: true,
      target: {
        kind: "eval-run",
        runId,
        runNote,
        runStatus,
        threshold: run.threshold,
        scored: n(run.scored),
        total: n(run.total),
        gateCorrect: n(run.gate_correct ?? run.gateCorrect),
        orderViolations: n(run.order_violations ?? run.orderViolations),
        margin: run.margin,
        errors: n(run.errors),
      },
    };
  }

  const item = (run.items ?? []).find(
    (i) => String(i.golden_id ?? i.goldenId ?? "") === goldenId,
  );
  if (!item) {
    return { ok: true, target: { kind: "pending", goldenName: "", runId } };
  }

  const submissionId = item.submission_id ?? item.submissionId;
  return {
    ok: true,
    target: {
      kind: "eval-item",
      runId,
      runNote,
      runStatus,
      threshold: run.threshold,
      goldenName: item.golden_name ?? item.goldenName ?? "",
      expectedGate: item.expected_gate ?? item.expectedGate ?? "",
      gateSide: item.gate_side ?? item.gateSide ?? "",
      matchScore: item.match_score ?? item.matchScore,
      fit: item.fit ?? "",
      passed: item.passed ?? false,
      error: item.error ?? "",
      submissionId:
        submissionId === undefined || submissionId === null || submissionId === 0
          ? undefined
          : String(submissionId),
    },
  };
}
