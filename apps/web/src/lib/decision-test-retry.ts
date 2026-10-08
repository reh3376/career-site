// Retrying a decision-test write that failed.
//
// **Why this exists.** The run screen used to post an answer and, on
// failure, swallow the error with a comment saying the row was lost but
// the run was not. That is true and it is not enough: a single dropped
// request silently destroys one answer, permanently, and the
// participant has no idea. It presents in the data as a question with
// no row at all, which is indistinguishable from one that was never
// reached.
//
// That is not hypothetical. A participant on a phone reported answering
// eight questions on 2026-10-07; three had rows and the rest had
// nothing. A phone on a weak signal is the normal case for this test,
// not the edge case, and fifteen minutes of somebody's attention is too
// expensive to lose to one flaky POST.
//
// **Why a queue rather than awaiting a retry.** The participant must
// never wait on the network. A question that hangs for four seconds
// while a retry backs off has changed the instrument: the next question
// starts late and the run is no longer the thing being measured. So the
// first attempt is awaited by the caller as before, and only failures
// come here, where they are retried in the background while the test
// carries on.
//
// Writes are idempotent at the database (`ON CONFLICT (session_id,
// position_overall) DO NOTHING`), so a retry that duplicates a request
// which actually succeeded is harmless. That is what makes retrying
// safe rather than a source of double-counted answers.

type Task = () => Promise<unknown>;

/**
 * Enough to resend a write without the page being alive to await it.
 *
 * `fetch` from a page that is closing is cancelled with it, so the
 * backoff below is worth nothing at the one moment a participant is
 * most likely to be leaving: interrupted, tab shut, answers still
 * queued. `navigator.sendBeacon` is the browser's answer to that, and
 * it needs the request as data rather than as a closure, which is why
 * this exists alongside `run`.
 *
 * Connect speaks JSON over a plain POST, and the api is same-origin at
 * /api, so a beacon is a valid request: no preflight, and the cookies
 * go with it.
 */
export type Beacon = {
  /** Full path, e.g. /api/career.v1.DecisionTestService/SubmitAnswer */
  path: string;
  /** The message, in Connect's JSON shape. */
  body: unknown;
};

type Pending = {
  label: string;
  run: Task;
  beacon?: Beacon;
  attempts: number;
};

/** Backoff in ms. Five attempts over roughly half a minute, which
 *  outlasts a lift, a tunnel or a handover between cells without
 *  outlasting the run itself. */
const BACKOFF_MS = [1_000, 3_000, 8_000, 20_000];

const queue: Pending[] = [];
let timer: ReturnType<typeof setTimeout> | null = null;

/** Exposed for the tests, and for a caller that wants to know whether
 *  anything is still outstanding. */
export function pendingCount(): number {
  return queue.length;
}

/** Drops everything, for test isolation. */
export function resetRetryQueue(): void {
  queue.length = 0;
  if (timer) {
    clearTimeout(timer);
    timer = null;
  }
}

/**
 * Takes a write that has already failed once and keeps trying.
 *
 * `label` is only for the console: when every attempt fails there is
 * nothing else to tell anybody, and a named failure in a browser log is
 * the difference between "some answers are missing" and "answer 12 of
 * session X never landed".
 */
export function enqueueRetry(label: string, run: Task, beacon?: Beacon): void {
  queue.push({ label, run, beacon, attempts: 0 });
  schedule();
}

/**
 * Last chance: hand everything still queued to the browser to deliver
 * after the page is gone.
 *
 * Called on `pagehide`. Up to this point the queue has been retrying on
 * a backoff measured in tens of seconds, which is the right pace for a
 * page that is still open and the wrong one for a tab about to close:
 * whatever is outstanding at that moment is simply lost, and the
 * participants it is lost for are exactly the ones who were interrupted
 * mid-run.
 *
 * `sendBeacon` queues the request with the browser, which delivers it
 * whether or not this page survives. There is no response to read and
 * no way to retry, so this is a best effort by construction. It is
 * still the difference between most of those answers arriving and none
 * of them.
 *
 * Writes are idempotent on (session, position), so a beacon that
 * duplicates one of the earlier attempts changes nothing.
 */
export function beaconPending(): void {
  if (typeof navigator === "undefined" || !navigator.sendBeacon) return;
  for (const item of queue.splice(0, queue.length)) {
    if (!item.beacon) continue;
    try {
      navigator.sendBeacon(
        item.beacon.path,
        new Blob([JSON.stringify(item.beacon.body)], { type: "application/json" }),
      );
    } catch {
      // Nothing useful to do: the page is going away.
    }
  }
}

function schedule(): void {
  if (timer || queue.length === 0) return;
  const next = queue[0];
  const wait = BACKOFF_MS[Math.min(next.attempts, BACKOFF_MS.length - 1)];
  timer = setTimeout(() => {
    timer = null;
    void flushOne();
  }, wait);
}

async function flushOne(): Promise<void> {
  const item = queue.shift();
  if (!item) return;
  try {
    await item.run();
  } catch {
    item.attempts += 1;
    if (item.attempts <= BACKOFF_MS.length) {
      // Back on the queue, at the back: a later answer should not be
      // held up behind one that keeps failing, because the later one
      // may be on a connection that has since recovered.
      queue.push(item);
    } else {
      // Out of attempts. Said out loud rather than swallowed, because
      // the silent version of this is the bug being fixed.
      console.error(
        `decision test: gave up resending ${item.label} after ${item.attempts} attempts`,
      );
    }
  }
  schedule();
}

/**
 * Tries everything outstanding now, ignoring the backoff.
 *
 * Called when the page becomes visible again. A phone that was switched
 * away from has had its timers throttled or frozen, so the queue may
 * have been sitting still for minutes; the moment the participant comes
 * back is both the earliest the network is likely to work again and the
 * last chance before they close the tab.
 */
export function flushNow(): void {
  if (timer) {
    clearTimeout(timer);
    timer = null;
  }
  void flushOne();
}
