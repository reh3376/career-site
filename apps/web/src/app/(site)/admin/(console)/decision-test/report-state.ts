// What the console says about a run's results report.
//
// Pure, and tested, for the reason the other readings on this page are:
// the sentence under a button decides whether the owner presses it, and
// "ranked 5th of 16" printed for a report that carries no ranking would
// be a claim about a participant that nothing made.

export type Report = {
  state?: string;
  version?: number;
  generatedAt?: string;
  compared?: boolean;
  noRankingReason?: string;
  others?: number;
  score?: number;
  total?: number;
  rank?: number;
  groupSize?: number;
  percentile?: number;
  participantPages?: number;
  ownerPages?: number;
  template?: string;
  error?: string;
  emailedAt?: string;
  emailedBy?: string;
  canSend?: boolean;
  participantFilename?: string;
  ownerFilename?: string;
  engine?: string;
};

export function ordinal(n: number): string {
  const tens = n % 100;
  if (tens >= 11 && tens <= 13) return `${n}th`;
  switch (n % 10) {
    case 1:
      return `${n}st`;
    case 2:
      return `${n}nd`;
    case 3:
      return `${n}rd`;
    default:
      return `${n}th`;
  }
}

// A fixed UTC rendering, so the server and the browser print the same
// thing and the page does not flicker on hydration.
export function when(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const s = d.toISOString();
  return `${s.slice(0, 10)} ${s.slice(11, 16)} UTC`;
}

// Why a report carries no ranking, in the report's own terms.
export function noRanking(reason: string | undefined, others: number): string {
  switch (reason) {
    case "repeat":
      return "No ranking: this was a repeat sitting, which is not compared with first sittings.";
    case "unplaced":
      return "No ranking: the test could not tell whether this was a first sitting.";
    case "synthetic":
      return "No ranking: this run was made by software to test the system.";
    default:
      return `No ranking: it needs ten other first-time participants on the same questions and time limits, and there ${
        others === 1 ? "is" : "are"
      } ${others}.`;
  }
}

export type ReportReading = {
  ready: boolean;
  headline: string;
  lines: string[];
  // Whether Send can be pressed, and the reason when it cannot.
  canSend: boolean;
  sendNote: string;
};

export function readReport(r?: Report): ReportReading {
  const state = r?.state ?? "none";
  if (state === "failed") {
    return {
      ready: false,
      headline: "The report could not be built",
      lines: [
        r?.error ? `Last error: ${r.error}` : "No error was recorded.",
        "A build that fails is tried again a few minutes apart, three times in all, unless the run itself is incomplete. After that the participant is sent their results without the PDF, and it can be built from here.",
      ],
      canSend: false,
      sendNote: "There is no report to send.",
    };
  }
  if (state !== "ready") {
    return {
      ready: false,
      headline: "No report yet",
      lines: [
        "One is built within a few minutes of a run finishing. A run that has not finished, or was closed without all thirty answers, does not get one.",
      ],
      canSend: false,
      sendNote: "There is no report to send.",
    };
  }

  const lines: string[] = [];
  const score = `${r?.score ?? 0} of ${r?.total ?? 0} correct.`;
  if (r?.compared && (r.rank ?? 0) > 0) {
    lines.push(
      `${score} Ranked ${ordinal(r.rank ?? 0)} of ${r.groupSize ?? 0} first-time participants, ${ordinal(
        Math.round(r.percentile ?? 0),
      )} percentile.`,
    );
  } else {
    lines.push(`${score} ${noRanking(r?.noRankingReason, r?.others ?? 0)}`);
  }
  lines.push(
    `Participant's copy ${r?.participantPages ?? 0} pages, owner's copy ${r?.ownerPages ?? 0} pages.`,
  );
  if (r?.emailedAt) {
    const by = r.emailedBy === "system" ? "automatically when the run finished" : `by ${r.emailedBy || "an admin"}`;
    lines.push(`Last emailed to the participant ${when(r.emailedAt)}, ${by}.`);
  } else {
    lines.push("Not emailed to the participant.");
  }
  if (r?.error) lines.push(`The last attempt to build it again failed: ${r.error}`);

  const canSend = Boolean(r?.canSend);
  return {
    ready: true,
    headline: `Version ${r?.version ?? 1}, built ${when(r?.generatedAt)}`,
    lines,
    canSend,
    sendNote: canSend
      ? "Sends the participant's copy to the address they left, with the results email."
      : "Cannot be sent: the participant did not ask for their results, or left no address.",
  };
}
