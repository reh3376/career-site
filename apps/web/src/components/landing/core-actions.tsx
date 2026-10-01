"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";

// The three things an account is for, with an explanation behind each.
//
// Roger's review of the live page: the two buttons were well placed,
// "Have a posting reviewed" did not say what the thing was, each needed
// a way to find out more, and Ask Roger was reachable only from a small
// button in a corner despite being a core function.
//
// So: three peers, each with a short label and a "What is this?" that
// opens a panel. The label says what it is and the panel says how it
// works, which is the split that lets the label stay short.
//
// The copy below is the same material as
// apps/web/content/other/career-site-guide.md, which is what the
// assistant answers site questions from. Two descriptions of one
// feature drift apart; if one changes, change both.

type ActionKey = "jd" | "meetings" | "ask";

type Action = {
  key: ActionKey;
  href: string;
  label: string;
  primary?: boolean;
  title: string;
  body: string[];
};

const ACTIONS: Action[] = [
  {
    key: "jd",
    href: "/jd-upload",
    label: "JD review",
    primary: true,
    title: "JD review",
    body: [
      "Paste a job description and it is read requirement by requirement against my records. Each requirement comes back met, partial or unmet, with the evidence behind the verdict so you can check the reasoning rather than take a number on trust.",
      "When the fit comes back strong or better it also writes a two-page résumé for that specific posting and returns it as a PDF. Every line of it is drawn from my actual records, with the sources recorded. Nothing is invented.",
      "There is a daily limit per account.",
    ],
  },
  {
    key: "meetings",
    href: "/meetings",
    label: "Book a meeting",
    title: "Booking time",
    body: [
      "Pick a length, 15, 30 or 45 minutes, then a day and a time. The slots come from my real calendar, so anything offered is genuinely free, and booking writes the meeting to it.",
      "Choose video or phone. For video you pick Google Meet, Microsoft Teams or Zoom and host the room. For phone, give the number you will call from.",
      "Both sides get an email with the details, and you can cancel your own booking from the same page. Times are shown in Eastern Time and follow daylight saving.",
    ],
  },
  {
    key: "ask",
    href: "/ask",
    label: "Ask Roger",
    title: "Ask Roger",
    body: [
      "An AI assistant that answers in my voice, from my own records, and shows you where each answer came from. Ask about my background, what I have built, or what I have written.",
      "Answers take a few tens of seconds. Everything runs on one small server and it reads my actual records before it says anything. Questions I have answered myself come back instantly.",
      "It will not discuss compensation, references, anything confidential to an employer, or my personal life. Those are better asked of me directly. Asked something my records do not cover, it says so rather than guessing.",
    ],
  },
];

// Where a button goes depends on who is pressing it.
//
// All three destinations are members-only. The first build showed the
// buttons to members only, which meant a hiring manager arriving cold
// saw none of the three things the site is for and learned Ask Roger
// existed from body copy further down. Roger's call: show everyone the
// three, and send a signed-out visitor through the login page to the
// thing they asked for rather than hiding it from them.
//
// `next` is what /ask and the other gated pages already redirect with,
// so a signed-out click lands on the destination after signing in
// instead of dumping the visitor on the home page.
function hrefFor(action: Action, signedIn: boolean): string {
  if (signedIn) return action.href;
  return `/login?next=${encodeURIComponent(action.href)}`;
}

export function CoreActions({ signedIn = false }: { signedIn?: boolean }) {
  const [open, setOpen] = useState<ActionKey | null>(null);
  const action = ACTIONS.find((a) => a.key === open) ?? null;

  return (
    <>
      {/* Three across where there is room, stacked where there is not.
          A wrapping row put two buttons on one line and one orphaned
          below, which reads as a ranking rather than three peers. */}
      <div className="mt-10 grid max-w-2xl gap-x-5 gap-y-4 sm:grid-cols-3">
        {ACTIONS.map((a) => (
          <div key={a.key} className="flex flex-col items-start gap-2">
            <Link
              href={hrefFor(a, signedIn)}
              className={
                a.primary
                  ? "inline-flex w-full items-center justify-center rounded-md bg-accent px-5 py-3 text-sm font-medium text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
                  : "inline-flex w-full items-center justify-center rounded-md border border-line bg-paper-2 px-5 py-3 text-sm font-medium text-ink no-underline transition-colors hover:border-accent hover:text-accent"
              }
            >
              {a.label}
            </Link>
            <button
              type="button"
              onClick={() => setOpen(a.key)}
              aria-haspopup="dialog"
              className="text-xs text-ink-3 underline decoration-line decoration-1 underline-offset-4 transition-colors hover:text-accent hover:decoration-accent"
            >
              What is this?
            </button>
          </div>
        ))}
      </div>

      {action ? (
        <AboutPanel
          action={action}
          signedIn={signedIn}
          onClose={() => setOpen(null)}
        />
      ) : null}
    </>
  );
}

function AboutPanel({
  action,
  signedIn,
  onClose,
}: {
  action: Action;
  signedIn: boolean;
  onClose: () => void;
}) {
  const panelRef = useRef<HTMLDivElement>(null);
  const returnFocusTo = useRef<HTMLElement | null>(null);

  // Remember what opened this and give focus back on close, so a
  // keyboard reader is returned to the button they pressed rather than
  // to the top of the page.
  useEffect(() => {
    returnFocusTo.current = document.activeElement as HTMLElement | null;
    panelRef.current?.focus();
    return () => returnFocusTo.current?.focus();
  }, []);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="about-action-title"
      className="fixed inset-0 z-50 flex items-end justify-center bg-ink/70 p-4 backdrop-blur-sm sm:items-center"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        tabIndex={-1}
        className="max-h-[85vh] w-full max-w-lg overflow-y-auto border border-line-strong bg-canvas p-6 shadow-2xl outline-none"
      >
        <div className="flex items-start justify-between gap-4">
          <h2
            id="about-action-title"
            className="font-display text-xl leading-tight text-ink"
          >
            {action.title}
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="shrink-0 font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase underline underline-offset-4 transition-colors hover:text-accent"
          >
            Close
          </button>
        </div>

        <div className="mt-4 space-y-3">
          {action.body.map((p) => (
            <p key={p.slice(0, 24)} className="text-sm leading-relaxed text-ink-2">
              {p}
            </p>
          ))}
        </div>

        <Link
          href={hrefFor(action, signedIn)}
          className="mt-5 inline-flex items-center rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white no-underline transition-colors hover:bg-accent-hover"
        >
          {signedIn ? action.label : `Sign in for ${action.label.toLowerCase()}`}
        </Link>
      </div>
    </div>
  );
}

// The same three actions in the HMI skin.
//
// OT mode had no signed-in branch at all, so a member there was offered
// the access panel an anonymous visitor sees and no route to any of the
// three things their account is for. Same gap Roger found in the
// editorial mode, in the other skin.
//
// Drawn as a status panel rather than as buttons, because that is the
// idiom of this mode: a row per function, its state, and a way to read
// what it does. The explanations are the same text, from the same
// array, so the two modes cannot describe the site differently.
export function CoreActionsOt({ signedIn = false }: { signedIn?: boolean }) {
  const [open, setOpen] = useState<ActionKey | null>(null);
  const action = ACTIONS.find((a) => a.key === open) ?? null;

  return (
    <>
      <div className="grid gap-px border border-line-strong bg-line-strong sm:grid-cols-3">
        {ACTIONS.map((a) => (
          <div key={a.key} className="bg-paper-2 p-4">
            <Link
              href={hrefFor(a, signedIn)}
              className="font-mono text-[13px] tracking-[0.08em] text-accent uppercase no-underline hover:underline"
            >
              {a.label}
            </Link>
            {/* Signed out the row still reads ONLINE, because it is: the
                gate is the account, not the service. The qualifier says
                which. */}
            <p className="mt-1 font-mono text-[10px] tracking-[0.14em] text-success uppercase">
              online
              {signedIn ? null : (
                <span className="text-ink-3"> · members</span>
              )}
            </p>
            <button
              type="button"
              onClick={() => setOpen(a.key)}
              aria-haspopup="dialog"
              className="mt-2 font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase underline underline-offset-4 transition-colors hover:text-accent"
            >
              What is this?
            </button>
          </div>
        ))}
      </div>

      {action ? (
        <AboutPanel
          action={action}
          signedIn={signedIn}
          onClose={() => setOpen(null)}
        />
      ) : null}
    </>
  );
}
