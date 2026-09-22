import "server-only";

import { callApi } from "@/lib/api-fetch";
import { getJdThreshold } from "@/lib/jd-threshold";

export type JdBands = {
  veryStrong: number;
  strong: number;
  possible: number;
  weak: number;
};

// The fit bands in force, from the api (owner-editable in /admin/jd).
// Falls back to the env gate when the call fails, so the JD pages
// always have a number to quote.
export async function getJdBands(cookie: string | null | undefined): Promise<JdBands> {
  const gate = Number(getJdThreshold());
  const fallback: JdBands = {
    veryStrong: Math.min(1, gate + 0.15),
    strong: gate,
    possible: Math.max(0.05, gate - 0.15),
    weak: Math.max(0.01, gate - 0.35),
  };
  if (!cookie) return fallback;
  try {
    const resp = await callApi({
      path: "/api/career.v1.JdService/GetJdReviewConfig",
      body: {},
      cookie,
    });
    if (!resp.ok) return fallback;
    const j = (await resp.json()) as {
      bands?: {
        veryStrong?: number;
        very_strong?: number;
        strong?: number;
        possible?: number;
        weak?: number;
      };
    };
    const b = j.bands;
    if (!b) return fallback;
    return {
      veryStrong: Number(b.veryStrong ?? b.very_strong ?? fallback.veryStrong),
      strong: Number(b.strong ?? fallback.strong),
      possible: Number(b.possible ?? fallback.possible),
      weak: Number(b.weak ?? fallback.weak),
    };
  } catch {
    return fallback;
  }
}
