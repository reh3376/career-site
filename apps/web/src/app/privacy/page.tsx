import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Privacy",
  description: "How this site collects, stores, and handles your information.",
};

export default function PrivacyPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        the small print
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Privacy.
      </h1>
      <div className="mt-10 space-y-6 text-base leading-relaxed text-ink-2">
        <p>
          This site is under construction. The full privacy policy &mdash;
          covering what is collected, how long it&rsquo;s retained, how to
          export or delete your data, and how conversations with Ask Roger are
          stored &mdash; ships alongside the launch (late 2026).
        </p>
        <p className="text-sm text-ink-3">
          Until then, the only data collected from a request-access form is
          your name, email, organization, and stated role. It is used only to
          review your access request and, if approved, to send you a sign-in
          notification. Nothing is sold, shared with third parties, or used
          for advertising.
        </p>
        <p className="text-sm text-ink-3">
          Questions?{" "}
          <a
            href="mailto:rogerhenley345@gmail.com"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            rogerhenley345@gmail.com
          </a>
        </p>
      </div>
    </div>
  );
}
