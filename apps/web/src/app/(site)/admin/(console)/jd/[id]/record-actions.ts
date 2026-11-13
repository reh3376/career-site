"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// The override of a judgment (FSD FR-HF-08): set or replace it, or
// withdraw it. The server validates the pair, logs the call and
// computes the record's numbers again; this only carries the form.

export type OverrideState = { saved?: "override" | "withdraw"; error?: string };

async function errorText(resp: Response): Promise<string> {
  try {
    const j = (await resp.json()) as { message?: string };
    if (j.message) return j.message;
  } catch {
    /* fall through */
  }
  return `HTTP ${resp.status}`;
}

export async function overrideJudgmentAction(
  _prev: OverrideState,
  formData: FormData,
): Promise<OverrideState> {
  const judgmentId = String(formData.get("judgment_id") ?? "").trim();
  if (!judgmentId) return { error: "The judgment is missing." };
  const withdraw = formData.get("intent") === "withdraw";
  const reason = String(formData.get("reason") ?? "").trim();
  if (!reason) return { error: "A reason is required." };
  const body = withdraw
    ? { judgmentId, withdraw: true, reason }
    : {
        judgmentId,
        determination: String(formData.get("determination") ?? "").trim(),
        basis: String(formData.get("basis") ?? "").trim(),
        role: String(formData.get("role") ?? "").trim(),
        reason,
      };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.HiringService/OverrideJudgment",
    body,
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  revalidatePath("/admin/jd", "layout");
  return { saved: withdraw ? "withdraw" : "override" };
}

// The résumé after review (FSD FR-HF-08): written when the owner asks,
// on the verdicts as they stand, for him alone. This only starts it;
// the page shows it being written and then the résumé.
export type ReviewResumeState = { started?: boolean; error?: string };

export async function writeReviewResumeAction(
  _prev: ReviewResumeState,
  formData: FormData,
): Promise<ReviewResumeState> {
  const runId = String(formData.get("run_id") ?? "").trim();
  if (!runId) return { error: "The run is missing." };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.HiringService/WriteReviewResume",
    body: { runId },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  revalidatePath("/admin/jd", "layout");
  return { started: true };
}

