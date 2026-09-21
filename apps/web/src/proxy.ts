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

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  if (isPublic(pathname)) return NextResponse.next();
  if (request.cookies.get(SESSION_COOKIE)?.value) return NextResponse.next();

  const login = new URL("/login", request.url);
  login.searchParams.set("next", pathname + request.nextUrl.search);
  return NextResponse.redirect(login);
}

export const config = {
  // Everything except the API proxy path, Next internals, and static
  // files (anything with an extension: favicon, images, fonts, robots).
  matcher: ["/((?!api/|_next/|.*\\..*).*)"],
};
