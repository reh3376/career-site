"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Queue a re-score for one submission; the page re-renders with the
// row back at "scoring" and the derivation refreshes as it completes.
export async function rescoreJdAction(formData: FormData): Promise<void> {
  const submissionId = String(formData.get("submission_id") ?? "");
  if (!submissionId) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/RescoreJd",
    body: { submissionId },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/jd", "layout");
}

export type FitBandsState = {
  saved?: boolean;
  error?: string;
  bands?: {
    veryStrong: number;
    strong: number;
    possible: number;
    weak: number;
  };
};

// Save the fit bands (very strong / strong / possible / weak edges).
export async function setFitBandsAction(
  _prev: FitBandsState,
  formData: FormData,
): Promise<FitBandsState> {
  const read = (k: string) => Number(String(formData.get(k) ?? "").trim());
  const bands = {
    veryStrong: read("veryStrong"),
    strong: read("strong"),
    possible: read("possible"),
    weak: read("weak"),
  };
  if (Object.values(bands).some((v) => !Number.isFinite(v))) {
    return { error: "Every band needs a number between 0 and 1.", bands };
  }
  if (
    !(
      bands.weak > 0 &&
      bands.weak < bands.possible &&
      bands.possible < bands.strong &&
      bands.strong < bands.veryStrong &&
      bands.veryStrong <= 1
    )
  ) {
    return {
      error:
        "Order must hold: weak < possible < strong < very strong, within 0 and 1.",
      bands,
    };
  }
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in.", bands };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/SetJdFitBands",
    body: { bands },
    cookie,
  });
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep default */
    }
    return { error: msg, bands };
  }
  revalidatePath("/admin/jd", "layout");
  revalidatePath("/jd-upload", "layout");
  return { saved: true, bands };
}

export type OutcomeState = { saved?: boolean; error?: string };

// Record what happened after a review. This is the only signal that
// says whether a score predicted anything, so the form is deliberately
// on the submission page rather than buried in a report.
export async function setJdOutcomeAction(
  _prev: OutcomeState,
  formData: FormData,
): Promise<OutcomeState> {
  const submissionId = String(formData.get("submission_id") ?? "").trim();
  const status = String(formData.get("status") ?? "").trim();
  if (!submissionId || !status) return { error: "Pick a status." };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/SetJdOutcome",
    body: {
      submissionId,
      status,
      decidedOn: String(formData.get("decided_on") ?? "").trim(),
      note: String(formData.get("note") ?? "").trim(),
    },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  revalidatePath("/admin/jd", "layout");
  return { saved: true };
}

export type FeedbackState = { saved?: boolean; error?: string };

// Record the owner's judgment of a run's output: was the score
// accurate, too generous or too harsh, and is the résumé sendable.
export async function recordJdFeedbackAction(
  _prev: FeedbackState,
  formData: FormData,
): Promise<FeedbackState> {
  const submissionId = String(formData.get("submission_id") ?? "").trim();
  const target = String(formData.get("target") ?? "").trim();
  const rating = String(formData.get("rating") ?? "").trim();
  if (!submissionId || !target || !rating) return { error: "Pick a rating." };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/RecordJdFeedback",
    body: {
      submissionId,
      runId: String(formData.get("run_id") ?? "").trim(),
      target,
      rating,
      note: String(formData.get("note") ?? "").trim(),
    },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  revalidatePath("/admin/jd", "layout");
  return { saved: true };
}

// Connect returns its own message on an error; surfacing it beats a
// bare status code, because the server is the one enforcing the
// vocabularies and it says which value it rejected.
async function errorText(resp: Response): Promise<string> {
  try {
    const j = (await resp.json()) as { message?: string };
    if (j.message) return j.message;
  } catch {
    /* fall through */
  }
  return `HTTP ${resp.status}`;
}

export type LimitState = { saved?: boolean; error?: string; limit?: number };

// How many postings one member may submit per rolling day. Stored in
// app_settings and cached by the api for 15 seconds, so a change is in
// force for the next submission without a deploy.
export async function setJdLimitAction(
  _prev: LimitState,
  formData: FormData,
): Promise<LimitState> {
  const raw = String(formData.get("limit") ?? "").trim();
  const limit = Number(raw);
  if (!Number.isInteger(limit) || limit < 0 || limit > 100) {
    return { error: "A whole number between 0 and 100." };
  }
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in.", limit };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/SetJdSubmissionLimit",
    body: { limit },
    cookie,
  });
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep default */
    }
    return { error: msg, limit };
  }
  revalidatePath("/admin/jd", "layout");
  revalidatePath("/jd-upload", "layout");
  return { saved: true, limit };
}

// The limit in force, for the console to render. Falls back to
// undefined rather than a guess: a number nobody set is worse than no
// number, because it looks authoritative.
export async function getJdLimit(): Promise<
  { limit: number; windowHours: number } | undefined
> {
  const cookie = await getSessionCookie();
  if (!cookie) return undefined;
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetJdSubmissionLimit",
    body: {},
    cookie,
  });
  if (!resp.ok) return undefined;
  try {
    const j = (await resp.json()) as { limit?: number; windowHours?: number };
    return { limit: j.limit ?? 0, windowHours: j.windowHours ?? 24 };
  } catch {
    return undefined;
  }
}
