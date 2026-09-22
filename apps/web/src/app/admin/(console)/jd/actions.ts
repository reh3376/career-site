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
  bands?: { veryStrong: number; strong: number; possible: number; weak: number };
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
  if (!(bands.weak > 0 && bands.weak < bands.possible && bands.possible < bands.strong && bands.strong < bands.veryStrong && bands.veryStrong <= 1)) {
    return { error: "Order must hold: weak < possible < strong < very strong, within 0 and 1.", bands };
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

