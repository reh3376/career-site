"use client";

import { useState } from "react";
import { useFormStatus } from "react-dom";

import { upsertAccessGrantAction, type UpsertGrantResult } from "./actions";

// Add-or-edit form for a whitelist entry. Renders inline; on submit,
// posts through the server action and clears itself on success.
export function GrantForm({
  initial,
  submitLabel = "Add entry",
}: {
  initial?: {
    email?: string;
    defaultTtl?: string;
    notes?: string;
    entryExpiresAt?: string;
  };
  submitLabel?: string;
}) {
  const [error, setError] = useState<string | null>(null);
  const [flash, setFlash] = useState<string | null>(null);

  async function onSubmit(formData: FormData): Promise<void> {
    setError(null);
    setFlash(null);
    const res: UpsertGrantResult = await upsertAccessGrantAction(formData);
    if (res.ok) {
      setFlash(`Saved ${String(formData.get("email"))}.`);
    } else {
      setError(res.message);
    }
  }

  return (
    <form
      action={onSubmit}
      className="grid gap-3 border border-line-strong bg-canvas-2 p-4 sm:grid-cols-[1fr_140px_1fr_180px_auto] sm:items-end"
    >
      <label className="block text-xs">
        <span className="font-mono uppercase tracking-[0.14em] text-ink-3">
          email
        </span>
        <input
          type="email"
          name="email"
          required
          defaultValue={initial?.email ?? ""}
          placeholder="person@example.com"
          className="mt-1 block w-full border border-line-strong bg-canvas px-2 py-1.5 font-mono text-sm text-ink outline-none focus:border-accent"
        />
      </label>
      <label className="block text-xs">
        <span className="font-mono uppercase tracking-[0.14em] text-ink-3">
          ttl
        </span>
        <select
          name="default_ttl"
          defaultValue={initial?.defaultTtl ?? "GRANT_TTL_7D"}
          className="mt-1 block w-full border border-line-strong bg-canvas px-2 py-1.5 font-mono text-sm text-ink outline-none focus:border-accent"
        >
          <option value="GRANT_TTL_1D">1 day</option>
          <option value="GRANT_TTL_3D">3 days</option>
          <option value="GRANT_TTL_7D">7 days</option>
          <option value="GRANT_TTL_30D">30 days</option>
          <option value="GRANT_TTL_PERMANENT">Permanent</option>
        </select>
      </label>
      <label className="block text-xs">
        <span className="font-mono uppercase tracking-[0.14em] text-ink-3">
          notes
        </span>
        <input
          type="text"
          name="notes"
          defaultValue={initial?.notes ?? ""}
          placeholder="context, why whitelisted?"
          className="mt-1 block w-full border border-line-strong bg-canvas px-2 py-1.5 font-mono text-sm text-ink outline-none focus:border-accent"
        />
      </label>
      <label className="block text-xs">
        <span className="font-mono uppercase tracking-[0.14em] text-ink-3">
          entry expires (opt.)
        </span>
        <input
          type="date"
          name="entry_expires_at"
          defaultValue={initial?.entryExpiresAt ?? ""}
          className="mt-1 block w-full border border-line-strong bg-canvas px-2 py-1.5 font-mono text-sm text-ink outline-none focus:border-accent"
        />
      </label>
      <SubmitBtn label={submitLabel} />
      {error ? (
        <p className="col-span-full border-l-2 border-signal bg-signal-soft/50 px-3 py-2 text-xs text-ink">
          {error}
        </p>
      ) : null}
      {flash ? (
        <p className="col-span-full border-l-2 border-success bg-success-soft/50 px-3 py-2 text-xs text-ink">
          {flash}
        </p>
      ) : null}
    </form>
  );
}

function SubmitBtn({ label }: { label: string }) {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex h-[34px] items-center border border-accent bg-accent px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-white transition-colors hover:bg-accent-strong disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Saving…" : label}
    </button>
  );
}
