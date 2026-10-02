// Podcast appearances, in both landing modes.
//
// Roger was a guest on "Two Guys and a PLC". For a hiring manager this
// is a different kind of evidence from the writing: someone else chose
// to put him on their show, twice, and the second time alongside
// another guest. That is third-party signal, which the rest of this
// page cannot provide about itself.
//
// Deliberately not a Spotify embed. The site's CSP is default-src
// 'self' with no frame-src, so an iframe is blocked today and allowing
// one would mean editing the production Caddyfile. More to the point,
// it would load Spotify's scripts and set their cookies for every
// visitor who reaches this part of the page, whether or not they press
// play, on a site that self-hosts its fonts specifically so builds
// never touch Google. Roger's call: link out, load nothing.
//
// The links are external, so the site's event beacon records a
// link.external click on each without any handler here.
//
// The show's logo is served from this origin, cropped from the tile the
// show uses, rather than hot-linked. Same reasoning as the embed: the
// page loads nothing from anywhere else.

import Image from "next/image";

type Episode = {
  title: string;
  url: string;
  // ISO date, formatted at render so the two modes cannot disagree.
  date: string;
};

const SHOW = "Two Guys and a PLC";

// Newest first. Titles and dates are the show's own, read from the
// episode pages rather than written from memory.
const EPISODES: Episode[] = [
  {
    title: "Two Guys, Roger Henley, and Gaurav Khanna",
    url: "https://open.spotify.com/episode/0Cvp9D0nwQAkpQOcksKYzC",
    date: "2026-10-01",
  },
  {
    title: "Two Guys and Roger Henley",
    url: "https://open.spotify.com/episode/5ZKhEjjJ1AwHLeQbxJj7OH",
    date: "2026-09-01",
  },
];

// Fixed to UTC so the server and the browser render the same string.
// Without it a visitor west of the meridian sees the previous day and
// React complains about the mismatch.
function longDate(iso: string): string {
  return new Date(`${iso}T12:00:00Z`).toLocaleDateString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
    timeZone: "UTC",
  });
}

export function Podcasts() {
  return (
    <section
      id="podcasts"
      aria-labelledby="podcasts-heading"
      className="scroll-mt-20 border-t border-line bg-canvas"
    >
      <div className="mx-auto max-w-6xl px-6 py-20 sm:px-10 sm:py-24">
        <div className="flex items-start gap-5 sm:gap-6">
          {/* The show's own cover art, cropped from the tile the show
              uses. alt is empty on purpose: the only thing the image
              says is the show's name, and the heading beside it
              already says that, so announcing it twice is noise for a
              screen reader rather than information. */}
          <Image
            src="/images/two-guys-and-a-plc.webp"
            alt=""
            width={256}
            height={256}
            sizes="80px"
            className="size-16 shrink-0 rounded-md border border-line sm:size-20"
          />
          <div className="min-w-0">
            <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
              on other people&rsquo;s shows
            </p>
            <h2
              id="podcasts-heading"
              className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
              style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
            >
              {SHOW}.
            </h2>
          </div>
        </div>
        <p className="mt-5 max-w-2xl text-base leading-relaxed text-ink-2">
          Two conversations about controls, plant systems and where applied
          AI actually earns its place on a production floor.
        </p>

        <ul className="mt-10 divide-y divide-line border-y border-line">
          {EPISODES.map((e) => (
            <li key={e.url}>
              <a
                href={e.url}
                rel="noopener noreferrer"
                target="_blank"
                className="group flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2 py-6 no-underline"
              >
                <span className="max-w-2xl text-lg leading-snug text-ink transition-colors group-hover:text-accent">
                  {e.title}
                </span>
                <span className="flex items-baseline gap-5">
                  <span className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
                    {longDate(e.date)}
                  </span>
                  <span className="font-mono text-[11px] tracking-[0.14em] text-accent uppercase">
                    Listen
                  </span>
                </span>
              </a>
            </li>
          ))}
        </ul>

        <p className="mt-6 text-sm text-ink-3">
          Opens on Spotify. Nothing from Spotify loads on this page until
          you choose to go.
        </p>
      </div>
    </section>
  );
}

// The same two episodes as an HMI panel. Same data, same order, so the
// modes cannot drift.
//
// No logo here, deliberately. OT mode renders this page as a plant
// overview screen, and a real HMI does not carry a vendor's brand art
// on a status tile. The show's name is already the first line of each
// panel, which is the information the logo would be carrying.
export function PodcastsOt() {
  return (
    <div className="grid gap-px border border-line-strong bg-line-strong sm:grid-cols-2">
      {EPISODES.map((e) => (
        <a
          key={e.url}
          href={e.url}
          rel="noopener noreferrer"
          target="_blank"
          className="bg-paper-2 p-4 no-underline"
        >
          <p className="font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
            {SHOW}
          </p>
          <p className="mt-1 font-mono text-[13px] tracking-[0.08em] text-accent uppercase hover:underline">
            {e.title}
          </p>
          <p className="mt-1 font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
            {longDate(e.date)} · ext
          </p>
        </a>
      ))}
    </div>
  );
}
