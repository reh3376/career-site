"use client";

import Link from "next/link";

export type Action = { action?: string; arg?: string };

// An action the assistant offered, rendered as something the member
// presses.
//
// This component is the whole of D-25 on the client: the assistant
// proposed, and nothing happens until a person acts. It is a link, not
// a fetch, so there is no code path here that submits anything. The
// member lands on the page with the form, sees what it says, and
// decides.
//
// That is why there is no "confirm" button that posts, and why nothing
// is pre-filled beyond a category: the prompt-injection cases in the
// golden set include pre-filling a contact form with content the member
// did not write, and a link to a blank form cannot do that.
//
// Anything the server did not validate never arrives here, and anything
// this component does not recognise renders nothing rather than
// guessing.
export function ProposedActionCard({ action }: { action?: Action | null }) {
  const card = describe(action);
  if (!card) return null;

  return (
    <div className="mt-3 border border-line bg-paper-2 px-4 py-3">
      <p className="text-sm text-ink">{card.prompt}</p>
      <Link
        href={card.href}
        className="mt-2 inline-flex items-center border border-accent px-4 py-1.5 font-mono text-[11px] tracking-[0.12em] text-accent uppercase transition-colors hover:bg-accent hover:text-white"
      >
        {card.label}
      </Link>
    </div>
  );
}

type Card = { prompt: string; label: string; href: string };

// The copy says what pressing it does, in the member's terms, and does
// not claim anything has happened yet.
function describe(a?: Action | null): Card | null {
  if (!a?.action) return null;
  switch (a.action) {
    case "open_scheduler":
      return {
        prompt: "You can book time with Roger directly.",
        label: "Open the scheduler",
        href: "/meetings",
      };
    case "open_contact_form":
      return {
        prompt: "You can send Roger a message.",
        label: "Open the contact form",
        href: a.arg ? `/contact?category=${encodeURIComponent(a.arg)}` : "/contact",
      };
    case "open_contributor_request":
      return {
        prompt: a.arg
          ? `You can ask Roger for contributor access to ${a.arg}.`
          : "You can ask Roger for contributor access.",
        label: "Open the request form",
        href: a.arg
          ? `/contact?category=contributor_access&repo=${encodeURIComponent(a.arg)}`
          : "/contact?category=contributor_access",
      };
    default:
      // An action this build does not know about renders nothing. A
      // newer server can add one without this one inventing a
      // destination for it.
      return null;
  }
}
