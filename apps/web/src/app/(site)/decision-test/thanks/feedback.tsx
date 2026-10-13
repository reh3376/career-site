"use client";

import { useState, useSyncExternalStore } from "react";

import { Turnstile } from "@/components/turnstile";

import { submitFeedbackAction } from "./actions";

// Feedback, written while the fifteen minutes are still fresh.
//
// Why here and not in a LinkedIn comment, which was the alternative:
// a participant describing the test in public primes every later
// reader, and the instrument cannot survive that. A private box is not
// just more convenient, it protects the measurement.
//
// Why not a link to /contact: that form requires a name and an email
// from anyone not signed in, and the paragraph directly above this one
// has just told the participant the measurement does not need their
// name. Sending them from that sentence to a form demanding both is a
// contradiction they would be right to notice.
//
// Everything here is optional, including the feedback itself. The
// participant has already given the thing that was actually needed.

type Who = { name?: string; email?: string };

// What the briefing stashed, and the run the run screen just finished.
//
// Read through useSyncExternalStore rather than an effect, which is
// both what this actually is (a snapshot of something outside React)
// and what the lint rules here allow. The snapshot is cached because
// React calls the getter on every render and must get the same object
// back, and because nothing rewrites these keys while this page is open.
type Handoff = { who: Who; run: string };

const EMPTY: Handoff = { who: {}, run: "" };
let cached: Handoff | null = null;

function snapshot(): Handoff {
  if (cached === null) {
    let who: Who = {};
    let run = "";
    try {
      const raw = sessionStorage.getItem("dt-who");
      if (raw) who = JSON.parse(raw) as Who;
      run = sessionStorage.getItem("dt-done") ?? "";
    } catch {
      // A private window throws. An empty form that still sends is a
      // better outcome than no form at all.
    }
    cached = { who, run };
  }
  return cached;
}

// Nothing ever changes, so there is nothing to subscribe to.
const subscribe = () => () => {};
const serverSnapshot = () => EMPTY;

const MAX = 5000;

export function Feedback({ turnstileSiteKey }: { turnstileSiteKey: string }) {
  const { who, run } = useSyncExternalStore(subscribe, snapshot, serverSnapshot);

  // null means "not edited", so the prefill shows through without the
  // fields being state derived from a snapshot, which is the pattern
  // that needs an effect to stay in sync and then drifts.
  const [nameEdit, setNameEdit] = useState<string | null>(null);
  const [emailEdit, setEmailEdit] = useState<string | null>(null);
  const [message, setMessage] = useState("");
  const [token, setToken] = useState("");
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");

  const name = nameEdit ?? who.name ?? "";
  const email = emailEdit ?? who.email ?? "";

  if (sent) {
    return (
      <div className="mt-12 border-l-2 border-success bg-success-soft/40 px-4 py-3 text-sm text-ink">
        Thank you, that is genuinely useful. Nothing else is needed from
        you.
      </div>
    );
  }

  const tooLong = message.length > MAX;

  async function send() {
    setError("");
    if (!message.trim()) {
      setError("There is nothing in the box yet.");
      return;
    }
    if (tooLong) {
      setError(`That is ${message.length} characters, and the limit is ${MAX}.`);
      return;
    }
    setSending(true);
    try {
      const res = await submitFeedbackAction(message, name, email, run, token);
      if (res.error) {
        setError(res.error);
        return;
      }
      setSent(true);
      // The handoff has done its job. Clearing it means a reload cannot
      // send the same note twice with the same run attached.
      try {
        sessionStorage.removeItem("dt-who");
        sessionStorage.removeItem("dt-done");
        cached = { who: {}, run: "" };
      } catch {
        // Nothing to clear.
      }
    } catch {
      // Said plainly, because the participant has already given fifteen
      // minutes and a silent failure here would be the second time the
      // site took something and showed nothing for it.
      setError("That did not send. Nothing was lost, so try again.");
    } finally {
      setSending(false);
    }
  }

  return (
    <section className="mt-16 border-t border-line pt-10">
      <h2
        className="font-display text-2xl leading-tight text-ink"
        style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}
      >
        Anything you want to tell me?
      </h2>
      <div className="mt-4 space-y-4 text-base leading-relaxed text-ink-2">
        <p>
          How it felt, what was confusing, what broke, whether a question
          seemed unfair. It is most useful now, while it is fresh, and it
          goes to me rather than into a public comment, so it cannot give
          anything away to whoever takes this next.
        </p>
      </div>

      <div className="mt-6 space-y-5">
        <div>
          <label
            htmlFor="dt-feedback"
            className="mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
          >
            Your note
          </label>
          <textarea
            id="dt-feedback"
            rows={6}
            value={message}
            maxLength={MAX + 1}
            onChange={(e) => setMessage(e.target.value)}
            className="block w-full resize-y border border-line bg-canvas px-3 py-2.5 text-base text-ink outline-none transition-colors placeholder:text-ink-4 focus:border-accent"
          />
        </div>

        {/* Prefilled from what they typed at the start, and editable,
            because somebody who gave an address for their results may
            not want this note attached to it, and the reverse. */}
        <div className="grid gap-5 sm:grid-cols-2">
          <div>
            <label
              htmlFor="dt-feedback-name"
              className="mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
            >
              Name, optional
            </label>
            <input
              id="dt-feedback-name"
              type="text"
              value={name}
              onChange={(e) => setNameEdit(e.target.value)}
              className="block w-full border border-line bg-canvas px-3 py-2.5 text-base text-ink outline-none transition-colors focus:border-accent"
            />
          </div>
          <div>
            <label
              htmlFor="dt-feedback-email"
              className="mb-2 block font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
            >
              Email, optional
            </label>
            <input
              id="dt-feedback-email"
              type="email"
              value={email}
              onChange={(e) => setEmailEdit(e.target.value)}
              className="block w-full border border-line bg-canvas px-3 py-2.5 text-base text-ink outline-none transition-colors focus:border-accent"
            />
            <p className="mt-2 text-xs text-ink-3">
              Only so I can reply. Leave it blank and the note still
              arrives.
            </p>
          </div>
        </div>

        <Turnstile
          siteKey={turnstileSiteKey}
          action="decision-test-feedback"
          resetOn={error}
          onToken={setToken}
        />

        {error ? (
          <p
            role="alert"
            className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink"
          >
            {error}
          </p>
        ) : null}

        <button
          type="button"
          onClick={send}
          disabled={sending}
          className="inline-flex items-center justify-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-60"
        >
          {sending ? "Sending…" : "Send it"}
        </button>
      </div>
    </section>
  );
}
