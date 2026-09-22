// The one list of what an anonymous visitor may see.
//
// Three places have to agree about this: the proxy, which redirects
// everything else to sign-in; robots.txt, which tells crawlers what to
// index; and the sitemap, which invites them. They used to hold three
// separate copies, so a new public page needed three edits and got two.
//
// Kept free of node imports because the proxy runs as middleware.
//
// The policy, decided 2026-09-22: evidence a hiring manager needs in
// order to decide whether to ask for access is public. Anything that
// costs compute, reveals a member, or is the thing access buys, is not.

// Public pages, exact matches.
export const PUBLIC_PATHS = [
  "/",
  "/articles",
  "/gallery",
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
] as const;

// Public prefixes. Article slugs are public; the token-bearing flows
// carry their token as a path segment.
const PUBLIC_PREFIXES = [
  "/articles/",
  "/verify/",
  "/reset-password/",
  "/admin/decision/",
] as const;

export function isPublicPath(pathname: string): boolean {
  if ((PUBLIC_PATHS as readonly string[]).includes(pathname)) return true;
  return PUBLIC_PREFIXES.some((p) => pathname.startsWith(p));
}

// Pages worth a crawler's time. The auth flows are public but pointless
// to index, so they are omitted here even though isPublicPath allows
// them.
export const INDEXABLE_PATHS = [
  "/",
  "/articles",
  "/gallery",
  "/contact",
  "/register",
  "/login",
  "/privacy",
  "/terms",
] as const;

// Everything a crawler should stay out of, member surfaces and the API.
export const CRAWLER_DISALLOW = [
  "/home",
  "/jd-upload",
  "/settings",
  "/admin",
  "/how-ask-roger-works",
  "/api/",
] as const;
