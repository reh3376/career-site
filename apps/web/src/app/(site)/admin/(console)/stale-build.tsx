"use client";

import { useEffect, useState } from "react";

// Tells the operator when the page in front of them belongs to a
// deploy that no longer exists.
//
// A tab opened before a rollout keeps server action identifiers from
// the build it was rendered by. After the containers restart, a form
// submitted from that tab is discarded rather than run: the click looks
// accepted, nothing is written, and no error appears anywhere the
// person is looking. That is how three golden-set labels were lost on
// 2026-09-23, and the loss was only found by reading the database.
//
// The check is the API's reported version, because web and api roll out
// together under one image tag, so the version the API reports is the
// identity of the build that rendered this page. If the two disagree,
// the page is stale and anything submitted from it may vanish.

export function StaleBuild({ renderedVersion }: { renderedVersion: string }) {
  const [liveVersion, setLiveVersion] = useState("");

  useEffect(() => {
    if (!renderedVersion) return;
    let cancelled = false;

    const check = async () => {
      try {
        const resp = await fetch("/api/readyz", { cache: "no-store" });
        if (!resp.ok) return;
        const body = (await resp.json()) as { version?: string };
        if (!cancelled && body.version) setLiveVersion(body.version);
      } catch {
        // Offline, or the API is mid-restart. Neither is evidence that
        // the page is stale, so say nothing.
      }
    };

    void check();
    // A rollout is most likely to have happened while the tab sat in
    // the background, so the useful moment to look is when it comes
    // back, plus a slow interval for a tab left open in view.
    const onVisible = () => {
      if (document.visibilityState === "visible") void check();
    };
    document.addEventListener("visibilitychange", onVisible);
    const timer = window.setInterval(check, 120_000);

    return () => {
      cancelled = true;
      document.removeEventListener("visibilitychange", onVisible);
      window.clearInterval(timer);
    };
  }, [renderedVersion]);

  if (!liveVersion || !renderedVersion || liveVersion === renderedVersion) {
    return null;
  }

  return (
    <div
      role="alert"
      className="fixed inset-x-0 top-0 z-50 border-b-2 border-danger bg-paper-2 px-6 py-3"
    >
      <div className="mx-auto flex max-w-6xl flex-wrap items-baseline gap-x-4 gap-y-1">
        <p className="text-sm text-ink">
          This page came from an earlier deploy. Anything you submit from it
          will be discarded without saving.
        </p>
        <button
          type="button"
          onClick={() => window.location.reload()}
          className="border border-danger px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-danger transition-colors hover:bg-danger hover:text-paper"
        >
          reload
        </button>
        <p className="font-mono text-[11px] text-ink-3">
          page {renderedVersion} <span className="text-ink-4">·</span> server{" "}
          {liveVersion}
        </p>
      </div>
    </div>
  );
}
