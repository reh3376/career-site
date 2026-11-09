"use server";

import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export type DeleteState = { error?: string };

async function message(resp: Response): Promise<string> {
  let m = `HTTP ${resp.status}`;
  try {
    const j = (await resp.json()) as { message?: string };
    if (j.message) m = j.message;
  } catch {
    /* keep the status */
  }
  return m;
}

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
  if (!resp.ok) return { error: `Couldn't delete the session, ${await message(resp)}.` };
  redirect("/admin/pilot");
}

export type NoteResult = { error?: string; savedAt?: string; removed?: boolean };

// Saves or removes one of the owner's notes (FSD FR-PLT-16). Called
// from the note boxes on the session page; an empty body removes.
export async function upsertPilotNoteAction(
  sessionId: string,
  targetKind: string,
  targetId: string,
  body: string,
): Promise<NoteResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.PilotService/UpsertPilotNote",
    body: { sessionId, targetKind, targetId, body },
    cookie,
  });
  if (!resp.ok) return { error: await message(resp) };
  const j = (await resp.json()) as { updatedAt?: string; removed?: boolean };
  return { savedAt: j.updatedAt, removed: j.removed };
}

export type ReviewState = { error?: string; saved?: boolean };

// Sets the owner's curation judgement on a session (FSD FR-PLT-17).
export async function setPilotReviewAction(_prev: ReviewState, formData: FormData): Promise<ReviewState> {
  const sessionId = String(formData.get("id") ?? "");
  const status = String(formData.get("status") ?? "");
  const note = String(formData.get("note") ?? "").trim();
  if (!sessionId || !status) return { error: "Pick a judgement." };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.PilotService/SetPilotSessionReview",
    body: { sessionId, status, note },
    cookie,
  });
  if (!resp.ok) return { error: await message(resp) };
  return { saved: true };
}
