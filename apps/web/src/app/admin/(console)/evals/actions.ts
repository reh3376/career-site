"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// The golden set and its evaluations (data layer D4). Running one
// re-scores every active posting through the real pipeline, which takes
// tens of minutes on the production box, so it goes through the job
// runner and the page polls it.

export type SaveState = { saved?: boolean; error?: string };

export async function upsertGoldenAction(
  _prev: SaveState,
  formData: FormData,
): Promise<SaveState> {
  const name = String(formData.get("name") ?? "").trim();
  const jdText = String(formData.get("jd_text") ?? "").trim();
  const expectedGate = String(formData.get("expected_gate") ?? "").trim();
  if (!name) return { error: "Give it a short name." };
  if (jdText.length < 40) return { error: "Paste the posting text." };
  if (expectedGate !== "above" && expectedGate !== "below") {
    return { error: "Say which side of the gate it belongs on." };
  }
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/UpsertGoldenPosting",
    body: {
      name,
      jdText,
      roleHint: String(formData.get("role_hint") ?? "").trim(),
      employerHint: String(formData.get("employer_hint") ?? "").trim(),
      expectedGate,
      expectedBand: String(formData.get("expected_band") ?? "").trim(),
      note: String(formData.get("note") ?? "").trim(),
    },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  revalidatePath("/admin/evals");
  return { saved: true };
}

export async function setGoldenActiveAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "").trim();
  const active = String(formData.get("active") ?? "") === "true";
  if (!id) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/SetGoldenActive",
    body: { id, active },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/evals");
}

async function errorText(resp: Response): Promise<string> {
  try {
    const j = (await resp.json()) as { message?: string };
    if (j.message) return j.message;
  } catch {
    /* fall through */
  }
  return `HTTP ${resp.status}`;
}
