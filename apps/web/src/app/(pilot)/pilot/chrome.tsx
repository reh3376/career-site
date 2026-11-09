import Link from "next/link";

import { signOutAction } from "@/app/actions/session";

import { PilotSteps, type StepProgress } from "./steps";

// The pilot's header: what this is, the four steps with the current
// one marked, who is signed in, and Sign out. No site menu, no social
// links, no assistant: a participant is offered the pilot and nothing
// else (owner, 2026-11-09).
export function PilotChrome({ name, admin, progress }: { name: string; admin: boolean; progress: StepProgress }) {
  const first = name ? name.split(" ")[0] : "";
  return (
    <header className="border-b border-line bg-paper">
      <div className="mx-auto flex max-w-4xl flex-wrap items-center justify-between gap-x-6 gap-y-3 px-6 py-4 sm:px-10">
        <div className="flex items-baseline gap-3">
          <Link href="/pilot" className="no-underline">
            <span className="font-display text-[20px] font-semibold leading-none tracking-tight text-ink">The pilot</span>
          </Link>
          <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">Roger Henley</span>
        </div>
        <div className="flex items-center gap-4">
          {admin ? (
            <Link href="/admin/pilot" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 no-underline hover:text-accent">
              admin · sessions
            </Link>
          ) : null}
          {first ? <span className="text-sm text-ink-2">{first}</span> : null}
          <form action={signOutAction} className="m-0">
            <button
              type="submit"
              className="inline-flex h-[30px] items-center border border-line-strong px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 transition-colors hover:border-accent hover:text-accent"
            >
              Sign out
            </button>
          </form>
        </div>
      </div>
      <div className="mx-auto max-w-4xl px-6 pb-4 sm:px-10">
        <PilotSteps progress={progress} />
      </div>
    </header>
  );
}
