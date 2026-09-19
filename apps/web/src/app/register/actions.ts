"use server";

import { create } from "@bufbuild/protobuf";
import { ConnectError } from "@connectrpc/connect";
import { redirect } from "next/navigation";

import { RegisterRequestSchema } from "@/gen/career/v1/auth_pb";
import { authClient } from "@/lib/api";

// Fields the form collects. `state` is what useActionState works with.
export type RegisterState = {
  error?: string;
  values?: {
    name?: string;
    email?: string;
    organization?: string;
    statedRole?: string;
  };
};

export async function registerAction(
  _prev: RegisterState,
  formData: FormData,
): Promise<RegisterState> {
  const name = String(formData.get("name") ?? "").trim();
  const email = String(formData.get("email") ?? "").trim();
  const password = String(formData.get("password") ?? "");
  const organization = String(formData.get("organization") ?? "").trim();
  const statedRole = String(formData.get("statedRole") ?? "").trim();
  const consentAccepted = formData.get("consent") === "on";

  const values = { name, email, organization, statedRole };

  if (!name || !email || !password) {
    return { error: "Name, email, and password are required.", values };
  }
  if (password.length < 12) {
    return { error: "Password must be at least 12 characters.", values };
  }
  if (!consentAccepted) {
    return {
      error:
        "You need to accept that your activity and Ask Roger conversations are stored and visible to Roger.",
      values,
    };
  }

  try {
    await authClient.register(
      create(RegisterRequestSchema, {
        name,
        email,
        password,
        organization,
        statedRole,
        consentVersion: "v1",
        consentAccepted: true,
      }),
    );
  } catch (err) {
    const message =
      err instanceof ConnectError
        ? err.rawMessage
        : err instanceof Error
          ? err.message
          : "Registration failed. Try again.";
    return { error: message, values };
  }

  // Same landing for happy path and duplicate email — the API returns the
  // generic "Check your email" response either way, so nothing to leak.
  redirect(`/register/check-email?email=${encodeURIComponent(email)}`);
}
