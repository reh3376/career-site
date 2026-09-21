import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "How Ask Roger works",
  description:
    "How the assistant is built, what it draws from, and how it decides what it can and can't answer.",
};

export default function HowAskRogerWorksPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        the assistant
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        How Ask Roger works.
      </h1>
      <p className="mt-6 inline-flex items-center gap-2 border border-signal bg-signal-soft/40 px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        <span className="pilot text-signal" aria-hidden="true" />
        status: not yet live
        <span className="text-ink-4">·</span>
        <span className="text-ink-2">available Q4 2026</span>
      </p>
      <div className="mt-10 space-y-6 text-base leading-relaxed text-ink-2">
        <p>
          This page is under construction. When Ask Roger launches (late
          2026), it will describe, plainly, how the assistant
          works: which model, which corpus, how it retrieves and cites, what
          it refuses to answer, how it&rsquo;s evaluated, and how you can
          hand a conversation to Roger directly.
        </p>
        <p className="text-sm text-ink-3">
          Short version, for now: Ask Roger is a first-person conversational
          assistant grounded in Roger&rsquo;s own writing, projects, and
          r&eacute;sum&eacute;. It cites its sources, admits what it
          doesn&rsquo;t know, and never speaks about compensation,
          references, current-employer confidential matters, or personal life.
        </p>
      </div>
    </div>
  );
}
