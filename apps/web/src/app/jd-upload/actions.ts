"use server";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export type SubmitState = {
  ok?: boolean;
  submission_id?: string;
  result_token?: string;
  message?: string;
  error?: string;
  values?: {
    jdText?: string;
    roleHint?: string;
    employerHint?: string;
    contactEmail?: string;
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

  const values = { jdText, roleHint, employerHint, contactEmail };

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
  };
  return {
    ok: true,
    submission_id: j.submissionId ?? j.submission_id ?? "",
    result_token: j.resultToken ?? j.result_token ?? "",
    message: j.message,
    values,
  };
}
