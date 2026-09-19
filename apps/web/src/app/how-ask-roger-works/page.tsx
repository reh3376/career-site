import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "How Ask Roger works",
  description:
    "How the assistant is built, what it draws from, and how it decides what it can and can't answer.",
};

export default function HowAskRogerWorksPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-16">
      <h1 className="mb-8 text-3xl font-semibold text-ink">How Ask Roger works</h1>
      <div className="space-y-6 leading-relaxed text-ink-2">
        <p>
          This page is under construction. When Ask Roger launches (late 2026), it will describe —
          plainly — how the assistant works: which model, which corpus, how it retrieves and cites,
          what it refuses to answer, how it&rsquo;s evaluated, and how you can hand a conversation
          to Roger directly.
        </p>
        <p className="text-sm text-ink-3">
          Short version, for now: Ask Roger is a first-person conversational assistant grounded in
          Roger&rsquo;s own writing, projects, and résumé. It cites its sources, admits what it
          doesn&rsquo;t know, and never speaks about compensation, references, current-employer
          confidential matters, or personal life.
        </p>
      </div>
    </div>
  );
}
