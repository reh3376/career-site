import type { Metadata } from "next";
import Link from "next/link";
import { notFound, redirect } from "next/navigation";

import { getSessionUser } from "@/lib/session-user";

import { FinishButton } from "./finish-button";
import { ConsentForm } from "./consent-form";
import { getPilot, progressOf } from "./lib";

export const metadata: Metadata = {
  title: "Pilot",
  description: "The employer-side pilot: a short survey, a mocked match record and a hiring workflow to comment on.",
};
export const dynamic = "force-dynamic";

// The pilot home (FSD §5.16). Members only, and only members with the
// pilot flag or the admin role: anyone else gets the same 404 a
// non-existent page gives, so the pilot's existence is not a thing a
// member without it can discover from here.
export default async function PilotHomePage() {
  const me = await getSessionUser();
  if (!me) redirect("/login?next=/pilot");
  const res = await getPilot();
  if (!res.ok) {
    if (res.status === 403) notFound();
    return <Shell><ErrorNote message={res.message} /></Shell>;
  }
  if (!res.data.allowed) notFound();

  const session = res.data.session;
  if (!session) {
    return (
      <Shell>
        <Consent />
      </Shell>
    );
  }
  const p = progressOf(session);
  // The one thing to do now, as a single button. The map of the three
  // parts follows for orientation; the button is the way through.
  const go = p.finished
    ? { href: "/pilot/done", label: "What happens next", hint: "You finished. Thank you." }
    : !p.surveyDone
      ? {
          href: p.surveyHref,
          label: session.sectionsDone?.length ? "Continue the survey" : "Start the survey",
          hint: "Part 1 of 3, about ten minutes.",
        }
      : !p.recordDone
        ? { href: "/pilot/record", label: "Read the match record", hint: "Part 2 of 3, a one-page read with comment boxes." }
        : !p.workflowDone
          ? { href: "/pilot/workflow", label: "Read the workflow", hint: "Part 3 of 3, five stages with one click per step." }
          : null;
  return (
    <Shell>
      <p className="mt-6 max-w-2xl text-lg leading-relaxed text-ink-2">
        Three parts, in order: the survey, then a match record to read, then
        a picture of how a company hires, one stage at a time. Leave and come back whenever you
        like; everything saves as you go.
      </p>

      {go ? (
        <div className="mt-8 flex flex-wrap items-center gap-4 border-l-2 border-accent bg-accent/10 px-5 py-4">
          <Link
            href={go.href}
            className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
          >
            {go.label}
          </Link>
          <span className="text-sm text-ink-2">{go.hint}</span>
        </div>
      ) : null}

      <ol className="mt-10 divide-y divide-line border-y border-line">
        <Part
          n={1}
          title="The survey"
          body={"About ten minutes. Every question has a box for \"this was unclear or didn't fit me\"; tick it freely, that is the point of the pilot."}
          href={p.surveyDone ? "/pilot/survey/A" : p.surveyHref}
          cta={p.surveyDone ? "Review your answers" : session.sectionsDone?.length ? "Continue the survey" : "Start the survey"}
          done={p.surveyDone}
          next={!p.surveyDone}
        />
        <Part
          n={2}
          title="A match record"
          body="One candidate against one posting, ten requirements, the way a screener would see it. Read it as if it arrived at work and leave a comment under any row."
          href="/pilot/record"
          cta={p.recordDone ? "Read it again" : "Read the record"}
          done={p.recordDone}
          next={p.surveyDone && !p.recordDone}
        />
        <Part
          n={3}
          title="How a company hires"
          body="A draft of the whole process, from the first sign a role is needed to the lessons captured when it closes. Tell us where it matches your organisation and where it does not."
          href="/pilot/workflow"
          cta={p.workflowDone ? "Read it again" : "Read the workflow"}
          done={p.workflowDone}
          next={p.surveyDone && p.recordDone && !p.workflowDone}
        />
      </ol>

      <div className="mt-10">
        {p.finished ? (
          <p className="text-sm text-ink-2">
            You finished on {fmtDate(session.finishedAt)}. Thank you.{" "}
            <Link href="/pilot/done" className="text-accent underline decoration-accent/40 underline-offset-4 hover:decoration-accent">
              What happens next
            </Link>
            .
          </p>
        ) : p.surveyDone && p.recordDone && p.workflowDone ? (
          <FinishButton />
        ) : (
          <p className="text-sm text-ink-3">
            {!p.surveyDone
              ? "Finish the survey, then read the record and the workflow; the session closes after all three."
              : !p.recordDone
                ? "Next: read the match record. The session closes after all three parts."
                : "Next: read the workflow. The session closes after all three parts."}
          </p>
        )}
      </div>
    </Shell>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">the pilot</p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        A survey about hiring, and two things to read.
      </h1>
      {children}
    </div>
  );
}

function ErrorNote({ message }: { message: string }) {
  return (
    <p className="mt-6 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
      The pilot could not be loaded, {message}. Try again in a moment.
    </p>
  );
}

// The consent screen (docs/framework-research/11-pilot-materials.md §1).
function Consent() {
  return (
    <div className="mt-8 max-w-2xl space-y-5 text-base leading-relaxed text-ink-2">
      <p>Run by Roger Henley, independently. No company is behind this.</p>
      <p>
        <strong className="text-ink">What this is for.</strong> I am designing software that screens job
        applications by evidence against each requirement rather than by keywords. Before I ask fifty
        recruiters and hiring managers which hiring problems matter most, I need ten people to try the
        survey and tell me where its wording fails. That is what you are being asked to do.
      </p>
      <p>
        <strong className="text-ink">What I am asking of you.</strong> About ten minutes on the survey here,
        then a short read of a one-page match record and a picture of how a company hires, in five stages, with a
        comment box under every row and every step. Afterwards, a call of about thirty minutes: fifteen
        about the survey&rsquo;s wording, the rest about what you wrote. Not what you think of the idea; what
        the questions did to you, and what the picture of hiring gets wrong.
      </p>
      <p>
        <strong className="text-ink">What is kept.</strong> Your answers and comments, under your account
        and never on a public page. Notes from the call, with quotes marked as quotes. The call is not
        recorded unless you tick the box below. Nothing here asks about any candidate or any named person.
      </p>
      <p>
        <strong className="text-ink">What you get.</strong> The summarised results when they exist, and a
        short list of what changed because of you.
      </p>
      <div className="pt-2">
        <ConsentForm />
      </div>
    </div>
  );
}

function Part({
  n,
  title,
  body,
  href,
  cta,
  done,
  next,
}: {
  n: number;
  title: string;
  body: string;
  href: string;
  cta: string;
  done: boolean;
  // The part the participant should do now; one row carries it.
  next?: boolean;
}) {
  return (
    <li className="grid gap-3 py-6 sm:grid-cols-[3rem_1fr_auto] sm:items-start">
      <span className={`font-display text-3xl leading-none ${next ? "text-accent" : "text-ink-3"}`}>{n}</span>
      <div>
        <p className="font-display text-xl leading-snug text-ink" style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}>
          {title}
          {done ? <span className="ml-3 font-mono text-[11px] uppercase tracking-[0.14em] text-success">done</span> : null}
          {next ? <span className="ml-3 font-mono text-[11px] uppercase tracking-[0.14em] text-accent">next</span> : null}
        </p>
        <p className="mt-2 text-sm leading-relaxed text-ink-2">{body}</p>
      </div>
      <Link
        href={href}
        className={
          next
            ? "inline-flex items-center justify-center rounded-md bg-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
            : "inline-flex items-center justify-center rounded-md border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent no-underline transition-colors hover:bg-accent hover:text-white"
        }
      >
        {cta}
      </Link>
    </li>
  );
}

function fmtDate(iso?: string): string {
  if (!iso) return "today";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "today" : d.toISOString().slice(0, 10);
}
