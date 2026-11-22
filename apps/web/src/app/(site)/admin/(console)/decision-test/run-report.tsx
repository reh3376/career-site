"use client";

import { useState } from "react";

import {
  downloadDecisionTestReportAction,
  generateDecisionTestReportAction,
  sendDecisionTestReportAction,
} from "./actions";
import { readReport, type Report } from "./report-state";

// The run's results report, from the console.
//
// Every finished run gets one, in two copies (FSD FR-DT-21). The
// participant's copy describes answers by kind and is the one that can
// be sent. The owner's copy adds the questions with the answers given
// and the correct answers, so it is the answer key for the questions
// this run drew, and its button says so before it is pressed.
//
// A client component because all four actions answer with something to
// show: a file to save, a new version, a send recorded, or the reason
// the api refused.

function savePdf(name: string, base64: string) {
  const raw = atob(base64);
  const bytes = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  const url = URL.createObjectURL(new Blob([bytes], { type: "application/pdf" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

const quiet =
  "rounded-md border border-line px-5 py-2.5 font-mono text-[11px] tracking-[0.14em] text-ink uppercase transition-colors hover:border-accent disabled:opacity-50 disabled:hover:border-line";
const loud =
  "rounded-md bg-accent px-5 py-2.5 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase disabled:opacity-50";

export function RunReport({ sessionKey, initial }: { sessionKey: string; initial?: Report }) {
  const [report, setReport] = useState<Report | undefined>(initial);
  const [busy, setBusy] = useState<string | null>(null);
  const [said, setSaid] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Sending reaches somebody outside this console, so it takes a second,
  // deliberate press rather than one.
  const [confirming, setConfirming] = useState(false);

  const reading = readReport(report);

  async function run(name: string, work: () => Promise<void>) {
    setBusy(name);
    setSaid(null);
    setError(null);
    try {
      await work();
    } catch (e) {
      setError(e instanceof Error ? e.message : "That did not work.");
    } finally {
      setBusy(null);
    }
  }

  const download = (owner: boolean) =>
    run(owner ? "owner" : "participant", async () => {
      const r = await downloadDecisionTestReportAction(sessionKey, owner);
      if ("error" in r) {
        setError(r.error);
        return;
      }
      savePdf(r.filename, r.pdfBase64);
      setSaid(`Saved ${r.filename} (version ${r.version}).`);
    });

  const rebuild = () =>
    run("build", async () => {
      const r = await generateDecisionTestReportAction(sessionKey);
      if (r.error) {
        setError(r.error);
        return;
      }
      setReport(r.report);
      if (r.report?.state === "ready" && !r.report.error) {
        setSaid(`Built version ${r.report.version}. Nothing was sent.`);
      } else {
        setError(r.report?.error ? `The build failed: ${r.report.error}` : "The build failed.");
      }
    });

  const send = () =>
    run("send", async () => {
      setConfirming(false);
      const r = await sendDecisionTestReportAction(sessionKey);
      if (r.error) {
        setError(r.error);
        return;
      }
      setReport(r.report);
      setSaid(`Sent version ${r.report?.version ?? ""} to the participant.`);
    });

  return (
    <div className="mt-10 max-w-2xl rounded-md border border-line bg-paper-2 px-6 py-5">
      <h2 className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">Results report</h2>
      <p className="mt-4 text-sm font-medium text-ink">{reading.headline}</p>
      <ul className="mt-2 space-y-1.5 text-sm leading-relaxed text-ink-2">
        {reading.lines.map((line) => (
          <li key={line}>{line}</li>
        ))}
      </ul>

      {reading.ready ? (
        <>
          <div className="mt-6 flex flex-wrap gap-3">
            <button type="button" onClick={() => download(false)} disabled={busy !== null} className={loud}>
              {busy === "participant" ? "Fetching" : "Participant's copy"}
            </button>
            <button type="button" onClick={() => download(true)} disabled={busy !== null} className={quiet}>
              {busy === "owner" ? "Fetching" : "Owner's copy"}
            </button>
          </div>
          <p className="mt-3 text-xs leading-relaxed text-ink-3">
            The owner&rsquo;s copy prints the thirty questions with the answers given and the
            correct answers. It is the answer key for this run&rsquo;s questions: not for the
            participant, or for anyone who may take the test.
          </p>
        </>
      ) : null}

      <div className="mt-6 border-t border-line pt-5">
        <div className="flex flex-wrap items-center gap-3">
          <button type="button" onClick={rebuild} disabled={busy !== null} className={quiet}>
            {busy === "build" ? "Building" : reading.ready ? "Build again" : "Build now"}
          </button>
          {reading.ready && !confirming ? (
            <button
              type="button"
              onClick={() => setConfirming(true)}
              disabled={busy !== null || !reading.canSend}
              className={quiet}
            >
              {busy === "send" ? "Sending" : "Send to participant"}
            </button>
          ) : null}
          {confirming ? (
            <>
              <span className="text-sm text-ink-2">
                Email version {report?.version} to the participant?
              </span>
              <button type="button" onClick={send} disabled={busy !== null} className={loud}>
                Send
              </button>
              <button type="button" onClick={() => setConfirming(false)} disabled={busy !== null} className={quiet}>
                Cancel
              </button>
            </>
          ) : null}
        </div>
        <p className="mt-3 text-xs leading-relaxed text-ink-3">
          Building again makes a new version against whoever has finished by now, so the
          ranking can move; earlier versions are kept and nothing is sent. {reading.sendNote}
        </p>
      </div>

      {said ? <p className="mt-4 text-sm text-ink-2">{said}</p> : null}
      {error ? <p className="mt-4 text-sm text-signal">{error}</p> : null}
    </div>
  );
}
