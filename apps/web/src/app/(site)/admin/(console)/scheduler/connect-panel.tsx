"use client";

import { useState, useTransition } from "react";

import { disconnectCalendar, getConnectUrl, type CalendarStatus } from "./actions";

function when(iso?: string) {
  if (!iso) return "";
  return new Date(iso).toLocaleString("en-US", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "America/New_York",
  });
}

export function ConnectPanel({ status }: { status: CalendarStatus }) {
  const [error, setError] = useState("");
  const [busy, start] = useTransition();
  const [confirming, setConfirming] = useState(false);

  function connect() {
    setError("");
    start(async () => {
      const { url, error: err } = await getConnectUrl();
      if (err || !url) {
        setError(err || "A connect link could not be built.");
        return;
      }
      // Google's consent screen, not an embedded frame: a page asking
      // for Google credentials inside our own chrome is the exact shape
      // of a phishing page and should be refused by habit.
      window.location.href = url;
    });
  }

  function disconnect() {
    setError("");
    start(async () => {
      const { ok, error: err } = await disconnectCalendar();
      if (!ok) setError(err || "The calendar could not be disconnected.");
      setConfirming(false);
    });
  }

  // Not configured is a different problem from not connected, and has a
  // different fix: environment variables rather than a button.
  if (!status.configured) {
    return (
      <div className="mt-6 rounded-md border border-signal/40 bg-paper-2/60 p-4">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">calendar</p>
        <p className="mt-2 text-sm text-ink-2">
          This deployment cannot connect a calendar yet. It needs{" "}
          <code className="font-mono text-xs">GOOGLE_CLIENT_ID</code>,{" "}
          <code className="font-mono text-xs">GOOGLE_CLIENT_SECRET</code> and{" "}
          <code className="font-mono text-xs">SECRETS_KEY</code> set in the environment.
        </p>
        <p className="mt-2 text-xs text-ink-3">
          The settings below are still saved and take effect the moment a calendar is connected.
        </p>
      </div>
    );
  }

  return (
    <div
      className={
        status.connected && !status.lastError
          ? "mt-6 rounded-md border border-line bg-paper-2/60 p-4"
          : "mt-6 rounded-md border border-signal/40 bg-paper-2/60 p-4"
      }
    >
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">calendar</p>

      {status.connected ? (
        <>
          <p className="mt-2 text-sm text-ink-2">
            Connected as <span className="text-ink">{status.accountEmail || "a Google account"}</span>
            {status.connectedAt ? ` since ${when(status.connectedAt)}` : ""}. Free/busy only, never
            event contents.
          </p>
          {/* "Never worked" and "worked until Tuesday" need different
              responses, so they are not shown the same way. */}
          {status.lastError ? (
            <p className="mt-2 text-sm text-signal">
              Last call failed: {status.lastError}
              {status.lastOkAt ? ` It last worked at ${when(status.lastOkAt)}.` : " It has never worked."}
            </p>
          ) : status.lastOkAt ? (
            <p className="mt-2 text-xs text-ink-3">Last answered at {when(status.lastOkAt)}.</p>
          ) : (
            <p className="mt-2 text-xs text-ink-3">Not used yet.</p>
          )}
        </>
      ) : (
        <p className="mt-2 text-sm text-ink-2">
          No calendar is connected, so nothing can be booked. Members see that booking is not
          switched on rather than an empty calendar.
        </p>
      )}

      {error ? (
        <p role="alert" className="mt-3 text-sm text-signal">
          {error}
        </p>
      ) : null}

      <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-3">
        <button
          type="button"
          onClick={connect}
          disabled={busy}
          className="inline-flex items-center rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-60"
        >
          {status.connected ? "Reconnect" : "Connect Google Calendar"}
        </button>
        {status.connected ? (
          confirming ? (
            <>
              <span className="text-sm text-ink-2">
                Disconnect? New bookings stop. Meetings already agreed are left alone.
              </span>
              <button
                type="button"
                onClick={disconnect}
                disabled={busy}
                className="text-sm text-signal underline decoration-line decoration-1 underline-offset-4"
              >
                Yes, disconnect
              </button>
              <button
                type="button"
                onClick={() => setConfirming(false)}
                className="text-sm text-ink-3 hover:text-accent"
              >
                Keep it
              </button>
            </>
          ) : (
            <button
              type="button"
              onClick={() => setConfirming(true)}
              className="text-sm text-ink-3 transition-colors hover:text-signal"
            >
              Disconnect
            </button>
          )
        ) : null}
      </div>
    </div>
  );
}
