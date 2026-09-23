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

  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };

  let resp: Response;
  try {
    resp = await callApi({
      path: "/api/career.v1.AdminService/ReviewDecision",
      body: { id, humanVerdict, humanNote },
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
