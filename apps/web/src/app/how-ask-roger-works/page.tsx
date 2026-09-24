import type { Metadata } from "next";
import Link from "next/link";

import { getJdBands } from "@/lib/jd-bands";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = {
  title: "How Ask Roger works",
  description:
    "How the JD reviewer works today, and how the Ask Roger assistant will be built, what it draws from, and how it decides what it can and can't answer.",
};
export const dynamic = "force-dynamic";

export default async function HowAskRogerWorksPage() {
  const bands = await getJdBands(await getSessionCookie());
  const veryStrong = bands.veryStrong.toFixed(2);
  const strong = bands.strong.toFixed(2);
  const possible = bands.possible.toFixed(2);
  const weak = bands.weak.toFixed(2);
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
          Ask Roger, the conversational assistant, is not built yet. When
          it launches (late 2026) this page will describe, plainly, how it
          works: which model, which corpus, how it retrieves and cites,
          what it refuses to answer, how it&rsquo;s evaluated, and how you
          can hand a conversation to Roger directly.
        </p>
        <p className="text-sm text-ink-3">
          Short version of the plan: Ask Roger will be a first-person
          conversational assistant grounded in Roger&rsquo;s own writing,
          projects, and r&eacute;sum&eacute;. It will cite its sources,
          admit what it doesn&rsquo;t know, and never speak about
          compensation, references, current-employer confidential matters,
          or personal life.
        </p>
      </div>

      <section className="mt-14 border-t border-line pt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
          live today
        </p>
        <h2
          className="font-display mt-3 text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          The JD reviewer.
        </h2>
        <p className="mt-4 text-base leading-relaxed text-ink-2">
          The review behind{" "}
          <Link
            href="/jd-upload"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            /jd-upload
          </Link>{" "}
          is the part of the system that already runs. It draws on the
          same career corpus Ask Roger will use, and it works like this.
        </p>
        <ol className="mt-6 ml-4 list-decimal space-y-3 text-sm leading-relaxed text-ink-2">
          <li>
            <span className="text-ink">Requirements.</span> The posting is
            broken into checkable requirements, each marked must or
            preferred and weighted 1 to 3. Clauses like &ldquo;or
            equivalent&rdquo; are kept as written.
          </li>
          <li>
            <span className="text-ink">Evidence.</span> For each
            requirement the reviewer retrieves the most relevant passages
            from Roger&rsquo;s career corpus and adds a short sheet of
            career facts, so the judge always sees the basics.
          </li>
          <li>
            <span className="text-ink">Verdicts.</span> A judge model reads
            that evidence and returns one verdict per requirement: met,
            partial or unmet, with a one-sentence reason. Each requirement
            gets its own call, so the model never has to hold the whole
            posting at once.
          </li>
          <li>
            <span className="text-ink">Score.</span> The score is computed
            in code from those verdicts, not guessed by the model. Met
            counts 1, partial 0.5, unmet 0, each weighted. Must-have items
            always count against the total; preferred items only count
            when they are evidenced.
          </li>
          <li>
            <span className="text-ink">Fit category.</span> The score
            lands in one of five categories: very strong (&ge; {veryStrong}),
            strong (&ge; {strong}), possible (&ge; {possible}), weak
            (&ge; {weak}), and very weak below that. Roger can adjust the
            bands; the numbers here are the ones in force.
          </li>
          <li>
            <span className="text-ink">R&eacute;sum&eacute;.</span> When
            the fit is strong or better, a second model writes a two-page
            r&eacute;sum&eacute; for that posting. Every claim in it carries
            a source id that is checked in code against the corpus before
            the r&eacute;sum&eacute; is rendered as a PDF. The PDF opens
            without a password but is locked against editing.
          </li>
        </ol>
        <p className="mt-6 text-sm leading-relaxed text-ink-2">
          Reviews run one at a time on Roger&rsquo;s own server and usually
          take 30 to 60 minutes, sometimes longer. You can close the page:
          your submissions
          list on /jd-upload reopens a review, and the outcome is emailed
          to you.
        </p>
        <p className="mt-4 text-sm leading-relaxed text-ink-2">
          The judge and the r&eacute;sum&eacute; writer are open-weight
          models served through Ollama. Every verdict is logged with the
          evidence the model saw and the model and prompt version that
          produced it. Roger reviews those decisions himself, and the
          labels are collected to evaluate and train the next version.
        </p>
      </section>
    </div>
  );
}
