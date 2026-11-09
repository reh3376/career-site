import type { Metadata } from "next";
import Link from "next/link";

import { BRANCH_LABEL, STATUS_TONE, fmt, listPilotSessions, type PilotSessionSummary } from "./lib";

export const metadata: Metadata = { title: "Admin · Pilot" };
export const dynamic = "force-dynamic";

// Every pilot session with where it stands (FSD FR-PLT-11). The
// participant is named here, because the owner has to know whom to
// call; the exports below carry P-NN codes instead, and are what
// leaves the site (FR-PLT-12).
export default async function AdminPilotPage() {
  const res = await listPilotSessions();
  if (!res.ok) {
    return (
      <>
        <Header />
        <p className="mt-6 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Couldn&rsquo;t load the pilot sessions, {res.message}.
        </p>
      </>
    );
  }
  const sessions = res.data.sessions ?? [];
  const finished = sessions.filter((s) => s.session?.status === "finished").length;
  const surveyDone = sessions.filter((s) => !!s.session?.surveyFinishedAt).length;

  return (
    <>
      <Header />
      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        One row per participant who consented. Instrument{" "}
        <code className="font-mono text-ink">{res.data.instrumentVersion ?? "?"}</code>. Measured minutes run
        from the first section opened to the last submitted; the self-report is what the participant typed
        at P1. Open a row for the answers, flags, comments and section timings.
      </p>

      <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-3">
        <Stat label="consented" value={sessions.length} />
        <Stat label="survey done" value={surveyDone} />
        <Stat label="finished" value={finished} />
      </dl>

      <section aria-label="Exports" className="mt-8">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">export · csv · participants as P-NN</p>
        <div className="mt-3 flex flex-wrap gap-2">
          {(["sessions", "answers", "comments"] as const).map((t) => (
            <a
              key={t}
              href={`/admin/pilot/export?table=${t}`}
              className="inline-flex h-[30px] items-center border border-accent px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-accent no-underline transition-colors hover:bg-accent hover:text-white"
            >
              {t}
            </a>
          ))}
        </div>
        <p className="mt-2 text-xs text-ink-3">
          Each export is audited. Files belong in the private repository&rsquo;s pilot folder, never in this one.
        </p>
      </section>

      <section aria-label="Sessions" className="mt-12">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">sessions · newest first</p>
        {sessions.length === 0 ? (
          <p className="mt-6 text-sm text-ink-3">Nobody has consented yet.</p>
        ) : (
          <ul className="mt-4 divide-y divide-line border-y border-line">
            {sessions.map((s) => (
              <SessionRow key={s.session?.id ?? s.memberId} s={s} />
            ))}
          </ul>
        )}
      </section>
    </>
  );
}

function Header() {
  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">admin surface</p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Pilot.
      </h1>
    </>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex items-baseline gap-3">
      <dt className="text-ink-3">{label}</dt>
      <dd className="m-0 tabular text-ink text-2xl">{value}</dd>
    </div>
  );
}

function SessionRow({ s }: { s: PilotSessionSummary }) {
  const ses = s.session;
  const status = ses?.status ?? "";
  const tone = STATUS_TONE[status] ?? "text-ink-2";
  const parts = [
    ses?.surveyFinishedAt || ses?.roleBranch === "end" ? "survey" : null,
    ses?.recordFinishedAt ? "record" : null,
    ses?.workflowFinishedAt ? "workflow" : null,
  ].filter(Boolean);
  return (
    <li className="py-4">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <Link href={`/admin/pilot/${ses?.id ?? ""}`} className="text-sm text-ink no-underline hover:text-accent">
          {s.name || s.email || s.memberId}
          {s.name ? <span className="ml-2 text-xs text-ink-3">{s.email}</span> : null}
        </Link>
        <span className={`font-mono text-[11px] uppercase tracking-[0.14em] ${tone}`}>{status || "unknown"}</span>
      </div>
      <div className="mt-2 flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        <span>{BRANCH_LABEL[ses?.roleBranch ?? ""] ?? ses?.roleBranch}</span>
        <span>consented · {fmt(ses?.consentAt)}</span>
        <span>done · {parts.length ? parts.join(", ") : "nothing yet"}</span>
        <span>
          minutes · {s.measuredMinutes || "-"} measured · {s.selfReportedMinutes || "-"} said
        </span>
        <span>{s.commentCount ?? 0} comments</span>
        <span>{s.flagCount ?? 0} flags</span>
        {ses?.recordingOk ? <span className="text-ink-2">recording ok</span> : null}
        {ses?.finishedAt ? <span className="text-ink-2">finished · {fmt(ses.finishedAt)}</span> : null}
      </div>
    </li>
  );
}
