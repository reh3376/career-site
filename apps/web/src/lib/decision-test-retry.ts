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

type Pending = {
  label: string;
  run: Task;
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
export function enqueueRetry(label: string, run: Task): void {
  queue.push({ label, run, attempts: 0 });
  schedule();
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
