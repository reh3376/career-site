import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import {
  BRANCH_LABEL,
  REVIEW_LABEL,
  REVIEW_TONE,
  STATUS_TONE,
  fmt,
  getPilotSession,
  type PilotAnswer,
  type PilotComment,
  type PilotNote,
} from "../lib";
import { DeleteSessionButton } from "./delete-session-button";
import { NoteBox } from "./note-box";
import { ReviewForm } from "./review-form";

export const metadata: Metadata = { title: "Admin · Pilot session" };
export const dynamic = "force-dynamic";

// One session in full: answers with flags in instrument order, comments
// by target, and the section open and submit instants (FSD FR-PLT-11),
// with the owner's notes beside each (FR-PLT-16) and the owner's
// judgement on the sitting (FR-PLT-17). Nothing a participant wrote is
// edited from here; the owner's own words and the delete at the foot
// (FR-PLT-15, for the owner's own runs) are the writes.
export default async function AdminPilotSessionPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const res = await getPilotSession(id);
  if (!res.ok) {
    if (res.status === 404) notFound();
    return (
      <>
        <Crumb />
        <p className="mt-6 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Couldn&rsquo;t load the session, {res.message}.
        </p>
      </>
    );
  }
  const d = res.data;
  const s = d.summary;
  const ses = s?.session;
  const status = ses?.status ?? "";
  const tone = STATUS_TONE[status] ?? "text-ink-2";
  const byKind = groupComments(d.comments ?? []);
  const events = d.events ?? [];
  const notes = new Map<string, PilotNote>();
  for (const n of d.notes ?? []) notes.set(`${n.targetKind}:${n.targetId ?? ""}`, n);
  const noteFor = (kind: string, id: string) => notes.get(`${kind}:${id}`)?.body ?? "";
  const sessionId = ses?.id ?? "";

  return (
    <>
      <Crumb />
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {s?.name || s?.email || "Session"}
      </h1>
      <p className="mt-2 text-sm text-ink-3">
        {s?.email}
        {s?.memberId ? (
          <>
            {" · "}
            <Link href={`/admin/registrations/${s.memberId}`} className="text-ink-2 hover:text-accent">
              member page
            </Link>
          </>
        ) : null}
      </p>

      <section aria-labelledby="session-heading" className="mt-10 border-t border-line pt-6">
        <p id="session-heading" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          session
        </p>
        <dl className="mt-4 grid gap-3 font-mono text-sm text-ink-2 sm:grid-cols-[11rem_1fr]">
          <dt className="text-ink-3">status</dt>
          <dd className={`m-0 ${tone}`}>{status || "unknown"}</dd>
          <dt className="text-ink-3">branch</dt>
          <dd className="m-0 text-ink">{BRANCH_LABEL[ses?.roleBranch ?? ""] ?? ses?.roleBranch}</dd>
          <dt className="text-ink-3">instrument</dt>
          <dd className="m-0 text-ink">{ses?.instrumentVersion}</dd>
          <dt className="text-ink-3">consented</dt>
          <dd className="m-0 text-ink">
            {fmt(ses?.consentAt)}
            {ses?.recordingOk ? " · recording ok" : " · no recording"}
          </dd>
          <dt className="text-ink-3">survey finished</dt>
          <dd className="m-0 text-ink">{fmt(ses?.surveyFinishedAt)}</dd>
          <dt className="text-ink-3">record read</dt>
          <dd className="m-0 text-ink">{fmt(ses?.recordFinishedAt)}</dd>
          <dt className="text-ink-3">workflow read</dt>
          <dd className="m-0 text-ink">{fmt(ses?.workflowFinishedAt)}</dd>
          <dt className="text-ink-3">finished</dt>
          <dd className="m-0 text-ink">{fmt(ses?.finishedAt)}</dd>
          <dt className="text-ink-3">minutes</dt>
          <dd className="m-0 text-ink">
            {s?.measuredMinutes || "-"} measured · {s?.selfReportedMinutes || "-"} self-reported
          </dd>
          <dt className="text-ink-3">sections done</dt>
          <dd className="m-0 text-ink">{ses?.sectionsDone?.join(" ") || "-"}</dd>
          <dt className="text-ink-3">your judgement</dt>
          <dd className={`m-0 ${REVIEW_TONE[s?.reviewStatus ?? "unreviewed"] ?? "text-ink-3"}`}>
            {REVIEW_LABEL[s?.reviewStatus ?? "unreviewed"] ?? s?.reviewStatus}
            {s?.reviewedAt ? <span className="text-ink-3"> · {fmt(s.reviewedAt)}</span> : null}
            {s?.reviewNote ? <span className="text-ink-2"> · {s.reviewNote}</span> : null}
          </dd>
        </dl>
      </section>

      {sessionId ? (
        <section aria-labelledby="review-heading" className="mt-12 border-t border-line pt-6">
          <p id="review-heading" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            your review of this sitting
          </p>
          <p className="mt-3 max-w-prose text-sm text-ink-3">
            The judgement says whether this sitting counts: reviewed is vouched for, excluded stays in the database and
            out of every figure, with the reason. The note is the call&rsquo;s record as a whole; notes on single answers,
            rows and steps sit beside them below.
          </p>
          <ReviewForm sessionId={sessionId} status={s?.reviewStatus ?? "unreviewed"} note={s?.reviewNote ?? ""} />
          <NoteBox sessionId={sessionId} targetKind="session" targetId="" initialBody={noteFor("session", "")} label="note on the sitting" />
        </section>
      ) : null}

      <section aria-labelledby="answers-heading" className="mt-12 border-t border-line pt-6">
        <p id="answers-heading" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          answers · {d.answers?.length ?? 0} · {s?.flagCount ?? 0} flagged
        </p>
        {d.answers?.length ? (
          <ul className="mt-4 divide-y divide-line border-y border-line">
            {d.answers.map((a) => (
              <AnswerRow key={a.itemId} a={a} sessionId={sessionId} note={noteFor("item", a.itemId)} />
            ))}
          </ul>
        ) : (
          <p className="mt-4 text-sm text-ink-3">No section submitted yet.</p>
        )}
      </section>

      <section aria-labelledby="comments-heading" className="mt-12 border-t border-line pt-6">
        <p id="comments-heading" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          comments · {d.comments?.length ?? 0}
        </p>
        {byKind.length === 0 ? (
          <p className="mt-4 text-sm text-ink-3">None yet.</p>
        ) : (
          byKind.map(([kind, list]) => (
            <div key={kind} className="mt-6">
              <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2">{KIND_LABEL[kind] ?? kind}</p>
              <ul className="mt-2 divide-y divide-line border-y border-line">
                {list.map((c) => (
                  <li key={`${c.targetKind}:${c.targetId}`} className="grid gap-2 py-3 sm:grid-cols-[6rem_1fr]">
                    <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">{c.targetId || "general"}</span>
                    <div>
                      <p className="whitespace-pre-wrap text-sm text-ink">{c.body}</p>
                      <p className="mt-1 font-mono text-[11px] text-ink-4">{fmt(c.updatedAt)}</p>
                      {sessionId ? (
                        <NoteBox
                          sessionId={sessionId}
                          targetKind={c.targetKind}
                          targetId={c.targetId ?? ""}
                          initialBody={noteFor(c.targetKind, c.targetId ?? "")}
                          label="your note"
                          compact
                        />
                      ) : null}
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          ))
        )}
      </section>

      <section aria-labelledby="events-heading" className="mt-12 border-t border-line pt-6">
        <p id="events-heading" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          section timings · oldest first
        </p>
        {events.length === 0 ? (
          <p className="mt-4 text-sm text-ink-3">No section opened yet.</p>
        ) : (
          <ul className="mt-4 space-y-1 font-mono text-xs text-ink-2">
            {events.map((e, i) => (
              <li key={`${e.section}-${e.event}-${i}`}>
                <span className="text-ink-3">{fmt(e.at)}</span> · {e.section} · {e.event}
              </li>
            ))}
          </ul>
        )}
      </section>

      {ses?.id ? (
        <section aria-labelledby="delete-heading" className="mt-12 border-t border-line pt-6">
          <p id="delete-heading" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            delete
          </p>
          <p className="mt-3 max-w-prose text-sm text-ink-3">
            For your own runs on the deployed build, so they never sit beside a participant&rsquo;s in the list or an
            export. Participant codes in the exports follow session order, so delete before the ten start, not after.
          </p>
          <div className="mt-4">
            <DeleteSessionButton id={ses.id} who={s?.email || s?.name || "this member"} />
          </div>
        </section>
      ) : null}
    </>
  );
}

const KIND_LABEL: Record<string, string> = {
  record_row: "match record · rows",
  record_general: "match record · general",
  workflow_node: "workflow · steps",
  workflow_missing: "workflow · missing steps",
  workflow_general: "workflow · general",
};

const KIND_ORDER = ["record_row", "record_general", "workflow_node", "workflow_missing", "workflow_general"];

function groupComments(list: PilotComment[]): [string, PilotComment[]][] {
  const m = new Map<string, PilotComment[]>();
  for (const c of list) {
    const arr = m.get(c.targetKind) ?? [];
    arr.push(c);
    m.set(c.targetKind, arr);
  }
  return [...m.entries()].sort(
    ([a], [b]) => (KIND_ORDER.indexOf(a) + 100) % 100 - ((KIND_ORDER.indexOf(b) + 100) % 100),
  );
}

function Crumb() {
  return (
    <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
      <Link href="/admin/pilot" className="text-ink-2 no-underline hover:text-accent">
        ← pilot
      </Link>
    </p>
  );
}

// An answer's JSON, shown the way a reader would want it: strings bare,
// arrays one per line, numbers as numbers, and the C3 grid as
// option → rating pairs.
function renderValue(json?: string): React.ReactNode {
  if (!json) return <span className="text-ink-4">-</span>;
  let v: unknown;
  try {
    v = JSON.parse(json);
  } catch {
    return <span className="font-mono text-xs">{json}</span>;
  }
  if (typeof v === "string") return v === "" ? <span className="text-ink-4">(empty)</span> : <span className="whitespace-pre-wrap">{v}</span>;
  if (typeof v === "number") return <span className="font-mono">{v}</span>;
  if (Array.isArray(v)) {
    return (
      <ul className="list-disc space-y-0.5 pl-5">
        {v.map((x) => (
          <li key={String(x)}>{String(x)}</li>
        ))}
      </ul>
    );
  }
  if (v && typeof v === "object") {
    const entries = Object.entries(v as Record<string, Record<string, string>>);
    return (
      <ul className="space-y-1">
        {entries.map(([opt, ratings]) => (
          <li key={opt}>
            <span className="text-ink">{opt}</span>
            <span className="ml-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              {Object.entries(ratings)
                .map(([k, val]) => `${k}: ${val}`)
                .join(" · ")}
            </span>
          </li>
        ))}
      </ul>
    );
  }
  return <span className="font-mono text-xs">{json}</span>;
}

function AnswerRow({ a, sessionId, note }: { a: PilotAnswer; sessionId: string; note: string }) {
  return (
    <li className="grid gap-2 py-3 sm:grid-cols-[6rem_1fr]">
      <div className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        <span className="text-ink">{a.itemId}</span>
        {a.flagged ? <span className="ml-2 text-signal">flag</span> : null}
      </div>
      <div>
        {a.prompt ? <p className="text-sm text-ink-3">{a.prompt}</p> : null}
        <div className={`text-sm text-ink ${a.prompt ? "mt-1" : ""}`}>{renderValue(a.valueJson)}</div>
        {a.note ? (
          <p className="mt-2 border-l-2 border-signal/60 pl-3 text-sm text-ink-2">
            <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">participant&rsquo;s note</span>
            <br />
            <span className="whitespace-pre-wrap">{a.note}</span>
          </p>
        ) : null}
        {sessionId ? (
          <NoteBox sessionId={sessionId} targetKind="item" targetId={a.itemId} initialBody={note} label="your note" compact />
        ) : null}
      </div>
    </li>
  );
}
