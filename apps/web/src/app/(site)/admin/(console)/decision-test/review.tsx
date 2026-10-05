import { reviewDecisionTestRunAction } from "./actions";

// The curation surface (roadmap M9, sprint S2).
//
// Server components and plain forms, deliberately. A review is one
// decision submitted once; there is no interactive state worth a client
// bundle, and a form that works before JavaScript loads is the right
// shape for a surface the owner uses to work through a queue.
//
// Four statuses, and **exactly one of them excludes anything**. The
// owner settled that: `incomplete` describes a run and does not block
// it, because whether an interrupted run is usable is a judgement he
// makes per run from the data rather than a rule. So the labels below
// say what each one means for the analysis, rather than leaving the
// reviewer to infer it.

export const REVIEW_STATUSES = [
  {
    value: "good",
    label: "Good",
    hint: "Usable. Counts everywhere.",
  },
  {
    value: "incomplete",
    label: "Incomplete",
    hint: "Did not finish, and not excluded by that alone. Still counts.",
  },
  {
    value: "hold",
    label: "Hold",
    hint: "Something is odd and not yet decided. Still counts.",
  },
  {
    value: "do_not_use",
    label: "Do not use",
    hint: "Excluded from every figure and from the export. Nothing is deleted.",
  },
] as const;

export const REVIEW_REASONS = [
  {
    value: "instrument_fault",
    label: "Instrument fault",
    hint: "The test did something wrong. A bug to go and fix.",
  },
  {
    value: "participant_reported",
    label: "Participant reported",
    hint: "They told you something that invalidates the run.",
  },
  { value: "duplicate", label: "Duplicate", hint: "The same sitting recorded twice." },
  { value: "other", label: "Other", hint: "Say why in the note." },
] as const;

export function statusLabel(status?: string): string {
  return REVIEW_STATUSES.find((s) => s.value === status)?.label ?? "Unreviewed";
}

/** The whole-run judgement. */
export function SessionReview({
  sessionKey,
  status,
  reason,
  note,
  reviewedAt,
  blocksExcluded,
}: {
  sessionKey: string;
  status?: string;
  reason?: string;
  note?: string;
  reviewedAt?: string;
  blocksExcluded?: number;
}) {
  const excluded = status === "do_not_use";
  return (
    <section
      className={`mt-12 rounded-md border px-6 py-6 ${
        excluded ? "border-signal bg-signal-soft/40" : "border-line bg-paper-2"
      }`}
    >
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <h2 className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
          Curation
        </h2>
        <p className="font-mono text-[11px] tracking-[0.1em] text-ink-3">
          {reviewedAt
            ? `${statusLabel(status)} · reviewed ${reviewedAt.slice(0, 16).replace("T", " ")}`
            : "Not yet reviewed"}
        </p>
      </div>

      {blocksExcluded ? (
        <p className="mt-4 text-sm leading-relaxed text-ink-2">
          <strong className="text-ink">
            {blocksExcluded} {blocksExcluded === 1 ? "block is" : "blocks are"} excluded
          </strong>{" "}
          individually, below. The rest of this run still counts.
        </p>
      ) : null}

      <form action={reviewDecisionTestRunAction} className="mt-6 space-y-6">
        <input type="hidden" name="session_key" value={sessionKey} />
        <input type="hidden" name="block_no" value={0} />

        <fieldset>
          <legend className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
            This run
          </legend>
          <div className="mt-3 space-y-2">
            {REVIEW_STATUSES.map((s) => (
              <label key={s.value} className="flex items-start gap-3 text-base text-ink-2">
                <input
                  type="radio"
                  name="status"
                  value={s.value}
                  defaultChecked={status === s.value}
                  className="mt-1.5"
                />
                <span>
                  <span className="text-ink">{s.label}</span>
                  <span className="block text-sm text-ink-3">{s.hint}</span>
                </span>
              </label>
            ))}
            {/* Clearing matters: a reviewer who marked the wrong row
                needs a way back to "nobody has looked at this", which is
                a different state from any verdict. */}
            <label className="flex items-start gap-3 text-base text-ink-2">
              <input
                type="radio"
                name="status"
                value=""
                defaultChecked={!status}
                className="mt-1.5"
              />
              <span>
                <span className="text-ink">Unreviewed</span>
                <span className="block text-sm text-ink-3">
                  Clear the judgement and put it back in the queue.
                </span>
              </span>
            </label>
          </div>
        </fieldset>

        <ReasonField name="reason" current={reason} />
        <NoteField name="note" current={note} />

        <button
          type="submit"
          className="rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase"
        >
          Save
        </button>
      </form>
    </section>
  );
}

/** One block's judgement, rendered inside the per-block table. */
export function BlockReview({
  sessionKey,
  blockNo,
  status,
  note,
}: {
  sessionKey: string;
  blockNo: number;
  status?: string;
  note?: string;
}) {
  const excluded = status === "do_not_use";
  return (
    <form action={reviewDecisionTestRunAction} className="flex flex-wrap items-center gap-2">
      <input type="hidden" name="session_key" value={sessionKey} />
      <input type="hidden" name="block_no" value={blockNo} />
      <input type="hidden" name="note" value={note ?? ""} />
      {/* A reason is required by the api only in the sense that it is
          allowed; a block exclusion from this control carries
          instrument_fault by default because that is what a single bad
          block usually is, and the run-level note is where anything
          else goes. */}
      <input type="hidden" name="reason" value={excluded ? "" : "instrument_fault"} />
      <input type="hidden" name="status" value={excluded ? "" : "do_not_use"} />
      <button
        type="submit"
        className={`rounded-sm px-3 py-1 font-mono text-[10px] tracking-[0.12em] uppercase ${
          excluded
            ? "bg-signal-soft/60 text-signal"
            : "border border-line text-ink-3 hover:text-accent"
        }`}
      >
        {excluded ? "excluded · undo" : "exclude"}
      </button>
    </form>
  );
}

function ReasonField({ name, current }: { name: string; current?: string }) {
  return (
    <fieldset>
      <legend className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        If excluding, why
      </legend>
      <p className="mt-2 max-w-xl text-sm text-ink-3">
        Instrument fault and participant reported point at opposite
        responses: the first is a bug to go and repair, the second costs
        a data point and has nothing to fix. Counting them together
        cannot tell you which.
      </p>
      <div className="mt-3 space-y-2">
        {REVIEW_REASONS.map((r) => (
          <label key={r.value} className="flex items-start gap-3 text-base text-ink-2">
            <input
              type="radio"
              name={name}
              value={r.value}
              defaultChecked={current === r.value}
              className="mt-1.5"
            />
            <span>
              <span className="text-ink">{r.label}</span>
              <span className="block text-sm text-ink-3">{r.hint}</span>
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}

function NoteField({ name, current }: { name: string; current?: string }) {
  return (
    <label className="block">
      <span className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        Note
      </span>
      <span className="mt-2 block max-w-xl text-sm text-ink-3">
        What the participant said, or what you saw. Kept out of the CSV
        export on purpose: a note is free text and can mention a person,
        and the export carries no identity.
      </span>
      <textarea
        name={name}
        rows={4}
        defaultValue={current ?? ""}
        className="mt-3 w-full max-w-xl rounded-md border border-line bg-canvas px-4 py-3 text-base text-ink"
      />
    </label>
  );
}
