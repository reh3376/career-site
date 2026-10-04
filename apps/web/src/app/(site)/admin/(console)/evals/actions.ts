"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// The golden set and its evaluations (data layer D4). Running one
// re-scores every active posting through the real pipeline, which takes
// tens of minutes on the production box, so it goes through the job
// runner and the page polls it.

export type SaveState = { saved?: boolean; error?: string };

export async function upsertGoldenAction(
  _prev: SaveState,
  formData: FormData,
): Promise<SaveState> {
  const name = String(formData.get("name") ?? "").trim();
  const jdText = String(formData.get("jd_text") ?? "").trim();
  const expectedGate = String(formData.get("expected_gate") ?? "").trim();
  if (!name) return { error: "Give it a short name." };
  if (jdText.length < 40) return { error: "Paste the posting text." };
  if (expectedGate !== "above" && expectedGate !== "below") {
    return { error: "Say which side of the gate it belongs on." };
  }
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/UpsertGoldenPosting",
    body: {
      name,
      jdText,
      roleHint: String(formData.get("role_hint") ?? "").trim(),
      employerHint: String(formData.get("employer_hint") ?? "").trim(),
      expectedGate,
      expectedBand: String(formData.get("expected_band") ?? "").trim(),
      note: String(formData.get("note") ?? "").trim(),
    },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  revalidatePath("/admin/evals");
  return { saved: true };
}

export async function setGoldenActiveAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "").trim();
  const active = String(formData.get("active") ?? "") === "true";
  if (!id) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/SetGoldenActive",
    body: { id, active },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/evals");
}

async function errorText(resp: Response): Promise<string> {
  try {
    const j = (await resp.json()) as { message?: string };
    if (j.message) return j.message;
  } catch {
    /* fall through */
  }
  return `HTTP ${resp.status}`;
}

export type LabelState = { saved?: boolean; error?: string };

// Record which side of the gate a posting belongs on. Separate from the
// upsert so labelling does not mean resending the posting text, and so
// an unlabelled posting is a state the console can act on.
export async function labelGoldenAction(
  _prev: LabelState,
  formData: FormData,
): Promise<LabelState> {
  const id = String(formData.get("id") ?? "").trim();
  const expectedGate = String(formData.get("expected_gate") ?? "").trim();
  if (!id || !expectedGate) return { error: "Pick a side." };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/LabelGoldenPosting",
    body: { id, expectedGate, note: String(formData.get("note") ?? "").trim() },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  revalidatePath("/admin/evals");
  return { saved: true };
}

// Find an evaluation the runner is already working on.
//
// The progress bar lived only in component state, set when the button
// was pressed, so navigating away and back lost the job id and the page
// looked as though nothing were running. The job was always fine; the
// page had simply forgotten it was watching. An evaluation takes four
// and a half hours, so leaving the page is the normal case rather than
// the exception.
//
// Read from GetOpsStatus, which the runner answers from memory, because
// that is already the authority for what is in flight and inventing a
// second one would create two things to keep in step.
export async function findRunningEvalAction(): Promise<{
  jobId: string;
  progressPct: number;
  summary: string;
} | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;

  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetOpsStatus",
    body: {},
    cookie,
  });
  if (!resp.ok) return null;

  const j = (await resp.json()) as {
    jobs?: {
      id?: string;
      kind?: string;
      progress?: number;
      summary?: string;
      finishedAt?: string;
      finished_at?: string;
    }[];
  };
  const live = (j.jobs ?? []).find(
    (x) =>
      (x.kind ?? "").includes("EVAL") &&
      !(x.finishedAt ?? x.finished_at) &&
      (x.id ?? "") !== "",
  );
  if (!live) return null;
  return {
    jobId: live.id ?? "",
    progressPct: live.progress ?? 0,
    summary: live.summary ?? "",
  };
}
