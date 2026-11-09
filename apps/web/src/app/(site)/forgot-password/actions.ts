"use server";

import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";

export type ForgotState = {
  error?: string;
  values?: { email?: string };
};

// Server action for the /forgot-password form. The backend returns the
// same fixed message whether the email matched an account or not, so
// the next screen can show it without leaking existence. On success the
// member is sent straight to the reset screen with the address filled
// in, because the code they are about to receive is entered there
// (FSD FR-AUTH-19).
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

  redirect(`/reset-password?email=${encodeURIComponent(email)}&sent=1`);
}
