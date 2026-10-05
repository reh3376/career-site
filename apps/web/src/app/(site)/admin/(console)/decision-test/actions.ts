"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Changing a timing changes the instrument, and the instrument version
// is derived from the timings rather than kept beside them, so there is
// no version to forget to bump. Every run after a change records the
// new one, and runs under different timings never pool into one dataset.
export async function setDecisionTestTimingsAction(formData: FormData): Promise<void> {
  const cookie = await getSessionCookie();
  if (!cookie) return;
  const sec = (name: string) => Math.round(Number(formData.get(name) ?? 0) * 1000);
  await callApi({
    path: "/api/career.v1.AdminService/SetDecisionTestSettings",
    body: {
      memoriseMs: sec("memorise_s"),
      questionMs: sec("question_s"),
      recallMs: sec("recall_s"),
    },
    cookie,
  });
  revalidatePath("/admin/decision-test");
}
