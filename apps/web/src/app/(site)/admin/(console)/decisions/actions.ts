"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export type ReviewState = { saved?: boolean; error?: string };

// Save the owner's verdict and note on one logged decision. The row
// keeps the model's output untouched; only the human_* columns change.
//
// This used to swallow every failure: it ignored the response status
// and caught the error, so a rejected review looked exactly like a
// saved one. The page revalidated, the row was still in the queue, and
// nothing said why. That is how a whole decision kind sat ungradeable
// without anyone noticing. A write that can fail must say so.
export async function reviewDecisionAction(
  _prev: ReviewState,
  formData: FormData,
): Promise<ReviewState> {
  const id = String(formData.get("decision_id") ?? "");
  const humanVerdict = String(formData.get("human_verdict") ?? "");
  const humanNote = String(formData.get("human_note") ?? "").slice(0, 4000);
  if (!id) return { error: "Missing decision id." };
  if (!humanVerdict) return { error: "Pick a verdict." };

  // The corrected wording, for a decision whose output is prose.
  //
  // The form seeds this box with the model's own answer so a correction
  // costs an edit rather than a retype, which is the difference between
  // grading a hundred answers and grading five. The cost of that is
  // that an untouched box looks like a correction, so the original
  // travels alongside and an unchanged answer is dropped here. Saving
  // human_answer identical to response_text would produce a preference
  // pair of a thing against itself: noise in the training set that
  // nothing downstream could detect.
  const modelAnswer = String(formData.get("model_answer") ?? "").trim();
  const typed = String(formData.get("human_answer") ?? "").slice(0, 8000);
  const humanAnswer = typed.trim() === modelAnswer ? "" : typed;

  // Per-rubric marks, sent as dim_<name>. Unknown keys and values are
  // refused by the server rather than dropped, so nothing is filtered
  // here: a mismatch between this form and the rubric should be visible
  // as an error, not silently swallowed.
  const humanDimensions: Record<string, string> = {};
  for (const [k, v] of formData.entries()) {
    if (k.startsWith("dim_") && typeof v === "string" && v) {
      humanDimensions[k.slice(4)] = v;
    }
  }

  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };

  let resp: Response;
  try {
    resp = await callApi({
      path: "/api/career.v1.AdminService/ReviewDecision",
      body: { id, humanVerdict, humanNote, humanAnswer, humanDimensions },
      cookie,
    });
  } catch (e) {
    return {
      error:
        e instanceof Error ? e.message : "The request did not reach the API.",
    };
  }
  if (!resp.ok) {
    // The server's message names the vocabulary that applies to this
    // decision kind, which is exactly what a reviewer needs to see.
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep the status */
    }
    return { error: msg };
  }
  revalidatePath("/admin/decisions", "layout");
  return { saved: true };
}
