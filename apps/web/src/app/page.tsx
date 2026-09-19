import Image from "next/image";
import Link from "next/link";

// Landing page. Restrained, one-column, on-the-floor image as visual
// anchor. Copy leads with the last decade of distillery-startup work
// (24/7 process control at scale) and names the product offerings so a
// visiting engineer / hiring manager sees the shape of the practice
// before the résumé.

const OFFERINGS = [
  {
    title: "Digital transformation",
    body: "Take a plant from paper logbooks and disconnected historians to a data-first operation — instrumentation, data model, ontology, event streams — without stopping production.",
  },
  {
    title: "Process optimization",
    body: "Deep instrumentation + statistical process control + advanced control (MPC, model-based). Multi-percent yield gains and step-change reliability from what the plant already has.",
  },
  {
    title: "Automation & control",
    body: "Greenfield and brownfield: PLC / DCS design, control-narrative to commissioning, safety-instrumented systems, and the migrations that most integrators won't touch.",
  },
  {
    title: "IT/OT convergence",
    body: "Bridge the plant network to the enterprise stack — safely. Segmented architectures, historian federation, MES/ERP integration, and the governance that keeps ops teams sleeping through the night.",
  },
  {
    title: "Industrial DataOps + applied AI",
    body: "MDEMG, Forge, and the AI-ML patterns that actually ship on the floor: retrieval-augmented decision support, anomaly detection, quality prediction, closed-loop optimisation.",
  },
];

const HIGHLIGHTS = [
  {
    title: "Eight years running distillery startups",
    body: "Whiskey House of Kentucky and predecessors. 120-hour weeks bringing new bourbon plants from concrete pour to steady-state production — process design, control system commissioning, plant IT, quality, safety. What it looks like when the abstract stuff meets a 24/7 fermentation cycle.",
  },
  {
    title: "Thirty years across regulated industries",
    body: "Electrical power infrastructure, mining, telecom, industrial automation, IT/OT convergence, applied AI. Full-cycle engineering leadership from FEED through commissioning through steady-state operations.",
  },
  {
    title: "Frameworks and products, not just projects",
    body: "MDEMG (Manufacturing Data & Event Model Graph), Forge, and other open-source infrastructure I designed for real plant use. When it applies to your problem, we skip the year of custom build-out.",
  },
];

export default function HomePage() {
  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:py-20">

      {/* Hero */}
      <section aria-labelledby="hero-heading" className="mb-20">
        <p className="mb-4 text-sm font-medium uppercase tracking-widest text-accent">
          Coming soon — late 2026
        </p>
        <h1
          id="hero-heading"
          className="mb-6 text-4xl font-semibold leading-tight tracking-tight text-ink sm:text-5xl"
        >
          Roger Henley.<br className="hidden sm:block" />
          <span className="text-ink-2">Industrial automation, plant operations, applied AI.</span>
        </h1>
        <p className="mb-8 text-lg leading-relaxed text-ink-2">
          Thirty years in regulated 24/7 manufacturing — the last eight running bourbon-distillery
          startups end-to-end. This is the site version of that practice: what I do, the projects
          behind it, and how to reach me.
        </p>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
          <Link
            href="/register"
            className="inline-flex items-center rounded-md bg-accent px-5 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          >
            Request access
          </Link>
          <Link href="/contact" className="text-sm text-ink-2 underline underline-offset-2 hover:text-accent">
            Or just get in touch
          </Link>
        </div>
      </section>

      {/* Image + caption — the on-the-floor visual anchor */}
      <figure className="mb-20 -mx-6 sm:mx-0">
        <div className="relative aspect-[16/9] overflow-hidden sm:rounded-lg">
          {/* unoptimized: the source is already ~1920x1080 / 323 KB; skipping
              the optimizer keeps the Docker standalone bundle from having to
              carry sharp + its native deps. Turn back on when we have a real
              asset pipeline (Phase 2 gallery work). */}
          <Image
            src="/images/hobet-dragline.jpeg"
            alt="A dragline excavator at Hobet Mining at dusk, its boom silhouetted against the sky over a stripped bench."
            fill
            priority
            unoptimized
            sizes="(min-width: 640px) 768px, 100vw"
            className="object-cover"
          />
        </div>
        <figcaption className="mt-3 px-6 text-xs text-ink-3 sm:px-0">
          Hobet dragline, West Virginia — one of a long list of on-the-floor places this
          work has taken me. More photos ship with the gallery in a later phase.
        </figcaption>
      </figure>

      {/* What I do */}
      <section aria-labelledby="offerings-heading" className="mb-20">
        <h2
          id="offerings-heading"
          className="mb-2 text-2xl font-semibold tracking-tight text-ink sm:text-3xl"
        >
          What I do
        </h2>
        <p className="mb-10 text-base leading-relaxed text-ink-3">
          Five practice areas. Every engagement mixes them; naming them separately makes it easier
          for you to figure out whether we should talk.
        </p>
        <ul className="space-y-9">
          {OFFERINGS.map((o) => (
            <li key={o.title}>
              <h3 className="mb-2 text-base font-semibold text-ink">{o.title}</h3>
              <p className="m-0 leading-relaxed text-ink-2">{o.body}</p>
            </li>
          ))}
        </ul>
      </section>

      {/* Background highlights */}
      <section
        aria-labelledby="background-heading"
        className="mb-20 border-t border-line pt-12"
      >
        <h2
          id="background-heading"
          className="mb-10 text-2xl font-semibold tracking-tight text-ink sm:text-3xl"
        >
          Background
        </h2>
        <ul className="space-y-9">
          {HIGHLIGHTS.map((h) => (
            <li key={h.title}>
              <h3 className="mb-2 text-base font-semibold text-ink">{h.title}</h3>
              <p className="m-0 leading-relaxed text-ink-2">{h.body}</p>
            </li>
          ))}
        </ul>
      </section>

      {/* About the build */}
      <section
        aria-labelledby="build-heading"
        className="rounded-lg border border-line bg-paper-2 p-6 text-sm leading-relaxed text-ink-3"
      >
        <h2 id="build-heading" className="mb-2 text-sm font-semibold text-ink-2">
          About the build
        </h2>
        <p className="m-0">
          The site is itself a live work sample: Go API + Python sidecar + Next.js frontend on
          Protobuf contracts, deployed with Docker Compose behind Caddy. Source, spec, and every
          decision that shaped it are public —{" "}
          <a
            href="https://github.com/reh3376/career-site"
            className="text-accent underline underline-offset-2 hover:text-accent-hover"
            rel="noopener noreferrer"
            target="_blank"
          >
            github.com/reh3376/career-site
          </a>
          .
        </p>
      </section>
    </div>
  );
}
