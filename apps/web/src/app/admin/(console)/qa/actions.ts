"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export type QaState = { saved?: boolean; error?: string };

type Source = { title: string; path: string };

// Every write here reports what actually happened.
//
// The decision console taught this the hard way: its review action
// ignored the response status and caught the error, so a rejected save
// looked exactly like a successful one and a whole decision kind sat
// ungradeable for weeks with nothing saying why. The server refuses a
// bad source path, an over-long answer and an unknown id, and each of
// those refusals is worth reading.
async function post(path: string, body: unknown): Promise<QaState> {
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };

  let resp: Response;
  try {
    resp = await callApi({ path, body, cookie });
  } catch (e) {
    return {
      error:
        e instanceof Error ? e.message : "The request did not reach the API.",
    };
  }
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep the status */
    }
    return { error: msg };
  }
  revalidatePath("/admin/qa", "layout");
  return { saved: true };
}

// Sources arrive as parallel source_title_N / source_path_N fields. A
// pair with neither filled in is a blank row the owner did not use, and
// is dropped rather than sent as an empty source.
function readSources(formData: FormData): Source[] {
  const out: Source[] = [];
  for (let i = 0; i < 8; i++) {
    const title = String(formData.get(`source_title_${i}`) ?? "").trim();
    const path = String(formData.get(`source_path_${i}`) ?? "").trim();
    if (title || path) out.push({ title, path });
  }
  return out;
}

function readTags(formData: FormData): string[] {
  return String(formData.get("tags") ?? "")
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean);
}

export async function createQaEntryAction(
  _prev: QaState,
  formData: FormData,
): Promise<QaState> {
  const question = String(formData.get("question") ?? "").trim();
  const answer = String(formData.get("answer") ?? "").trim();
  if (!question) return { error: "The question is empty." };
  if (!answer) return { error: "The answer is empty." };

  // Extra phrasings, one per line. The question itself becomes the
  // canonical phrasing on the server, so it is not repeated here.
  const phrasings = String(formData.get("phrasings") ?? "")
    .split("\n")
    .map((p) => p.trim())
    .filter(Boolean);

  return post("/api/career.v1.AdminService/CreateQaEntry", {
    question,
    answer,
    sources: readSources(formData),
    tags: readTags(formData),
    coversRestricted: formData.get("covers_restricted") === "on",
    enabled: formData.get("enabled") === "on",
    phrasings,
  });
}

export async function updateQaEntryAction(
  _prev: QaState,
  formData: FormData,
): Promise<QaState> {
  const id = String(formData.get("id") ?? "");
  const question = String(formData.get("question") ?? "").trim();
  const answer = String(formData.get("answer") ?? "").trim();
  if (!id) return { error: "Missing entry id." };
  if (!question) return { error: "The question is empty." };
  if (!answer) return { error: "The answer is empty." };

  return post("/api/career.v1.AdminService/UpdateQaEntry", {
    id,
    question,
    answer,
    sources: readSources(formData),
    tags: readTags(formData),
    coversRestricted: formData.get("covers_restricted") === "on",
    enabled: formData.get("enabled") === "on",
  });
}

export async function setQaEnabledAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "");
  const enabled = String(formData.get("enabled") ?? "") === "true";
  if (!id) return;
  await post("/api/career.v1.AdminService/SetQaEntryEnabled", { id, enabled });
}

export async function deleteQaEntryAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "");
  if (!id) return;
  await post("/api/career.v1.AdminService/DeleteQaEntry", { id });
}

export async function addQaPhrasingAction(
  _prev: QaState,
  formData: FormData,
): Promise<QaState> {
  const entryId = String(formData.get("entry_id") ?? "");
  const text = String(formData.get("text") ?? "").trim();
  if (!entryId) return { error: "Missing entry id." };
  if (!text) return { error: "The phrasing is empty." };
  return post("/api/career.v1.AdminService/AddQaPhrasing", { entryId, text });
}

export async function deleteQaPhrasingAction(
  formData: FormData,
): Promise<void> {
  const entryId = String(formData.get("entry_id") ?? "");
  const phrasingId = String(formData.get("phrasing_id") ?? "");
  if (!entryId || !phrasingId) return;
  await post("/api/career.v1.AdminService/DeleteQaPhrasing", {
    entryId,
    phrasingId,
  });
}
