"use client";

import { useEffect, useState } from "react";

import { chatClient } from "@/lib/chat-client";

type Query = { id: string; label: string; detail: string };
type Result = {
  label: string;
  detail: string;
  columns: string[];
  rows: string[][];
  at: string;
};

// The admin's database dropdown.
//
// Roger asked for the assistant to be able to query the database, and
// then for the admin to choose the query from a list. That second
// instruction is what makes this both safe and fast, and it is why
// there is no model anywhere in this component.
//
// The admin picks a query, the server runs a fixed statement, and the
// number comes back exact in milliseconds. Asking a language model the
// same question would take twenty to thirty seconds on this hardware
// and could get it wrong, which is a poor trade for a count.
//
// Rendered above the thread rather than inside it: it is a different
// kind of thing from a conversation, and burying a live number in a
// scrollback is how it gets missed.
export function AdminQueries() {
  const [queries, setQueries] = useState<Query[]>([]);
  const [selected, setSelected] = useState("");
  const [result, setResult] = useState<Result | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await chatClient.listAdminQueries({});
        if (cancelled) return;
        setQueries(
          (res.queries ?? []).map((q) => ({
            id: q.id,
            label: q.label,
            detail: q.detail,
          })),
        );
      } catch {
        // A member who is not an admin gets permission denied here, and
        // that is the normal case rather than a fault. The panel simply
        // does not appear.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (queries.length === 0) return null;

  const chosen = queries.find((q) => q.id === selected);

  async function run(id: string) {
    if (!id) return;
    setRunning(true);
    setError("");
    try {
      const res = await chatClient.runAdminQuery({ id });
      setResult({
        label: res.query?.label ?? id,
        detail: res.query?.detail ?? "",
        columns: res.columns ?? [],
        rows: (res.rows ?? []).map((r) => r.cells ?? []),
        at: new Date().toLocaleTimeString(),
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "The query did not run.");
    } finally {
      setRunning(false);
    }
  }

  return (
    <section className="mb-4 border border-line bg-paper-2 px-4 py-3">
      <p className="font-mono text-[11px] tracking-[0.12em] text-ink-3 uppercase">
        Look something up
      </p>

      <div className="mt-2 flex flex-col gap-2 sm:flex-row">
        <label htmlFor="admin-query" className="sr-only">
          Database query
        </label>
        <select
          id="admin-query"
          value={selected}
          onChange={(e) => {
            setSelected(e.target.value);
            setResult(null);
            run(e.target.value);
          }}
          className="w-full border border-line bg-canvas px-3 py-2 text-sm text-ink focus:border-accent focus:outline-none"
        >
          <option value="">Choose a query</option>
          {queries.map((q) => (
            <option key={q.id} value={q.id}>
              {q.label}
            </option>
          ))}
        </select>
        <button
          type="button"
          onClick={() => run(selected)}
          disabled={!selected || running}
          className="shrink-0 border border-accent px-4 py-2 font-mono text-[11px] tracking-[0.12em] text-accent uppercase transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
        >
          {running ? "Running" : "Run again"}
        </button>
      </div>

      {chosen && !result ? (
        <p className="mt-2 text-sm text-ink-3">{chosen.detail}</p>
      ) : null}

      {result ? (
        <div className="mt-3">
          {result.rows.length === 0 ? (
            <p className="text-sm text-ink-2">
              No rows. That is an answer: there is nothing of this kind yet.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-sm">
                <thead>
                  <tr className="border-b border-line">
                    {result.columns.map((c, i) => (
                      <th
                        key={c}
                        scope="col"
                        className={`py-1.5 pr-4 font-mono text-[11px] font-normal tracking-[0.1em] text-ink-3 uppercase ${
                          i === 0 ? "text-left" : "text-right"
                        }`}
                      >
                        {c.replace(/_/g, " ")}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {result.rows.map((row, ri) => (
                    <tr key={ri} className="border-b border-line last:border-0">
                      {row.map((cell, ci) => (
                        <td
                          key={ci}
                          className={`py-1.5 pr-4 ${
                            ci === 0
                              ? "text-left text-ink"
                              : "text-right font-mono tabular-nums text-ink"
                          }`}
                        >
                          {cell === "" ? "·" : cell}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className="mt-2 font-mono text-[11px] text-ink-3">
            {result.label} · {result.at}
          </p>
          {result.detail ? (
            <p className="mt-1 text-[13px] text-ink-3">{result.detail}</p>
          ) : null}
        </div>
      ) : null}

      {error ? <p className="mt-2 text-sm text-danger">{error}</p> : null}
    </section>
  );
}
