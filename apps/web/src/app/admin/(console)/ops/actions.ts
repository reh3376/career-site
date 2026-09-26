"use server";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export type JobEvent = {
  at?: string;
  progress?: number;
  summary?: string;
};

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
