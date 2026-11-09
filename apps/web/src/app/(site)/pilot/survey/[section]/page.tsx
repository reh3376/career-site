import type { Metadata } from "next";
import Link from "next/link";

import { requireSignedIn, unwrap } from "../../gate";
import { getPilotSection } from "../../lib";

import { SectionForm } from "./section-form";

export const metadata: Metadata = { title: "Pilot · Survey" };
export const dynamic = "force-dynamic";

// One survey section per screen (FSD FR-PLT-04..07). The section
// arrives with the caller's branch applied and the rows of C2 and C3
// resolved from C1, so this page renders what it is given and knows
// nothing about the instrument beyond the item types.
export default async function PilotSectionPage({ params }: { params: Promise<{ section: string }> }) {
  const { section } = await params;
  const id = section.toUpperCase().slice(0, 2);
  await requireSignedIn(`/pilot/survey/${id}`);
  const sec = unwrap(await getPilotSection(id));
  const index = sec.index ?? 0;
  const total = sec.total ?? 0;
  const pct = total > 0 ? Math.round((Math.max(index - 1, 0) / total) * 100) : 0;

  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:px-10 sm:py-24">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
          the pilot · survey
        </p>
        <Link href="/pilot" className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 no-underline hover:text-accent">
          ← pilot home
        </Link>
      </div>
      <p className="mt-6 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        section {index} of {total}
      </p>
      <div className="mt-2 h-[3px] w-full bg-paper-2" aria-hidden="true">
        <div className="h-full bg-accent transition-all" style={{ width: `${pct}%` }} />
      </div>
      <h1
        className="font-display mt-6 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {sec.title ?? id}
      </h1>
      {sec.intro ? <p className="mt-6 max-w-2xl text-base leading-relaxed text-ink-2">{sec.intro}</p> : null}

      <div className="mt-10">
        <SectionForm
          sectionId={sec.id}
          items={sec.items ?? []}
          answers={sec.answers ?? []}
          isLast={total > 0 && index >= total}
        />
      </div>
    </div>
  );
}
