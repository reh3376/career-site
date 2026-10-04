// The metronome: one tick per second, unchanging, for the whole test.
//
// It is the constant, not the ramp. Effort escalates through the digits
// a participant holds (3, 4, 4+1, 4+3); the tick never changes. Holding
// it constant is what makes it useful: identical in every block and
// every condition, it cannot differ between the things being compared,
// so it drops out of the analysis instead of entering it as a confound.
//
// # Why this is not setInterval
//
// A fifteen-minute test is 900 ticks. setInterval drifts: it schedules
// against the event loop, so a busy frame pushes a tick late and the
// error accumulates. Over 900 ticks that is audible, and worse, it is
// audible by a different amount on a fast laptop than on a loaded
// phone, which would mean participants were not hearing the same thing.
// Cadence is part of the instrument.
//
// Web Audio has its own clock, independent of the event loop, and
// oscillators scheduled against it fire sample-accurately. So this
// schedules ticks ahead of time in small batches and only uses a timer
// to decide when to schedule the next batch. A late timer delays
// scheduling, never playback.
//
// The click is synthesised rather than loaded. An audio file would be
// an asset request the CSP has to allow and a download that can fail
// mid-test; an oscillator is neither.

/** How far ahead ticks are scheduled, in seconds. */
const LOOKAHEAD_S = 0.3;
/** How often the scheduler wakes to queue more, in milliseconds. */
const SCHEDULE_EVERY_MS = 100;
/** The interval the whole instrument is built on. */
export const TICK_INTERVAL_S = 1.0;

export type Signal = {
  /** Starts ticking. Must be called from a user gesture. */
  start: () => Promise<void>;
  /** Stops and releases the audio context. */
  stop: () => void;
  /**
   * Fires on every tick, for the visual mode and for the tap-check,
   * which needs to know when a tick actually sounded rather than when
   * a timer thought it did.
   */
  onTick: (fn: (audioTime: number) => void) => void;
};

/**
 * Creates the metronome.
 *
 * `audible` false still ticks, silently, so the visual heartbeat beats
 * on exactly the same clock as the sound would have. A participant in
 * visual mode is in a different condition, which is recorded, but they
 * are not on a different timebase.
 */
export function createMetronome(audible: boolean): Signal {
  let ctx: AudioContext | null = null;
  let nextTickTime = 0;
  let timer: number | null = null;
  const listeners: ((t: number) => void)[] = [];

  function scheduleClick(at: number) {
    if (!ctx || !audible) return;
    // Short, dry, and quiet enough to live under for fifteen minutes.
    // A long or bright tick becomes unbearable well before block 5.
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.frequency.value = 1000;
    gain.gain.setValueAtTime(0.0001, at);
    gain.gain.exponentialRampToValueAtTime(0.18, at + 0.001);
    gain.gain.exponentialRampToValueAtTime(0.0001, at + 0.03);
    osc.connect(gain).connect(ctx.destination);
    osc.start(at);
    osc.stop(at + 0.04);
  }

  function pump() {
    if (!ctx) return;
    while (nextTickTime < ctx.currentTime + LOOKAHEAD_S) {
      scheduleClick(nextTickTime);
      const at = nextTickTime;
      // Notify on the audio clock, not the wall clock, so a visual beat
      // and the tap-check agree with what was actually heard.
      const delayMs = Math.max(0, (at - ctx.currentTime) * 1000);
      window.setTimeout(() => listeners.forEach((fn) => fn(at)), delayMs);
      nextTickTime += TICK_INTERVAL_S;
    }
  }

  return {
    async start() {
      if (ctx) return;
      // Browsers refuse audio that starts without a user gesture. The
      // caller supplies one; if the context still arrives suspended,
      // resume() is the documented escape.
      const Ctor =
        window.AudioContext ??
        (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
      if (!Ctor) return;
      ctx = new Ctor();
      if (ctx.state === "suspended") await ctx.resume();
      nextTickTime = ctx.currentTime + 0.1;
      pump();
      timer = window.setInterval(pump, SCHEDULE_EVERY_MS);
    },
    stop() {
      if (timer !== null) window.clearInterval(timer);
      timer = null;
      void ctx?.close();
      ctx = null;
      listeners.length = 0;
    },
    onTick(fn) {
      listeners.push(fn);
    },
  };
}

/**
 * Scores a tap-along check.
 *
 * The check has to reject two different things and a hit rate alone
 * cannot separate them: a participant who hears nothing (few taps near
 * a tick) and one who is mashing (taps unrelated to the signal). At 1 Hz
 * a generous window makes random pressing pass by chance, so the third
 * test is consistency: somebody tracking a metronome has a steady lag,
 * somebody mashing does not.
 *
 * The thresholds are a starting point to pilot against, not a
 * calibration. Setting them from a guess is how a check becomes either
 * a nuisance that fails good participants or a formality that passes
 * anyone, which is why this is still open in the FSD.
 */
export function scoreTapCheck(
  tickTimes: number[],
  tapTimes: number[],
): { passed: boolean; meanOffsetMs: number; sdMs: number; hits: number } {
  const offsets: number[] = [];
  for (const tap of tapTimes) {
    let best = Infinity;
    for (const tick of tickTimes) {
      const d = Math.abs(tap - tick);
      if (d < best) best = d;
    }
    if (best <= 0.3) offsets.push(best * 1000);
  }
  const hits = offsets.length;
  const mean = hits ? offsets.reduce((a, b) => a + b, 0) / hits : 0;
  const variance = hits ? offsets.reduce((a, b) => a + (b - mean) ** 2, 0) / hits : 0;
  const sd = Math.sqrt(variance);

  // Too many taps means mashing, whatever the hit rate looks like.
  const notMashing = tapTimes.length <= tickTimes.length + 2;
  const enoughHits = hits >= Math.ceil(tickTimes.length * 0.7);
  // 80 ms, and the number is arithmetic rather than taste. Taps that
  // all land inside the 300 ms window can have a standard deviation of
  // at most 150 ms (perfectly bimodal at the two edges), so a 150 ms
  // cut is close to unreachable and the consistency test would never
  // fire. Measured reference points: somebody genuinely tracking a
  // metronome lands around 8 ms of jitter, and taps spread uniformly
  // across the window land around 96 ms. 80 sits between them with
  // room on both sides.
  const steady = hits < 3 ? false : sd <= 80;

  return {
    passed: notMashing && enoughHits && steady,
    meanOffsetMs: Math.round(mean),
    sdMs: Math.round(sd),
    hits,
  };
}
