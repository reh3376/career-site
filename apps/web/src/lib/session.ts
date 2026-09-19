import "server-only";

import { cookies } from "next/headers";

export const SESSION_COOKIE = "career_site_session";

type CookieOpts = {
  path?: string;
  domain?: string;
  maxAge?: number;
  expires?: number;
  httpOnly?: boolean;
  secure?: boolean;
  sameSite?: "lax" | "strict" | "none";
};

// relayResponseCookies extracts Set-Cookie headers from an API response
// and mirrors them onto the Next.js response so the browser sees them.
// Server Actions do not automatically forward Set-Cookie from downstream
// fetches, so we do it explicitly here.
export async function relaySetCookies(headers: Headers): Promise<void> {
  const raw = headers.getSetCookie?.() ?? [];
  if (raw.length === 0) return;
  const jar = await cookies();
  for (const line of raw) {
    const parsed = parseSetCookie(line);
    if (!parsed) continue;
    jar.set(parsed.name, parsed.value, parsed.opts);
  }
}

// Node's fetch surfaces Set-Cookie as one string per cookie via
// getSetCookie(). Each looks like: "name=value; Path=/; HttpOnly; ...".
function parseSetCookie(
  line: string,
): { name: string; value: string; opts: CookieOpts } | null {
  const parts = line.split(";").map((p) => p.trim());
  const [nameValue, ...attrs] = parts;
  const eq = nameValue.indexOf("=");
  if (eq < 0) return null;
  const name = nameValue.slice(0, eq);
  const value = nameValue.slice(eq + 1);
  const opts: CookieOpts = {};
  for (const a of attrs) {
    const [k, v] = a.includes("=") ? a.split("=", 2) : [a, ""];
    switch (k.toLowerCase()) {
      case "path":
        opts.path = v;
        break;
      case "domain":
        opts.domain = v;
        break;
      case "max-age":
        opts.maxAge = Number(v);
        break;
      case "expires":
        opts.expires = new Date(v).getTime();
        break;
      case "httponly":
        opts.httpOnly = true;
        break;
      case "secure":
        opts.secure = true;
        break;
      case "samesite":
        opts.sameSite = v.toLowerCase() as "lax" | "strict" | "none";
        break;
    }
  }
  return { name, value, opts };
}

// getSessionCookie returns the request-scoped session cookie value or
// null. Server components use it to detect a signed-in visitor.
export async function getSessionCookie(): Promise<string | null> {
  const jar = await cookies();
  return jar.get(SESSION_COOKIE)?.value ?? null;
}

// clearSession clears the session cookie in this response.
export async function clearSession(): Promise<void> {
  const jar = await cookies();
  jar.delete(SESSION_COOKIE);
}
