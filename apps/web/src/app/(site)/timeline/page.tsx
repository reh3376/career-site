import type { Metadata } from "next";

import { careerSpan, listRoles } from "@/lib/roles";

// The career timeline (FR-CNT-01).
//
// A server component with no client JavaScript at all, which is
// deliberate rather than incidental: this is the page a recruiter opens
// on a phone on a bad connection, and there is nothing on it that needs
// a browser to run. FR-PUB-05's check does not cover this route today,
// but the page is built so it would pass.
//
// Dates, titles and organisations come from content/roles/*.md, whose
// authority is docs/personal/career-facts.md. Nothing here restates a
// date, so nothing here can disagree with that sheet.

export const metadata: Metadata = {
  title: "Timeline",
  description:
    "Thirty years of engineering roles, in order: organisation, title, dates, and what was built.",
  alternates: { canonical: "/timeline" },
};

export default async function TimelinePage() {
  const roles = await listRoles();
  const span = careerSpan(roles);

  return (
    <div className="mx-auto max-w-3xl px-6 py-24 sm:px-10 sm:py-32">
      <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        the record
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Where the thirty years went.
      </h1>

      {roles.length ? (
        <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
          {span.from} to {span.to}, {roles.length} roles. Regulated
          manufacturing throughout, in three industries that each punish
          the same mistake differently.
        </p>
      ) : null}

      {roles.length === 0 ? (
        <p className="mt-10 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          No roles are published yet.
        </p>
      ) : (
        <ol className="mt-16 space-y-14 border-l border-line pl-6 sm:pl-8">
          {roles.map((r) => (
            /* The id is the anchor a skill's evidence links to
               (/timeline#role-joy-global). scroll-mt keeps the heading
               clear of the sticky header when somebody arrives on it. */
            <li key={r.id} id={r.id} className="relative scroll-mt-24">
              {/* The marker sits on the rule rather than beside it, so
                  the eye follows one line down the page. */}
              <span
                aria-hidden="true"
                className="absolute -left-[31px] top-2 h-2 w-2 rounded-full bg-accent sm:-left-[39px]"
              />
              <p className="font-mono text-[11px] tracking-[0.14em] text-accent uppercase tabular-nums">
                {r.start} to {r.end}
              </p>
              <h2
                className="font-display mt-2 text-2xl leading-tight text-ink"
                style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}
              >
                {r.title}
              </h2>
              <p className="mt-1 text-sm text-ink-2">
                <span className="text-ink">{r.organization}</span>
                {r.location ? ` · ${r.location}` : null}
              </p>

              <div className="mt-4 space-y-4 text-base leading-relaxed text-ink-2">
                {r.body.split(/\n\s*\n/).map((p) => (
                  <p key={p.slice(0, 32)}>{p.replace(/\n/g, " ")}</p>
                ))}
              </div>

              {r.technologies.length ? (
                <ul className="mt-5 flex flex-wrap gap-x-2 gap-y-2">
                  {r.technologies.map((t) => (
                    <li
                      key={t}
                      className="border border-line bg-paper-2 px-2.5 py-1 font-mono text-[10px] tracking-[0.1em] text-ink-3 uppercase"
                    >
                      {t}
                    </li>
                  ))}
                </ul>
              ) : null}
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
