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
//
// Without a session the member-only config call cannot be made, so the
// public status endpoint is asked instead: it carries the same stored
// bands. Only if both fail do we derive numbers from the env gate, and
// those are right only until the owner edits the bands. That mattered
// once /how-ask-roger-works became public, because the page tells the
// reader these are the numbers in force.
export async function getJdBands(cookie: string | null | undefined): Promise<JdBands> {
  const gate = Number(getJdThreshold());
  const fallback: JdBands = {
    veryStrong: Math.min(1, gate + 0.15),
    strong: gate,
    possible: Math.max(0.05, gate - 0.15),
    weak: Math.max(0.01, gate - 0.35),
  };
  if (!cookie) return (await publicBands()) ?? fallback;
  try {
    const resp = await callApi({
      path: "/api/career.v1.JdService/GetJdReviewConfig",
      body: {},
      cookie,
    });
    if (!resp.ok) return (await publicBands()) ?? fallback;
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

// The stored bands from the public status endpoint, or undefined.
async function publicBands(): Promise<JdBands | undefined> {
  try {
    const resp = await callApi({
      path: "/api/career.v1.SystemService/GetReviewerStatus",
      body: {},
    });
    if (!resp.ok) return undefined;
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
    if (!b || typeof b.strong !== "number") return undefined;
    const veryStrong = b.veryStrong ?? b.very_strong;
    if (
      typeof veryStrong !== "number" ||
      typeof b.possible !== "number" ||
      typeof b.weak !== "number"
    ) {
      return undefined;
    }
    return {
      veryStrong,
      strong: b.strong,
      possible: b.possible,
      weak: b.weak,
    };
  } catch {
    return undefined;
  }
}
