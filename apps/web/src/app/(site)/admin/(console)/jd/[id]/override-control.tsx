"use client";

import Link from "next/link";
import { useActionState, useState } from "react";
import { useFormStatus } from "react-dom";

import type { OverrideChoice, VocabCode } from "@/lib/match-record";

import { overrideJudgmentAction, type OverrideState } from "./record-actions";

// The controls under one row of the match record, for an admin (FSD
// FR-HF-08, FR-HF-17): open the audit trail, and set, replace or
// withdraw a person's verdict.
//
// The form offers only pairs the server accepts: the determinations and
// the bases each may carry arrive with the record, so the rule lives in
// one place. Leaving the verdict as the model had it and saving is a
// confirmation, which is as much a signal as a correction is; both are
// what the agreement measures count.

const label = "font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3";
const field =
  "mt-1 block w-full border border-line bg-transparent px-3 py-2 text-sm text-ink focus:border-accent focus:outline-none";
const button =
  "inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50";
const quiet =
  "inline-flex h-[30px] items-center border border-line px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 no-underline transition-colors hover:border-accent hover:text-accent";

export type OverrideTarget = {
  judgmentId: string;
  rowId: string;
  // What the record counts now.
  effectiveDetermination: string;
  effectiveBasis: string;
  // The override in place, if any.
  overridden: boolean;
  overrideRole?: string;
  // The override was carried from an earlier run; saving it again makes
  // it this run's own.
  carried?: boolean;
  // Why the row takes no override, when it takes none.
  closed?: string;
};

function nameOf(list: VocabCode[], code: string): string {
  return list.find((c) => c.code === code)?.label ?? code.replace(/_/g, " ");
}

export function OverrideControl({
  target,
  auditHref,
  choices,
  determinations,
  bases,
  roles,
}: {
  target: OverrideTarget;
  auditHref: string;
  choices: OverrideChoice[];
  determinations: VocabCode[];
  bases: VocabCode[];
  roles: VocabCode[];
}) {
  return (
    <div>
      <div className="flex flex-wrap items-center gap-3">
        <Link href={auditHref} className={quiet}>
          Audit
        </Link>
        {target.closed ? <span className="text-xs text-ink-3">{target.closed}</span> : null}
      </div>
      {target.closed ? null : (
        <details className="mt-3">
          <summary className={`${label} cursor-pointer text-accent hover:text-accent-hover`}>
            {target.carried
              ? "Confirm, change or withdraw the carried override"
              : target.overridden
                ? "Change or withdraw the override"
                : "Override or confirm this verdict"}
          </summary>
          <OverrideForm target={target} choices={choices} determinations={determinations} bases={bases} roles={roles} />
          {target.overridden ? <WithdrawForm target={target} /> : null}
        </details>
      )}
    </div>
  );
}

function OverrideForm({
  target,
  choices,
  determinations,
  bases,
  roles,
}: {
  target: OverrideTarget;
  choices: OverrideChoice[];
  determinations: VocabCode[];
  bases: VocabCode[];
  roles: VocabCode[];
}) {
  const [state, action] = useActionState<OverrideState, FormData>(overrideJudgmentAction, {});
  const offered = choices.filter((c) => c.determination);
  const start = offered.some((c) => c.determination === target.effectiveDetermination)
    ? target.effectiveDetermination
    : (offered[0]?.determination ?? "");
  const [determination, setDetermination] = useState(start);
  const allowed = offered.find((c) => c.determination === determination)?.bases ?? [];
  const [basis, setBasis] = useState(
    allowed.includes(target.effectiveBasis) ? target.effectiveBasis : (allowed[0] ?? ""),
  );
  // A basis the new verdict cannot carry is replaced by the first one
  // it can, so the form never holds a pair the server would refuse.
  const shownBasis = allowed.includes(basis) ? basis : (allowed[0] ?? "");

  return (
    <form action={action} className="mt-3 grid gap-4 border border-line bg-canvas p-4">
      <input type="hidden" name="judgment_id" value={target.judgmentId} />
      <input type="hidden" name="intent" value="override" />
      <div className="grid gap-4 sm:grid-cols-3">
        <label className="block">
          <span className={label}>verdict</span>
          <select
            name="determination"
            value={determination}
            onChange={(e) => setDetermination(e.target.value)}
            className={field}
          >
            {offered.map((c) => (
              <option key={c.determination} value={c.determination}>
                {nameOf(determinations, c.determination ?? "")}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className={label}>basis</span>
          <select name="basis" value={shownBasis} onChange={(e) => setBasis(e.target.value)} className={field}>
            {allowed.map((b) => (
              <option key={b} value={b}>
                {nameOf(bases, b)}
              </option>
            ))}
          </select>
          <span className="mt-1 block text-xs text-ink-3">Only the bases this verdict can rest on are offered.</span>
        </label>
        <label className="block">
          <span className={label}>acting as</span>
          <select key={target.overrideRole ?? ""} name="role" defaultValue={target.overrideRole || roles[0]?.code} className={field}>
            {roles.map((r) => (
              <option key={r.code} value={r.code}>
                {r.label ?? r.code}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label className="block">
        <span className={label}>reason</span>
        <textarea name="reason" required maxLength={1000} rows={2} className={field} />
        <span className="mt-1 block text-xs text-ink-3">
          Required. What you know that the documents or the model did not show. Leave the verdict as it is to
          confirm it. The model&rsquo;s own verdict is kept beside yours either way, and the recruiter is shown
          only the model&rsquo;s.
        </span>
      </label>
      <div className="flex flex-wrap items-center gap-4">
        <SubmitButton idle="Save this verdict" busy="Saving..." />
        {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
        {state.saved === "override" ? (
          <span className="text-sm text-success">Saved. The figures at the top count it.</span>
        ) : null}
      </div>
    </form>
  );
}

function WithdrawForm({ target }: { target: OverrideTarget }) {
  const [state, action] = useActionState<OverrideState, FormData>(overrideJudgmentAction, {});
  return (
    <form action={action} className="mt-3 grid gap-3 border border-line bg-canvas p-4">
      <input type="hidden" name="judgment_id" value={target.judgmentId} />
      <input type="hidden" name="intent" value="withdraw" />
      <label className="block">
        <span className={label}>withdraw the override on {target.rowId}</span>
        <input name="reason" required maxLength={1000} className={field} placeholder="Why it is being withdrawn" />
        <span className="mt-1 block text-xs text-ink-3">
          The model&rsquo;s verdict counts again. The override and its withdrawal both stay in the decision log.
        </span>
      </label>
      <div className="flex flex-wrap items-center gap-4">
        <SubmitButton idle="Withdraw" busy="Withdrawing..." />
        {state.error ? <span className="text-sm text-danger">{state.error}</span> : null}
        {state.saved === "withdraw" ? <span className="text-sm text-success">Withdrawn.</span> : null}
      </div>
    </form>
  );
}

function SubmitButton({ idle, busy }: { idle: string; busy: string }) {
  const { pending } = useFormStatus();
  return (
    <button type="submit" disabled={pending} className={button}>
      {pending ? busy : idle}
    </button>
  );
}
