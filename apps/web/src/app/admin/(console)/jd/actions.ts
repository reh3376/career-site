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
