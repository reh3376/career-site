import type { Metadata } from "next";
import Link from "next/link";

import { byCategory, DEPTH_LABEL, listSkills } from "@/lib/skills";

// The skills matrix (FR-CNT-09).
//
// Every row carries its evidence as links, because that is the row's
// actual condition and the only thing that makes a skills table worth
// reading. The evidence is resolved against the published timeline,
// projects and articles at render time, so a row cannot outlive the
// thing it points at: see lib/skills.ts.

export const metadata: Metadata = {
  title: "Skills",
  description:
    "What I can do, how deep it goes, how long I have done it, and where to check each one.",
  alternates: { canonical: "/skills" },
};

export default async function SkillsPage() {
  const skills = await listSkills();
  const groups = byCategory(skills);

  return (
    <div className="mx-auto max-w-3xl px-6 py-24 sm:px-10 sm:py-32">
      <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        with the receipts
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Skills, and where to check them.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
        Every line here links to something you can read: a job, a
        repository, or a piece of writing. A skills list without that is
        a list of adjectives, and you already know to discount those.
      </p>

      {skills.length === 0 ? (
        <p className="mt-10 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          No skills are published yet.
        </p>
      ) : (
        <div className="mt-16 space-y-16">
          {groups.map((g) => (
            <section key={g.category}>
              <h2 className="font-mono text-[11px] tracking-[0.14em] text-accent uppercase">
                {g.category}
              </h2>
              <ul className="mt-5 divide-y divide-line border-y border-line">
                {g.skills.map((s) => (
                  <li key={s.id} className="py-6">
                    <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                      <h3 className="text-lg font-medium text-ink">
                        {s.title}
                      </h3>
                      <p className="font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase tabular-nums">
                        {DEPTH_LABEL[s.depth]} · {s.years}{" "}
                        {s.years === 1 ? "year" : "years"}
                      </p>
                    </div>

                    {s.body ? (
                      <p className="mt-2 text-base leading-relaxed text-ink-2">
                        {s.body}
                      </p>
                    ) : null}

                    <p className="mt-3 flex flex-wrap items-baseline gap-x-4 gap-y-1 text-sm">
                      <span className="font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
                        evidence
                      </span>
                      {s.evidence.map((e) => (
                        <Link
                          key={e.href}
                          href={e.href}
                          className="text-accent no-underline decoration-accent/40 decoration-1 underline-offset-4 hover:underline"
                        >
                          {e.label}
                        </Link>
                      ))}
                    </p>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
