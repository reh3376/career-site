"use server";

import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { withPlusPilot } from "@/lib/plus-pilot";
import { relaySetCookies } from "@/lib/session";
import { TURNSTILE_FIELD } from "@/lib/turnstile";

export type LoginState = {
  error?: string;
  values?: { email?: string };
};

export async function loginAction(
  _prev: LoginState,
  formData: FormData,
): Promise<LoginState> {
  let email = String(formData.get("email") ?? "").trim();
  const password = String(formData.get("password") ?? "");
  // The optional pilot switch adds "+pilot" to the address for someone
  // whose pilot account is their regular address plus that suffix.
  if (formData.get("plusPilot") === "on") email = withPlusPilot(email);
  // Written into the form by the Turnstile widget (FR-AUTH-09).
  const turnstileToken = String(formData.get(TURNSTILE_FIELD) ?? "");
  const values = { email };

  if (!email || !password) {
    return { error: "Email and password are required.", values };
  }

  const resp = await callApi({
    path: "/api/career.v1.AuthService/Login",
    body: { email, password, turnstileToken },
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
  // A pilot participant (the flag without the admin role) lands in the
  // pilot unless the link that brought them here said otherwise (FSD
  // FR-PLT-18).
  let pilotOnly = false;
  try {
    const j = (await resp.json()) as { me?: { pilotApproved?: boolean; role?: string } };
    pilotOnly = j.me?.pilotApproved === true && j.me?.role !== "MEMBER_ROLE_ADMIN";
  } catch {
    /* no body, no change */
  }
  // Honor ?next= when it's a same-origin path, the login page passes
  // the query through as a hidden form field. Anything that isn't a
  // plain "/foo" path is ignored so this can't be turned into an open
  // redirect.
  const nextRaw = String(formData.get("next") ?? "");
  const next = nextRaw ? safeNext(nextRaw) : pilotOnly ? "/pilot" : "/home";
  redirect(next);
}

function safeNext(v: string): string {
  if (!v.startsWith("/") || v.startsWith("//")) return "/home";
  return v;
}
