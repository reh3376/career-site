"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";

import { decisionTestClient } from "@/lib/decision-test-client";
import { createMetronome, type Signal } from "@/lib/metronome";
import { beaconPending, enqueueRetry, flushNow } from "@/lib/decision-test-retry";

// The run screen.
//
// Deliberately sparse: one question, the options, the confidence
// control, and a counter. No navigation, no links away, no clock.
//
// There is no countdown on screen. The tick already makes time salient
// and each phase is bounded, so a visible clock would stack a second,
// uncontrolled pressure on top of the thing being measured. The counter
// is a question count and never a time remaining, and never a block
// count: "block 3 of 5" would leak the structure, and a participant who
// works out there are five blocks may infer the last one is easier,
// which is precisely the block the fatigue control depends on being met
// naively.
//
// Nothing here ever says whether an answer was right. The browser is
// never told: grading happens in the api and the response carries no
// verdict, so this screen could not leak it even by mistake.

// The timings come from the server, not from here.
//
// They were constants until the owner had taken the test twice and moved
// them both times, each move costing a build and a deploy to change one
// number. They now live in app_settings, are editable at
// /admin/decision-test, and are handed over with the session, so a run
// cannot drift from the settings it started under. The fallbacks below
// only matter if the handoff is somehow incomplete.
const FALLBACK = { memoriseMs: 6_000, questionMs: 25_000, recallMs: 20_000 };

/** Five levels, because a slider costs seconds this budget does not have. */
const CONFIDENCE = [
  { label: "Guessing", value: 10 },
  { label: "Doubtful", value: 30 },
  { label: "Unsure", value: 50 },
  { label: "Fairly sure", value: 70 },
  { label: "Certain", value: 90 },
];

type Question = {
  code: string;
  version: number;
  prompt: string;
  reminder: string;
  options: string[];
  positionOverall: number;
};
type Block = {
  blockNo: number;
  load: string;
  digits: string;
  transform: string;
  questions: Question[];
};
type Phase = "memorise" | "question" | "confidence" | "recall" | "debrief" | "finishing";

type Timings = { memoriseMs: number; questionMs: number; recallMs: number };
type Handoff = {
  sessionKey: string;
  audioMode: string;
  practice: Block;
  questionCount: number;
  timings?: Timings;
  // Set by ?synthetic=1 on the briefing. Carried through the handoff so
  // the run screen can keep saying so, rather than the warning
  // disappearing at the navigation and leaving somebody fifteen minutes
  // into a run they think counts.
  synthetic?: boolean;
};

/**
 * Reads the session the briefing handed over.
 *
 * sessionStorage rather than the URL: a session key in the address bar
 * would survive in history, be shareable, and appear in a screenshot.
 * sessionStorage dies with the tab, which is also the lifetime of a run
 * that cannot be resumed.
 *
 * A lazy initialiser rather than an effect, which is safe because this
 * component never renders on the server (see page.tsx).
 */
function readHandoff(): Handoff | null {
  try {
    const raw = sessionStorage.getItem("dt");
    return raw ? (JSON.parse(raw) as Handoff) : null;
  } catch {
    return null;
  }
}

export function RunScreen() {
  const router = useRouter();
  const [handoff] = useState<Handoff | null>(readHandoff);
  const sessionKey = handoff?.sessionKey ?? "";
  const audioMode = handoff?.audioMode ?? "sound";
  const questionCount = handoff?.questionCount ?? 30;
  const timings = handoff?.timings ?? FALLBACK;
  const practice: Block = handoff?.practice ?? {
    blockNo: 0,
    load: "practice",
    digits: "",
    transform: "none",
    questions: [],
  };
  const [block, setBlock] = useState<Block>(practice);
  const [blockNo, setBlockNo] = useState(0); // 0 is the unscored practice block
  const [qi, setQi] = useState(0);
  const [phase, setPhase] = useState<Phase>("memorise");
  const [chosen, setChosen] = useState<number | null>(null);
  const [recall, setRecall] = useState("");
  const [beat, setBeat] = useState(false);

  const shownAt = useRef(0);
  // When an option was picked, as distinct from when the answer was
  // submitted. Only used on the timeout path: an answer recovered
  // because the clock ran out during the rating step must not be
  // credited with the seconds the participant spent not rating it.
  const chosenAt = useRef(0);
  const signal = useRef<Signal | null>(null);
  const question = block.questions[qi];
  const scored = blockNo > 0;

  // The metronome runs for the whole test, in both modes. Visual mode
  // keeps the same clock with the sound muted, so the heartbeat beats
  // where the tick would have.
  useEffect(() => {
    const m = createMetronome(audioMode === "sound");
    signal.current = m;
    m.onTick(() => {
      setBeat(true);
      window.setTimeout(() => setBeat(false), 140);
    });
    void m.start();
    return () => m.stop();
  }, [audioMode]);

  // Arriving here directly, with no session, means the briefing was
  // skipped. Send them to it rather than starting a test they have not
  // been warned about.
  useEffect(() => {
    if (!handoff) router.replace("/decision-test");
  }, [handoff, router]);

  // Coming back to the tab is the moment to retry anything that failed.
  //
  // A phone that was switched away from has had its timers throttled or
  // stopped outright, so a queued answer may have been sitting still
  // for minutes. Returning is both the earliest the network is likely
  // to be working again and the last chance before the tab is closed.
  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState === "visible") flushNow();
    };
    // pagehide rather than beforeunload: it fires when a phone moves the
    // page into the back/forward cache, which is exactly what happens
    // when somebody switches apps mid-run and never comes back, and
    // beforeunload does not fire reliably on mobile at all.
    //
    // Anything still queued at this point would otherwise be lost with
    // the tab. A beacon is handed to the browser and survives the page.
    const onHide = () => beaconPending();
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("pagehide", onHide);
    return () => {
      document.removeEventListener("visibilitychange", onVisible);
      window.removeEventListener("pagehide", onHide);
    };
  }, []);

  // Leaving loses the run: there is no resume, because a question
  // answered after a four-minute interruption is not the same question
  // and nothing in the data would say so. This is the one browser nag
  // that earns its place, since the accident costs somebody ten minutes.
  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, []);

  const nextBlock = useCallback(
    async (n: number) => {
      if (n > 5) {
        // The run is closed BEFORE the debrief question, not after.
        //
        // Thirty answered questions are already stored by this point,
        // and if the participant closes the tab on the debrief the
        // session would otherwise sit at `running` for ever: a complete
        // run labelled as abandoned, which is worse than a complete run
        // with an unknown strategy. finishSession is an update of status
        // and strategy, so answering calls it again and fills in the
        // answer.
        setPhase("debrief");
        try {
          await decisionTestClient.finishSession({ sessionKey, recallStrategy: "" });
        } catch {
          /* the session stays abandoned, which is kept rather than lost */
        }
        return;
      }
      const res = await decisionTestClient.getBlock({ sessionKey, blockNo: n });
      const b = res.block;
      if (!b) return;
      setBlock({
        blockNo: b.blockNo,
        load: b.load,
        digits: b.digits,
        transform: b.transform,
        questions: b.questions.map((q) => ({
          code: q.code,
          version: q.version,
          prompt: q.prompt,
          reminder: q.reminder,
          options: q.options,
          positionOverall: q.positionOverall,
        })),
      });
      setBlockNo(n);
      setQi(0);
      setRecall("");
      setPhase("memorise");
    },
    // No router here any more: the redirect moved to answerDebrief when
    // the debrief step was added between the last recall and finishing.
    [sessionKey],
  );

  const advance = useCallback(
    async (chosenIndex: number, confidence: number, latencyOverride?: number) => {
      // latencyOverride exists for one case: the clock ran out while the
      // participant was on the rating step, so the answer is kept but
      // the time it took is the time to the CHOICE, not the time to the
      // timeout. Without it every recovered answer would be recorded at
      // exactly the question budget and look like deliberation it never
      // had.
      const latency = latencyOverride ?? Math.round(performance.now() - shownAt.current);
      if (scored && question) {
        const payload = {
          sessionKey,
          blockNo,
          positionOverall: question.positionOverall,
          chosenIndex,
          latencyMs: chosenIndex < 0 ? 0 : latency,
          confidence: chosenIndex < 0 ? 0 : confidence,
        };
        const send = () => decisionTestClient.submitAnswer(payload);
        try {
          await send();
        } catch {
          // A failed post must not strand the participant mid-test, and
          // must not quietly destroy the answer either. This used to do
          // the first by doing the second: the comment said "the row is
          // lost; the run is not", and one dropped request meant one
          // question that nobody could tell from a question never
          // reached. A participant on a phone lost answers that way.
          //
          // So the test carries on immediately, as before, and the
          // write is retried in the background. The insert is
          // idempotent on (session, position), so a retry that
          // duplicates a request which did land changes nothing.
          enqueueRetry(`answer ${question.positionOverall}`, send, {
            // The same request as data, so it can still be delivered by
            // the browser if the tab closes before a retry lands.
            path: "/api/career.v1.DecisionTestService/SubmitAnswer",
            body: payload,
          });
        }
      }
      setChosen(null);
      if (qi + 1 < block.questions.length) {
        setQi(qi + 1);
        setPhase("question");
        shownAt.current = performance.now();
      } else {
        setPhase(scored ? "recall" : "finishing");
        shownAt.current = performance.now();
        if (!scored) void nextBlock(1);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [block, qi, blockNo, question, scored, sessionKey],
  );

  const submitRecall = useCallback(async () => {
    const latency = Math.round(performance.now() - shownAt.current);
    const payload = { sessionKey, blockNo, digits: recall, latencyMs: latency };
    const send = () => decisionTestClient.submitRecall(payload);
    try {
      await send();
    } catch {
      enqueueRetry(`recall block ${blockNo}`, send, {
        path: "/api/career.v1.DecisionTestService/SubmitRecall",
        body: payload,
      });
      /* a missed recall is a data point, not a gate */
    }
    void nextBlock(blockNo + 1);
  }, [blockNo, recall, sessionKey, nextBlock]);

  // Phase timers. A question that runs out moves on: no going back, no
  // pause, no explanation, and it is stored as expired rather than as
  // an error or a dropped row, because a timeout under load is a result.
  useEffect(() => {
    if (phase === "memorise") {
      const t = window.setTimeout(() => {
        setPhase("question");
        shownAt.current = performance.now();
      }, timings.memoriseMs);
      return () => window.clearTimeout(t);
    }
    if (phase === "question" || phase === "confidence") {
      const left = timings.questionMs - (performance.now() - shownAt.current);
      const t = window.setTimeout(() => {
        // An answer that was given is kept.
        //
        // This budget spans the question AND the rating that follows
        // it, and it used to fire `advance(-1, 0)` unconditionally.
        // So a participant who read the question, chose an option, and
        // was then slow on the five confidence buttons had their choice
        // thrown away and stored as `expired`, indistinguishable from
        // never having answered at all.
        //
        // That is not a neutral loss. The answers it destroys are the
        // ones where somebody deliberated, which are exactly the
        // answers the instrument is about, and it falls hardest on a
        // phone where the rating targets are smaller. It took eight of
        // one participant's answers on 2026-10-07 and was found by
        // driving the page: choose an option, wait, and watch the row
        // come back expired with a null chosen_index.
        //
        // Expiry now means what it says: nothing was chosen.
        if (chosen !== null) {
          void advance(chosen, 0, Math.round(chosenAt.current - shownAt.current));
        } else {
          void advance(-1, 0);
        }
      }, Math.max(0, left));
      return () => window.clearTimeout(t);
    }
    if (phase === "recall") {
      const timer = window.setTimeout(() => void submitRecall(), timings.recallMs);
      return () => window.clearTimeout(timer);
    }
    return;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase, qi, blockNo]);

  // The debrief question (FSD 3.1).
  //
  // A participant told to recall each digit plus one can do it two ways,
  // and they load the questions differently: convert at encoding and you
  // carry an ordinary four-digit number, which is block 2's load; hold
  // the raw digits and convert at the end and you carry a pending
  // operation through every question, which is not. Nobody is told which
  // to use and people pick differently, so without asking, blocks 3 and
  // 4 contain a mixture of two conditions with no way to separate them.
  //
  // Asking turns an uncontrolled variable into a recorded one. It is
  // also what makes "blocks 3 and 4 look like block 2" diagnosable
  // rather than merely disappointing, which the roadmap lists as one of
  // three reasons to stop and re-plan.
  //
  // Asked here rather than in the email, because an email reaches only
  // the participants who left an address and this has to be answerable
  // by everyone.
  const answerDebrief = useCallback(
    async (strategy: string) => {
      setPhase("finishing");
      try {
        await decisionTestClient.finishSession({ sessionKey, recallStrategy: strategy });
      } catch (e) {
        // Logged rather than swallowed. This catch used to read "the run
        // is already closed; only the strategy is lost", which was both
        // the cause and the cover: the api refused this call for every
        // participant because the run screen closes the run before
        // asking, and the comment made the refusal sound expected. The
        // field was empty on every browser run and nothing anywhere
        // said so.
        //
        // The participant is still sent on. Thirty answers are already
        // stored and a lost debrief answer is not worth holding
        // somebody on a spinner for. But it is now visible to anyone
        // with a console open, which is the live pass this surface
        // needed and did not have.
        console.error("decision test: the debrief answer was not stored", e);
      }
      sessionStorage.removeItem("dt");
      router.replace("/decision-test/thanks");
    },
    [router, sessionKey],
  );

  // The transformation, and an example of it applied to this block's
  // own digits.
  //
  // The first live run returned the raw number in both transformation
  // blocks, having never registered that a rule had appeared. The
  // instruction was on screen the whole time, which is the point: it
  // read as a small grey line saying "give it back exactly as shown"
  // for practice, block 1 and block 2, so by block 3 it had taught the
  // participant that it never changes. Habituation, not absence.
  //
  // So a block that transforms now says so as the loudest thing on the
  // screen, and shows the answer worked out on the very digits in front
  // of them. A rule stated in the abstract is a rule to be applied
  // later; a rule shown applied is one that has already been understood.
  const transform = useMemo(() => {
    const add = block.transform === "plus1" ? 1 : block.transform === "plus3" ? 3 : 0;
    if (!add) return null;
    const example = block.digits
      .split("")
      .map((d) => String((Number(d) + add) % 10))
      .join("");
    return { add, example };
  }, [block.transform, block.digits]);

  if (!handoff) return null;

  return (
    <div className="flex min-h-screen flex-col bg-paper text-ink">
      <header className="flex items-center justify-between px-6 py-5 sm:px-10">
        <span className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
          {scored && question
            ? `${question.positionOverall} of ${questionCount}`
            : "practice"}
        </span>
        {/* A label in the chrome rather than a banner in the page. The
            run screen is a timed instrument and the participant is
            reading the middle of it under load, so the warning goes
            where it cannot move a prompt or an option by a pixel. It
            stays on screen for the whole run. */}
        {handoff.synthetic ? (
          <span className="font-mono text-[11px] tracking-[0.14em] text-signal uppercase">
            synthetic, not counted
          </span>
        ) : null}
        {/* The visual beat, for a participant who cannot use sound. It
            runs on the same clock as the tick rather than its own. */}
        {audioMode === "visual" ? (
          <span
            aria-hidden
            className={`size-3 rounded-full transition-opacity duration-100 ${
              beat ? "bg-accent opacity-100" : "bg-ink-4 opacity-30"
            }`}
          />
        ) : null}
      </header>

      <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col justify-center px-6 pb-24 sm:px-10">
        {phase === "memorise" ? (
          <div className="text-center">
            {transform ? (
              <p className="mx-auto max-w-md rounded-md border border-accent bg-accent-soft/60 px-5 py-3 text-base font-semibold text-accent">
                Add {transform.add} to every digit before you give it back.
              </p>
            ) : (
              <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
                Hold this number
              </p>
            )}
            <p className="font-display mt-6 text-7xl tracking-[0.2em] text-ink tabular-nums">
              {block.digits}
            </p>
            {transform ? (
              <p className="mt-6 text-base text-ink-2">
                So you will type{" "}
                <span className="font-display tracking-[0.1em] text-ink tabular-nums">
                  {transform.example}
                </span>
                . Digits wrap, so 9 plus {transform.add} is{" "}
                {(9 + transform.add) % 10}.
              </p>
            ) : (
              <p className="mt-6 text-sm text-ink-2">Give it back exactly as shown.</p>
            )}
          </div>
        ) : null}

        {phase === "question" && question ? (
          <div>
            {question.reminder ? (
              <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
                {question.reminder}
              </p>
            ) : null}
            <p className="mt-4 text-xl leading-relaxed text-ink">{question.prompt}</p>
            <ul className="mt-8 space-y-3">
              {question.options.map((opt, i) => (
                <li key={i}>
                  <button
                    type="button"
                    onClick={() => {
                      chosenAt.current = performance.now();
                      setChosen(i);
                      setPhase("confidence");
                    }}
                    className="w-full rounded-md border border-line bg-canvas px-5 py-4 text-left text-base text-ink transition-colors hover:border-accent hover:text-accent"
                  >
                    {opt}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        ) : null}

        {phase === "confidence" && question ? (
          <div>
            <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
              How sure are you?
            </p>
            <p className="mt-4 text-base text-ink-2">{question.prompt}</p>
            <ul className="mt-8 grid gap-3 sm:grid-cols-5">
              {CONFIDENCE.map((c) => (
                <li key={c.value}>
                  <button
                    type="button"
                    onClick={() => void advance(chosen ?? -1, c.value)}
                    className="w-full rounded-md border border-line bg-canvas px-3 py-4 text-sm text-ink transition-colors hover:border-accent hover:text-accent"
                  >
                    {c.label}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        ) : null}

        {phase === "recall" ? (
          <div className="text-center">
            <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
              The number
            </p>
            {transform ? (
              <p className="mx-auto mt-4 max-w-md rounded-md border border-accent bg-accent-soft/60 px-5 py-3 text-base font-semibold text-accent">
                Add {transform.add} to every digit.
              </p>
            ) : (
              <p className="mt-4 text-base text-ink-2">Exactly as it was shown.</p>
            )}
            <input
              autoFocus
              inputMode="numeric"
              value={recall}
              onChange={(e) => setRecall(e.target.value.replace(/\D/g, "").slice(0, 8))}
              onKeyDown={(e) => {
                if (e.key === "Enter") void submitRecall();
              }}
              className="font-display mt-8 w-56 rounded-md border border-line bg-canvas px-5 py-4 text-center text-4xl tracking-[0.2em] text-ink tabular-nums"
            />
            <div>
              <button
                type="button"
                onClick={() => void submitRecall()}
                className="mt-8 rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase"
              >
                Continue
              </button>
            </div>
          </div>
        ) : null}

        {phase === "debrief" ? (
          <div className="mx-auto max-w-lg">
            <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
              last one, and it is about method rather than memory
            </p>
            <h2 className="font-display mt-3 text-2xl leading-tight text-ink">
              On the blocks where you had to add to each digit, what did
              you do?
            </h2>
            <p className="mt-4 text-base leading-relaxed text-ink-2">
              There is no right answer and it does not affect your score.
              Both approaches are sensible and people split roughly evenly,
              which is exactly why it is worth knowing which you used.
            </p>
            <div className="mt-8 space-y-3">
              <button
                type="button"
                onClick={() => void answerDebrief("encode")}
                className="w-full rounded-md border border-line bg-canvas px-5 py-4 text-left text-base text-ink transition-colors hover:border-accent"
              >
                I did the addition straight away
                <span className="mt-1 block text-sm text-ink-3">
                  Worked out the new number during the few seconds it was
                  shown, then held that.
                </span>
              </button>
              <button
                type="button"
                onClick={() => void answerDebrief("defer")}
                className="w-full rounded-md border border-line bg-canvas px-5 py-4 text-left text-base text-ink transition-colors hover:border-accent"
              >
                I held the original and added at the end
                <span className="mt-1 block text-sm text-ink-3">
                  Remembered the digits as shown and did the arithmetic
                  when it came time to type them.
                </span>
              </button>
              {/* A third option, because forcing one of two manufactures
                  a label. Somebody who switched between blocks, or who
                  cannot remember, is giving a different answer from
                  somebody who did one consistently, and a wrong label is
                  worse than a missing one: the analysis counts it. Same
                  reasoning as insufficient_evidence in the decision
                  log. */}
              <button
                type="button"
                onClick={() => void answerDebrief("unsure")}
                className="w-full rounded-md border border-line bg-canvas px-5 py-4 text-left text-base text-ink transition-colors hover:border-accent"
              >
                A bit of both, or I do not remember
                <span className="mt-1 block text-sm text-ink-3">
                  Also a real answer. Switching between blocks is common.
                </span>
              </button>
            </div>
          </div>
        ) : null}

        {phase === "finishing" ? (
          <p className="text-center text-base text-ink-2">One moment.</p>
        ) : null}
      </main>
    </div>
  );
}
