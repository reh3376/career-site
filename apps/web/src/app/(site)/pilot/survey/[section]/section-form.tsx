"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";

import { submitSectionAction, type AnswerInput } from "../../actions";
import type { PilotAnswer, PilotItem } from "../../lib";

// The seven item types the instrument uses, rendered from data
// (FSD FR-PLT-04..06). Values are kept in the exact shape the API
// validates: a string for single and single_from, an array for multi,
// a string for text, a number for number and scale, and for the C3
// grid an object keyed by the option selected in C1 holding one value
// per dimension. Every item carries the per-item flag (FR-PLT-05).

type Ratings = Record<string, Record<string, string>>;
type Value = string | string[] | number | Ratings;

function parseAnswers(answers: PilotAnswer[]): { values: Record<string, Value>; flags: Record<string, boolean> } {
  const values: Record<string, Value> = {};
  const flags: Record<string, boolean> = {};
  for (const a of answers) {
    if (a.flagged) flags[a.itemId] = true;
    if (!a.valueJson) continue;
    try {
      values[a.itemId] = JSON.parse(a.valueJson) as Value;
    } catch {
      /* an unreadable saved value is simply not pre-filled */
    }
  }
  return { values, flags };
}

// The rows a derived item (single_from, rating_per_selected) offers:
// what the participant has selected in the source item right now, in
// the order selected, when the source is on the same page; otherwise
// what the API resolved from the saved answer. C1, C2 and C3 share a
// section, so without the live read C2 and C3 would have no rows
// until C1 had been saved, and C1 cannot be saved without them.
function rowsFor(item: PilotItem, values: Record<string, Value>): string[] {
  if (item.type !== "single_from" && item.type !== "rating_per_selected") return item.options ?? [];
  const live = item.sourceItem ? values[item.sourceItem] : undefined;
  if (Array.isArray(live)) return live as string[];
  return item.options ?? [];
}

// Whether an item has an answer worth sending. Optional items are
// only ever text, so an empty optional text is sent only when flagged,
// so the flag has a row to live on.
function answered(item: PilotItem, v: Value | undefined, rows: string[]): boolean {
  if (v === undefined || v === null) return false;
  switch (item.type) {
    case "single":
    case "single_from":
    case "text":
      return typeof v === "string" && v.trim() !== "";
    case "multi":
      return Array.isArray(v) && v.length > 0;
    case "number":
    case "scale":
      return typeof v === "number" && Number.isFinite(v);
    case "rating_per_selected": {
      if (typeof v !== "object" || Array.isArray(v)) return false;
      const m = v as Ratings;
      return rows.length > 0 && rows.every((opt) => (item.dimensions ?? []).every((d) => !!m[opt]?.[d.id]));
    }
    default:
      return false;
  }
}

export function SectionForm({
  sectionId,
  items,
  answers,
  isLast,
}: {
  sectionId: string;
  items: PilotItem[];
  answers: PilotAnswer[];
  isLast: boolean;
}) {
  const router = useRouter();
  const initial = parseAnswers(answers);
  const [values, setValues] = useState<Record<string, Value>>(initial.values);
  const [flags, setFlags] = useState<Record<string, boolean>>(initial.flags);
  const [itemErrors, setItemErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [pending, start] = useTransition();

  function setValue(id: string, v: Value | undefined) {
    setValues((prev) => {
      const next = { ...prev };
      if (v === undefined) delete next[id];
      else next[id] = v;
      // A change to a source item (C1) drops any derived answer that
      // named an option no longer selected, so the page never submits
      // a rating for a row the participant has just removed.
      const rows = Array.isArray(v) ? (v as string[]) : [];
      for (const d of items) {
        if (d.sourceItem !== id) continue;
        const cur = next[d.id];
        if (d.type === "single_from" && typeof cur === "string" && !rows.includes(cur)) delete next[d.id];
        if (d.type === "rating_per_selected" && cur && typeof cur === "object" && !Array.isArray(cur)) {
          const pruned: Ratings = {};
          for (const [opt, r] of Object.entries(cur as Ratings)) if (rows.includes(opt)) pruned[opt] = r;
          next[d.id] = pruned;
        }
      }
      return next;
    });
    setItemErrors((prev) => {
      if (!prev[id]) return prev;
      const next = { ...prev };
      delete next[id];
      return next;
    });
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const errs: Record<string, string> = {};
    const out: AnswerInput[] = [];
    for (const it of items) {
      const v = values[it.id];
      const has = answered(it, v, rowsFor(it, values));
      if (!has) {
        if (it.required) {
          errs[it.id] = requiredMessage(it);
        } else if (flags[it.id] && it.type === "text") {
          out.push({ itemId: it.id, valueJson: JSON.stringify(""), flagged: true });
        }
        continue;
      }
      if (it.type === "multi" && it.maxSelect && Array.isArray(v) && v.length > it.maxSelect) {
        errs[it.id] = `Select at most ${it.maxSelect}.`;
        continue;
      }
      out.push({ itemId: it.id, valueJson: JSON.stringify(v), flagged: !!flags[it.id] });
    }
    if (Object.keys(errs).length) {
      setItemErrors(errs);
      setFormError("A required question is unanswered. It is marked below.");
      const first = items.find((it) => errs[it.id]);
      if (first) document.getElementById(`item-${first.id}`)?.scrollIntoView({ behavior: "smooth", block: "center" });
      return;
    }
    setFormError(null);
    start(async () => {
      const res = await submitSectionAction(sectionId, out);
      if (res.error) {
        // The API names the item in front of a colon ("C1: select at
        // most 5"), so the message can sit under the question it is
        // about as well as at the top.
        const m = /^([A-Z]\d+[a-z]?):\s*(.+)$/.exec(res.error);
        if (m && items.some((it) => it.id === m[1])) {
          setItemErrors({ [m[1]]: m[2] });
          document.getElementById(`item-${m[1]}`)?.scrollIntoView({ behavior: "smooth", block: "center" });
        }
        setFormError(res.error);
        return;
      }
      if (res.next) router.push(res.next);
    });
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-12">
      {formError ? (
        <p role="alert" className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          {formError}
        </p>
      ) : null}

      {items.map((it) => (
        <Item
          key={it.id}
          item={it}
          rows={rowsFor(it, values)}
          value={values[it.id]}
          flagged={!!flags[it.id]}
          error={itemErrors[it.id]}
          onChange={(v) => setValue(it.id, v)}
          onFlag={(f) => setFlags((prev) => ({ ...prev, [it.id]: f }))}
        />
      ))}

      <div className="flex flex-wrap items-center gap-4 border-t border-line pt-8">
        <button
          type="submit"
          disabled={pending}
          className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-60"
        >
          {pending ? "Saving…" : isLast ? "Save and finish the survey" : "Save and continue"}
        </button>
        <p className="text-xs text-ink-3">Your answers are saved when you continue. You can come back and change them.</p>
      </div>
    </form>
  );
}

function requiredMessage(it: PilotItem): string {
  switch (it.type) {
    case "multi":
      return "Select at least one.";
    case "rating_per_selected":
      return "Rate every row on both scales.";
    case "text":
      return "A sentence or two is needed here.";
    default:
      return "An answer is needed here.";
  }
}

function Item({
  item,
  rows,
  value,
  flagged,
  error,
  onChange,
  onFlag,
}: {
  item: PilotItem;
  rows: string[];
  value: Value | undefined;
  flagged: boolean;
  error?: string;
  onChange: (v: Value | undefined) => void;
  onFlag: (f: boolean) => void;
}) {
  const labelId = `label-${item.id}`;
  return (
    <fieldset id={`item-${item.id}`} aria-labelledby={labelId} className={error ? "border-l-2 border-signal pl-4" : ""}>
      <div className="flex items-baseline gap-3">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">{item.id}</span>
        {item.required ? null : (
          <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-4">optional</span>
        )}
      </div>
      <p id={labelId} className="mt-2 text-base leading-relaxed text-ink">
        {item.prompt}
      </p>
      {item.help ? <p className="mt-1 text-sm text-ink-3">{item.help}</p> : null}

      <div className="mt-4">
        <Control item={item} rows={rows} value={value} onChange={onChange} />
      </div>

      {error ? (
        <p role="alert" className="mt-3 text-sm text-signal">
          {error}
        </p>
      ) : null}

      <label className="mt-4 flex items-center gap-2 text-xs text-ink-3">
        <input type="checkbox" checked={flagged} onChange={(e) => onFlag(e.target.checked)} className="h-3.5 w-3.5" />
        This was unclear or didn&rsquo;t fit me
      </label>
    </fieldset>
  );
}

const optionClass =
  "flex items-start gap-3 border border-line px-4 py-3 text-sm text-ink transition-colors hover:border-accent has-[:checked]:border-accent has-[:checked]:bg-accent/10";

function Control({
  item,
  rows,
  value,
  onChange,
}: {
  item: PilotItem;
  rows: string[];
  value: Value | undefined;
  onChange: (v: Value | undefined) => void;
}) {
  switch (item.type) {
    case "single":
    case "single_from": {
      const v = typeof value === "string" ? value : "";
      const opts = item.type === "single_from" ? rows : (item.options ?? []);
      if (item.type === "single_from" && opts.length === 0) {
        return <p className="text-sm text-ink-3">Select something in {item.sourceItem || "the question above"} first; your choices appear here.</p>;
      }
      return (
        <div className="grid gap-2">
          {opts.map((o) => (
            <label key={o} className={optionClass}>
              <input
                type="radio"
                name={item.id}
                value={o}
                checked={v === o}
                onChange={() => onChange(o)}
                className="mt-0.5 h-4 w-4 shrink-0"
              />
              <span>{o}</span>
            </label>
          ))}
        </div>
      );
    }
    case "multi": {
      const v = Array.isArray(value) ? (value as string[]) : [];
      const max = item.maxSelect ?? 0;
      const full = max > 0 && v.length >= max;
      return (
        <div>
          {max > 0 ? (
            <p className="mb-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              {v.length} of {max} selected
            </p>
          ) : null}
          <div className="grid gap-2">
            {(item.options ?? []).map((o) => {
              const on = v.includes(o);
              return (
                <label key={o} className={optionClass + (full && !on ? " opacity-50" : "")}>
                  <input
                    type="checkbox"
                    name={item.id}
                    value={o}
                    checked={on}
                    disabled={full && !on}
                    onChange={(e) => onChange(e.target.checked ? [...v, o] : v.filter((x) => x !== o))}
                    className="mt-0.5 h-4 w-4 shrink-0"
                  />
                  <span>{o}</span>
                </label>
              );
            })}
          </div>
        </div>
      );
    }
    case "text":
      return (
        <textarea
          name={item.id}
          value={typeof value === "string" ? value : ""}
          onChange={(e) => onChange(e.target.value)}
          rows={3}
          maxLength={4000}
          className="block w-full border border-line-strong bg-canvas px-3 py-2 text-sm text-ink outline-none focus:border-accent"
        />
      );
    case "number":
      return (
        <input
          type="number"
          inputMode="numeric"
          name={item.id}
          min={0}
          max={100000}
          step={1}
          value={typeof value === "number" ? value : ""}
          onChange={(e) => onChange(e.target.value === "" ? undefined : Number(e.target.value))}
          className="block w-32 border border-line-strong bg-canvas px-3 py-2 text-sm text-ink outline-none focus:border-accent"
        />
      );
    case "scale": {
      const min = item.scale?.min ?? 1;
      const max = item.scale?.max ?? 5;
      const steps: number[] = [];
      for (let n = min; n <= max; n++) steps.push(n);
      const v = typeof value === "number" ? value : undefined;
      return (
        <div>
          <div className="flex flex-wrap gap-2">
            {steps.map((n) => (
              <label
                key={n}
                className={
                  "inline-flex h-11 w-11 cursor-pointer items-center justify-center border font-mono text-sm transition-colors " +
                  (v === n ? "border-accent bg-accent text-white" : "border-line text-ink hover:border-accent")
                }
              >
                <input type="radio" name={item.id} value={n} checked={v === n} onChange={() => onChange(n)} className="sr-only" />
                {n}
              </label>
            ))}
          </div>
          <div className="mt-2 flex justify-between font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            <span>
              {min} · {item.scale?.minLabel ?? ""}
            </span>
            <span>
              {max} · {item.scale?.maxLabel ?? ""}
            </span>
          </div>
        </div>
      );
    }
    case "rating_per_selected": {
      const dims = item.dimensions ?? [];
      const m: Ratings = value && typeof value === "object" && !Array.isArray(value) ? (value as Ratings) : {};
      if (rows.length === 0) {
        return <p className="text-sm text-ink-3">Select something in {item.sourceItem || "the question above"} first; each choice gets a row here.</p>;
      }
      return (
        <ul className="divide-y divide-line border-y border-line">
          {rows.map((opt) => (
            <li key={opt} className="grid gap-3 py-4 sm:grid-cols-[1fr_auto]">
              <p className="text-sm text-ink">{opt}</p>
              <div className="flex flex-wrap gap-3">
                {dims.map((d) => (
                  <label key={d.id} className="flex flex-col gap-1">
                    <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">{d.label ?? d.id}</span>
                    <select
                      value={m[opt]?.[d.id] ?? ""}
                      onChange={(e) => {
                        const next: Ratings = { ...m, [opt]: { ...(m[opt] ?? {}) } };
                        if (e.target.value === "") delete next[opt][d.id];
                        else next[opt][d.id] = e.target.value;
                        onChange(next);
                      }}
                      className="border border-line-strong bg-canvas px-2 py-1.5 text-sm text-ink outline-none focus:border-accent"
                    >
                      <option value="">Choose</option>
                      {(d.options ?? []).map((o) => (
                        <option key={o} value={o}>
                          {o}
                        </option>
                      ))}
                    </select>
                  </label>
                ))}
              </div>
            </li>
          ))}
        </ul>
      );
    }
    default:
      return <p className="text-sm text-signal">This question type ({item.type}) cannot be shown. Please flag it.</p>;
  }
}
