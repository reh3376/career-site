"use server";

import { callApi } from "@/lib/api-fetch";

export type ForgotState = {
  submitted?: boolean;
  message?: string;
  error?: string;
  values?: { email?: string };
};

// Server action for the /forgot-password form. The backend deliberately
// returns the same fixed message whether the email matched an account or
// not, so the UI can safely render that message without leaking existence.
export async function forgotPasswordAction(
  _prev: ForgotState,
  formData: FormData,
): Promise<ForgotState> {
  const email = String(formData.get("email") ?? "").trim();
  if (!email) return { error: "Enter your email address.", values: { email } };

  const resp = await callApi({
    path: "/api/career.v1.AuthService/ForgotPassword",
    body: { email },
  });

  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep default */
    }
    return { error: msg, values: { email } };
  }

  const j = (await resp.json()) as { message?: string };
  return {
    submitted: true,
    message:
      j.message ??
      "If an account exists for that email, we've sent a reset link.",
    values: { email },
  };
}
