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

// ---------------------------------------------------------------
// Saved queries (per-admin, DB-backed)
// ---------------------------------------------------------------

export type SavedQuery = {
  id: string;
  name: string;
  sql: string;
  created_at?: string;
  updated_at?: string;
};

export type ListSavedResult =
  | { ok: true; queries: SavedQuery[] }
  | { ok: false; error: string };

export async function listSavedQueriesAction(): Promise<ListSavedResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListSavedQueries",
    body: {},
    cookie,
  });
  if (!resp.ok) return { ok: false, error: `HTTP ${resp.status}` };
  const j = (await resp.json()) as { queries?: SavedQuery[] };
  return { ok: true, queries: j.queries ?? [] };
}

export type SaveResult =
  | { ok: true; query: SavedQuery; created: boolean }
  | { ok: false; error: string };

export async function upsertSavedQueryAction(
  name: string,
  sql: string,
): Promise<SaveResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/UpsertSavedQuery",
    body: { name, sql },
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
  const j = (await resp.json()) as { query?: SavedQuery; created?: boolean };
  return {
    ok: true,
    query: j.query ?? { id: "", name, sql },
    created: Boolean(j.created),
  };
}

export async function deleteSavedQueryAction(id: string): Promise<void> {
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/DeleteSavedQuery",
    body: { id },
    cookie,
  }).catch(() => undefined);
}
