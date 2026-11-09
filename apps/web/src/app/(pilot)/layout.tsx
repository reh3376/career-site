import type { ReactNode } from "react";
import { notFound, redirect } from "next/navigation";

import { canPilot, getSessionUser, isAdmin } from "@/lib/session-user";

import { PilotChrome } from "./pilot/chrome";
import { getPilot, progressOf } from "./pilot/lib";

// The pilot's own chrome (FSD FR-PLT-18). A participant is here for
// one thing, and the owner's first run as one (2026-11-09) showed the
// site's menu, links and assistant around it made the three parts hard
// to find and easy to leave half done. So the pilot sits outside the
// (site) route group, as the decision test does, and draws a header
// of its own: the steps, where the participant is, and a way out.
// Nothing else. The owner, as admin, sees the same chrome plus a link
// back to the admin pilot list.
//
// Signed out: to the login with the pilot as the destination. Signed
// in without the flag or the admin role: the same 404 a missing page
// gives, so the pilot's existence is not something a member can learn
// from here; every page and RPC re-checks this.
export default async function PilotLayout({ children }: { children: ReactNode }) {
  const me = await getSessionUser();
  if (!me) redirect("/login?next=/pilot");
  if (!canPilot(me)) notFound();
  const res = await getPilot();
  if (!res.ok && res.status === 403) notFound();
  const session = res.ok ? res.data.session : undefined;
  const p = progressOf(session);
  return (
    <>
      <PilotChrome
        name={me.name}
        admin={isAdmin(me)}
        progress={{
          started: !!session,
          surveyDone: p.surveyDone,
          surveyHref: p.surveyHref,
          recordDone: p.recordDone,
          workflowDone: p.workflowDone,
          finished: p.finished,
        }}
      />
      <main className="flex-1">{children}</main>
      <footer className="border-t border-line bg-paper">
        <div className="mx-auto flex max-w-4xl flex-wrap items-center justify-between gap-3 px-6 py-6 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 sm:px-10">
          <span>The pilot · run by Roger Henley, independently</span>
          <span>Your answers are kept under your account and never on a public page.</span>
        </div>
      </footer>
    </>
  );
}
