"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Save the owner's verdict and note on one logged decision. The row
// keeps the model's output untouched; only the human_* columns change.
export async function reviewDecisionAction(formData: FormData): Promise<void> {
  const id = String(formData.get("decision_id") ?? "");
  const humanVerdict = String(formData.get("human_verdict") ?? "");
  const humanNote = String(formData.get("human_note") ?? "").slice(0, 4000);
  if (!id || !humanVerdict) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/ReviewDecision",
    body: { id, humanVerdict, humanNote },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/decisions", "layout");
}
