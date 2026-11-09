"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import type { PilotComment, PilotSession } from "./lib";

// Server actions behind the pilot pages (FSD §5.16). Every write
// reports what the API said; a refused section submission names the
// item, and the form shows it beside the question.

type Result<T> = { ok: true; data: T } | { ok: false; message: string };

async function post<T>(method: string, body: unknown): Promise<Result<T>> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, message: "Not signed in." };
  let resp: Response;
  try {
    resp = await callApi({ path: `/api/career.v1.PilotService/${method}`, body, cookie });
  } catch (e) {
    return { ok: false, message: e instanceof Error ? e.message : "The request did not reach the API." };
  }
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep the status */
    }
    return { ok: false, message: msg };
  }
  return { ok: true, data: (await resp.json()) as T };
}

export type ConsentState = { error?: string };

// Records consent and opens the session, then sends the participant to
// the first section (or the one they had reached, on a resume).
export async function startPilotAction(_prev: ConsentState, formData: FormData): Promise<ConsentState> {
  if (formData.get("consent") !== "on") {
    return { error: "Tick the consent box to take part." };
  }
  const recordingOk = formData.get("recording_ok") === "on";
  const res = await post<{ session?: PilotSession }>("StartPilot", { consent: true, recordingOk });
  if (!res.ok) return { error: res.message };
  revalidatePath("/pilot", "layout");
  const next = res.data.session?.nextSection;
  redirect(next ? `/pilot/survey/${next}` : "/pilot");
}

export type AnswerInput = { itemId: string; valueJson: string; flagged: boolean };
export type SubmitResult = { error?: string; next?: string };

// Saves a section and says where to go next: the following section,
// or the pilot home when the survey is complete or ended.
export async function submitSectionAction(sectionId: string, answers: AnswerInput[]): Promise<SubmitResult> {
  const res = await post<{ session?: PilotSession; ended?: boolean }>("SubmitPilotSection", {
    sectionId,
    answers,
  });
  if (!res.ok) return { error: res.message };
  revalidatePath("/pilot", "layout");
  const next = res.data.session?.nextSection;
  if (res.data.ended || !next) return { next: "/pilot" };
  return { next: `/pilot/survey/${next}` };
}

export type CommentResult = { error?: string; savedAt?: string; removed?: boolean };

// Saves, replaces or removes one comment.
export async function upsertCommentAction(
  targetKind: string,
  targetId: string,
  body: string,
): Promise<CommentResult> {
  const res = await post<{ comment?: PilotComment }>("UpsertPilotComment", { targetKind, targetId, body });
  if (!res.ok) return { error: res.message };
  if (!res.data.comment) return { removed: true };
  return { savedAt: res.data.comment.updatedAt ?? new Date().toISOString() };
}

// Marks the record or the workflow part done and returns to the home.
export async function markPartAction(part: "record" | "workflow"): Promise<void> {
  await post("MarkPilotPart", { part });
  revalidatePath("/pilot", "layout");
  redirect("/pilot");
}

export type FinishState = { error?: string };

// Closes the session once the survey is complete.
export async function finishPilotAction(): Promise<FinishState> {
  const res = await post("FinishPilot", {});
  if (!res.ok) return { error: res.message };
  revalidatePath("/pilot", "layout");
  redirect("/pilot/done");
}
