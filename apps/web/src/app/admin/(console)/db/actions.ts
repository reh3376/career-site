"use server";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export type DbRow = { cells: string[]; null_mask?: string };
export type QueryResult =
  | {
      ok: true;
      columns: string[];
      column_types: string[];
      rows: DbRow[];
      truncated: boolean;
      row_count: number;
      elapsed_ms: number;
    }
  | { ok: false; error: string };

// Server action for the /admin/db query console. Forwards the admin
// session cookie and returns a JSON-friendly shape the client can
// render into a table. Errors from the safety-rail guard (SELECT-only,
// forbidden token, empty, etc.) come back with code=invalid_argument
// and are surfaced as `ok: false` for the UI.
export async function runDbQueryAction(sql: string): Promise<QueryResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in" };

  const resp = await callApi({
    path: "/api/career.v1.AdminService/RunDbQuery",
    body: { sql, timeoutMs: 5000 },
    cookie,
  });

  if (!resp.ok) {
    let error = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) error = j.message;
    } catch {
      /* keep default */
    }
    return { ok: false, error };
  }

  const j = (await resp.json()) as {
    columns?: string[];
    columnTypes?: string[];
    rows?: DbRow[];
    truncated?: boolean;
    rowCount?: number;
    elapsedMs?: number;
    // ConnectRPC's JSON encoding can flip between camelCase and
    // snake_case depending on version. Accept both.
    column_types?: string[];
    row_count?: number;
    elapsed_ms?: number;
  };
  return {
    ok: true,
    columns: j.columns ?? [],
    column_types: j.columnTypes ?? j.column_types ?? [],
    rows: j.rows ?? [],
    truncated: Boolean(j.truncated),
    row_count: j.rowCount ?? j.row_count ?? (j.rows?.length ?? 0),
    elapsed_ms: j.elapsedMs ?? j.elapsed_ms ?? 0,
  };
}
