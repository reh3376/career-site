import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { JobList } from "./job-list";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = { title: "Admin · Ops" };
export const dynamic = "force-dynamic";

// What the box is doing.
//
// This page exists because of a specific failure: the job runner keeps
// jobs in memory, so a deploy recreates the api container and anything
// running dies with it. That happened twice on 2026-09-24, the second
// time losing an eight-posting evaluation at seven. Whether a deploy is
// safe depends entirely on whether something is running, and that was
// visible nowhere.
//
// So the first thing on the page is the answer to that one question,
// and everything else is below it.

type Job = {
  id?: string;
  kind?: string;
  status?: string;
  progress?: number;
  summary?: string;
  startedAt?: string;
  finishedAt?: string;
};

type PipelineCount = { status?: string; count?: number; oldest?: string };

type Ops = {
  jobs?: Job[];
  busy?: boolean;
  pipeline?: PipelineCount[];
  latestEval?: {
    id?: string;
    status?: string;
    scored?: number;
    total?: number;
    gateCorrect?: number;
    note?: string;
  };
  callsLastHour?: number;
  callFailuresLastHour?: number;
  avgCallSeconds?: number;
  load1?: number;
  load5?: number;
  memTotalMb?: number;
  memAvailableMb?: number;
  diskFreeGb?: number;
  diskTotalGb?: number;
  warnings?: string[];
};

async function fetchOps(): Promise<Ops | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetOpsStatus",
    body: {},
    cookie,
  });
  if (!resp.ok) return null;
  return (await resp.json()) as Ops;
}

function n(v: number | undefined): number {
  return typeof v === "number" ? v : 0;
}

function when(iso: string | undefined): string {
  return iso ? new Date(iso).toLocaleString() : "";
}

export default async function AdminOpsPage() {
  const ops = await fetchOps();
  const running = (ops?.jobs ?? []).filter((j) => !j.finishedAt);
  const finished = (ops?.jobs ?? []).filter((j) => j.finishedAt);
  const memUsedPct =
    n(ops?.memTotalMb) > 0
      ? Math.round(
          (100 * (n(ops?.memTotalMb) - n(ops?.memAvailableMb))) /
            n(ops?.memTotalMb),
        )
      : 0;
  const diskUsedPct =
    n(ops?.diskTotalGb) > 0
      ? Math.round(
          (100 * (n(ops?.diskTotalGb) - n(ops?.diskFreeGb))) /
            n(ops?.diskTotalGb),
        )
      : 0;

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        ops
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        What the box is doing.
      </h1>

      {ops === null ? (
        <p className="mt-8 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Could not read the runtime state.
        </p>
      ) : (
        <>
          {(ops.warnings ?? []).length > 0 ? (
            <ul className="mt-8 space-y-2">
              {(ops.warnings ?? []).map((w) => (
                <li
                  key={w}
                  className="border-l-2 border-danger bg-paper-2 px-4 py-3 text-sm text-ink"
                >
                  {w}
                </li>
              ))}
            </ul>
          ) : null}

          <div
            className={
              "mt-8 border-l-2 px-5 py-4 " +
              (ops.busy
                ? "border-signal bg-signal-soft/40"
                : "border-success bg-paper-2")
            }
          >
            <p className="text-base text-ink">
              {ops.busy
                ? "A job is running. Do not deploy."
                : "Nothing is running. Safe to deploy."}
            </p>
            <p className="mt-1 text-sm leading-relaxed text-ink-3">
              Jobs live in the api process, so recreating that container
              kills whatever is running and the work is lost. An evaluation
              is hours of it.
            </p>
          </div>

          <Section title="jobs the runner remembers">
            <JobList jobs={[...running, ...finished]} />
          </Section>

          <Section title="submissions by status">
            {(ops.pipeline ?? []).length === 0 ? (
              <p className="text-sm text-ink-3">
                {(ops.warnings ?? []).some((w) => w.startsWith("submissions"))
                  ? "Not read. See the warning above."
                  : "No submissions."}
              </p>
            ) : (
              <ul className="space-y-px">
                {(ops.pipeline ?? []).map((p) => (
                  <li
                    key={p.status}
                    className="flex flex-wrap items-baseline justify-between gap-x-4 bg-paper-2 px-4 py-2.5"
                  >
                    <span className="font-mono text-sm text-ink">
                      {p.status}
                      <span className="ml-3 text-ink-2">{n(p.count)}</span>
                    </span>
                    <span className="font-mono text-[11px] text-ink-4">
                      oldest {when(p.oldest)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
            <p className="mt-3 text-sm leading-relaxed text-ink-3">
              Evaluation submissions are excluded. Anything sitting at
              scoring or generating with an old timestamp is stuck, most
              likely because a restart took the run with it.
            </p>
          </Section>

          <Section title="the model, last hour">
            <dl className="grid gap-4 font-mono text-sm sm:grid-cols-3">
              <Stat label="calls" value={String(n(ops.callsLastHour))} />
              <Stat
                label="failed"
                value={String(n(ops.callFailuresLastHour))}
                alarm={n(ops.callFailuresLastHour) > 0}
              />
              <Stat
                label="avg seconds"
                value={n(ops.avgCallSeconds).toFixed(1)}
              />
            </dl>
            <p className="mt-3 text-sm leading-relaxed text-ink-3">
              Calls in the last hour is the honest test of whether work is
              actually happening. A job that says running while this reads
              zero has stalled.
            </p>
          </Section>

          <Section title="the host">
            <dl className="grid gap-4 font-mono text-sm sm:grid-cols-4">
              <Stat label="load 1m" value={n(ops.load1).toFixed(2)} />
              <Stat label="load 5m" value={n(ops.load5).toFixed(2)} />
              <Stat
                label="memory used"
                value={`${memUsedPct}%`}
                alarm={memUsedPct >= 90}
              />
              <Stat
                label="disk used"
                value={`${diskUsedPct}%`}
                alarm={diskUsedPct >= 85}
              />
            </dl>
            <p className="mt-3 text-sm leading-relaxed text-ink-3">
              {n(ops.memAvailableMb)} MB available of {n(ops.memTotalMb)} MB,
              and {n(ops.diskFreeGb)} GB free of {n(ops.diskTotalGb)} GB.
              Read from the host rather than this container. Disk filled to
              93 percent once and broke a deploy part way through, which is
              why it is on this page.
            </p>
          </Section>
        </>
      )}
    </>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="mt-10">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        {title}
      </p>
      <div className="mt-4">{children}</div>
    </section>
  );
}

function Stat({
  label,
  value,
  alarm = false,
}: {
  label: string;
  value: string;
  alarm?: boolean;
}) {
  return (
    <div>
      <dt className="text-[10px] uppercase tracking-[0.14em] text-ink-4">
        {label}
      </dt>
      <dd className={"m-0 mt-1 text-lg " + (alarm ? "text-danger" : "text-ink")}>
        {value}
      </dd>
    </div>
  );
}
