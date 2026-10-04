import Link from "next/link";
import type { Metadata } from "next";

export const metadata: Metadata = { title: "Admin · Overview" };

// Directory of the admin console.
//
// This page was written when the console was four surfaces, three of
// them scaffolded and one of them "comes online in Phase 4". All three
// shipped and Phase 4 shipped, and the page went on saying otherwise
// until 2026-10-04. A status board that is wrong is worse than no
// status board: it is read once, found to be false, and then never
// trusted again.
//
// So the chip now marks the exception rather than the rule. Everything
// here is live, nothing carries a chip, and a surface that is genuinely
// partial gets one at the moment it becomes partial. A label on
// everything is a label on nothing.
//
// Grouped rather than listed flat: fourteen entries in one column is a
// list nobody reads to the bottom of.

type Surface = { href: string; title: string; body: string; status?: "scaffolded" | "later" };

const GROUPS: { heading: string; note: string; surfaces: Surface[] }[] = [
  {
    heading: "People",
    note: "Who gets in, who is in, and what they have been doing.",
    surfaces: [
      {
        href: "/admin/registrations",
        title: "Registrations",
        body: "Pending and decided in one place, so approvals do not only live in email. Approve with a TTL, decline, extend, resend.",
      },
      {
        href: "/admin/access",
        title: "Access & whitelist",
        body: "Email whitelist, active grants with their expiry, and a TTL bump for a member whose access is running out.",
      },
      {
        href: "/admin/contacts",
        title: "Contact messages",
        body: "The support inbox by category, including contributor-access requests. Replies go from your own mailbox.",
      },
      {
        href: "/admin/activity",
        title: "Activity",
        body: "What members did, from the same event stream as anonymous visits.",
      },
    ],
  },
  {
    heading: "The JD reviewer",
    note: "Submissions, and whether the machine's judgement is any good.",
    surfaces: [
      {
        href: "/admin/jd",
        title: "JD submissions",
        body: "Every posting submitted, its assessment, and the fit bands that decide what a submitter is told.",
      },
      {
        href: "/admin/decisions",
        title: "Decision review",
        body: "Grade the model against yourself, requirement by requirement. This is the training data, which is why nothing here is ever deleted.",
      },
      {
        href: "/admin/evals",
        title: "Evaluations",
        body: "Run the golden set through the production pipeline and compare against the last run. Ten postings, about five hours.",
      },
      {
        href: "/admin/gate",
        title: "Gate",
        body: "The seven criteria as pass, not met, or not enough data yet. A criterion with four data points has not earned a verdict.",
      },
    ],
  },
  {
    heading: "Ask Roger",
    note: "What it reads from, and what it is allowed to say.",
    surfaces: [
      {
        href: "/admin/corpus",
        title: "Corpus",
        body: "Documents, chunks and embeddings, with reindex and embed sweeps. Public and private mounts are walked separately.",
      },
      {
        href: "/admin/qa",
        title: "Q&A bank",
        body: "Your own words, served verbatim and never paraphrased. The only place an answer on a restricted topic can exist.",
      },
    ],
  },
  {
    heading: "Running it",
    note: "The numbers, the box, and a way in when something is odd.",
    surfaces: [
      {
        href: "/admin/analytics",
        title: "Analytics",
        body: "The headline measurements, arranged by the criteria you set. A criterion with no data says so rather than showing a zero.",
      },
      {
        href: "/admin/ops",
        title: "Operations",
        body: "What is running, what failed, and what is queued.",
      },
      {
        href: "/admin/scheduler",
        title: "Scheduler",
        body: "Availability, booked meetings, and the Google Calendar connection with its last error and last success.",
      },
      {
        href: "/admin/db",
        title: "Query console",
        body: "Read-only SQL against the metric views. For the question that does not have a tile.",
      },
    ],
  },
];

export default function AdminOverviewPage() {
  return (
    <>
      <h1
        className="font-display text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Overview.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
        Fourteen surfaces, all of them live. Anything partial carries a
        label; nothing here does.
      </p>

      <div className="mt-14 space-y-14">
        {GROUPS.map((g) => (
          <section key={g.heading}>
            <h2 className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
              {g.heading}
            </h2>
            <p className="mt-2 max-w-xl text-sm text-ink-3">{g.note}</p>
            <ul className="mt-6 divide-y divide-line border-y border-line">
              {g.surfaces.map((s) => (
                <SurfaceRow key={s.href} {...s} />
              ))}
            </ul>
          </section>
        ))}
      </div>
    </>
  );
}

function SurfaceRow({ href, title, body, status }: Surface) {
  const chip = status
    ? {
        scaffolded: { label: "scaffolded", className: "text-signal bg-signal-soft/60" },
        later: { label: "later", className: "text-ink-3 bg-paper-3" },
      }[status]
    : null;

  return (
    <li>
      <Link
        href={href}
        className="group grid gap-2 py-6 no-underline sm:grid-cols-[minmax(0,220px)_1fr] sm:gap-10"
      >
        <div>
          <h3
            className="font-display text-xl leading-tight text-ink transition-colors group-hover:text-accent"
            style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
          >
            {title}
          </h3>
          {chip ? (
            <p
              className={`mt-2 inline-block rounded-sm px-2 py-0.5 font-mono text-[10px] tracking-[0.14em] uppercase ${chip.className}`}
            >
              {chip.label}
            </p>
          ) : null}
        </div>
        <p className="max-w-2xl text-base leading-relaxed text-ink-2">{body}</p>
      </Link>
    </li>
  );
}
