import Image from "next/image";
import Link from "next/link";

import { setUiModeAction } from "@/app/actions/ui-mode";
import { GithubReposOt } from "@/components/github-repos";
import { ModeToggle } from "@/components/mode-toggle";
import { getSocialLinks } from "@/lib/social-links";

// OT-mode landing. Renders the site as a plant HMI overview screen.
// Layout draws on Roger's actual distillery HMIs: a title bar with
// plant identity + nav pills, tabbed left rail, a grid of KPI tiles
// with mono-set values, a practice-area panel with running-state
// pilot chips, a small event log, four "camera feed" thumbnails
// down the right rail, and a bottom status ribbon. Every piece is
// static content, the "live" values (uptime, historian rate) are
// design elements, not data hookups, and are labeled as such where
// a plant operator would recognize the fiction.
//
// Interaction parity with the IT landing (owner, 2026-09-22: "OT mode
// needs cleaned up, it is not nearly as easy to interact with as IT
// mode"): the same three actions (request access, contact, sign in)
// are visible at every width, the nav pills are real anchors, the
// mode switch actually switches, status values are honest, and body
// text never drops below 13px.

const PRACTICE_TAGS = [
  {
    tag: "P-101",
    title: "Digital transformation",
    state: "RUNNING",
    desc: "instrumentation → data model → ontology → event streams",
  },
  {
    tag: "P-102",
    title: "Process optimization",
    state: "RUNNING",
    desc: "SPC + model-based advanced control (MPC / RTO)",
  },
  {
    tag: "P-103",
    title: "Automation & control",
    state: "RUNNING",
    desc: "PLC / DCS design, control narrative → commissioning, SIS",
  },
  {
    tag: "P-104",
    title: "IT / OT convergence",
    state: "RUNNING",
    desc: "segmented network, historian federation, MES / ERP",
  },
  {
    tag: "P-105",
    title: "Industrial DataOps + applied AI",
    state: "RUNNING",
    desc: "MDEMG · Forge · RAG for ops · anomaly detection",
  },
];

const EVENT_LOG = [
  { ts: "2026-09-19", src: "SITE.WEB", msg: "career-site.OT surface online" },
  { ts: "2026-09-15", src: "PRACTICE", msg: "8 yrs distillery startups → steady-state" },
  { ts: "2020-06-01", src: "PRACTICE", msg: "distillery commissioning cycle 4/n" },
  { ts: "2018-04-01", src: "PRACTICE", msg: "distillery commissioning cycle 1/n" },
  { ts: "1996-01-01", src: "PRACTICE", msg: "career start · regulated 24/7 systems" },
];

const CAMERAS = [
  { id: "CAM-01", label: "workshop", src: "/images/home-office.jpeg" },
  { id: "CAM-02", label: "hobet dragline", src: "/images/hobet-dragline.jpeg" },
  { id: "CAM-03", label: "column · top bubble tray", src: "/images/bubble-tray.jpeg" },
  { id: "CAM-04", label: "whiskey house team", src: "/images/whiskey-house-team.jpeg" },
];

export function OtLanding() {
  return (
    <div className="min-h-[calc(100vh-160px)] bg-paper text-ink">
      {/* Plant title strip. Left = nav pills, center = plant name, */}
      {/* right = live status. The visual grammar is 100% "SCADA top    */}
      {/* bar", Roger's Bardstown HMI has the exact same structure.    */}
      <div className="border-b border-line-strong bg-paper-2">
        <div className="mx-auto flex max-w-[1600px] flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 sm:px-6">
          <nav
            aria-label="Screens"
            className="flex flex-wrap items-center gap-2 font-mono text-[12px] uppercase tracking-[0.14em] text-ink-2"
          >
            <NavPill active>OVERVIEW</NavPill>
            <NavPill href="#practice">PRACTICE</NavPill>
            <NavPill href="#timeline">TIMELINE</NavPill>
            <NavPill href="/contact">CONTACT</NavPill>
            <NavPill href="/register" primary>
              REQUEST ACCESS
            </NavPill>
          </nav>
          <div className="order-first w-full text-center font-mono text-[13px] uppercase tracking-[0.2em] text-ink sm:order-none sm:w-auto sm:flex-1">
            R. HENLEY <span className="text-ink-3">/</span> CAREER-SITE
            <span className="text-ink-3">.</span>OT
          </div>
          <div className="flex flex-wrap items-center gap-2 font-mono text-[12px] uppercase tracking-[0.14em] text-ink-2">
            <StatusChip color="success">RUN</StatusChip>
            <ModeToggle current="ot" />
            <span className="hidden items-center gap-2 lg:flex">
              <OtSocialPills />
            </span>
            <Link
              href="/login"
              className="inline-flex min-h-[36px] items-center border border-line-strong px-3 py-1.5 text-ink-2 no-underline transition-colors hover:border-accent hover:text-accent"
            >
              [ SIGN IN ]
            </Link>
          </div>
        </div>
      </div>

      {/* Main HMI body, 12-col grid on wide screens, stacked on phone. */}
      <div className="mx-auto grid max-w-[1600px] gap-4 px-4 py-4 sm:px-6 md:grid-cols-12">
        {/* OPERATOR NOTE: the one panel a first-time visitor needs. Says */}
        {/* who this is and what to do next, with the same three actions  */}
        {/* the IT landing puts in its hero, at button size.              */}
        <section aria-labelledby="op-note-heading" className="md:col-span-12">
          <PanelHeader tag="OP-1" title="OPERATOR NOTE · READ FIRST" />
          <div className="grid gap-6 border border-line-strong bg-paper-2 p-5 sm:p-6 lg:grid-cols-[1.4fr_1fr] lg:items-center">
            <div>
              <h1
                id="op-note-heading"
                className="font-mono text-[15px] leading-snug text-ink sm:text-[17px]"
              >
                Roger Henley. Thirty years of regulated, 24-hour manufacturing; the
                last eight commissioning distilleries from concrete pour to
                steady-state. Controls, plant systems, industrial data, applied AI.
              </h1>
              <p className="mt-3 max-w-2xl text-[13px] leading-relaxed text-ink-2">
                Hiring managers: request access, then upload a job description.
                The reviewer scores it against thirty years of records, shows which
                requirements are evidenced, and above the gate returns a two-page
                résumé written for that posting.
              </p>
            </div>
            <div className="flex flex-wrap gap-3 font-mono text-[12px] uppercase tracking-[0.14em] lg:justify-end">
              <Link
                href="/register"
                className="inline-flex min-h-[44px] items-center bg-accent px-5 text-white no-underline transition-colors hover:bg-accent-hover"
              >
                [ REQUEST ACCESS ]
              </Link>
              <Link
                href="/contact"
                className="inline-flex min-h-[44px] items-center border border-line-strong px-5 text-ink no-underline transition-colors hover:border-accent hover:text-accent"
              >
                [ CONTACT ]
              </Link>
              <Link
                href="/login"
                className="inline-flex min-h-[44px] items-center border border-line-strong px-5 text-ink-2 no-underline transition-colors hover:border-accent hover:text-accent"
              >
                [ SIGN IN ]
              </Link>
            </div>
          </div>
        </section>

        {/* Left rail: system + KPI tiles ---------------------------- */}
        <section aria-label="Overview KPIs" className="md:col-span-8">
          <PanelHeader tag="OV-1" title="OVERVIEW · KPIs" />
          <div className="grid grid-cols-1 gap-4 border border-line-strong bg-paper-2 p-4 sm:grid-cols-3">
            <KpiTile
              tag="YEARS.FIELD"
              value="30"
              unit="yr"
              subtitle="Power · mining · telecom · industrial automation · applied AI"
            />
            <KpiTile
              tag="YEARS.DISTILLERY"
              value="8"
              unit="yr"
              subtitle="Whiskey House of Kentucky + predecessors · concrete → steady-state"
            />
            <KpiTile
              tag="SHIFT.PATTERN"
              value="24/7"
              unit=""
              subtitle="Regulated, continuous production · pages at 03:00"
            />
          </div>
        </section>

        {/* Right rail row 1: LIVE-STATUS panel ---------------------- */}
        <section
          aria-label="Live status"
          className="md:col-span-4"
        >
          <PanelHeader tag="LS-1" title="LIVE STATUS" />
          <dl className="grid grid-cols-2 gap-x-4 gap-y-2 border border-line-strong bg-paper-2 p-4 font-mono text-[13px]">
            <dt className="text-ink-3">CAREER</dt>
            <dd className="m-0 text-right tabular text-ink">30 yr</dd>
            <dt className="text-ink-3">DISTILLERY</dt>
            <dd className="m-0 text-right tabular text-ink">8 yr</dd>
            <dt className="text-ink-3">SHIFT</dt>
            <dd className="m-0 text-right tabular text-ink">24/7</dd>
            <dt className="text-ink-3">AVAILABILITY</dt>
            <dd className="m-0 text-right text-success">OPEN</dd>
            <dt className="text-ink-3">JD.REVIEW</dt>
            <dd className="m-0 text-right text-success">ONLINE · MEMBERS</dd>
            <dt className="text-ink-3">ASK.ROGER</dt>
            <dd className="m-0 text-right text-ink-3">PLANNED</dd>
          </dl>
        </section>

        {/* Right rail row 2: NAV / helpers -------------------------- */}
        <section aria-label="Navigation" className="md:col-span-4">
          <PanelHeader tag="NV-1" title="NAV" />
          <div className="border border-line-strong bg-paper-2 p-4">
            <ul className="space-y-0.5 font-mono text-[13px]">
              <li>
                <form action={setUiModeAction} className="m-0">
                  <input type="hidden" name="mode" value="it" />
                  <button
                    type="submit"
                    className="flex min-h-[40px] w-full items-center justify-between gap-4 bg-transparent text-left text-ink-2 hover:text-accent"
                  >
                    <span className="flex items-center gap-3">
                      <span className="text-ink-3">mode</span>
                      <span>SWITCH.TO.IT</span>
                    </span>
                    <span className="text-[10px] uppercase tracking-[0.14em] text-ink-3">editorial surface</span>
                  </button>
                </form>
              </li>
              <NavRow tag="/register" label="ACCESS.REQ" href="/register" note="hiring managers" />
              <NavRow tag="/login" label="SESSION.NEW" href="/login" note="members" />
              <NavRow tag="/contact" label="MSG.OUT" href="/contact" note="email roger" />
              <SocialNavRows />
            </ul>
          </div>
        </section>

        {/* PRACTICE AREAS panel ------------------------------------ */}
        <section id="practice" aria-label="Practice areas" className="scroll-mt-4 md:col-span-8 md:row-span-2">
          <PanelHeader tag="PA-1" title="PRACTICE AREAS" />
          <div className="border border-line-strong bg-paper-2">
            <div className="grid grid-cols-[80px_1fr_120px] gap-x-4 border-b border-line px-4 py-2 font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
              <span>TAG</span>
              <span>DESCRIPTION</span>
              <span className="text-right">STATE</span>
            </div>
            <ul className="divide-y divide-line">
              {PRACTICE_TAGS.map((row) => (
                <li
                  key={row.tag}
                  className="grid grid-cols-[80px_1fr_120px] items-center gap-x-4 px-4 py-3 font-mono text-[13px] text-ink-2"
                >
                  <span className="text-ink">{row.tag}</span>
                  <span className="min-w-0">
                    <span className="block text-ink">{row.title}</span>
                    <span className="block text-[12px] leading-snug text-ink-3">
                      {row.desc}
                    </span>
                  </span>
                  <span className="flex items-center justify-end gap-2">
                    <span className="pilot text-success" aria-hidden="true" />
                    <span className="text-success">{row.state}</span>
                  </span>
                </li>
              ))}
            </ul>
          </div>
        </section>

        {/* EVENT LOG panel ------------------------------------------ */}
        <section id="timeline" aria-label="Event log" className="scroll-mt-4 md:col-span-4">
          <PanelHeader tag="EL-1" title="EVENT LOG" />
          <div className="border border-line-strong bg-paper-2 p-4 font-mono text-[12px]">
            <ul className="space-y-2">
              {EVENT_LOG.map((e, i) => (
                <li key={i} className="text-ink-2">
                  <span className="text-ink-3">{e.ts}</span>
                  <span className="mx-2 text-ink-3">·</span>
                  <span className="text-accent">{e.src}</span>
                  <span className="mx-2 text-ink-3">·</span>
                  <span className="text-ink">{e.msg}</span>
                </li>
              ))}
            </ul>
          </div>
        </section>

        {/* CAMERAS row --------------------------------------------- */}
        <section aria-label="Cameras" className="md:col-span-12">
          <PanelHeader tag="CM-1" title="CAMERAS" />
          <div className="grid grid-cols-2 gap-3 border border-line-strong bg-paper-2 p-3 md:grid-cols-4">
            {CAMERAS.map((c) => (
              <figure key={c.id} className="m-0">
                <div className="relative aspect-[4/3] overflow-hidden bg-paper-3">
                  <Image
                    src={c.src}
                    alt={c.label}
                    fill
                    sizes="(min-width: 768px) 320px, 50vw"
                    className="object-cover"
                    unoptimized={c.src.endsWith("/hobet-dragline.jpeg")}
                  />
                  {/* Camera-feed hairline overlay */}
                  <div className="pointer-events-none absolute inset-0 border border-line-strong/40" />
                </div>
                <figcaption className="mt-1 flex items-center justify-between font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
                  <span className="text-ink">{c.id}</span>
                  <span>{c.label}</span>
                </figcaption>
              </figure>
            ))}
          </div>
        </section>
      </div>

      {/* PUBLIC REPOS panel, cached server fetch, fails soft. */}
      <GithubReposOt />

      {/* Bottom status ribbon. */}
      <div className="border-t border-line-strong bg-paper-2">
        <div className="mx-auto flex max-w-[1600px] flex-wrap items-center gap-x-6 gap-y-2 px-4 py-2 font-mono text-[10px] uppercase tracking-[0.14em] text-ink-2 sm:px-6">
          <span>
            CAREER <span className="text-ink">30 yr</span>
          </span>
          <span>
            DISTILLERY <span className="text-ink">8 yr</span>
          </span>
          <span>
            SHIFT <span className="text-ink">24/7</span>
          </span>
          <span>
            JD.REVIEW <span className="text-ink">ONLINE</span>
          </span>
          <span className="ml-auto flex items-center gap-2">
            <span className="pilot text-success" aria-hidden="true" />
            <span className="text-success">SYSTEM · NOMINAL</span>
          </span>
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Sub-components                                                     */
/* ------------------------------------------------------------------ */

function PanelHeader({ tag, title }: { tag: string; title: string }) {
  return (
    <div className="mb-1 flex items-baseline justify-between font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
      <span>{title}</span>
      <span className="text-ink-4">[{tag}]</span>
    </div>
  );
}

function KpiTile({
  tag,
  value,
  unit,
  subtitle,
}: {
  tag: string;
  value: string;
  unit: string;
  subtitle: string;
}) {
  return (
    <div className="border border-line bg-paper p-4">
      <div className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
        {tag}
      </div>
      <div className="mt-2 flex items-baseline gap-1 font-mono text-4xl leading-none tabular text-ink sm:text-5xl">
        <span>{value}</span>
        {unit ? <span className="text-lg text-ink-3">{unit}</span> : null}
      </div>
      <p className="mt-3 text-[11px] leading-snug text-ink-3">{subtitle}</p>
    </div>
  );
}

function NavPill({
  children,
  active = false,
  primary = false,
  href,
}: {
  children: React.ReactNode;
  active?: boolean;
  primary?: boolean;
  href?: string;
}) {
  const cls =
    "inline-flex min-h-[36px] items-center px-3 py-1.5 no-underline transition-colors " +
    (active
      ? "border border-ink bg-ink text-paper"
      : primary
        ? "bg-accent text-white hover:bg-accent-hover"
        : "border border-line-strong text-ink-2 hover:border-accent hover:text-accent");
  if (href) {
    return (
      <Link href={href} className={cls}>
        {children}
      </Link>
    );
  }
  return (
    <span aria-current="page" className={cls}>
      {children}
    </span>
  );
}

function StatusChip({
  color,
  children,
}: {
  color: "success" | "signal" | "danger";
  children: React.ReactNode;
}) {
  const cls =
    color === "success"
      ? "text-success border-success/40"
      : color === "signal"
        ? "text-signal border-signal/40"
        : "text-danger border-danger/40";
  return (
    <span
      className={`inline-flex items-center gap-1.5 border px-2 py-0.5 ${cls}`}
    >
      <span className={`pilot text-${color}`} aria-hidden="true" />
      <span>{children}</span>
    </span>
  );
}

// Top-bar profile pills next to [ SIGN IN ], so LinkedIn and GitHub are
// visible without scrolling to the NAV panel.
function OtSocialPills() {
  const s = getSocialLinks();
  return (
    <>
      {s.linkedin ? (
        <a
          href={s.linkedin}
          rel="noopener noreferrer"
          target="_blank"
          className="inline-flex min-h-[36px] items-center border border-line-strong px-3 py-1.5 text-ink-2 no-underline transition-colors hover:border-accent hover:text-accent"
        >
          [ LINKEDIN ]
        </a>
      ) : null}
      {s.github ? (
        <a
          href={s.github}
          rel="noopener noreferrer"
          target="_blank"
          className="inline-flex min-h-[36px] items-center border border-line-strong px-3 py-1.5 text-ink-2 no-underline transition-colors hover:border-accent hover:text-accent"
        >
          [ GITHUB ]
        </a>
      ) : null}
    </>
  );
}

// Outbound profile links, rendered only when configured (see
// lib/social-links). External targets open in a new tab.
function SocialNavRows() {
  const s = getSocialLinks();
  return (
    <>
      {s.linkedin ? (
        <li>
          <a
            href={s.linkedin}
            rel="noopener noreferrer"
            target="_blank"
            className="flex min-h-[40px] items-center justify-between gap-4 no-underline text-ink-2 hover:text-accent"
          >
            <span className="flex items-center gap-3">
              <span className="text-ink-3">ext</span>
              <span>LINKEDIN</span>
            </span>
            <span className="text-[10px] uppercase tracking-[0.14em] text-ink-3">profile</span>
          </a>
        </li>
      ) : null}
      {s.github ? (
        <li>
          <a
            href={s.github}
            rel="noopener noreferrer"
            target="_blank"
            className="flex min-h-[40px] items-center justify-between gap-4 no-underline text-ink-2 hover:text-accent"
          >
            <span className="flex items-center gap-3">
              <span className="text-ink-3">ext</span>
              <span>GITHUB</span>
            </span>
            <span className="text-[10px] uppercase tracking-[0.14em] text-ink-3">public repos</span>
          </a>
        </li>
      ) : null}
    </>
  );
}

function NavRow({
  tag,
  label,
  href,
  note,
}: {
  tag: string;
  label: string;
  href: string;
  note?: string;
}) {
  return (
    <li>
      <Link
        href={href}
        className="flex min-h-[40px] items-center justify-between gap-4 no-underline text-ink-2 hover:text-accent"
      >
        <span className="flex items-center gap-3">
          <span className="text-ink-3">{tag}</span>
          <span>{label}</span>
        </span>
        {note ? (
          <span className="text-[10px] uppercase tracking-[0.14em] text-ink-3">
            {note}
          </span>
        ) : null}
      </Link>
    </li>
  );
}
