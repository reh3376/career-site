import "server-only";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Server-side reads for the pilot pages (FSD §5.16). Mirrors
// career.v1.PilotService in wire form: camelCase from the ConnectRPC
// JSON codec, with unpopulated fields absent, so everything optional is
// typed optional and read with a default.

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

export type PilotStatus = {
  allowed?: boolean;
  admin?: boolean;
  instrumentVersion?: string;
  session?: PilotSession;
};

export type PilotScale = { min?: number; max?: number; minLabel?: string; maxLabel?: string };
export type PilotDimension = { id: string; label?: string; options?: string[] };
export type PilotItem = {
  id: string;
  type: string;
  prompt: string;
  help?: string;
  options?: string[];
  maxSelect?: number;
  required?: boolean;
  scale?: PilotScale;
  dimensions?: PilotDimension[];
  pilotOnly?: boolean;
  // For single_from and rating_per_selected, the multi item whose
  // selection supplies the rows; the form reads it live.
  sourceItem?: string;
};
export type PilotAnswer = { itemId: string; valueJson?: string; flagged?: boolean };
export type PilotSectionView = {
  id: string;
  title?: string;
  intro?: string;
  items?: PilotItem[];
  answers?: PilotAnswer[];
  index?: number;
  total?: number;
  session?: PilotSession;
};
export type PilotComment = { targetKind: string; targetId?: string; body: string; updatedAt?: string };
export type PilotRecordView = { recordJson?: string; comments?: PilotComment[]; recordVersion?: string };
export type PilotWorkflowNode = {
  id: string;
  phase?: string;
  name: string;
  owner?: string;
  summary?: string;
  decision?: boolean;
  open?: boolean;
};
export type PilotWorkflowView = {
  intro?: string;
  prompts?: string[];
  phases?: { id: string; title?: string }[];
  nodes?: PilotWorkflowNode[];
  comments?: PilotComment[];
  workflowVersion?: string;
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

export const getPilot = () => get<PilotStatus>("GetPilot", {});
export const getPilotSection = (sectionId: string) =>
  get<PilotSectionView>("GetPilotSection", { sectionId });
export const getPilotRecord = () => get<PilotRecordView>("GetPilotRecord", {});
export const getPilotWorkflow = () => get<PilotWorkflowView>("GetPilotWorkflow", {});

// The three parts and where the participant stands in each, derived
// from the session alone so every page agrees.
export function progressOf(s: PilotSession | undefined) {
  const surveyDone = !!s?.surveyFinishedAt || s?.roleBranch === "end";
  return {
    surveyDone,
    surveyHref: surveyDone || !s ? "/pilot/survey/A" : `/pilot/survey/${s.nextSection || "A"}`,
    recordDone: !!s?.recordFinishedAt,
    workflowDone: !!s?.workflowFinishedAt,
    finished: s?.status === "finished",
  };
}
