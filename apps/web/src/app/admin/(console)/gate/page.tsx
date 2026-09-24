import type { Metadata } from "next";
import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = { title: "Admin · Gate" };
export const dynamic = "force-dynamic";

// The criteria as a gate rather than a dashboard.
//
// /admin/analytics shows the same numbers arranged for reading, which
// leaves the reader to decide whether each one is good enough. That is
// the decision worth making once and writing down, because a dashboard
// can be read favourably on a bad day and a pass cannot.
//
// Three states, and the third carries most of the weight: a criterion
// with no target set, or with too little data to judge, is unanswered.
// It is never drawn as a failure. A young system that renders as seven
// red rows is a system nobody opens again.

type Row = {
  criterion?: string;
  pass?: boolean;
  value?: string;
  target?: string;
  asOf?: string;
  as_of?: string;
  detail?: string;
};

async function fetchGate(): Promise<Row[] | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetGate",
    body: {},
    cookie,
  });
  if (!resp.ok) return null;
  const j = (await resp.json()) as { rows?: Row[] };
  return j.rows ?? [];
}

export default async function AdminGatePage() {
  const rows = await fetchGate();
  const answered = (rows ?? []).filter((r) => r.pass !== undefined);
  const met = answered.filter((r) => r.pass).length;

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        gate
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Is it good enough yet?
      </h1>
      <p className="mt-5 max-w-2xl text-base leading-relaxed text-ink-2">
        The same measurements as{" "}
        <Link
          href="/admin/analytics"
          className="text-accent no-underline hover:text-accent-strong"
        >
          analytics
        </Link>
        , answered instead of displayed. Each criterion is met, not met, or
        cannot be answered yet, and the last of those is not a failure: it
        means the sample is too short or nobody has set a target.
      </p>

      {rows === null ? (
        <p className="mt-8 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Could not read the gate. Nothing is shown rather than a guess.
        </p>
      ) : (
        <>
          <p className="mt-8 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            {met} of {answered.length} answerable criteria met
            <span className="text-ink-4"> · </span>
            {rows.length - answered.length} not yet answerable
          </p>

          <ul className="mt-6 space-y-px">
            {rows.map((r) => (
              <GateRow key={r.criterion} row={r} />
            ))}
          </ul>
        </>
      )}
    </>
  );
}

function GateRow({ row }: { row: Row }) {
  const asOf = row.asOf ?? row.as_of;
  const state =
    row.pass === undefined ? "open" : row.pass ? "met" : "not met";
  const border =
    state === "met"
      ? "border-l-success"
      : state === "not met"
        ? "border-l-danger"
        : "border-l-line-strong";
  const label =
    state === "met"
      ? "text-success"
      : state === "not met"
        ? "text-danger"
        : "text-ink-4";

  return (
    <li className={"border-l-2 bg-paper-2 px-5 py-4 " + border}>
      <div className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-1">
        <p className="text-base text-ink">{row.criterion}</p>
        <p
          className={
            "font-mono text-[11px] uppercase tracking-[0.14em] " + label
          }
        >
          {state}
        </p>
      </div>
      <p className="mt-1 font-mono text-sm text-ink">{row.value}</p>
      <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink-3">
        {row.target}
      </p>
      {row.detail ? (
        <p className="mt-1 max-w-2xl text-sm leading-relaxed text-ink-3">
          {row.detail}
        </p>
      ) : null}
      {asOf ? (
        <p className="mt-2 font-mono text-[11px] text-ink-4">
          as of {new Date(asOf).toLocaleString()}
        </p>
      ) : null}
    </li>
  );
}
