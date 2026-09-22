import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import { SESSION_COOKIE } from "@/lib/session";

// Access policy for anonymous visitors: the landing page (base info
// plus request access), the contact form, the legal pages, and the
// flows needed to become a member (register, verify, sign in, password
// reset, the one-click approval link). Everything else needs a session.
//
// This is an optimistic check on cookie presence so a signed-out
// visitor is sent to /login instead of seeing an empty page; the API
// enforces the real authorization on every data call, and server
// components re-validate the session where it matters.
const PUBLIC_PATHS = new Set([
  "/",
  "/contact",
  "/register",
  "/register/check-email",
  "/login",
  "/verify",
  "/forgot-password",
  "/reset-password",
  "/privacy",
  "/terms",
  "/admin/decision",
]);

function isPublic(pathname: string): boolean {
  if (PUBLIC_PATHS.has(pathname)) return true;
  // Token-bearing flows carry the token as a path segment on some routes.
  return (
    pathname.startsWith("/verify/") ||
    pathname.startsWith("/reset-password/") ||
    pathname.startsWith("/admin/decision/")
  );
}

// First-party anonymous id for the product event stream
// (docs/events/README.md). Set on the first page load, HttpOnly so no
// script reads it, 13 months like a consent-free analytics cookie. The
// api reads it from the Cookie header on beacon calls; it never carries
// identity by itself and is blanked from old rows by a nightly job.
export const ANON_COOKIE = "career_anon";
const ANON_MAX_AGE = 60 * 60 * 24 * 400;

function withAnonId(request: NextRequest, res: NextResponse): NextResponse {
  if (request.cookies.get(ANON_COOKIE)?.value) return res;
  res.cookies.set(ANON_COOKIE, crypto.randomUUID(), {
    path: "/",
    maxAge: ANON_MAX_AGE,
    httpOnly: true,
    sameSite: "lax",
    secure: request.nextUrl.protocol === "https:",
  });
  return res;
}

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  if (isPublic(pathname)) return withAnonId(request, NextResponse.next());
  if (request.cookies.get(SESSION_COOKIE)?.value) {
    // Member pages are never indexed, whatever a crawler holds.
    const res = NextResponse.next();
    res.headers.set("X-Robots-Tag", "noindex, nofollow");
    return withAnonId(request, res);
  }

  const login = new URL("/login", request.url);
  login.searchParams.set("next", pathname + request.nextUrl.search);
  return NextResponse.redirect(login);
}

export const config = {
  // Everything except the API proxy path, Next internals, and static
  // files (anything with an extension: favicon, images, fonts, robots).
  matcher: ["/((?!api/|_next/|.*\\..*).*)"],
};
