"use server";

import { cookies } from "next/headers";
import { revalidatePath } from "next/cache";

import { UI_MODE_COOKIE, type UiMode } from "@/lib/ui-mode";

// Persist the selected mode for a year and revalidate the current
// path so the very next paint (this same request's response cache
// invalidation + the client's follow-up navigation) picks up the new
// data-mode attribute on <html>. Called from the segmented toggle in
// /settings and from the hamburger drawer.
export async function setUiModeAction(formData: FormData): Promise<void> {
  const raw = String(formData.get("mode") ?? "");
  const mode: UiMode = raw === "ot" ? "ot" : "it";
  const jar = await cookies();
  jar.set(UI_MODE_COOKIE, mode, {
    path: "/",
    sameSite: "lax",
    // Not HttpOnly on purpose, the client toggle applies the change
    // optimistically to the DOM before the server round-trip so the
    // switch feels immediate, and reading the cookie is what lets it.
    httpOnly: false,
    secure: process.env.NODE_ENV === "production",
    maxAge: 60 * 60 * 24 * 365,
  });
  revalidatePath("/", "layout");
}
