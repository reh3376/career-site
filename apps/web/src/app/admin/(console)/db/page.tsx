import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { QueryConsole } from "./query-console";

export const metadata: Metadata = { title: "Admin — DB query" };
export const dynamic = "force-dynamic";

type DbColumn = {
  name: string;
  data_type: string;
  nullable?: boolean;
};

type DbTable = {
  name: string;
  columns?: DbColumn[];
  approx_row_count?: string; // int64 comes across as a string
};

type ListResp = { tables?: DbTable[] };

async function fetchTables(): Promise<DbTable[]> {
  const cookie = await getSessionCookie();
  if (!cookie) return [];
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListDbTables",
    body: {},
    cookie,
  });
  if (!resp.ok) return [];
  const j = (await resp.json()) as ListResp;
  return j.tables ?? [];
}

export default async function AdminDbPage() {
  const tables = await fetchTables();

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        read-only · SELECT only
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        DB query.
      </h1>
      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        Ad-hoc SQL against the API&rsquo;s Postgres. MVP is read-only:
        only <code className="font-mono text-ink">SELECT</code> /{" "}
        <code className="font-mono text-ink">WITH</code> statements pass
        the server-side guard, single statement per query, 500-row cap,
        5s statement timeout. Every query is logged with your user_id.
      </p>

      <div className="mt-10 grid gap-8 md:grid-cols-[260px_minmax(0,1fr)]">
        <SchemaPanel tables={tables} />
        <QueryConsole tables={tables.map((t) => t.name)} />
      </div>
    </>
  );
}

function SchemaPanel({ tables }: { tables: DbTable[] }) {
  return (
    <aside aria-label="Schema" className="md:sticky md:top-24 md:self-start">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        schema
      </p>
      {tables.length === 0 ? (
        <p className="mt-4 text-sm text-ink-3">
          Couldn&rsquo;t reach the DB (or no public tables). If this is
          unexpected, check the API logs.
        </p>
      ) : (
        <details className="mt-4" open>
          <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 hover:text-accent">
            {tables.length} tables
          </summary>
          <ul className="mt-3 space-y-4 border-l border-line pl-3 text-xs">
            {tables.map((t) => (
              <li key={t.name}>
                <p className="font-mono text-[12px] text-ink">
                  {t.name}
                  {t.approx_row_count ? (
                    <span className="ml-2 text-ink-3">
                      ~{Number(t.approx_row_count).toLocaleString()} rows
                    </span>
                  ) : null}
                </p>
                <ul className="mt-1 space-y-0.5">
                  {(t.columns ?? []).map((c) => (
                    <li
                      key={c.name}
                      className="font-mono text-[11px] text-ink-3"
                    >
                      <span className="text-ink-2">{c.name}</span>{" "}
                      <span>{c.data_type}</span>
                      {c.nullable ? null : (
                        <span className="ml-1 text-signal">·NN</span>
                      )}
                    </li>
                  ))}
                </ul>
              </li>
            ))}
          </ul>
        </details>
      )}
    </aside>
  );
}
