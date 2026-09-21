import type { Metadata } from "next";

import { JdForm } from "./form";

export const metadata: Metadata = {
  title: "JD upload",
  description:
    "Paste a job description; Roger scores it against his experience and — when the fit clears the threshold — sends back a résumé tailored to that specific posting.",
};

export default function JdUploadPage() {
  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        for hiring managers
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Paste your JD.
      </h1>
      <p className="mt-6 max-w-2xl text-lg leading-relaxed text-ink-2">
        Roger keeps one master corpus — 30 years across control-room
        engineering, manufacturing AI, and decision infrastructure —
        and produces a 2-page résumé tailored to whatever role
        you&rsquo;re considering him for. Drop the posting below;
        if the match clears the threshold, you get the tailored PDF.
      </p>

      <section className="mt-12 grid gap-4 border-l-2 border-line pl-5 text-sm leading-relaxed text-ink-2">
        <p>
          <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            what happens
          </span>
        </p>
        <ol className="ml-4 list-decimal space-y-1.5 text-sm">
          <li>The full JD text lands in Roger&rsquo;s inbox with a match score.</li>
          <li>
            <span className="text-ink">Score ≥ 0.65:</span> a 2-page résumé is
            generated from the master corpus, tailored to the posting.
          </li>
          <li>
            <span className="text-ink">Below threshold:</span> Roger will still
            take a look and reply personally if the role looks worth it.
          </li>
        </ol>
        <p className="text-xs text-ink-3">
          The scoring + generation pipeline is being built out;
          today the acknowledgement is manual. See{" "}
          <a
            href="/how-ask-roger-works"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            how this will work
          </a>
          .
        </p>
      </section>

      <div className="mt-14">
        <JdForm />
      </div>

      <p className="mt-14 border-t border-line pt-6 text-sm text-ink-3">
        Rather email?{" "}
        <a
          href="/contact"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          Use the contact form
        </a>
        .
      </p>
    </div>
  );
}
