import type { Metadata } from "next";

import {
  inSection,
  listCredentials,
  SECTIONS,
  STATUS_LABEL,
} from "@/lib/credentials";

// Awards, education, service, certifications and eligibility
// (FR-CNT-10).
//
// The lapsed certifications are published as lapsed rather than
// omitted. Listing only what is current is the CV convention, and the
// convention is what lets a reader assume the omissions are flattering.
// Saying "held it, let it go, kept doing the work" costs nothing and is
// the only version of this page worth reading.

export const metadata: Metadata = {
  title: "Awards and credentials",
  description:
    "Recognition, degrees, certifications including the lapsed ones, military service, and availability.",
  alternates: { canonical: "/credentials" },
};

export default async function CredentialsPage() {
  const credentials = await listCredentials();

  return (
    <div className="mx-auto max-w-3xl px-6 py-24 sm:px-10 sm:py-32">
      <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        on the record
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Awards and credentials.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
        Including the certifications that have lapsed. Listing only what
        is current is the convention, and the convention is what lets a
        reader assume the gaps are flattering.
      </p>

      {credentials.length === 0 ? (
        <p className="mt-10 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Nothing is published yet.
        </p>
      ) : (
        <div className="mt-16 space-y-14">
          {SECTIONS.map(({ category, heading }) => {
            const items = inSection(credentials, category);
            if (items.length === 0) return null;
            return (
              <section key={category}>
                <h2 className="font-mono text-[11px] tracking-[0.14em] text-accent uppercase">
                  {heading}
                </h2>
                <ul className="mt-5 divide-y divide-line border-y border-line">
                  {items.map((c) => (
                    <li key={c.id} className="py-5">
                      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                        <h3 className="text-base font-medium text-ink">
                          {c.title}
                          {c.status && STATUS_LABEL[c.status] ? (
                            <span className="ml-2 font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
                              {STATUS_LABEL[c.status]}
                            </span>
                          ) : null}
                        </h3>
                        {c.awarded ? (
                          <p className="font-mono text-[11px] text-ink-3 tabular-nums">
                            {c.awarded}
                          </p>
                        ) : null}
                      </div>
                      {c.issuer ? (
                        <p className="mt-1 text-sm text-ink-2">{c.issuer}</p>
                      ) : null}
                      {c.note ? (
                        <p className="mt-2 text-base leading-relaxed text-ink-2">
                          {c.note}
                        </p>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}
