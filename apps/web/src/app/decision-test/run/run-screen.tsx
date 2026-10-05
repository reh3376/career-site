"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";

import { decisionTestClient } from "@/lib/decision-test-client";
import { createMetronome, type Signal } from "@/lib/metronome";

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

/** Hard limit per question, covering reading, deciding and rating. */
const QUESTION_MS = 20_000;
/**
 * How long the digits are shown before the questions begin.
 *
 * Five seconds, raised from two after the first live run. Two was the
 * original specification and it was wrong in practice: four digits plus
 * a transformation rule is not readable in two seconds, and a
 * participant who spends all of it on the digits has none left for the
 * instruction. The whole budget cost is fifteen seconds across five
 * blocks.
 */
const MEMORISE_MS = 5_000;
/** Limit on entering the number at the end of a block. */
const RECALL_MS = 20_000;

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
type Phase = "memorise" | "question" | "confidence" | "recall" | "finishing";

type Handoff = {
  sessionKey: string;
  audioMode: string;
  practice: Block;
  questionCount: number;
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
        setPhase("finishing");
        try {
          await decisionTestClient.finishSession({ sessionKey, recallStrategy: "" });
        } catch {
          /* the session stays abandoned, which is kept rather than lost */
        }
        sessionStorage.removeItem("dt");
        router.replace("/decision-test/thanks");
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
    [router, sessionKey],
  );

  const advance = useCallback(
    async (chosenIndex: number, confidence: number) => {
      const latency = Math.round(performance.now() - shownAt.current);
      if (scored && question) {
        try {
          await decisionTestClient.submitAnswer({
            sessionKey,
            blockNo,
            positionOverall: question.positionOverall,
            chosenIndex,
            latencyMs: chosenIndex < 0 ? 0 : latency,
            confidence: chosenIndex < 0 ? 0 : confidence,
          });
        } catch {
          // A failed post must not strand the participant mid-test.
          // The row is lost; the run is not, and a partial session is
          // still data.
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
    try {
      await decisionTestClient.submitRecall({
        sessionKey,
        blockNo,
        digits: recall,
        latencyMs: latency,
      });
    } catch {
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
      }, MEMORISE_MS);
      return () => window.clearTimeout(t);
    }
    if (phase === "question" || phase === "confidence") {
      const left = QUESTION_MS - (performance.now() - shownAt.current);
      const t = window.setTimeout(() => void advance(-1, 0), Math.max(0, left));
      return () => window.clearTimeout(t);
    }
    if (phase === "recall") {
      const t = window.setTimeout(() => void submitRecall(), RECALL_MS);
      return () => window.clearTimeout(t);
    }
    return;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phase, qi, blockNo]);

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

        {phase === "finishing" ? (
          <p className="text-center text-base text-ink-2">One moment.</p>
        ) : null}
      </main>
    </div>
  );
}
