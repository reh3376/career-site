import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Privacy",
  description:
    "What this site collects, why, how long it is kept, and how to ask for it to be removed.",
};

const UPDATED = "22 September 2026";

// Plain statements, one per paragraph, in the order a visitor meets
// them: anonymous visit, contact, registration, JD review, then the
// rules that apply to all of it. Kept in step with docs/events/README.md
// and the retention job in the api; change both when either changes.
export default function PrivacyPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        the small print
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Privacy.
      </h1>
      <p className="mt-4 text-sm text-ink-3">Last updated {UPDATED}.</p>

      <div className="mt-10 space-y-8 text-base leading-relaxed text-ink-2">
        <Section title="Who runs this site">
          <p>
            This is a personal site run by Roger Henley. There is no company
            behind it, no advertising, and nothing collected here is sold or
            shared with a third party for their own use. The site runs on one
            rented server in the United States.
          </p>
        </Section>

        <Section title="Anonymous visits">
          <p>
            Every visit writes a short record of what happened: the page, when
            it was opened and left, the previous site if the browser reports
            one, a coarse device class (phone, tablet, desktop), the display
            mode you picked, and which links you followed. These records exist
            so I can see what works on the site and what nobody reads.
          </p>
          <p>
            A random id is set in a first-party cookie on your first visit so
            repeat visits from the same browser can be counted as one visitor.
            It carries no personal data and is not shared with anyone. Your
            network address is stored only as a salted hash, which cannot be
            turned back into the address. No third-party analytics or tracking
            scripts run on this site.
          </p>
        </Section>

        <Section title="Contact form">
          <p>
            A message through the contact form stores your name, email address,
            the message, and a hashed network address. It is used to answer you
            and for nothing else.
          </p>
        </Section>

        <Section title="Registered members">
          <p>
            Requesting access stores your name, email address, organization, and
            stated role, plus the approval decision and when your access ends.
            Signing in stores a session record with a hashed network address and
            browser type. Members&rsquo; page views and actions on the site are
            recorded against their account in the same event stream as anonymous
            visits.
          </p>
        </Section>

        <Section title="Job description reviews">
          <p>
            A job description you submit is stored in full, along with the
            requirement-by-requirement assessment the site produces from it and
            the résumé written for it. The text is processed by a language model
            that runs on this server; it is not sent to any outside AI service.
            Every model decision is logged so I can review and correct it.
            Reviews stay attached to your account and are mailed to you and to
            me when they finish.
          </p>
        </Section>

        <Section title="Email">
          <p>
            Transactional email (verification, approval, review results) is sent
            through Resend, which sees the recipient address and the message.
            Delivery status is logged. No marketing email is sent.
          </p>
        </Section>

        <Section title="How long things are kept">
          <ul className="list-disc space-y-2 pl-5">
            <li>
              Event records lose their identity columns (account, session,
              visitor id, address hash) after 13 months. The anonymous counts
              remain.
            </li>
            <li>
              Sessions end on sign-out or after 30 days, and are removed when
              access expires.
            </li>
            <li>
              Accounts, contact messages, and job description reviews are kept
              until you ask for them to be removed.
            </li>
          </ul>
        </Section>

        <Section title="Your choices">
          <p>
            You can ask for a copy of everything held about you, or for it to be
            deleted, by writing to the address below. Removing an account ends
            its sessions and detaches its reviews and event records from it; say
            so if you want the reviews themselves removed too. Blocking cookies
            in your browser stops the visitor id; the site still works.
          </p>
          <p className="text-sm text-ink-3">
            <a
              href="mailto:rogerhenley345@gmail.com"
              className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              rogerhenley345@gmail.com
            </a>
          </p>
        </Section>
      </div>
    </div>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <h2 className="font-display text-xl tracking-tight text-ink">{title}</h2>
      <div className="mt-3 space-y-3">{children}</div>
    </section>
  );
}
