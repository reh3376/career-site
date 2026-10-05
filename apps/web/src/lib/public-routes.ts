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
//
// /how-ask-roger-works became public on 2026-09-24 under that same
// policy rather than as an exception to it. It is the page that says
// which model reads a posting, that the model was never trained on the
// career it is judging, and how far the reviewer currently disagrees
// with its owner. Asking someone to register before they can read any
// of that is asking them to decide without the evidence. The thing
// access buys, /jd-upload, stays behind the wall: it spends real
// compute and carries the daily quota.

// Public pages, exact matches.
export const PUBLIC_PATHS = [
  "/",
  "/articles",
  "/gallery",
  "/contact",
  "/how-ask-roger-works",
  "/register",
  "/register/check-email",
  "/login",
  "/verify",
  "/forgot-password",
  "/reset-password",
  "/privacy",
  "/terms",
  // The decision test. Public because its participants are recruited
  // volunteers rather than members, and because a member and an
  // anonymous visitor must be in the same condition. Reachable by URL
  // and deliberately not linked from anywhere until the owner has taken
  // it end to end on a deployed build (roadmap M5b): a discoverable
  // link before then invites a stranger into a fifteen-minute test that
  // has not been validated, and each volunteer can only be asked once.
  // Both pages are noindex while collection is open.
  "/decision-test",
  "/decision-test/run",
  "/decision-test/thanks",
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

// Member pages that actually exist.
//
// The gate used to be "public, or else sign in", which meant any path
// at all redirected to the login page. A stale or mistyped link, say
// /timeline or /projects or /downloads, none of which have ever
// existed, sent a visitor to sign in, took their password, and then
// showed them a 404. The sign-in was real work demanded for a page
// that was never there.
//
// So the redirect is now limited to routes that exist. Anything else
// falls through and Next renders not-found, signed in or not.
//
// This list is the member half of the route table; adding a gated page
// means adding it here, and forgetting to shows up immediately as a
// 404 rather than as a silent leak, which is the safe direction for a
// mistake to fall.
export const MEMBER_PATHS = [
  "/home",
  "/ask",
  "/jd-upload",
  "/meetings",
  "/settings",
  "/version",
] as const;

// Gated prefixes: the admin console and the id-bearing member pages.
const MEMBER_PREFIXES = ["/admin", "/jd-upload/"] as const;

export function isMemberPath(pathname: string): boolean {
  // Public wins. The one-click Accept/Decline link lives under
  // /admin/decision and is opened from an email with no session, so it
  // is public while everything else under /admin is not. The proxy
  // happens to test public first, but relying on call order would make
  // this a bug waiting for someone to reorder two lines.
  if (isPublicPath(pathname)) return false;
  if ((MEMBER_PATHS as readonly string[]).includes(pathname)) return true;
  return MEMBER_PREFIXES.some((p) => pathname.startsWith(p));
}

// Pages worth a crawler's time. The auth flows are public but pointless
// to index, so they are omitted here even though isPublicPath allows
// them.
export const INDEXABLE_PATHS = [
  "/",
  "/articles",
  "/gallery",
  "/contact",
  "/how-ask-roger-works",
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
  "/api/",
] as const;
