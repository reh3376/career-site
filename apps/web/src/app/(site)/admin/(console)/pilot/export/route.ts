import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export const dynamic = "force-dynamic";

const TABLES = new Set(["sessions", "answers", "comments"]);

// Downloads one pilot table as CSV (FSD FR-PLT-11). The RPC requires
// the admin session and is audited, so an anonymous or non-admin
// caller gets the api's status back rather than any data. Participants
// appear as P-NN codes, never by name or address (FR-PLT-12); the file
// is for the private repository's pilot folder.
export async function GET(request: Request): Promise<Response> {
  const cookie = await getSessionCookie();
  if (!cookie) return new Response("Not signed in", { status: 401 });
  const url = new URL(request.url);
  const table = url.searchParams.get("table") ?? "";
  if (!TABLES.has(table)) return new Response("table must be sessions, answers or comments", { status: 400 });
  const resp = await callApi({
    path: "/api/career.v1.PilotService/ExportPilot",
    body: { table },
    cookie,
  });
  if (!resp.ok) return new Response(`Export failed: HTTP ${resp.status}`, { status: resp.status });
  const data = (await resp.json()) as { csv?: string; rowCount?: number };
  const stamp = new Date().toISOString().slice(0, 10);
  return new Response(data.csv ?? "", {
    status: 200,
    headers: {
      "Content-Type": "text/csv; charset=utf-8",
      "Content-Disposition": `attachment; filename="pilot-${table}-${stamp}.csv"`,
      "Cache-Control": "no-store",
      "X-Row-Count": String(data.rowCount ?? 0),
    },
  });
}
