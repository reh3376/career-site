import { describe, expect, it } from "vitest";

import { abbreviate } from "./abbreviate";

// The abbreviation carries a promise, so it is tested rather than eyeballed.
//
// The owner asked for the run id on screen for one reason: to name a run
// unambiguously when discussing it. A shortened id that could name two
// runs would defeat that while appearing to satisfy it, and it would
// fail silently, on exactly the day a second run happened to share a
// prefix. The failure would look like a conversation about the wrong
// participant's data.

describe("abbreviate", () => {
  it("uses the minimum width when that is already unique", () => {
    const ids = [
      "11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
      "22222222-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
    ];
    const m = abbreviate(ids);
    expect(m.get(ids[0])).toBe("11111111");
    expect(m.get(ids[1])).toBe("22222222");
  });

  it("widens until every prefix is distinct", () => {
    // These first differ at index 12 ("...cde1" against "...cde2"), so
    // eight characters collide and thirteen is the shortest width that
    // separates them.
    const ids = [
      "abcdefab-cde1-4aaa-8aaa-aaaaaaaaaaaa",
      "abcdefab-cde2-4bbb-8bbb-bbbbbbbbbbbb",
    ];
    const m = abbreviate(ids);
    const got = ids.map((i) => m.get(i)!);
    expect(new Set(got).size).toBe(2);
    expect(got[0]).toBe("abcdefab-cde1");
    expect(got[0].length).toBe(13);
  });

  it("never returns the same abbreviation for two different runs", () => {
    // A hundred ids sharing a long common head, which is the shape a
    // sequential or time-prefixed id generator would produce.
    const ids = Array.from(
      { length: 100 },
      (_, i) => `aaaaaaaa-aaaa-4aaa-8aaa-${String(i).padStart(12, "0")}`,
    );
    const m = abbreviate(ids);
    expect(new Set([...m.values()]).size).toBe(ids.length);
    for (const id of ids) expect(id.startsWith(m.get(id)!)).toBe(true);
  });

  it("falls back to the full id rather than returning a collision", () => {
    // Two identical ids cannot be told apart at any width. Returning
    // the full id is wrong but honest; returning a shared prefix would
    // be wrong and hidden.
    const ids = ["same-id", "same-id"];
    const m = abbreviate(ids);
    expect(m.get("same-id")).toBe("same-id");
  });

  it("handles an empty list", () => {
    expect(abbreviate([]).size).toBe(0);
  });

  it("always yields a real prefix of the id it maps", () => {
    const ids = [
      "0f8c2d41-9b3e-4c7a-8d2f-1a2b3c4d5e6f",
      "0f8c2d41-9b3e-4c7a-8d2f-1a2b3c4d5e70",
      "ff000000-0000-4000-8000-000000000000",
    ];
    const m = abbreviate(ids);
    for (const id of ids) expect(id.startsWith(m.get(id)!)).toBe(true);
    expect(new Set([...m.values()]).size).toBe(3);
  });
});
