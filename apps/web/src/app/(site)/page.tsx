import { ItLanding } from "@/components/landing/it-landing";
import { OtLanding } from "@/components/landing/ot-landing";
import { getSessionUser } from "@/lib/session-user";
import { personJsonLd } from "@/lib/site";
import { getSocialLinks } from "@/lib/social-links";
import { getUiMode } from "@/lib/ui-mode";

// Landing page. Reads the visitor's saved UI mode server-side and
// renders the matching landing surface. IT mode is the editorial
// default; OT mode re-renders the same content as a plant HMI
// overview screen. See ADR-0029 (frontend IT/OT dual mode).
export const dynamic = "force-dynamic";

export default async function HomePage() {
  const mode = await getUiMode();
  // Signed-out visitors get the section explaining what an account
  // adds. A member already has it and does not need selling.
  const signedIn = (await getSessionUser()) !== null;
  const social = getSocialLinks();
  return (
    <>
      {/* Structured data, so a search engine resolving "Roger Henley"
          has something better than a guess between namesakes. sameAs is
          the load-bearing part: it ties this site to the LinkedIn and
          GitHub profiles a recruiter already trusts. Emitted on the
          landing page only, which is the page that describes a person
          rather than a piece of writing. */}
      <script
        type="application/ld+json"
        // The payload is built in this file from a fixed shape and two
        // validated URLs, so there is no caller-supplied string in it.
        dangerouslySetInnerHTML={{
          __html: JSON.stringify(
            personJsonLd({ linkedin: social.linkedin, github: social.github }),
          ),
        }}
      />
      {mode === "ot" ? (
        <OtLanding signedIn={signedIn} />
      ) : (
        <ItLanding signedIn={signedIn} />
      )}
    </>
  );
}
