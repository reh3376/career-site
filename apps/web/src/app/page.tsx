import Link from "next/link";

// What visitors see today. Explains what the site is becoming, sets the
// ETA the owner picked (late 2026), and offers the single call to action:
// request access. Registration is approval-gated (D-02, ADR-0002), so the
// button lands on a form that emails Roger for a decision.

const HIGHLIGHTS = [
  {
    title: "A guided tour of a 30-year career",
    body: "Electrical power infrastructure, mining, telecom, regulated 24/7 manufacturing, industrial automation and process control, IT/OT convergence, industrial DataOps and ontology, and applied AI/ML — organized so the parts that matter to your role are surfaced first.",
  },
  {
    title: "Projects, writing, and evidence — not claims",
    body: "Open-source portfolio, published articles, presentations, and quantified outcomes. Every accomplishment links to the work behind it.",
  },
  {
    title: "Ask Roger",
    body: "A first-person conversational assistant grounded in Roger's own material. It cites its sources, admits what it doesn't know, and hands the conversation to him when it matters.",
  },
];

export default function HomePage() {
  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:py-24">
      <section aria-labelledby="hero-heading" className="mb-16">
        <p className="mb-4 text-sm font-medium uppercase tracking-widest text-accent">
          Coming soon — late 2026
        </p>
        <h1
          id="hero-heading"
          className="mb-6 text-4xl font-semibold leading-tight tracking-tight text-ink sm:text-5xl"
        >
          Roger Henley&rsquo;s interactive career portfolio.
        </h1>
        <p className="mb-8 text-lg leading-relaxed text-ink-2">
          A gated, personalized site that acts like a well-briefed representative: it learns what a
          visiting employer cares about, puts the most relevant evidence first, and answers
          questions in Roger&rsquo;s own voice from his own material.
        </p>
        <div className="flex flex-wrap items-center gap-4">
          <Link
            href="/register"
            className="inline-flex items-center rounded-md bg-accent px-5 py-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          >
            Request access
          </Link>
          <p className="text-sm text-ink-3">
            Registration is reviewed by Roger — usually within a day.
          </p>
        </div>
      </section>

      <section
        aria-labelledby="what-heading"
        className="border-t border-line pt-12"
      >
        <h2
          id="what-heading"
          className="mb-8 text-xl font-semibold text-ink"
        >
          What you&rsquo;ll find when it opens
        </h2>
        <ul className="space-y-8">
          {HIGHLIGHTS.map((h) => (
            <li key={h.title}>
              <h3 className="mb-2 text-base font-semibold text-ink">{h.title}</h3>
              <p className="m-0 leading-relaxed text-ink-2">{h.body}</p>
            </li>
          ))}
        </ul>
      </section>

      <section
        aria-labelledby="build-heading"
        className="mt-16 rounded-lg border border-line bg-paper-2 p-6 text-sm leading-relaxed text-ink-3"
      >
        <h2 id="build-heading" className="mb-2 text-sm font-semibold text-ink-2">
          About the build
        </h2>
        <p className="m-0">
          The site itself is a live work sample: Go API + Python sidecar + Next.js frontend on
          Protobuf contracts, deployed with Docker Compose. Source and spec are public —{" "}
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
