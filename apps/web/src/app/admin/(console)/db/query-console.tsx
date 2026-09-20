"use client";

import { useState, useTransition } from "react";

import { runDbQueryAction, type QueryResult } from "./actions";

// Client-side query editor + results table for /admin/db. Uses
// useTransition so the button reflects pending state and the page
// doesn't block while the RPC is in flight.
export function QueryConsole({ tables }: { tables: string[] }) {
  const [sql, setSql] = useState<string>(defaultQuery(tables));
  const [result, setResult] = useState<QueryResult | null>(null);
  const [pending, startTransition] = useTransition();

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!sql.trim() || pending) return;
    startTransition(async () => {
      const r = await runDbQueryAction(sql);
      setResult(r);
    });
  };

  const preset = (s: string) => {
    setSql(s);
    startTransition(async () => {
      const r = await runDbQueryAction(s);
      setResult(r);
    });
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
          <PresetBar tables={tables} onPick={preset} disabled={pending} />
        </div>
      </form>

      <div className="mt-8">
        <Result result={result} />
      </div>
    </section>
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
