"use client";

import { useActionState } from "react";
import { useFormStatus } from "react-dom";

import {
  releaseRequirementsAction,
  updateRequirementAction,
  type EditState,
  type Original,
  type ReleaseState,
  type Thresholds,
} from "./requirements-actions";

// The requirement editor (FSD §5.15 FR-HF-03, FR-HF-23; J8).
//
// The posting was read into requirements by the model. Here a person
// checks that reading before it is relied on: corrects the wording,
// says how much each item counts, sets the numbers the software will
// compare, and writes down what is accepted in place of a years or
// credential item. Every save is a new version with a reason. Nothing
// is judged against an edit until the drafts are released, and a
// judgment always names the version it judged.

export type VocabCode = { code: string; label?: string; description?: string };

export type Requirement = {
  id: string;
  roleId?: string;
  localId: string;
  version?: number;
  text?: string;
  sourceQuote?: string;
  type?: string;
  category?: string;
  requirementClass?: string;
  weight?: number;
  evidenceStandard?: string;
  evidenceStandardTemplate?: string;
  alternatives?: string[];
  equivalenceNotes?: string;
  equivalency?: string;
  thresholds?: Thresholds;
  namedParties?: string[];
  included?: boolean;
  inclusionReason?: string;
  editedByDisplay?: string;
  editedAt?: string;
  editReason?: string;
  status?: string;
  releasedByPerson?: boolean;
  typeSource?: string;
};

export type Vocab = {
  types: VocabCode[];
  classes: VocabCode[];
  scopeUnits: VocabCode[];
  credentialLevels: VocabCode[];
  relationshipKinds: VocabCode[];
};

const DRAFT = "REQUIREMENT_STATUS_DRAFT";
const ACTIVE = "REQUIREMENT_STATUS_ACTIVE";
const RETIRED = "REQUIREMENT_STATUS_RETIRED";

const label =
  "font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";
const field =
  "mt-1 block w-full border border-line bg-transparent px-3 py-2 text-sm text-ink focus:border-accent focus:outline-none";
const button =
  "inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50";

function nameOf(list: VocabCode[], code?: string): string {
  if (!code) return "";
  return list.find((c) => c.code === code)?.label ?? code;
}

function localNumber(id: string): number {
  const n = Number(id.replace(/\D/g, ""));
  return Number.isFinite(n) && n > 0 ? n : Number.MAX_SAFE_INTEGER;
}

type Group = {
  localId: string;
  current: Requirement; // the pending draft when there is one, else the released version
  released?: Requirement;
  earlier: Requirement[];
};

function group(requirements: Requirement[]): Group[] {
  const byLocal = new Map<string, Requirement[]>();
  for (const q of requirements) {
    const list = byLocal.get(q.localId) ?? [];
    list.push(q);
    byLocal.set(q.localId, list);
  }
  const out: Group[] = [];
  for (const [localId, list] of byLocal) {
    const draft = list.find((q) => q.status === DRAFT);
    const released = list.find((q) => q.status === ACTIVE);
    const current = draft ?? released;
    if (!current) continue; // a requirement the extraction dropped
    out.push({
      localId,
      current,
      released,
      earlier: list
        .filter((q) => q.status === RETIRED)
        .sort((a, b) => (b.version ?? 0) - (a.version ?? 0)),
    });
  }
  return out.sort((a, b) => localNumber(a.localId) - localNumber(b.localId));
}

export function RequirementsEditor({
  roleId,
  requirements,
  pendingDrafts,
  humanEdited,
  vocab,
  busy,
}: {
  roleId: string;
  requirements: Requirement[];
  pendingDrafts: number;
  humanEdited: boolean;
  vocab: Vocab;
  busy: boolean;
}) {
  const groups = group(requirements);
  const excluded = groups.filter((g) => g.current.included === false).length;

  return (
    <section className="mt-10 border border-line-strong bg-paper-2 p-5">
      <p className={label}>requirements</p>
      <p className="mt-2 max-w-3xl text-sm leading-relaxed text-ink-2">
        The posting was read into these requirements by the model. Check the
        reading before relying on what is judged against it. Each save is a
        new version with your reason, and the version before it is kept.
        Nothing is judged against an edit until you release it.
      </p>
      <p className="mt-3 font-mono text-xs text-ink-3">
        {groups.length} requirements
        {pendingDrafts > 0 ? (
          <span className="text-signal"> · {pendingDrafts} edited, not yet released</span>
        ) : null}
        {excluded > 0 ? <span> · {excluded} not counted</span> : null}
        {humanEdited ? <span> · edited by a person</span> : <span> · as extracted</span>}
      </p>

      <ul className="mt-5 divide-y divide-line border-y border-line">
        {groups.map((g) => (
          <RequirementItem key={g.localId} g={g} vocab={vocab} />
        ))}
      </ul>

      <ReleaseForm roleId={roleId} pendingDrafts={pendingDrafts} busy={busy} />
    </section>
  );
}

function RequirementItem({ g, vocab }: { g: Group; vocab: Vocab }) {
  const q = g.current;
  const isDraft = q.status === DRAFT;
  return (
    <li className="py-4">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        <span className="text-ink">{g.localId}</span>
        <span>{nameOf(vocab.classes, q.requirementClass)}</span>
        <span>weight {q.weight ?? "-"}</span>
        <span>kind: {nameOf(vocab.types, q.type)}</span>
        <span>version {q.version ?? 1}</span>
        {isDraft ? <span className="text-signal">edited, not yet released</span> : null}
        {!isDraft && q.editedByDisplay ? <span className="text-accent">edited</span> : null}
        {q.included === false ? <span className="text-signal">not counted</span> : null}
      </div>

      <p className="mt-2 text-sm text-ink">{q.text}</p>
      {q.sourceQuote ? (
        <p className="mt-1 text-xs text-ink-3">
          As the posting worded it: {q.sourceQuote}
        </p>
      ) : null}
      {isDraft && g.released && g.released.text !== q.text ? (
        <p className="mt-2 border-l-2 border-line-strong pl-2 text-xs text-ink-3">
          Released version {g.released.version ?? 1}, which is what a run
          judges until you release: {g.released.text}
        </p>
      ) : null}
      {q.editedByDisplay && q.editReason ? (
        <p className="mt-2 border-l-2 border-accent pl-2 text-xs text-ink-2">
          {q.editedByDisplay}: {q.editReason}
        </p>
      ) : null}
      {q.inclusionReason ? (
        <p
          className={
            "mt-2 border-l-2 pl-2 text-xs " +
            (q.included === false ? "border-signal text-ink-2" : "border-line-strong text-ink-3")
          }
        >
          {q.inclusionReason}
        </p>
      ) : null}

      <details className="mt-3">
        <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
          {isDraft ? "edit this draft" : "edit"}
        </summary>
        <EditForm q={q} vocab={vocab} />
      </details>

      {g.earlier.length > 0 ? (
        <details className="mt-2">
          <summary className="cursor-pointer font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            earlier versions ({g.earlier.length})
          </summary>
          <ul className="mt-2 space-y-2">
            {g.earlier.map((v) => (
              <li key={v.id} className="border-l-2 border-line pl-2 text-xs text-ink-3">
                <span className="font-mono">version {v.version ?? 1}</span>
                {v.editedByDisplay ? <span> · edited by {v.editedByDisplay}</span> : <span> · as extracted</span>}
                <span className="block text-ink-2">{v.text}</span>
                {v.editReason ? <span className="block">Reason: {v.editReason}</span> : null}
              </li>
            ))}
          </ul>
        </details>
      ) : null}
    </li>
  );
}

// Every select below carries a key equal to its default. React applies
// a changed defaultValue to an input or a textarea but not to a select,
// which keeps the option it was mounted with: after a save the form is
// reset and the kind would read as it was before the edit, and a second
// save would quietly put it back. The key remounts the select when the
// version it describes changes.
function EditForm({ q, vocab }: { q: Requirement; vocab: Vocab }) {
  const [state, action] = useActionState<EditState, FormData>(updateRequirementAction, {});
  const t = q.thresholds ?? {};
  const original: Original = {
    text: q.text ?? "",
    type: q.type ?? "",
    requirementClass: q.requirementClass ?? "",
    weight: q.weight ?? 2,
    thresholds: t,
    equivalency: q.equivalency ?? "",
    equivalenceNotes: q.equivalenceNotes ?? "",
    evidenceStandard: q.evidenceStandard ?? "",
    alternatives: q.alternatives ?? [],
    namedParties: q.namedParties ?? [],
  };
  const standardEdited = (q.evidenceStandardTemplate ?? "").startsWith("edited:");

  return (
    <form action={action} className="mt-3 grid gap-4 border border-line bg-canvas p-4">
      <input type="hidden" name="requirement_id" value={q.id} />
      <input type="hidden" name="original" value={JSON.stringify(original)} />

      <label className="block">
        <span className={label}>requirement, as it will be judged</span>
        <textarea name="text" required maxLength={500} rows={3} defaultValue={original.text} className={field} />
        <span className="mt-1 block text-xs text-ink-3">
          Keep the words that settle which field a term belongs to. The
          posting&rsquo;s own wording above is not changed by this.
        </span>
      </label>

      <div className="grid gap-4 sm:grid-cols-3">
        <label className="block">
          <span className={label}>kind</span>
          <select key={original.type} name="type" defaultValue={original.type} className={field}>
            {vocab.types.map((c) => (
              <option key={c.code} value={c.code}>
                {c.label ?? c.code}
              </option>
            ))}
          </select>
          <span className="mt-1 block text-xs text-ink-3">
            A behavioral item is never judged from documents.
          </span>
        </label>
        <label className="block">
          <span className={label}>class</span>
          <select
            key={original.requirementClass}
            name="requirement_class"
            defaultValue={original.requirementClass}
            className={field}
          >
            {vocab.classes.map((c) => (
              <option key={c.code} value={c.code}>
                {c.label ?? c.code}
              </option>
            ))}
          </select>
          <span className="mt-1 block text-xs text-ink-3">
            Required counts in full, preferred at a fraction, ideal as bonus
            only. The posting said: {q.category === "must" ? "required" : "preferred"}.
          </span>
        </label>
        <label className="block">
          <span className={label}>weight</span>
          <select key={original.weight} name="weight" defaultValue={String(original.weight)} className={field}>
            <option value="3">3, the core of the role</option>
            <option value="2">2, a stated requirement</option>
            <option value="1">1, a preference</option>
          </select>
        </label>
      </div>

      <fieldset className="grid gap-4 border border-line p-3 sm:grid-cols-3">
        <legend className={label + " px-1"}>numbers the software checks</legend>
        <label className="block">
          <span className={label}>years asked for</span>
          <input name="min_years" type="number" min="0" max="60" step="0.5" defaultValue={t.minYears ?? ""} className={field} />
        </label>
        <label className="block">
          <span className={label}>within the last (years)</span>
          <input name="recency_years" type="number" min="0" max="60" step="0.5" defaultValue={t.recencyYears ?? ""} className={field} />
        </label>
        <div className="grid grid-cols-2 gap-2">
          <label className="block">
            <span className={label}>scale</span>
            <select
              key={t.minScope?.unit ?? ""}
              name="scope_unit"
              defaultValue={t.minScope?.unit ?? ""}
              className={field}
            >
              <option value="">none</option>
              {vocab.scopeUnits.map((c) => (
                <option key={c.code} value={c.code}>
                  {c.label ?? c.code}
                </option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className={label}>amount</span>
            <input name="scope_value" type="number" min="0" step="any" defaultValue={t.minScope?.value ?? ""} className={field} />
          </label>
        </div>
        <label className="block">
          <span className={label}>credential level</span>
          <select
            key={t.credentialLevel ?? ""}
            name="credential_level"
            defaultValue={t.credentialLevel ?? ""}
            className={field}
          >
            <option value="">none</option>
            {vocab.credentialLevels.map((c) => (
              <option key={c.code} value={c.code}>
                {c.label ?? c.code}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className={label}>credential field</span>
          <input name="credential_field" type="text" maxLength={120} defaultValue={t.credentialField ?? ""} className={field} />
        </label>
        <label className="block">
          <span className={label}>relationship</span>
          <select
            key={t.relationshipKind ?? ""}
            name="relationship_kind"
            defaultValue={t.relationshipKind ?? ""}
            className={field}
          >
            <option value="">none</option>
            {vocab.relationshipKinds.map((c) => (
              <option key={c.code} value={c.code}>
                {c.label ?? c.code}
              </option>
            ))}
          </select>
        </label>
        <p className="text-xs text-ink-3 sm:col-span-3">
          Read from the wording when the requirement was drafted. Years, dates
          and scale are compared in code with what the documents state, never
          left to the model. Empty means no comparison is made. Relationship
          is the kind of working relationship asked for with the organisations
          named below.
        </p>
      </fieldset>

      <label className="block">
        <span className={label}>accepted in place of this requirement</span>
        <textarea name="equivalency" maxLength={500} rows={2} defaultValue={original.equivalency} className={field} />
        <span className="mt-1 block text-xs text-ink-3">
          For example: a graduate engineering degree counts as two of the ten
          years. A years or credential item counts in the figures only once
          this is written, or the posting itself accepts an equivalent.
        </span>
      </label>

      <label className="block">
        <span className={label}>known equivalents, from the catalogue</span>
        <textarea name="equivalence_notes" maxLength={1000} rows={3} defaultValue={original.equivalenceNotes} className={field} />
        <span className="mt-1 block text-xs text-ink-3">
          What the catalogue already says counts as this, may count, or does
          not. Strike or rewrite anything that is wrong for this role.
        </span>
      </label>

      <label className="block">
        <span className={label}>what counts as met, unresolved and not met</span>
        <textarea name="evidence_standard" maxLength={500} rows={4} defaultValue={original.evidenceStandard} className={field} />
        <span className="mt-1 block text-xs text-ink-3">
          {standardEdited
            ? "Rewritten by a person. The judge is shown it, and it will not be replaced when the kind changes."
            : "From the template for this kind, for your reading. Rewrite it to tell the judge what counts for this requirement; the template itself is not sent."}
        </span>
      </label>

      <div className="grid gap-4 sm:grid-cols-2">
        <label className="block">
          <span className={label}>options the posting offers, one per line</span>
          <textarea name="alternatives" rows={3} defaultValue={original.alternatives.join("\n")} className={field} />
        </label>
        <label className="block">
          <span className={label}>organisations named, one per line</span>
          <textarea name="named_parties" rows={3} defaultValue={original.namedParties.join("\n")} className={field} />
        </label>
      </div>

      <label className="block">
        <span className={label}>why are you changing it</span>
        <input name="edit_reason" type="text" required maxLength={500} className={field} />
        <span className="mt-1 block text-xs text-ink-3">
          Kept with the version. It is what tells us where the reading of
          postings goes wrong.
        </span>
      </label>

      <div className="flex flex-wrap items-center gap-4">
        <SaveButton />
        {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
        {state.saved ? (
          <span className="text-sm text-success">
            Saved as a draft. Release it below to have it judged.
          </span>
        ) : null}
      </div>
    </form>
  );
}

function SaveButton() {
  const { pending } = useFormStatus();
  return (
    <button type="submit" disabled={pending} className={button}>
      {pending ? "Saving..." : "Save as a new version"}
    </button>
  );
}

function ReleaseForm({
  roleId,
  pendingDrafts,
  busy,
}: {
  roleId: string;
  pendingDrafts: number;
  busy: boolean;
}) {
  const [state, action] = useActionState<ReleaseState, FormData>(releaseRequirementsAction, {});
  return (
    <form action={action} className="mt-5 flex flex-wrap items-center gap-4">
      <input type="hidden" name="role_id" value={roleId} />
      <ReleaseButton pendingDrafts={pendingDrafts} busy={busy} />
      <p className="max-w-2xl text-xs text-ink-3">
        {busy
          ? "A run is in progress. Release when it has finished."
          : "Releasing makes the edited versions the list a run judges and starts that run. The posting is not read into requirements again. Once anything here has been edited, Re-score judges this list too."}
      </p>
      {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
      {state.done ? (
        <span className="text-sm text-success">
          {state.released ? `${state.released} released. ` : ""}
          {state.runStarted
            ? "The run has started; this page shows it as it completes."
            : "No run was started: the reviewer is not available here."}
        </span>
      ) : null}
    </form>
  );
}

function ReleaseButton({ pendingDrafts, busy }: { pendingDrafts: number; busy: boolean }) {
  const { pending } = useFormStatus();
  const text =
    pendingDrafts > 0
      ? `Release ${pendingDrafts} and judge again`
      : "Judge this list again";
  return (
    <button type="submit" disabled={pending || busy} className={button}>
      {pending ? "Starting..." : text}
    </button>
  );
}
