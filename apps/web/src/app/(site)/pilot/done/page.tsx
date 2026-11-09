import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";

import { requireSignedIn, unwrap } from "../gate";
import { getPilot, progressOf } from "../lib";

export const metadata: Metadata = { title: "Pilot · Thank you" };
export const dynamic = "force-dynamic";

// What happens next (docs/framework-research/15-pilot-on-site.md §1
// step 8). Reached by finishing; a participant who has not finished is
// sent back to the home, which knows what is left.
export default async function PilotDonePage() {
  await requireSignedIn("/pilot/done");
  const status = unwrap(await getPilot());
  if (!status.allowed) redirect("/pilot");
  const p = progressOf(status.session);
  if (!p.finished) redirect("/pilot");

  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">the pilot · done</p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Thank you.
      </h1>
      <div className="mt-8 max-w-2xl space-y-5 text-base leading-relaxed text-ink-2">
        <p>Your answers and comments are saved under your account. Nothing from them appears on any public page.</p>
        <p>
          <strong className="text-ink">The call.</strong> Roger will be in touch to find about thirty minutes. The
          first half is about the survey itself: which questions you read twice, which answers did not fit, how
          long it took. The second half is about what you wrote on the match record and the workflow chart.
        </p>
        <p>
          <strong className="text-ink">Before then.</strong> You can come back to{" "}
          <Link href="/pilot/record" className="text-accent underline decoration-accent/40 underline-offset-4 hover:decoration-accent">
            the match record
          </Link>{" "}
          and{" "}
          <Link href="/pilot/workflow" className="text-accent underline decoration-accent/40 underline-offset-4 hover:decoration-accent">
            the workflow
          </Link>{" "}
          and add or change comments. The survey answers stay as you submitted them.
        </p>
        <p>
          <strong className="text-ink">Afterwards.</strong> You get the summarised results when they exist, and a
          short list of what changed in the survey because of you.
        </p>
      </div>
      <p className="mt-10">
        <Link href="/pilot" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 no-underline hover:text-accent">
          ← pilot home
        </Link>
      </p>
    </div>
  );
}
