"use server";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// What is left of the member's daily allowance. Mirrors career.v1.JdQuota.
export type Quota = {
  limit: number;
  used: number;
  remaining: number;
  windowHours: number;
  nextSlotAt?: string;
};

export type SubmitState = {
  ok?: boolean;
  submission_id?: string;
  result_token?: string;
  message?: string;
  error?: string;
  quota?: Quota;
  values?: {
    jdText?: string;
    roleHint?: string;
    employerHint?: string;
    contactEmail?: string;
    applyUrl?: string;
  };
};

// Server action for the /jd-upload textarea. Members only: forwards
// the session cookie to career.v1.JdService.SubmitJd, which rejects
// anonymous calls; rate-limited server-side (5 per 15 min per member+JD).
export async function submitJdAction(
  _prev: SubmitState,
  formData: FormData,
): Promise<SubmitState> {
  const cookie = await getSessionCookie();
  if (!cookie) {
    return { error: "Sign in to submit a job description." };
  }
  const jdText = String(formData.get("jd_text") ?? "").trim();
  const roleHint = String(formData.get("role_hint") ?? "").trim();
  const employerHint = String(formData.get("employer_hint") ?? "").trim();
  const contactEmail = String(formData.get("contact_email") ?? "")
    .trim()
    .toLowerCase();

  const applyUrl = String(formData.get("apply_url") ?? "").trim();

  const values = { jdText, roleHint, employerHint, contactEmail, applyUrl };

  if (applyUrl && !/^https?:\/\/\S+$/i.test(applyUrl)) {
    return {
      error: "The application link needs to be a full URL starting with http:// or https://.",
      values,
    };
  }

  if (jdText.length < 100) {
    return {
      error: "Paste the full JD, we need at least 100 characters to work with.",
      values,
    };
  }

  const resp = await callApi({
    path: "/api/career.v1.JdService/SubmitJd",
    body: {
      jdText,
      source: "JD_SOURCE_PASTE",
      roleHint,
      employerHint,
      contactEmail,
      applyUrl,
    },
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
    return { error: msg, values };
  }

  const j = (await resp.json()) as {
    submissionId?: string;
    submission_id?: string;
    resultToken?: string;
    result_token?: string;
    message?: string;
    quota?: RawQuota;
  };
  return {
    ok: true,
    submission_id: j.submissionId ?? j.submission_id ?? "",
    result_token: j.resultToken ?? j.result_token ?? "",
    message: j.message,
    quota: readQuota(j.quota),
    values,
  };
}

type RawQuota = {
  limit?: number;
  used?: number;
  remaining?: number;
  windowHours?: number;
  window_hours?: number;
  nextSlotAt?: string;
  next_slot_at?: string;
};

// The wire shape uses proto3 JSON, which omits zero values, so a member
// with nothing left sends no `remaining` field at all. Defaulting those
// to 0 is what makes "none left" readable rather than missing.
function readQuota(raw: RawQuota | undefined): Quota | undefined {
  if (!raw || !raw.limit) return undefined;
  return {
    limit: raw.limit,
    used: raw.used ?? 0,
    remaining: raw.remaining ?? 0,
    windowHours: raw.windowHours ?? raw.window_hours ?? 24,
    nextSlotAt: raw.nextSlotAt ?? raw.next_slot_at,
  };
}

// The member's allowance as it stands, for the page to state before
// anyone spends it. Returns undefined when no limit applies, which is
// the admin and the case where the limit is switched off.
export async function getJdQuota(): Promise<Quota | undefined> {
  const cookie = await getSessionCookie();
  if (!cookie) return undefined;
  const resp = await callApi({
    path: "/api/career.v1.JdService/GetJdReviewConfig",
    body: {},
    cookie,
  });
  if (!resp.ok) return undefined;
  try {
    const j = (await resp.json()) as { quota?: RawQuota };
    return readQuota(j.quota);
  } catch {
    return undefined;
  }
}
