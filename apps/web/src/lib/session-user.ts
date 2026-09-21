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
