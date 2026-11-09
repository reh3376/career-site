import type { Metadata } from "next";
import Link from "next/link";

import { markPartAction } from "../actions";
import { CommentBox } from "../comment-box";
import { requireSignedIn, unwrap } from "../gate";
import { getPilotRecord, type PilotComment } from "../lib";

import { RecordView, type RecordData } from "./record-view";

export const metadata: Metadata = { title: "Pilot · Match record" };
export const dynamic = "force-dynamic";

// The mocked match record, read-only, in the site's own design, with a
// comment box under every requirement row and one general box
// (FSD FR-PLT-08). Comments never change the record: the record is a
// string of JSON the API hands over, rendered here and nowhere written.
export default async function PilotRecordPage() {
  await requireSignedIn("/pilot/record");
  const data = unwrap(await getPilotRecord());
  let record: RecordData | null = null;
  try {
    record = data.recordJson ? (JSON.parse(data.recordJson) as RecordData) : null;
  } catch {
    record = null;
  }
  const comments = new Map<string, PilotComment>();
  for (const c of data.comments ?? []) comments.set(`${c.targetKind}:${c.targetId ?? ""}`, c);
  const markRecordDone = markPartAction.bind(null, "record");

  return (
    <div className="mx-auto max-w-4xl px-6 py-16 sm:px-10 sm:py-24">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">the pilot · part 2 of 3</p>
        <Link href="/pilot" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 no-underline hover:text-accent">
          ← pilot home
        </Link>
      </div>
      <h1
        className="font-display mt-6 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        A match record.
      </h1>
      <p className="mt-6 max-w-2xl text-base leading-relaxed text-ink-2">
        One candidate against one posting, as a screener would see it. The names are made up; the shape is
        real. Read it as if it had arrived at work this morning. Under every row there is a box for whatever
        you would say about that row, and at the end one for the record as a whole. Nothing you write changes
        the record.
      </p>

      {record ? (
        <RecordView
          record={record}
          rowComment={(id) => (
            <CommentBox
              targetKind="record_row"
              targetId={id}
              initialBody={comments.get(`record_row:${id}`)?.body ?? ""}
              label={`your comment on ${id}`}
              placeholder="Would you trust this verdict? What would you check? What is missing?"
            />
          )}
        />
      ) : (
        <p className="mt-10 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          The record could not be read. Try again in a moment.
        </p>
      )}

      <section className="mt-14 border-t border-line pt-8">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">the record as a whole</p>
        <p className="mt-3 max-w-2xl text-base leading-relaxed text-ink">
          Which row would you check first, and why? Anything else about the record goes here too.
        </p>
        <CommentBox
          targetKind="record_general"
          targetId=""
          initialBody={comments.get("record_general:")?.body ?? ""}
          label="your general comment"
        />
      </section>

      <form action={markRecordDone} className="mt-12 flex flex-wrap items-center gap-4 border-t border-line pt-8">
        <button
          type="submit"
          className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover"
        >
          I have read the record
        </button>
        <p className="text-xs text-ink-3">Marks this part done and takes you on to the workflow. You can still come back and change your comments.</p>
      </form>
    </div>
  );
}
