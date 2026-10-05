"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";

import { decisionTestClient } from "@/lib/decision-test-client";
import { createMetronome, scoreTapCheck, type Signal } from "@/lib/metronome";

// The briefing.
//
// Not a pitch. Participants arrive having already agreed to help, so
// this page's job is preparation, not persuasion: here is what the next
// fifteen minutes involves, here is what you need, start when ready.
//
// THE ORDER IS THE DESIGN. The effort warning comes first, before any
// field is shown, for three reasons. It is honest at the only moment it
// can be, since a warning after the form is a warning after the
// commitment. It is data minimisation: somebody who reads it and leaves
// has given us nothing, no name, no email, no row. And it is a screening
// device, which is a feature rather than a cost, because the people who
// opt out here are the ones who would have produced the noisiest data
// and there is no traffic target to protect.
//
// Nothing is posted until the start button. The form state lives in this
// component and goes up with the session or not at all.

const AGES = ["18 to 24", "25 to 34", "35 to 44", "45 to 54", "55 to 64", "65 or over"];
const EDUCATION = [
  "GED",
  "High School",
  "Skilled Trades Apprenticeship",
  "Some College",
  "Associate Degree",
  "Bachelor's Degree",
  "Master's Degree",
  "PhD",
  "Multiple Degrees",
];

type Step = "warning" | "sound" | "tap" | "form";

export function Briefing() {
  const router = useRouter();
  const [step, setStep] = useState<Step>("warning");
  const [audioMode, setAudioMode] = useState<"sound" | "visual">("sound");
  const [tap, setTap] = useState<{ passed: boolean; rt: number; sd: number } | null>(null);
  const [tapRunning, setTapRunning] = useState(false);
  // idle, then an unmeasured lead-in, then the scored window. The
  // participant is told which they are in, because "tap along" and
  // "you are being measured now" are different instructions.
  const [tapPhase, setTapPhase] = useState<"idle" | "lead" | "measuring">("idle");
  const [beat, setBeat] = useState(false);
  const [attempts, setAttempts] = useState(0);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState("");

  const [form, setForm] = useState({
    displayName: "",
    ageRange: "",
    education: "",
    occupation: "",
    email: "",
    wantsResults: false,
  });

  const signal = useRef<Signal | null>(null);
  const ticks = useRef<number[]>([]);
  const taps = useRef<number[]>([]);
  const measureFrom = useRef(0);
  const startedAt = useRef(0);

  // A phone has no space bar, so the tap check was impossible on one:
  // the instruction said "press the space bar" and there was nothing to
  // press. Found by the owner.
  //
  // The fix is a tap target rather than forcing the software keyboard
  // up. A keyboard covers much of a phone screen, including where the
  // visual beat lives for anyone in visual mode, and it appears and
  // disappears with a layout shift in the middle of a timing
  // measurement. Tapping a large target is also the native gesture on a
  // phone, where pressing a space bar is a desktop idiom in disguise.
  //
  // Both inputs are live on every device and the wording names both,
  // which is simpler than detecting the device and cannot be wrong
  // about it. A tablet with a keyboard can use either.

  useEffect(() => () => signal.current?.stop(), []);

  // The tap check. Pressing in time with the signal is observed
  // behaviour, where "can you hear it?" is only a claim: a muted laptop
  // cannot be tapped along to. It also yields an unloaded reaction time
  // and its variability, which turn latency in the test from an absolute
  // into a measure relative to this participant's own speed.
  // Three seconds of unmeasured lead-in before the window opens.
  //
  // The owner found this by taking it: the check used to start the
  // ticks, start the clock and render the tap pad in the same instant,
  // so a participant met the target for the first time with the
  // measurement already running. Their first few taps are orientation
  // rather than tracking, and orientation lands wide, which inflates
  // both the mean offset and the standard deviation that decides
  // pass or fail. The check was therefore hardest on people doing
  // nothing wrong, and the baseline reaction time it produced, which
  // every latency in the test is read against, was measured during
  // the one part of the run the participant was not yet ready for.
  //
  // So: the ticks start, three of them pass unscored while the screen
  // says to find the rhythm, and only then does the scored window
  // open. Ticks and taps before that point are discarded rather than
  // weighted, because a tap during orientation is not a worse
  // measurement, it is not a measurement.
  const LEAD_IN_MS = 3_000;
  const MEASURE_MS = 10_000;

  const runTap = useCallback(async () => {
    setTap(null);
    ticks.current = [];
    taps.current = [];
    setTapRunning(true);
    setTapPhase("lead");
    const m = createMetronome(audioMode === "sound");
    signal.current = m;
    m.onTick((t) => {
      ticks.current.push(t);
      setBeat(true);
      window.setTimeout(() => setBeat(false), 140);
    });
    // start() is called from this click, which is the user gesture
    // browsers require before any audio will play.
    await m.start();
    startedAt.current = performance.now();

    window.setTimeout(() => {
      // The window opens. Everything collected during the lead-in is
      // dropped, so the score describes tracking only.
      measureFrom.current = performance.now();
      ticks.current = ticks.current.filter(
        (t) => t >= (ticks.current[0] ?? 0) + LEAD_IN_MS / 1000,
      );
      taps.current = [];
      setTapPhase("measuring");
    }, LEAD_IN_MS);

    window.setTimeout(() => {
      m.stop();
      signal.current = null;
      setTapRunning(false);
      setTapPhase("idle");
      const r = scoreTapCheck(ticks.current, taps.current);
      setTap({ passed: r.passed, rt: r.meanOffsetMs, sd: r.sdMs });
      setAttempts((n) => n + 1);
    }, LEAD_IN_MS + MEASURE_MS + 1_000);
  }, [audioMode]);

  // One path for both inputs, so a tap and a key press are timed
  // identically and the scoring cannot differ by device.
  const recordTap = useCallback(() => {
    if (!tapRunning) return;
    // A tap during the lead-in is deliberately not recorded. It is the
    // participant finding the beat, which is the thing the lead-in
    // exists to let them do without it counting against them.
    if (tapPhase !== "measuring") return;
    const elapsed = (performance.now() - startedAt.current) / 1000;
    taps.current.push((ticks.current[0] ?? 0) + elapsed);
  }, [tapRunning, tapPhase]);

  // Taps are recorded on the same clock the ticks were scheduled on, so
  // the offsets compare like with like.
  useEffect(() => {
    if (!tapRunning) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.code !== "Space") return;
      e.preventDefault();
      recordTap();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [tapRunning, recordTap]);

  const start = useCallback(async () => {
    setStarting(true);
    setError("");
    try {
      const res = await decisionTestClient.startSession({
        intake: form,
        conditions: {
          audioMode,
          deviceClass: /Mobi|Android/i.test(navigator.userAgent)
            ? "phone"
            : /iPad|Tablet/i.test(navigator.userAgent)
              ? "tablet"
              : "desktop",
          tapCheckPassed: tap?.passed ?? false,
          baselineRtMs: tap?.rt ?? 0,
          baselineRtSdMs: tap?.sd ?? 0,
        },
        synthetic: false,
      });
      const p = res.practice;
      sessionStorage.setItem(
        "dt",
        JSON.stringify({
          sessionKey: res.sessionKey,
          audioMode,
          questionCount: res.questionCount,
          // Served per session so a run cannot drift from the settings
          // it started under, even if the owner edits them mid-run.
          timings: res.timings
            ? {
                memoriseMs: res.timings.memoriseMs,
                questionMs: res.timings.questionMs,
                recallMs: res.timings.recallMs,
              }
            : undefined,
          practice: {
            blockNo: 0,
            load: "practice",
            digits: "",
            transform: "none",
            questions: (p?.questions ?? []).map((q) => ({
              code: q.code,
              version: q.version,
              prompt: q.prompt,
              reminder: q.reminder,
              options: q.options,
              positionOverall: q.positionOverall,
            })),
          },
        }),
      );
      router.push("/decision-test/run");
    } catch {
      setError("The test could not be started. Try again in a moment.");
      setStarting(false);
    }
  }, [audioMode, form, router, tap]);

  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        a supplement to the decision-quality series
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Fifteen minutes, and real effort.
      </h1>

      {step === "warning" ? (
        <section className="mt-10 space-y-5 text-base leading-relaxed text-ink-2">
          <p className="text-ink">
            <strong>This is deliberately demanding.</strong> It asks for
            fifteen unbroken minutes of full attention. If you cannot give
            that right now, please come back when you can. A distracted run
            is worse than no run, for you and for the data.
          </p>
          <p className="rounded-md border border-accent bg-accent-soft/60 px-5 py-4 text-base text-accent">
            <strong>
              The number you are given at the start of each block is the
              most important part of this test.
            </strong>{" "}
            Hold on to it while you answer the questions, and give it back
            at the end of the block. The questions matter less than that
            number does.
          </p>
          <ul className="list-disc space-y-2 pl-5">
            <li>Fifteen minutes, timed. There is no pause.</li>
            <li>
              If you leave, you start over. There is no way to resume
              partway.
            </li>
            <li>
              Questions are timed. One that runs out moves on by itself:
              no going back, no explanation. That is the test working, not
              the site breaking.
            </li>
            <li>You will need sound, or the on-screen alternative.</li>
          </ul>
          <p>
            Nothing has been collected yet. Closing this page now leaves
            nothing behind.
          </p>
          <button
            type="button"
            onClick={() => setStep("sound")}
            className="mt-4 rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase"
          >
            I can give it fifteen minutes
          </button>
        </section>
      ) : null}

      {step === "sound" ? (
        <section className="mt-10 space-y-5 text-base leading-relaxed text-ink-2">
          <p>
            A metronome ticks once a second throughout. It never changes,
            and it is part of the test rather than decoration.{" "}
            <strong className="text-ink">
              Connect headphones or speakers and turn the volume up now.
            </strong>
          </p>
          <fieldset className="mt-6">
            <legend className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
              Can you use sound?
            </legend>
            <div className="mt-4 flex flex-wrap gap-3">
              {(
                [
                  ["sound", "Yes, I have sound"],
                  ["visual", "No, show me a beat instead"],
                ] as const
              ).map(([v, label]) => (
                <button
                  key={v}
                  type="button"
                  onClick={() => {
                    setAudioMode(v);
                    setStep("tap");
                  }}
                  className="rounded-md border border-line bg-canvas px-5 py-3 text-sm text-ink transition-colors hover:border-accent hover:text-accent"
                >
                  {label}
                </button>
              ))}
            </div>
          </fieldset>
          <p className="text-sm text-ink-3">
            No reason needed. Choosing the beat costs nothing and changes
            nothing except how the second is marked.
          </p>
        </section>
      ) : null}

      {step === "tap" ? (
        <section className="mt-10 space-y-5 text-base leading-relaxed text-ink-2">
          {/* The explanation comes first and nothing is running while it
              is read. The check used to start the ticks, start the clock
              and render the tap pad in the same instant, so a
              participant met the target for the first time with the
              measurement already under way. Their first taps were
              orientation rather than tracking, and that lands wide,
              which inflated both the offset and the jitter that decides
              pass or fail. It was hardest on people doing nothing
              wrong. */}
          {tapPhase === "idle" ? (
            <>
              <p>
                <strong className="text-ink">
                  Next, a ten second check, and nothing has started yet.
                </strong>{" "}
                It confirms the{" "}
                {audioMode === "sound" ? "sound is actually reaching you" : "beat is visible to you"},
                which a yes-or-no question cannot: a muted laptop cannot
                be tapped along to. It also reads your unloaded timing,
                which is what the rest of the test gets measured against,
                so a naturally fast or slow person is compared with
                themselves rather than with anyone else.
              </p>
              <p>
                When you start, you will{" "}
                {audioMode === "sound" ? "hear a tick" : "see the dot flash"}{" "}
                once a second. Tap the pad below, or press the space bar,
                in time with it. You get a few seconds to find the rhythm
                before anything is measured, and the screen will tell you
                when the measured part begins.
              </p>
            </>
          ) : null}

          {tapPhase === "lead" ? (
            <p className="rounded-md border border-line bg-paper-2 px-5 py-4 text-base text-ink-2">
              <strong className="text-ink">Find the rhythm.</strong> Not
              measured yet. Tap along until the screen says otherwise.
            </p>
          ) : null}

          {tapPhase === "measuring" ? (
            <p className="rounded-md border border-accent bg-accent-soft/40 px-5 py-4 text-base text-ink-2">
              <strong className="text-ink">Measuring now.</strong> Keep
              tapping in time for about ten seconds.
            </p>
          ) : null}

          {/* The pad is always rendered, inert when idle. Conditional
              rendering moved the layout at the exact moment the clock
              started, which is the worst possible moment to move it. */}
          <button
            type="button"
            disabled={tapPhase !== "measuring" && tapPhase !== "lead"}
            // onPointerDown rather than onClick: a click fires after the
            // gesture completes, which adds the press duration to every
            // offset and would make a slow finger look like a late tap.
            onPointerDown={(e) => {
              e.preventDefault();
              recordTap();
            }}
            className={`flex w-full touch-manipulation select-none flex-col items-center gap-4 rounded-md border py-16 ${
              tapPhase === "measuring"
                ? "border-accent bg-canvas active:border-accent"
                : tapPhase === "lead"
                  ? "border-line bg-canvas active:border-accent"
                  : "border-line bg-paper-2 opacity-60"
            }`}
          >
            <span
              aria-hidden
              className={`size-10 rounded-full transition-opacity duration-100 ${
                beat ? "bg-accent opacity-100" : "bg-ink-4 opacity-25"
              }`}
            />
            <span className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
              {tapPhase === "idle"
                ? "this is the pad, not started"
                : "tap here, or space bar"}
            </span>
          </button>

          {tapPhase === "idle" && tap && !tap.passed ? (
            <p className="rounded-md border border-line bg-paper-2 px-5 py-4 text-sm text-ink-2">
              That did not look like tracking.{" "}
              {audioMode === "sound"
                ? "Check the volume is up and that sound is not muted, then try again."
                : "Watch the dot and press in time with it, then try again."}
              {attempts >= 2 && audioMode === "sound" ? (
                <>
                  {" "}
                  If sound is not going to work on this device, switch to
                  the on-screen beat instead.
                </>
              ) : null}
            </p>
          ) : null}

          {tapPhase === "idle" && tap?.passed ? (
            <p className="rounded-md border border-line bg-paper-2 px-5 py-4 text-sm text-ink-2">
              Good. Your unloaded reaction time is about {tap.rt} ms, which
              is what the rest of the test will be read against.
            </p>
          ) : null}

          <div className="flex flex-wrap gap-3">
            {tapPhase === "idle" ? (
              <button
                type="button"
                onClick={() => void runTap()}
                className="rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase"
              >
                {tap ? "Try again" : "Start the check"}
              </button>
            ) : null}
            {attempts >= 2 && audioMode === "sound" && tapPhase === "idle" ? (
              <button
                type="button"
                onClick={() => {
                  setAudioMode("visual");
                  setTap(null);
                  setAttempts(0);
                }}
                className="rounded-md border border-line bg-canvas px-6 py-3 text-sm text-ink transition-colors hover:border-accent hover:text-accent"
              >
                Use the on-screen beat
              </button>
            ) : null}
            {tap?.passed && tapPhase === "idle" ? (
              <button
                type="button"
                onClick={() => setStep("form")}
                className="rounded-md border border-line bg-canvas px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-ink uppercase transition-colors hover:border-accent hover:text-accent"
              >
                Continue
              </button>
            ) : null}
          </div>
        </section>
      ) : null}

      {step === "form" ? (
        <section className="mt-10 space-y-6 text-base leading-relaxed text-ink-2">
          <p>
            <strong className="text-ink">Every field is optional.</strong>{" "}
            They are here because they are what makes the results worth
            anything: without them the data says what happened, but not to
            whom.
          </p>

          <div className="grid gap-5">
            <Field label="Name">
              <input
                className={inputClass}
                value={form.displayName}
                onChange={(e) => setForm({ ...form, displayName: e.target.value })}
              />
            </Field>
            <Field label="Age range">
              <select
                className={inputClass}
                value={form.ageRange}
                onChange={(e) => setForm({ ...form, ageRange: e.target.value })}
              >
                <option value="">Prefer not to say</option>
                {AGES.map((a) => (
                  <option key={a}>{a}</option>
                ))}
              </select>
            </Field>
            <Field label="Highest level of education completed">
              <select
                className={inputClass}
                value={form.education}
                onChange={(e) => setForm({ ...form, education: e.target.value })}
              >
                <option value="">Prefer not to say</option>
                {EDUCATION.map((a) => (
                  <option key={a}>{a}</option>
                ))}
              </select>
            </Field>
            <Field label="Occupation">
              <input
                className={inputClass}
                value={form.occupation}
                onChange={(e) => setForm({ ...form, occupation: e.target.value })}
              />
            </Field>
            <Field label="Email address">
              <input
                type="email"
                className={inputClass}
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
              />
              <p className="mt-2 text-sm text-ink-3">
                Without an address there is no way to send you your own
                results. The explanation of what the test was doing reaches
                everyone either way, at the end. The address is also how a
                repeat run is recognised.
              </p>
            </Field>
            <label className="flex items-start gap-3 text-sm text-ink-2">
              <input
                type="checkbox"
                checked={form.wantsResults}
                onChange={(e) => setForm({ ...form, wantsResults: e.target.checked })}
                className="mt-1"
              />
              Send me my results.
            </label>
          </div>

          <p className="text-sm text-ink-3">
            What is kept and for how long is in the{" "}
            <Link href="/privacy">privacy notice</Link>.
          </p>

          {error ? <p className="text-sm text-signal">{error}</p> : null}

          <button
            type="button"
            disabled={starting}
            onClick={() => void start()}
            className="rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase disabled:opacity-50"
          >
            {starting ? "Starting" : "Start the test"}
          </button>
          <p className="text-sm text-ink-3">
            The next screen begins immediately and the clock starts with it.
            Remember: holding the number is the part that counts.
          </p>
        </section>
      ) : null}
    </div>
  );
}

const inputClass =
  "w-full rounded-md border border-line bg-canvas px-4 py-3 text-base text-ink";

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        {label}
      </span>
      <span className="mt-2 block">{children}</span>
    </label>
  );
}
