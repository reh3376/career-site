"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// The candidate's side of the clarification loop (FSD FR-HF-13, 14):
// save an answer, take one back, submit the lot, and ask for questions
// on a posting. The server checks every value against the inputs the
// question offered; these only carry the forms.

async function errorText(resp: Response): Promise<string> {
  try {
    const j = (await resp.json()) as { message?: string };
    if (j.message) return j.message;
  } catch {
    /* fall through */
  }
  return `HTTP ${resp.status}`;
}

function refresh(): void {
  revalidatePath("/admin/questions", "layout");
  revalidatePath("/admin/jd", "layout");
}

export type AnswerState = { saved?: string; error?: string };

// One answer to one question. `intent` is the button pressed: answered,
// not_applicable ("I have not done this") or declined ("prefer not to
// answer"). Inputs travel as f:<name>; every value is sent as a list,
// which is how the API takes them.
export async function answerItemAction(_prev: AnswerState, formData: FormData): Promise<AnswerState> {
  const itemId = String(formData.get("item_id") ?? "").trim();
  const intent = String(formData.get("intent") ?? "").trim();
  if (!itemId) return { error: "The question is missing." };
  if (!["answered", "not_applicable", "declined"].includes(intent)) return { error: "Choose how to answer." };
  const values: Record<string, { values: string[] }> = {};
  if (intent === "answered") {
    for (const key of new Set(Array.from(formData.keys()))) {
      if (!key.startsWith("f:")) continue;
      const list = formData
        .getAll(key)
        .map((v) => String(v).trim())
        .filter(Boolean);
      if (list.length) values[key.slice(2)] = { values: list };
    }
  }
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.HiringService/AnswerClarificationItem",
    body: {
      clarificationItemId: itemId,
      answer: { status: intent, values, freeText: String(formData.get("free_text") ?? "").trim() },
    },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  refresh();
  return { saved: intent };
}

export type WithdrawState = { withdrawn?: boolean; stale?: number; error?: string };

export async function withdrawAnswerAction(_prev: WithdrawState, formData: FormData): Promise<WithdrawState> {
  const itemId = String(formData.get("item_id") ?? "").trim();
  if (!itemId) return { error: "The question is missing." };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.HiringService/WithdrawClarificationAnswer",
    body: { clarificationItemId: itemId },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  let stale = 0;
  try {
    stale = Number(((await resp.json()) as { staleJudgments?: number }).staleJudgments ?? 0);
  } catch {
    /* keep zero */
  }
  refresh();
  return { withdrawn: true, stale };
}

export type SubmitState = { submitted?: boolean; runStarted?: boolean; error?: string };

export async function submitAnswersAction(_prev: SubmitState, formData: FormData): Promise<SubmitState> {
  const id = String(formData.get("request_id") ?? "").trim();
  if (!id) return { error: "The questionnaire is missing." };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.HiringService/SubmitClarificationAnswers",
    body: { clarificationRequestId: id },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  let runStarted = false;
  try {
    runStarted = Boolean(((await resp.json()) as { runStarted?: boolean }).runStarted);
  } catch {
    /* keep false */
  }
  refresh();
  return { submitted: true, runStarted };
}

export type AskState = { requestId?: string; added?: number; error?: string };

// Questions at the candidate's own request: one for every requirement
// the posting's newest record left unresolved and nobody has asked
// about.
export async function requestClarificationAction(_prev: AskState, formData: FormData): Promise<AskState> {
  const submissionId = String(formData.get("submission_id") ?? "").trim();
  if (!submissionId) return { error: "The posting is missing." };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.HiringService/RequestClarification",
    body: { submissionId },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  let out: AskState = {};
  try {
    const j = (await resp.json()) as { request?: { id?: string }; added?: number };
    out = { requestId: j.request?.id, added: Number(j.added ?? 0) };
  } catch {
    /* the page re-reads either way */
  }
  refresh();
  return out;
}
