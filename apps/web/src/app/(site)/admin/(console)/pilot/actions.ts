"use server";

import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export type DeleteState = { error?: string };

// Deletes one pilot session with everything it holds (FSD FR-PLT-15):
// the owner's own runs on the deployed build, removed before the ten
// start so they never sit beside a participant's in the list or an
// export. The button confirms first; on success the admin lands back
// on the list, which no longer shows the row.
export async function deletePilotSessionAction(_prev: DeleteState, formData: FormData): Promise<DeleteState> {
  const id = String(formData.get("id") ?? "");
  if (!id) return { error: "No session id." };
  const cookie = await getSessionCookie();
  if (!cookie) redirect("/login?next=/admin/pilot");

  const resp = await callApi({
    path: "/api/career.v1.PilotService/DeletePilotSession",
    body: { sessionId: id },
    cookie,
  });
  if (!resp.ok) {
    let message = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) message = j.message;
    } catch {
      /* keep the status */
    }
    return { error: `Couldn't delete the session, ${message}.` };
  }
  redirect("/admin/pilot");
}
