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
// instrument permanently and only has to escape once. Somebody who asked
// for their results gets them by email; the explanation of what the test
// was doing reaches everyone, once collection closes.
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
            No answers are published while the test is open.
          </strong>{" "}
          Everyone sees the same thirty questions, so one leaked answer
          would spoil it for everyone after you. The explanation of what
          was being measured, and why the wrong answers feel so right,
          goes up once collection closes.
        </p>
        <p>
          If you left an address, your own results come by email. If you
          did not, that is genuinely fine: the measurement does not need
          your name to work.
        </p>
      </div>
      <div className="mt-12">
        <Link
          href="/"
          className="rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase no-underline"
        >
          Back to the site
        </Link>
      </div>
    </div>
  );
}
