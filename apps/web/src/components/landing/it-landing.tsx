import Image from "next/image";
import Link from "next/link";

import { GithubReposIt } from "@/components/github-repos";
import {
  ACCESS_CAVEAT,
  OPEN_TO_ANYONE,
  WITH_AN_ACCOUNT,
} from "@/lib/access-tiers";
import { getSocialLinks } from "@/lib/social-links";

// Landing. Editorial, one column, with image bands set into the reading
// gutter rather than filling the viewport. The type does most of the
// work, Fraunces variable at display sizes for the hero, real numbers
// set in the display face too so "30" and "8" carry weight instead of
// being data points glued next to prose.
//
// The four supporting photographs alternate between industrial work
// (distillation-column bubble tray, Hobet dragline) and human context (teaching at UK,
// the home workshop desk with the dog underfoot) so a visiting hiring
// manager sees a working engineer with a life, not a résumé PDF in HTML.

const OFFERINGS = [
  {
    title: "Digital transformation",
    body: "Take a plant from paper logbooks and disconnected historians to a data-first operation, instrumentation, data model, ontology, event streams, without stopping production.",
  },
  {
    title: "Process optimization",
    body: "Deep instrumentation, statistical process control, and model-based advanced control. Multi-percent yield gains and step-change reliability from what the plant already has.",
  },
  {
    title: "Automation & control",
    body: "Greenfield and brownfield: PLC / DCS design, control-narrative to commissioning, safety-instrumented systems, and the migrations most integrators won't touch.",
  },
  {
    title: "IT / OT convergence",
    body: "Bridge the plant network to the enterprise stack, safely. Segmented architectures, historian federation, MES / ERP integration, and the governance that keeps ops teams sleeping through the night.",
  },
  {
    title: "Industrial DataOps + applied AI",
    body: "MDEMG, Forge, and the AI-ML patterns that actually ship on the floor: retrieval-augmented decision support, anomaly detection, quality prediction, closed-loop optimisation.",
  },
];

export function ItLanding({ signedIn = false }: { signedIn?: boolean }) {
  return (
    <>
      {/* -----------------------------------------------------------------
       * HERO
       * A single big Fraunces headline. The em-dash break is not decoration,
       * it's the point where the persona ("Roger Henley") stops and the
       * practice ("Industrial automation…") starts. The eyebrow is deliberately
       * lowercase mono; the AI default here would be tracked-out ALL CAPS.
       * ----------------------------------------------------------------- */}
      <section
        aria-labelledby="hero-heading"
        className="mx-auto max-w-5xl px-6 pb-16 pt-20 sm:px-10 sm:pb-24 sm:pt-28"
      >
        <p className="font-mono text-[11px] tracking-[0.14em] text-signal">
          coming soon <span className="text-ink-4">·</span> late 2026
        </p>
        <h1
          id="hero-heading"
          className="font-display mt-6 text-[clamp(3rem,7.4vw,6.75rem)] font-medium leading-[0.95] tracking-[-0.02em] text-ink"
          style={{ fontVariationSettings: '"opsz" 144, "SOFT" 40' }}
        >
          Industrial automation, plant operations,{" "}
          <span
            className="italic text-accent"
            style={{ fontVariationSettings: '"opsz" 144, "SOFT" 100' }}
          >
            applied&nbsp;AI.
          </span>
        </h1>
        <p className="mt-10 max-w-2xl text-lg leading-relaxed text-ink-2">
          Thirty years running regulated, 24-hour manufacturing. The last eight
          in bourbon, commissioning distillery startups from concrete pour
          to steady-state, in the kind of shifts you don&rsquo;t brag about. This
          site is the working version of that practice: what I do, the projects
          behind it, and how to reach me.
        </p>

        <div className="mt-10 flex flex-wrap items-center gap-x-5 gap-y-3">
          <Link
            href="/register"
            className="inline-flex items-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
          >
            Considering me for a role? Request access →
          </Link>
          <Link
            href="/contact"
            className="text-sm text-ink-2 underline decoration-line decoration-1 underline-offset-4 transition-colors hover:text-accent hover:decoration-accent"
          >
            Or reach out directly
          </Link>
          <Link
            href="/login"
            className="text-sm text-ink-3 no-underline transition-colors hover:text-accent"
          >
            Already a member? Sign in
          </Link>
        </div>
        <p className="mt-3 max-w-xl text-xs leading-relaxed text-ink-3">
          The writing and the work photos are open; read them first. An
          account is for the reviewer: paste a posting and it reads it
          requirement by requirement against thirty years of manufacturing
          and applied-AI records, then writes a two-page résumé for that
          specific job.
        </p>
      </section>

      {/* Rule with a thicker cap on one side, reads as a plotted trend
          starting, which is the right visual metaphor for this site. */}
      <div className="mx-auto max-w-5xl px-6 sm:px-10">
        <div className="rule-plot" />
      </div>

      {/* -----------------------------------------------------------------
       * NUMBERS BAND, real values used typographically. Not a stats
       * row of identical cards; a set of three assertions with a big
       * Fraunces number and a plain-language second line.
       * ----------------------------------------------------------------- */}
      <section
        aria-label="At-a-glance"
        className="mx-auto max-w-5xl px-6 py-20 sm:px-10 sm:py-24"
      >
        <dl className="grid gap-14 sm:grid-cols-3 sm:gap-10">
          <div>
            <dt className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              in the field
            </dt>
            <dd className="mt-3">
              <span
                className="font-display block text-6xl leading-none text-ink sm:text-7xl"
                style={{ fontVariationSettings: '"opsz" 144' }}
              >
                30<span className="text-ink-4">yr</span>
              </span>
              <span className="mt-3 block max-w-[20ch] text-sm leading-relaxed text-ink-2">
                Across power, mining, telecom, industrial automation, and applied AI.
              </span>
            </dd>
          </div>
          <div>
            <dt className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              distillery startups
            </dt>
            <dd className="mt-3">
              <span
                className="font-display block text-6xl leading-none text-ink sm:text-7xl"
                style={{ fontVariationSettings: '"opsz" 144' }}
              >
                8<span className="text-ink-4">yr</span>
              </span>
              <span className="mt-3 block max-w-[22ch] text-sm leading-relaxed text-ink-2">
                Whiskey House of Kentucky and its predecessors. Concrete pour to steady-state.
              </span>
            </dd>
          </div>
          <div>
            <dt className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              schedule
            </dt>
            <dd className="mt-3">
              <span
                className="font-display block text-6xl leading-none text-ink sm:text-7xl tabular"
                style={{ fontVariationSettings: '"opsz" 144' }}
              >
                24<span className="text-ink-4">/</span>7
              </span>
              <span className="mt-3 block max-w-[22ch] text-sm leading-relaxed text-ink-2">
                Regulated, continuous production. The kind that pages you at 3 a.m.
              </span>
            </dd>
          </div>
        </dl>
      </section>

      {/* -----------------------------------------------------------------
       * IMAGE + PULL QUOTE, top bubble tray of a continuous distillation
       * column, post-run. Set at full-column width, offset with a Fraunces
       * pull quote so the image reads as evidence, not decoration.
       * ----------------------------------------------------------------- */}
      <section aria-label="From the plant floor" className="bg-paper-2/60">
        <div className="mx-auto grid max-w-6xl gap-10 px-6 py-20 sm:px-10 md:grid-cols-[1.4fr_1fr] md:items-center md:py-28">
          <figure className="m-0">
            <div className="relative aspect-[4/3] overflow-hidden">
              <Image
                src="/images/bubble-tray.jpeg"
                alt="Top bubble tray of a continuous distillation column, viewed through the sight glass, rows of vapor caps against a wet copper wash."
                fill
                sizes="(min-width: 768px) 640px, 100vw"
                className="object-cover"
              />
            </div>
            <figcaption className="mt-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              column top bubble tray <span className="text-ink-4">·</span> post-run
            </figcaption>
          </figure>
          <blockquote className="border-l-2 border-accent pl-6 text-ink">
            <p
              className="font-display text-2xl leading-snug sm:text-3xl"
              style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
            >
              The abstract stuff has to survive a copper still, a
              fermentation cycle, and a plant manager who&rsquo;s been up
              for eighteen hours.
            </p>
            <footer className="mt-4 text-sm text-ink-3">
              On why control theory and production reality have to meet somewhere.
            </footer>
          </blockquote>
        </div>
      </section>

      {/* -----------------------------------------------------------------
       * WHAT I DO
       * Five practice areas. NOT numbered (they're not a sequence), NOT
       * cards (each is a distinct offering, not a swappable tile). Left
       * gutter carries the offering name in the display face; right
       * column carries the body. Two-up on wide screens, stacked on
       * mobile with the same visual rhythm.
       * ----------------------------------------------------------------- */}
      <section
        aria-labelledby="offerings-heading"
        className="mx-auto max-w-5xl px-6 py-24 sm:px-10 sm:py-32"
      >
        <div className="max-w-2xl">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            the practice
          </p>
          <h2
            id="offerings-heading"
            className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
            style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
          >
            Five areas, usually mixed together on one engagement.
          </h2>
          <p className="mt-5 text-base leading-relaxed text-ink-2">
            Naming them separately makes it easier for you to decide whether we
            should talk. Every real project pulls from at least three.
          </p>
        </div>

        <ul className="mt-16 divide-y divide-line border-y border-line">
          {OFFERINGS.map((o) => (
            <li key={o.title} className="grid gap-4 py-8 sm:grid-cols-[minmax(0,240px)_1fr] sm:gap-10 sm:py-10">
              <h3
                className="font-display text-2xl leading-tight text-ink"
                style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
              >
                {o.title}
              </h3>
              <p className="max-w-2xl text-base leading-relaxed text-ink-2">
                {o.body}
              </p>
            </li>
          ))}
        </ul>
      </section>

      {/* -----------------------------------------------------------------
       * IMAGE BAND, Roger speaking at UK podium (paired with a note
       * about writing, speaking, teaching). Explicit human-context image.
       * ----------------------------------------------------------------- */}
      <section aria-label="Talks and teaching" className="border-y border-line bg-paper">
        <div className="mx-auto grid max-w-6xl gap-12 px-6 py-20 sm:px-10 md:grid-cols-[1fr_1.2fr] md:items-center md:py-24">
          <figure className="m-0 md:order-2">
            <div className="relative aspect-[4/5] overflow-hidden">
              <Image
                src="/images/roger-podium-uk.jpeg"
                alt="Roger at a University of Kentucky podium giving a talk on enabling citizen developers with low-code tooling and BI."
                fill
                sizes="(min-width: 768px) 520px, 100vw"
                className="object-cover"
              />
            </div>
            <figcaption className="mt-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              University of Kentucky <span className="text-ink-4">·</span> talk on citizen developers
            </figcaption>
          </figure>
          <div className="md:order-1 md:pr-8">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              alongside the work
            </p>
            <h2
              className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
              style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
            >
              Talks, notes, and open source.
            </h2>
            <p className="mt-6 text-base leading-relaxed text-ink-2">
              I write about the parts of digital transformation that don&rsquo;t
              survive the vendor slides, how a control room actually
              adopts a new tool, how &ldquo;citizen developer&rdquo; enablement
              works when the citizens have wrenches on their belts, why the
              plant historian is a distributed database whether IT knows it
              or not.
            </p>
            <p className="mt-4 text-base leading-relaxed text-ink-2">
              Frameworks like <span className="font-mono text-ink">MDEMG</span> and{" "}
              <span className="font-mono text-ink">Forge</span> live on GitHub,
              free for use.
            </p>
            <SocialInline />
          </div>
        </div>
      </section>

      {/* Articles are members-only; the recent-writing strip lives on /home. */}

      {/* -----------------------------------------------------------------
       * HERO IMAGE BAND, the Hobet dragline, edge-to-edge, as the
       * anchor for the "background" section. Not the same treatment
       * as the smaller images above; this one is meant to feel like
       * standing on a bench at dusk.
       * ----------------------------------------------------------------- */}
      <section aria-label="Where the work happens" className="bg-ink text-paper">
        <figure className="m-0">
          <div className="relative aspect-[16/9] overflow-hidden sm:aspect-[21/9]">
            <Image
              src="/images/hobet-dragline.jpeg"
              alt="A dragline excavator at Hobet Mining at dusk, its boom silhouetted against the sky over a stripped bench."
              fill
              priority
              unoptimized
              sizes="100vw"
              className="object-cover"
            />
          </div>
        </figure>
        <div className="mx-auto max-w-5xl px-6 py-16 sm:px-10 sm:py-20">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-4">
            hobet mining <span className="text-ink-3">·</span> west virginia
          </p>
          <h2
            className="font-display mt-4 max-w-3xl text-4xl leading-[1.05] tracking-tight sm:text-5xl"
            style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
          >
            Thirty years of on-the-floor places.
          </h2>
          <p className="mt-6 max-w-2xl text-base leading-relaxed text-paper/80">
            Coal draglines, telecom central offices, high-voltage substations,
            SCADA rooms, and, for the last eight, distillery
            fermentation floors. The photograph is one of a long list. More
            ship with the gallery in a later phase.
          </p>
        </div>
      </section>

      {/* -----------------------------------------------------------------
       * IMAGE BAND, home office. Explicitly personal: the dog is in
       * the frame on purpose. This is the "professional with a life"
       * moment Roger asked for; not decorated up.
       * ----------------------------------------------------------------- */}
      <section aria-label="Workshop" className="bg-paper">
        <div className="mx-auto grid max-w-6xl gap-12 px-6 py-24 sm:px-10 md:grid-cols-[1.2fr_1fr] md:items-center md:py-28">
          <figure className="m-0">
            <div className="relative aspect-[4/5] overflow-hidden">
              <Image
                src="/images/home-office.jpeg"
                alt="A multi-monitor engineering workstation in a home office. A goldendoodle sits underfoot."
                fill
                sizes="(min-width: 768px) 560px, 100vw"
                className="object-cover"
              />
            </div>
            <figcaption className="mt-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              home workshop <span className="text-ink-4">·</span> the dog is not for scale
            </figcaption>
          </figure>
          <div>
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              when the shift ends
            </p>
            <h2
              className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
              style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
            >
              A working room, a family, a dog with strong opinions.
            </h2>
            <p className="mt-6 text-base leading-relaxed text-ink-2">
              Long hours in industrial jobs are only sustainable if the rest of
              a life is real. The workshop above is where the R&amp;D happens
              between shifts, frameworks written, models trained, papers
              read, and the occasional deploy at midnight. The goldendoodle
              supervises.
            </p>
          </div>
        </div>
      </section>

      {/* -----------------------------------------------------------------
       * TEAM IMAGE + LEADERSHIP NOTE
       * Whiskey House team meeting, makes the "distillery startups"
       * claim tangible with a real leadership scene.
       * ----------------------------------------------------------------- */}
      <section aria-label="Leadership" className="bg-paper-2/60">
        <div className="mx-auto grid max-w-6xl gap-12 px-6 py-24 sm:px-10 md:grid-cols-[1fr_1fr] md:items-center md:py-28">
          <figure className="m-0 md:order-2">
            <div className="relative aspect-[4/5] overflow-hidden">
              <Image
                src="/images/whiskey-house-team.jpeg"
                alt="A Whiskey House of Kentucky team meeting on the production floor during commissioning."
                fill
                sizes="(min-width: 768px) 520px, 100vw"
                className="object-cover"
              />
            </div>
            <figcaption className="mt-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              Whiskey House of Kentucky <span className="text-ink-4">·</span> team, mid-commissioning
            </figcaption>
          </figure>
          <div className="md:order-1">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              built with a team
            </p>
            <h2
              className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
              style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
            >
              None of this is a solo act.
            </h2>
            <p className="mt-6 text-base leading-relaxed text-ink-2">
              Distillery startups are built with operators, control engineers,
              instrument techs, safety officers, IT, quality, and a couple of
              contractors you learn to trust. The best days on this job are the
              ones where the team fixes something the org didn&rsquo;t know it
              had, and no one takes the credit.
            </p>
          </div>
        </div>
      </section>

      {/* -----------------------------------------------------------------
       * PUBLIC REPOSITORIES, Roger's GitHub. Server-rendered, cached
       * for an hour. Fails soft: if api.github.com is unreachable the
       * component renders a direct link and moves on.
       * ----------------------------------------------------------------- */}
      <GithubReposIt />

      {/* -----------------------------------------------------------------
       * ABOUT THE BUILD, colophon-style closer. Explains the site as a
       * live work sample and points at the repo. Not a card grid.
       * ----------------------------------------------------------------- */}
      {!signedIn ? (
        <section
          aria-labelledby="access-heading"
          className="border-y border-line bg-paper-2/60"
        >
          <div className="mx-auto max-w-5xl px-6 py-24 sm:px-10 sm:py-32">
            <div className="grid gap-10 md:grid-cols-[1fr_1.5fr] md:items-baseline">
              <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
                what you can do here
              </p>
              <div>
                <h2
                  id="access-heading"
                  className="font-display text-3xl leading-tight text-ink sm:text-4xl"
                  style={{ fontVariationSettings: '"opsz" 100, "SOFT" 40' }}
                >
                  Most of this is open. One thing is not.
                </h2>
                <p className="mt-6 max-w-2xl text-base leading-relaxed text-ink-2">
                  The evidence is public, because you should be able to judge
                  the work without asking anyone for anything. The reviewer
                  needs an account, because running it spends real time on a
                  real machine.
                </p>

                <div className="mt-10 grid gap-10 sm:grid-cols-2">
                  <div>
                    <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                      open to anyone
                    </p>
                    <dl className="mt-4 space-y-4">
                      {OPEN_TO_ANYONE.map((item) => (
                        <div key={item.label}>
                          <dt className="text-sm text-ink">{item.label}</dt>
                          <dd className="mt-1 text-sm leading-relaxed text-ink-3">
                            {item.detail}
                          </dd>
                        </div>
                      ))}
                    </dl>
                  </div>
                  <div className="border-l-2 border-accent pl-5">
                    <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
                      with an account
                    </p>
                    <dl className="mt-4 space-y-4">
                      {WITH_AN_ACCOUNT.map((item) => (
                        <div key={item.label}>
                          <dt className="text-sm text-ink">{item.label}</dt>
                          <dd className="mt-1 text-sm leading-relaxed text-ink-3">
                            {item.detail}
                          </dd>
                        </div>
                      ))}
                    </dl>
                  </div>
                </div>

                <p className="mt-10 text-sm leading-relaxed text-ink-3">
                  {ACCESS_CAVEAT}
                </p>
                <div className="mt-6 flex flex-wrap items-center gap-6">
                  <Link
                    href="/register"
                    className="inline-flex items-center border border-accent px-5 py-2.5 font-mono text-[11px] uppercase tracking-[0.14em] text-accent no-underline transition-colors hover:bg-accent hover:text-paper"
                  >
                    Request access
                  </Link>
                  <Link
                    href="/how-ask-roger-works"
                    className="text-sm text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
                  >
                    Read how the reviewer works first
                  </Link>
                </div>
              </div>
            </div>
          </div>
        </section>
      ) : null}

      <section
        aria-labelledby="build-heading"
        className="mx-auto max-w-5xl px-6 py-24 sm:px-10 sm:py-32"
      >
        <div className="grid gap-10 md:grid-cols-[1fr_1.5fr] md:items-baseline">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
            about this site
          </p>
          <div>
            <h2
              id="build-heading"
              className="font-display text-3xl leading-tight text-ink sm:text-4xl"
              style={{ fontVariationSettings: '"opsz" 100, "SOFT" 40' }}
            >
              The site is itself a live work sample.
            </h2>
            <p className="mt-6 max-w-2xl text-base leading-relaxed text-ink-2">
              Go API and Python sidecar behind a Next.js front-end, all speaking
              Protobuf. Deployed with Docker Compose behind Caddy on a small
              Hetzner box. Source, spec, ADRs, and every decision that shaped
              it are public.
            </p>
            <dl className="mt-8 grid gap-4 font-mono text-sm text-ink-2 sm:grid-cols-2">
              <div className="flex gap-3">
                <dt className="text-ink-3">repo</dt>
                <dd className="m-0">
                  <a
                    href="https://github.com/reh3376/career-site"
                    rel="noopener noreferrer"
                    target="_blank"
                    className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
                  >
                    reh3376/career-site
                  </a>
                </dd>
              </div>
              <div className="flex gap-3">
                <dt className="text-ink-3">stack</dt>
                <dd className="m-0 text-ink">go · python · next.js · postgres</dd>
              </div>
              <div className="flex gap-3">
                <dt className="text-ink-3">deploy</dt>
                <dd className="m-0 text-ink">docker · caddy · hetzner</dd>
              </div>
              <div className="flex gap-3">
                <dt className="text-ink-3">contribute</dt>
                <dd className="m-0">
                  <a
                    href="https://github.com/reh3376/career-site/blob/main/CONTRIBUTING.md"
                    rel="noopener noreferrer"
                    target="_blank"
                    className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
                  >
                    open to contributors
                  </a>
                </dd>
              </div>
            </dl>
          </div>
        </div>
      </section>
    </>
  );
}

// SocialInline renders the outbound social links inside the "writing"
// section: GitHub always (falls back to reh3376's profile), LinkedIn
// only when NEXT_PUBLIC_LINKEDIN_URL is set. Order is deliberate,
// the site's story is code first, professional network second.
//
// The hero used to carry the same two links as bordered buttons. They
// were removed on 2026-09-24: the header already offers both as logo
// links on every page, and in the hero they sat on `sm:ml-auto`, which
// pushed them to the far right of the action row and read as a
// separate, stray group rather than part of it.

function SocialInline() {
  const s = getSocialLinks();
  if (!s.github && !s.linkedin) return null;
  return (
    <p className="mt-8 flex flex-wrap gap-x-6 gap-y-2 text-sm">
      {s.github ? (
        <a
          href={s.github}
          rel="noopener noreferrer"
          target="_blank"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          {s.githubHandle ?? "GitHub"}
        </a>
      ) : null}
      {s.linkedin ? (
        <a
          href={s.linkedin}
          rel="noopener noreferrer"
          target="_blank"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          LinkedIn
        </a>
      ) : null}
    </p>
  );
}
