"use client";

import {
  useCallback,
  useEffect,
  useState,
  useSyncExternalStore,
  useTransition,
} from "react";

import {
  deleteSavedQueryAction,
  listSavedQueriesAction,
  runDbQueryAction,
  type SavedQuery,
  upsertSavedQueryAction,
  type QueryResult,
} from "./actions";

// localStorage key + cap for the history list. Per-browser, per-admin
// only implicitly (the whole /admin/db surface is admin-gated).
const HISTORY_KEY = "admin.db.history.v1";
const HISTORY_MAX = 10;

type HistoryEntry = { sql: string; ts: number };

function loadHistory(): HistoryEntry[] {
  try {
    const raw = window.localStorage.getItem(HISTORY_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter(
        (x): x is HistoryEntry =>
          typeof x === "object" &&
          x !== null &&
          typeof (x as HistoryEntry).sql === "string" &&
          typeof (x as HistoryEntry).ts === "number",
      )
      .slice(0, HISTORY_MAX);
  } catch {
    return [];
  }
}

function pushHistory(sql: string): HistoryEntry[] {
  const trimmed = sql.trim();
  if (!trimmed) return loadHistory();
  const prev = loadHistory().filter((h) => h.sql !== trimmed);
  const next = [{ sql: trimmed, ts: Date.now() }, ...prev].slice(0, HISTORY_MAX);
  try {
    window.localStorage.setItem(HISTORY_KEY, JSON.stringify(next));
    // Notify same-tab listeners; the native `storage` event fires
    // only on *other* tabs.
    window.dispatchEvent(new Event("admin.db.history"));
  } catch {
    /* quota exceeded / private tab — ignore */
  }
  return next;
}

const EMPTY_HISTORY: HistoryEntry[] = [];

// useSyncExternalStore-friendly view of the history list. The
// snapshot is cached in a module-local ref so React sees a stable
// reference across renders when nothing changed — otherwise
// useSyncExternalStore would tear on every re-render.
let cachedHistory: HistoryEntry[] = EMPTY_HISTORY;
let cachedHistoryRaw = "";

function historySubscribe(cb: () => void): () => void {
  const handler = () => cb();
  window.addEventListener("storage", handler);
  window.addEventListener("admin.db.history", handler);
  return () => {
    window.removeEventListener("storage", handler);
    window.removeEventListener("admin.db.history", handler);
  };
}

function historyGetSnapshot(): HistoryEntry[] {
  const raw = window.localStorage.getItem(HISTORY_KEY) ?? "";
  if (raw === cachedHistoryRaw) return cachedHistory;
  cachedHistoryRaw = raw;
  cachedHistory = loadHistory();
  return cachedHistory;
}

function historyGetServerSnapshot(): HistoryEntry[] {
  return EMPTY_HISTORY;
}

// Client-side query editor + results table for /admin/db. Uses
// useTransition so the button reflects pending state and the page
// doesn't block while the RPC is in flight.
export function QueryConsole({ tables }: { tables: string[] }) {
  const [sql, setSql] = useState<string>(defaultQuery(tables));
  const [result, setResult] = useState<QueryResult | null>(null);
  const [pending, startTransition] = useTransition();
  const history = useSyncExternalStore(
    historySubscribe,
    historyGetSnapshot,
    historyGetServerSnapshot,
  );
  const [saved, setSaved] = useState<SavedQuery[]>([]);
  const [flash, setFlash] = useState<string | null>(null);

  // Load saved queries from the server on mount. Not derivable from
  // props, and the RPC needs the session cookie — so an effect is
  // the right shape here.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const res = await listSavedQueriesAction();
      if (!cancelled && res.ok) setSaved(res.queries);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const run = useCallback(
    (text: string) => {
      const t = text.trim();
      if (!t) return;
      setSql(text);
      startTransition(async () => {
        const r = await runDbQueryAction(t);
        setResult(r);
        pushHistory(t); // fires admin.db.history → useSyncExternalStore reruns
      });
    },
    [startTransition],
  );

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!sql.trim() || pending) return;
    run(sql);
  };

  const onSave = async () => {
    const name = window.prompt(
      "Save this query as… (an existing name overwrites)",
      "",
    );
    if (!name || !name.trim()) return;
    const res = await upsertSavedQueryAction(name.trim(), sql);
    if (!res.ok) {
      setFlash(`Save failed: ${res.error}`);
      return;
    }
    const list = await listSavedQueriesAction();
    if (list.ok) setSaved(list.queries);
    setFlash(`${res.created ? "Saved" : "Updated"} "${res.query.name}".`);
  };

  const onLoadSaved = (id: string) => {
    if (!id) return;
    const q = saved.find((s) => s.id === id);
    if (q) setSql(q.sql);
  };

  const onDeleteSaved = async () => {
    if (saved.length === 0) return;
    const name = window.prompt(
      `Delete which saved query? Type the name exactly.\n\nAvailable:\n${saved
        .map((s) => `  ${s.name}`)
        .join("\n")}`,
      "",
    );
    if (!name) return;
    const target = saved.find((s) => s.name === name.trim());
    if (!target) {
      setFlash(`No saved query named "${name}".`);
      return;
    }
    await deleteSavedQueryAction(target.id);
    const list = await listSavedQueriesAction();
    if (list.ok) setSaved(list.queries);
    setFlash(`Deleted "${target.name}".`);
  };

  const onExportCsv = () => {
    if (!result || !result.ok || result.rows.length === 0) return;
    downloadCsv(result);
  };

  return (
    <section aria-label="Query console">
      <form onSubmit={submit} className="flex flex-col gap-3">
        <label
          htmlFor="db-sql"
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
        >
          sql
        </label>
        <textarea
          id="db-sql"
          value={sql}
          onChange={(e) => setSql(e.target.value)}
          spellCheck={false}
          rows={8}
          className="block w-full border border-line-strong bg-paper-2 px-3 py-2 font-mono text-[13px] leading-snug text-ink outline-none focus:border-accent"
        />
        <div className="flex flex-wrap items-center gap-3">
          <button
            type="submit"
            disabled={pending || !sql.trim()}
            className="inline-flex items-center rounded-md bg-accent px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-50"
          >
            {pending ? "Running…" : "Run query"}
          </button>
          <PresetBar tables={tables} onPick={run} disabled={pending} />
        </div>

        <QueryToolbar
          history={history}
          saved={saved}
          onLoadHistory={(sqlText) => setSql(sqlText)}
          onLoadSaved={onLoadSaved}
          onSave={onSave}
          onDeleteSaved={onDeleteSaved}
          onExportCsv={onExportCsv}
          exportEnabled={
            result !== null && result.ok && result.rows.length > 0
          }
        />

        {flash ? (
          <p className="border-l-2 border-accent bg-accent/10 px-3 py-2 font-mono text-[11px] text-ink">
            {flash}
          </p>
        ) : null}
      </form>

      <div className="mt-8">
        <Result result={result} />
      </div>
    </section>
  );
}

function QueryToolbar({
  history,
  saved,
  onLoadHistory,
  onLoadSaved,
  onSave,
  onDeleteSaved,
  onExportCsv,
  exportEnabled,
}: {
  history: HistoryEntry[];
  saved: SavedQuery[];
  onLoadHistory: (sql: string) => void;
  onLoadSaved: (id: string) => void;
  onSave: () => void;
  onDeleteSaved: () => void;
  onExportCsv: () => void;
  exportEnabled: boolean;
}) {
  return (
    <div className="flex flex-wrap items-center gap-3 border-t border-line pt-3 text-xs">
      <label className="flex items-center gap-2">
        <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
          history
        </span>
        <select
          disabled={history.length === 0}
          defaultValue=""
          onChange={(e) => {
            const v = e.target.value;
            if (v) onLoadHistory(v);
            e.currentTarget.selectedIndex = 0;
          }}
          className="border border-line-strong bg-canvas px-2 py-1 font-mono text-xs text-ink outline-none focus:border-accent disabled:opacity-50"
        >
          <option value="">
            {history.length === 0 ? "(none yet)" : "load recent…"}
          </option>
          {history.map((h, i) => (
            <option key={i} value={h.sql}>
              {truncate(h.sql, 80)}
            </option>
          ))}
        </select>
      </label>

      <label className="flex items-center gap-2">
        <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
          saved
        </span>
        <select
          disabled={saved.length === 0}
          defaultValue=""
          onChange={(e) => {
            const v = e.target.value;
            if (v) onLoadSaved(v);
            e.currentTarget.selectedIndex = 0;
          }}
          className="border border-line-strong bg-canvas px-2 py-1 font-mono text-xs text-ink outline-none focus:border-accent disabled:opacity-50"
        >
          <option value="">
            {saved.length === 0 ? "(none saved)" : "load saved…"}
          </option>
          {saved.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>

      <button
        type="button"
        onClick={onSave}
        className="border border-line-strong px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 transition-colors hover:border-accent hover:text-accent"
      >
        Save current
      </button>
      <button
        type="button"
        onClick={onDeleteSaved}
        disabled={saved.length === 0}
        className="border border-line-strong px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 transition-colors hover:border-signal hover:text-signal disabled:cursor-not-allowed disabled:opacity-50"
      >
        Delete saved
      </button>
      <button
        type="button"
        onClick={onExportCsv}
        disabled={!exportEnabled}
        className="border border-line-strong px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 transition-colors hover:border-accent hover:text-accent disabled:cursor-not-allowed disabled:opacity-50"
      >
        Export CSV
      </button>
    </div>
  );
}

function PresetBar({
  tables,
  onPick,
  disabled,
}: {
  tables: string[];
  onPick: (sql: string) => void;
  disabled: boolean;
}) {
  const first = tables[0];
  const presets: { label: string; sql: string; enabled: boolean }[] = [
    {
      label: "tables",
      sql: "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY 1",
      enabled: true,
    },
    {
      label: `first 25 rows of ${first ?? "…"}`,
      sql: first ? `SELECT * FROM ${first} LIMIT 25` : "",
      enabled: !!first,
    },
    {
      label: "row counts per table",
      sql: "SELECT relname AS table, reltuples::bigint AS approx_rows FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname='public' AND c.relkind='r' ORDER BY approx_rows DESC",
      enabled: true,
    },
  ];
  return (
    <span className="flex flex-wrap gap-2 font-mono text-[11px] uppercase tracking-[0.14em]">
      {presets
        .filter((p) => p.enabled)
        .map((p) => (
          <button
            key={p.label}
            type="button"
            disabled={disabled}
            onClick={() => onPick(p.sql)}
            className="border border-line-strong px-3 py-1 text-ink-2 transition-colors hover:border-accent hover:text-accent disabled:cursor-not-allowed disabled:opacity-50"
          >
            {p.label}
          </button>
        ))}
    </span>
  );
}

function Result({ result }: { result: QueryResult | null }) {
  if (!result) return null;
  if (!result.ok) {
    return (
      <div className="border-l-2 border-signal bg-signal-soft/40 px-4 py-3 font-mono text-sm text-ink">
        {result.error}
      </div>
    );
  }
  const cols = result.columns;
  if (cols.length === 0) {
    return (
      <p className="font-mono text-sm text-ink-3">
        Query returned no columns.
      </p>
    );
  }
  return (
    <div>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        {result.row_count} row{result.row_count === 1 ? "" : "s"} · {result.elapsed_ms}ms
        {result.truncated ? " · truncated at 500" : ""}
      </p>
      <div className="mt-3 overflow-x-auto border border-line-strong">
        <table className="w-full min-w-full border-collapse font-mono text-[12px]">
          <thead>
            <tr className="border-b border-line-strong bg-paper-2">
              {cols.map((c, i) => (
                <th
                  key={c}
                  className="whitespace-nowrap px-3 py-2 text-left text-ink"
                >
                  {c}
                  {result.column_types[i] ? (
                    <span className="ml-2 text-[10px] uppercase text-ink-3">
                      {result.column_types[i]}
                    </span>
                  ) : null}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {result.rows.length === 0 ? (
              <tr>
                <td
                  colSpan={cols.length}
                  className="px-3 py-4 text-center text-ink-3"
                >
                  no rows
                </td>
              </tr>
            ) : (
              result.rows.map((r, i) => {
                const mask = BigInt(r.null_mask ?? "0");
                return (
                  <tr
                    key={i}
                    className="border-b border-line last:border-b-0 odd:bg-paper-2/40"
                  >
                    {r.cells.map((v, j) => {
                      const isNull = (mask >> BigInt(j)) & 1n;
                      return (
                        <td
                          key={j}
                          className="whitespace-pre-wrap px-3 py-1.5 align-top"
                        >
                          {isNull ? (
                            <span className="text-ink-4">NULL</span>
                          ) : (
                            <span className="text-ink">{v}</span>
                          )}
                        </td>
                      );
                    })}
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function defaultQuery(tables: string[]): string {
  const t = tables.includes("users") ? "users" : (tables[0] ?? "");
  if (!t) return "SELECT 1";
  return `SELECT id, email, name, status, role, created_at\nFROM ${t}\nORDER BY created_at DESC\nLIMIT 25`;
}

function truncate(s: string, max: number): string {
  const oneLine = s.replace(/\s+/g, " ").trim();
  if (oneLine.length <= max) return oneLine;
  return oneLine.slice(0, max - 1) + "…";
}

// Build an RFC-4180 CSV from the current result and trigger a browser
// download. Handles NULLs (null_mask bit → empty cell) and escapes
// commas, quotes, and newlines by double-quoting + doubling internal
// quotes. Filename includes a timestamp so multiple exports don't
// collide.
function downloadCsv(result: {
  columns: string[];
  rows: { cells: string[]; null_mask?: string }[];
}) {
  const escape = (v: string): string => {
    if (v.includes('"') || v.includes(",") || v.includes("\n") || v.includes("\r")) {
      return `"${v.replace(/"/g, '""')}"`;
    }
    return v;
  };
  const lines: string[] = [];
  lines.push(result.columns.map(escape).join(","));
  for (const r of result.rows) {
    const mask = BigInt(r.null_mask ?? "0");
    const cells = r.cells.map((v, i) => {
      const isNull = (mask >> BigInt(i)) & 1n;
      return isNull ? "" : escape(v);
    });
    lines.push(cells.join(","));
  }
  const blob = new Blob([lines.join("\r\n") + "\r\n"], {
    type: "text/csv;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  const stamp = new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19);
  a.href = url;
  a.download = `admin-db-${stamp}.csv`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
