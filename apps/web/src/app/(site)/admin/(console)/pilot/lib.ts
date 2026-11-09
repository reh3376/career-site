import "server-only";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Admin reads of the pilot (FSD FR-PLT-11), in wire form.
export type PilotSession = {
  id: string;
  status?: string;
  roleBranch?: string;
  instrumentVersion?: string;
  consentAt?: string;
  recordingOk?: boolean;
  surveyFinishedAt?: string;
  recordFinishedAt?: string;
  workflowFinishedAt?: string;
  finishedAt?: string;
  sectionsDone?: string[];
  nextSection?: string;
};

export type PilotSessionSummary = {
  session?: PilotSession;
  memberId?: string;
  name?: string;
  email?: string;
  measuredMinutes?: number;
  selfReportedMinutes?: number;
  commentCount?: number;
  flagCount?: number;
  reviewStatus?: string;
  reviewNote?: string;
  reviewedAt?: string;
};

export type PilotAnswer = { itemId: string; valueJson?: string; flagged?: boolean; prompt?: string; section?: string; note?: string };
export type PilotNote = { targetKind: string; targetId?: string; body: string; author?: string; updatedAt?: string };
export type PilotComment = { targetKind: string; targetId?: string; body: string; updatedAt?: string };
export type PilotSectionEvent = { section: string; event: string; at?: string };

export type ListResp = { sessions?: PilotSessionSummary[]; instrumentVersion?: string };
export type DetailResp = {
  summary?: PilotSessionSummary;
  answers?: PilotAnswer[];
  comments?: PilotComment[];
  events?: PilotSectionEvent[];
  notes?: PilotNote[];
};

export type Fetched<T> = { ok: true; data: T } | { ok: false; status: number; message: string };

async function get<T>(method: string, body: unknown): Promise<Fetched<T>> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, status: 401, message: "Not signed in" };
  const resp = await callApi({ path: `/api/career.v1.PilotService/${method}`, body, cookie });
  if (!resp.ok) {
    let message = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) message = j.message;
    } catch {
      /* keep the status */
    }
    return { ok: false, status: resp.status, message };
  }
  return { ok: true, data: (await resp.json()) as T };
}

export const listPilotSessions = () => get<ListResp>("ListPilotSessions", {});
export const getPilotSession = (sessionId: string) => get<DetailResp>("GetPilotSession", { sessionId });

export function fmt(iso?: string): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toISOString().slice(0, 16).replace("T", " ") + " UTC";
}

export const BRANCH_LABEL: Record<string, string> = {
  recruiter: "recruiter",
  hm: "hiring manager",
  both: "both",
  end: "ended at A1",
  "": "not yet routed",
};

export const STATUS_TONE: Record<string, string> = {
  consented: "text-signal",
  finished: "text-success",
};

// The owner's curation judgement (FSD FR-PLT-17).
export const REVIEW_LABEL: Record<string, string> = {
  unreviewed: "unreviewed",
  reviewed: "reviewed",
  excluded: "excluded",
};
export const REVIEW_TONE: Record<string, string> = {
  unreviewed: "text-ink-3",
  reviewed: "text-success",
  excluded: "text-danger",
};
