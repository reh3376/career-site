import "server-only";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Small shape mirrored from the proto Me. The API returns the enum as
// its stringified name (e.g. "MEMBER_ROLE_ADMIN"), which the callers
// downstream compare against isAdmin() below rather than parsing here.
export type SessionUser = {
  id: string;
  name: string;
  email: string;
  status: string;
  role: string;
  // The pilot flag (FSD FR-PLT-01). Absent from the JSON when false.
  pilotApproved?: boolean;
};

// Resolve the calling browser's session to a SessionUser, or null when
// there is no session or the API rejects it. Server-only, do not call
// from client components. Cheap enough to invoke from every layout that
// needs role-aware navigation (the whole request already fans out from
// the RSC render).
export async function getSessionUser(): Promise<SessionUser | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.MemberService/GetMe",
    body: {},
    cookie,
  });
  if (!resp.ok) return null;
  const j = (await resp.json()) as { me?: SessionUser };
  return j.me ?? null;
}

export function isAdmin(u: SessionUser | null): boolean {
  return u?.role === "MEMBER_ROLE_ADMIN";
}

// Whether the pilot area is open to this member: the flag, or the
// admin role, which passes the gate whatever the flag says (owner,
// 2026-11-08). The menu and the home card read this; the pages and
// every PilotService RPC enforce the same rule server-side.
export function canPilot(u: SessionUser | null): boolean {
  return !!u && (u.pilotApproved === true || isAdmin(u));
}

// A pilot participant and nothing else (FSD FR-PLT-18): the flag
// without the admin role. Owner, 2026-11-09: participants do not need
// the rest of the site; they land in the pilot, see only the pilot,
// and are sent back to it from any member page. The owner, as admin,
// keeps the whole site.
export function isPilotOnly(u: SessionUser | null): boolean {
  return !!u && u.pilotApproved === true && !isAdmin(u);
}
