import { afterEach, describe, expect, it, vi } from "vitest";

import {
  beaconPending,
  enqueueRetry,
  flushNow,
  pendingCount,
  resetRetryQueue,
} from "./decision-test-retry";

// The retry queue, which exists because one dropped POST used to
// destroy one answer permanently and silently.
//
// These run on fake timers: the backoff is tens of seconds by design,
// and a test that actually waited for it would be a test nobody runs.

afterEach(() => {
  resetRetryQueue();
  vi.useRealTimers();
});

describe("the decision test retry queue", () => {
  it("resends a write that failed, and stops once it lands", async () => {
    vi.useFakeTimers();
    let attempts = 0;
    enqueueRetry("answer 7", async () => {
      attempts += 1;
      if (attempts < 3) throw new Error("network");
      return "ok";
    });

    expect(pendingCount()).toBe(1);
    // Three backoff windows is enough for two failures and a success.
    for (let i = 0; i < 4; i++) {
      await vi.advanceTimersByTimeAsync(25_000);
    }
    expect(attempts).toBe(3);
    expect(pendingCount()).toBe(0);
  });

  it("gives up out loud rather than silently", async () => {
    vi.useFakeTimers();
    const err = vi.spyOn(console, "error").mockImplementation(() => {});
    enqueueRetry("answer 12", async () => {
      throw new Error("still down");
    });

    for (let i = 0; i < 8; i++) {
      await vi.advanceTimersByTimeAsync(30_000);
    }
    expect(pendingCount()).toBe(0);
    // The whole point: a lost answer must be nameable afterwards.
    expect(err).toHaveBeenCalled();
    expect(String(err.mock.calls[0]?.[0])).toContain("answer 12");
    err.mockRestore();
  });

  it("does not hold a later write behind one that keeps failing", async () => {
    vi.useFakeTimers();
    vi.spyOn(console, "error").mockImplementation(() => {});
    const landed: string[] = [];
    enqueueRetry("stuck", async () => {
      throw new Error("down");
    });
    enqueueRetry("fine", async () => {
      landed.push("fine");
    });

    for (let i = 0; i < 6; i++) {
      await vi.advanceTimersByTimeAsync(30_000);
    }
    // The second write is on a connection that may have recovered, and
    // must not wait for the first to exhaust its attempts.
    expect(landed).toContain("fine");
  });

  it("flushNow retries immediately, without waiting out the backoff", async () => {
    vi.useFakeTimers();
    let attempts = 0;
    enqueueRetry("answer 3", async () => {
      attempts += 1;
      return "ok";
    });

    // Nothing has fired yet: the first backoff is a second away.
    expect(attempts).toBe(0);
    flushNow();
    await vi.advanceTimersByTimeAsync(0);
    expect(attempts).toBe(1);
    expect(pendingCount()).toBe(0);
  });
});

describe("beaconPending", () => {
  it("hands everything outstanding to the browser and empties the queue", () => {
    vi.useFakeTimers();
    const sent: { url: string; type: string; body: string }[] = [];
    const beacon = vi.fn((url: string, blob: Blob) => {
      sent.push({ url, type: blob.type, body: "" });
      return true;
    });
    vi.stubGlobal("navigator", { sendBeacon: beacon });

    enqueueRetry("answer 4", async () => undefined, {
      path: "/api/career.v1.DecisionTestService/SubmitAnswer",
      body: { sessionKey: "s", positionOverall: 4 },
    });
    enqueueRetry("recall 1", async () => undefined, {
      path: "/api/career.v1.DecisionTestService/SubmitRecall",
      body: { sessionKey: "s", blockNo: 1 },
    });

    expect(pendingCount()).toBe(2);
    beaconPending();

    // Both delivered, as JSON, to the right endpoints.
    expect(beacon).toHaveBeenCalledTimes(2);
    expect(sent.map((s) => s.url)).toEqual([
      "/api/career.v1.DecisionTestService/SubmitAnswer",
      "/api/career.v1.DecisionTestService/SubmitRecall",
    ]);
    expect(sent.every((s) => s.type === "application/json")).toBe(true);
    // And nothing is left to retry against a page that no longer exists.
    expect(pendingCount()).toBe(0);
    vi.unstubAllGlobals();
  });

  it("does nothing when the browser cannot beacon, rather than throwing", () => {
    vi.useFakeTimers();
    vi.stubGlobal("navigator", {});
    enqueueRetry("answer 9", async () => undefined, { path: "/x", body: {} });
    expect(() => beaconPending()).not.toThrow();
    // Left queued: a browser without sendBeacon still has the backoff.
    expect(pendingCount()).toBe(1);
    vi.unstubAllGlobals();
  });
});
