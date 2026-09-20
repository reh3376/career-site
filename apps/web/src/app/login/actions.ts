"use server";

import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { relaySetCookies } from "@/lib/session";

export type LoginState = {
  error?: string;
  values?: { email?: string };
};

export async function loginAction(
  _prev: LoginState,
  formData: FormData,
): Promise<LoginState> {
  const email = String(formData.get("email") ?? "").trim();
  const password = String(formData.get("password") ?? "");
  const values = { email };

  if (!email || !password) {
    return { error: "Email and password are required.", values };
  }

  const resp = await callApi({
    path: "/api/career.v1.AuthService/Login",
    body: { email, password },
  });

  if (!resp.ok) {
    // Connect returns { code, message } on error.
    let message = "Sign-in failed.";
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) message = j.message;
    } catch {
      // ignore; message stays generic
    }
    return { error: message, values };
  }

  // Success: relay the API's Set-Cookie so the browser session sticks.
  await relaySetCookies(resp.headers);
  // Honor ?next= when it's a same-origin path — the login page passes
  // the query through as a hidden form field. Anything that isn't a
  // plain "/foo" path is ignored so this can't be turned into an open
  // redirect.
  const nextRaw = String(formData.get("next") ?? "");
  const next = safeNext(nextRaw);
  redirect(next);
}

function safeNext(v: string): string {
  if (!v.startsWith("/") || v.startsWith("//")) return "/home";
  return v;
}
