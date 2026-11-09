"use client";

import { useActionState, useState } from "react";
import { useFormStatus } from "react-dom";

import { createAgentPasswordAction, type CreateState } from "./actions";

const initial: CreateState = {};

// The one button on the page. The password it returns is shown once,
// here, and the panel says so; reloading the page loses it, which is
// the point.
export function CreatePanel({ username }: { username: string }) {
  const [state, formAction] = useActionState(createAgentPasswordAction, initial);
  const [copied, setCopied] = useState(false);

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="border border-line-strong bg-paper-2 p-4">
      <form action={formAction} className="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
        <label className="block text-xs">
          <span className="font-mono uppercase tracking-[0.14em] text-ink-3">
            reason (optional)
          </span>
          <input
            type="text"
            name="reason"
            maxLength={1000}
            placeholder="e.g. live pass after the pilot deploy"
            className="mt-1 block w-full border border-line-strong bg-canvas px-2 py-1.5 font-mono text-sm text-ink outline-none focus:border-accent"
          />
        </label>
        <SubmitBtn />
      </form>

      {state.error ? (
        <p
          role="alert"
          className="mt-3 border-l-2 border-signal bg-signal-soft/50 px-3 py-2 text-xs text-ink"
        >
          {state.error}
        </p>
      ) : null}

      {state.plaintext ? (
        <div className="mt-4 border-t border-line pt-4">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            one-time password · shown once
          </p>
          <dl className="mt-2 grid gap-2 font-mono text-sm sm:grid-cols-[8rem_1fr]">
            <dt className="text-ink-3">username</dt>
            <dd className="m-0 text-ink">{state.username ?? username}</dd>
            <dt className="text-ink-3">password</dt>
            <dd className="m-0 flex flex-wrap items-center gap-3">
              <code className="select-all border border-line-strong bg-canvas px-2 py-1 text-base tracking-[0.08em] text-ink">
                {state.plaintext}
              </code>
              <button
                type="button"
                onClick={() => copy(state.plaintext ?? "")}
                className="inline-flex h-[30px] items-center border border-accent px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white"
              >
                {copied ? "Copied" : "Copy"}
              </button>
            </dd>
            <dt className="text-ink-3">expires</dt>
            <dd className="m-0 text-ink">
              {state.expiresAt
                ? new Date(state.expiresAt).toLocaleTimeString()
                : "in 15 minutes"}
            </dd>
          </dl>
          <p className="mt-3 text-xs text-ink-3">
            Good for one sign-in within 15 minutes. It is not stored
            anywhere readable; if it is lost, create another, which
            also revokes this one.
          </p>
        </div>
      ) : null}
    </div>
  );
}

function SubmitBtn() {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex h-[34px] items-center border border-accent bg-accent px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-white transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Creating…" : "Create one-time password"}
    </button>
  );
}
