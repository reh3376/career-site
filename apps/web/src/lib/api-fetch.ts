import "server-only";

// Low-level fetch to the Go API that (optionally) forwards the caller's
// session cookie and returns the raw Response so callers can inspect
// Set-Cookie headers for relay. Used by handlers that need cookie plumbing
// on top of the vanilla @connectrpc/connect client in src/lib/api.ts.

const apiBase =
  process.env.API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type ApiCall = {
  path: string; // e.g. "/api/career.v1.AuthService/Login"
  body?: unknown;
  cookie?: string | null;
};

export async function callApi(req: ApiCall): Promise<Response> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "Connect-Protocol-Version": "1",
  };
  if (req.cookie) headers.Cookie = `career_site_session=${req.cookie}`;

  return fetch(`${apiBase}${req.path}`, {
    method: "POST",
    headers,
    body: req.body === undefined ? undefined : JSON.stringify(req.body),
    cache: "no-store",
  });
}
