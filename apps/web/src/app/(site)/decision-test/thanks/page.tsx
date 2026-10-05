import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Thank you",
  robots: { index: false, follow: false },
};

// The end of the run.
//
// No score here and no per-question answers anywhere, ever. The item set
// is standardized, so an answer key that escapes contaminates the
// instrument permanently and only has to escape once.
//
// The explanation reaches everyone from here, now, whether or not they
// left an address. Until 2026-10-05 this page said it would go up "once
// collection closes" and linked only to the landing page, which meant a
// participant who gave nothing finished fifteen minutes of deliberate
// effort, was led into confident error by design, and was told nothing.
// That is the gotcha outcome the ethics section exists to prevent, and
// it fell hardest on the most generous participants. The personal
// numbers still need an address; the explanation never did.
export default function ThanksPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-24 sm:px-10 sm:py-32">
      <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        done
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Thank you. That was the hard part.
      </h1>
      <div className="mt-8 space-y-5 text-base leading-relaxed text-ink-2">
        <p>
          Fifteen minutes of real attention is a lot to give someone, and
          it is the whole input to this. Your run is recorded.
        </p>
        <p>
          <strong className="text-ink">
            What was being measured, and why the wrong answers feel so
            right, is explained now.
          </strong>{" "}
          You do not need to have left an address to read it, and it
          reveals no answers: everyone sees the same thirty questions,
          so one leaked answer would spoil it for everyone after you.
          The questions themselves are published once collection closes.
        </p>
        <p>
          If you left an address, your own numbers come by email. If you
          did not, that is genuinely fine. The measurement does not need
          your name to work, and the explanation above was never the
          part that depended on it.
        </p>
      </div>
      <div className="mt-12 flex flex-wrap gap-4">
        <Link
          href="/decision-test/about"
          className="rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase no-underline"
        >
          What this was measuring
        </Link>
        <Link
          href="/"
          className="rounded-md border border-line px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-ink-2 uppercase no-underline"
        >
          Back to the site
        </Link>
      </div>
    </div>
  );
}
