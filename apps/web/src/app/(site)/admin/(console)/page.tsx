import type { Metadata } from "next";

export const metadata: Metadata = { title: "Admin · Overview" };

// Landing page for the admin console. Right now it's a directory of
// what lives under /admin. As each surface goes live (contacts,
// registrations, access), it gets a real dashboard tile with counts
// pulled from the AdminService RPCs, no proto for those yet, so this
// stays a text list until the next PR.
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
        Everything you do to keep this site running, reviewing
        registrations, replying to contact messages, managing access
        grants, routes through here. Each surface is being built out
        one PR at a time.
      </p>

      <ul className="mt-12 divide-y divide-line border-y border-line">
        <SurfaceRow
          title="Contact messages"
          status="scaffolded"
          body="List, filter by category, mark resolved, jump to reply. Backed by the ContactService submissions table; RPCs land next PR."
        />
        <SurfaceRow
          title="Registrations"
          status="scaffolded"
          body="Pending and recent decisions in one place, so approvals don't only live in email. Shows the whitelist match, the applicant's stated role, and their approval TTL choice."
        />
        <SurfaceRow
          title="Access & whitelist"
          status="scaffolded"
          body="Email whitelist CRUD, active access grants (with expiry), and a one-click TTL bump for members whose access is running out."
        />
        <SurfaceRow
          title="Corpus & Ask Roger"
          status="later"
          body="Ingest new docs, test retrieval, review Ask Roger conversations, escalate a thread to a real reply. Comes online in Phase 4."
        />
      </ul>
    </>
  );
}

function SurfaceRow({
  title,
  status,
  body,
}: {
  title: string;
  status: "live" | "scaffolded" | "later";
  body: string;
}) {
  const chip = {
    live: {
      label: "live",
      className: "text-accent bg-accent-soft/60",
    },
    scaffolded: {
      label: "scaffolded",
      className: "text-signal bg-signal-soft/60",
    },
    later: {
      label: "later",
      className: "text-ink-3 bg-paper-3",
    },
  }[status];

  return (
    <li className="grid gap-4 py-8 sm:grid-cols-[minmax(0,220px)_1fr] sm:gap-10 sm:py-10">
      <div>
        <h3
          className="font-display text-2xl leading-tight text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          {title}
        </h3>
        <p
          className={`mt-2 inline-block rounded-sm px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.14em] ${chip.className}`}
        >
          {chip.label}
        </p>
      </div>
      <p className="max-w-2xl text-base leading-relaxed text-ink-2">{body}</p>
    </li>
  );
}
