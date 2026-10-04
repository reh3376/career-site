import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export const dynamic = "force-dynamic";

// Streams the decision log as a JSONL download. The RPC itself requires
// the admin session, so an anonymous or non-admin caller gets the api's
// status back rather than any data.
export async function GET(request: Request): Promise<Response> {
  const cookie = await getSessionCookie();
  if (!cookie) return new Response("Not signed in", { status: 401 });
  const url = new URL(request.url);
  const reviewedOnly = url.searchParams.get("reviewed") === "1";
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ExportDecisionLog",
    body: { reviewedOnly },
    cookie,
  });
  if (!resp.ok) return new Response(`Export failed: HTTP ${resp.status}`, { status: resp.status });
  const data = (await resp.json()) as { jsonl?: string; rowCount?: number; row_count?: number };
  const stamp = new Date().toISOString().slice(0, 10);
  const name = `decision-log-${reviewedOnly ? "reviewed" : "all"}-${stamp}.jsonl`;
  return new Response(data.jsonl ?? "", {
    status: 200,
    headers: {
      "Content-Type": "application/x-ndjson; charset=utf-8",
      "Content-Disposition": `attachment; filename="${name}"`,
      "Cache-Control": "no-store",
    },
  });
}
