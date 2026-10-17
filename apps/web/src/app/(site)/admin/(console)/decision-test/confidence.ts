// Answers the participant never rated.
//
// The clock can run out while somebody is still on the confidence step.
// That answer has an outcome, a latency and a lure reading, and nothing
// to say about calibration. The database stores NULL for it on purpose
// and every view skips it, so the aggregates have always been right.
//
// What went wrong was further out: one projection coalesced the NULL to
// zero, and zero is a real rating on a scale whose lowest label is
// "Guessing" at 10. The console printed "0%" and the CSV wrote 0 into
// the column an analysis averages. In run c7cbd814 that turned a
// correct answer given at 24.7 seconds of a 25-second limit into the
// worst-calibrated point in the run.
//
// So the absence is now carried as -1 and named wherever it is shown,
// and the count is put on the page. A figure that quietly excludes some
// of its own rows should say how many, every time, rather than wait for
// somebody to read thirty rows and notice.

const UNRATED = -1;

export function isRated(a: { outcome?: string; confidence?: number }): boolean {
  if (a.outcome === "expired") return false;
  return (a.confidence ?? UNRATED) >= 0;
}

/** Non-expired answers with no confidence rating. */
export function unratedCount(
  answers: { outcome?: string; confidence?: number }[],
): number {
  return answers.filter(
    (a) => a.outcome !== "expired" && (a.confidence ?? UNRATED) < 0,
  ).length;
}

export function unratedNote(n: number, answered: number): string | null {
  if (n <= 0) return null;
  const s = n === 1 ? "answer carries" : "answers carry";
  return (
    `${n} of ${answered} ${s} no confidence rating, because the clock ran ` +
    `out on the rating step. They count towards accuracy and are left out ` +
    `of every confidence figure here, which is why the two denominators differ.`
  );
}
