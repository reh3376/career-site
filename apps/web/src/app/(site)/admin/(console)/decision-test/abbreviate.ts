// The shortest prefix of each run id that is unique across the runs on
// screen, minimum eight characters.
//
// Truncating to a fixed width would be guessing. The owner's reason for
// wanting the id shown was to name a run unambiguously, so an
// abbreviation that could name two runs would defeat the request while
// looking like it had satisfied it. This widens until every prefix is
// distinct, the way an abbreviated commit hash does, and falls back to
// the full id if two ever collide outright.
export function abbreviate(ids: string[], min = 8): Map<string, string> {
  const full = ids.length ? ids[0].length : 0;
  for (let n = min; n <= full; n++) {
    const seen = new Set(ids.map((i) => i.slice(0, n)));
    if (seen.size === ids.length) {
      return new Map(ids.map((i) => [i, i.slice(0, n)]));
    }
  }
  return new Map(ids.map((i) => [i, i]));
}
