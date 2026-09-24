import type { Metadata } from "next";
import { redirect } from "next/navigation";

import { getJdBands } from "@/lib/jd-bands";
import { getSessionCookie } from "@/lib/session";
import { getSessionUser } from "@/lib/session-user";

import { getJdQuota } from "./actions";
import { JdForm } from "./form";
import { MySubmissions } from "./my-submissions";

export const metadata: Metadata = {
  title: "JD upload",
  description:
    "Paste a job description; Roger's reviewer scores it against his experience, requirement by requirement, and when the fit is strong or better sends back a résumé tailored to that specific posting.",
};
export const dynamic = "force-dynamic";

// Members only: the API rejects anonymous submissions too, this just
// sends a visitor to sign in instead of showing a form that cannot work.
export default async function JdUploadPage() {
  const me = await getSessionUser();
  if (!me) redirect("/login?next=/jd-upload");
  const bands = await getJdBands(await getSessionCookie());
  const quota = await getJdQuota();
  const threshold = bands.strong.toFixed(2);
  const weakEdge = bands.weak.toFixed(2);
  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        for hiring managers
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Upload JD for review.
      </h1>
      <p className="mt-6 max-w-2xl text-lg leading-relaxed text-ink-2">
        Roger keeps a master career corpus, 30 years across
        control rooms, manufacturing plants, decision infrastructure,
        and AI/ML. This JD review evaluates his skill set against the
        JD criteria, requirement by requirement, and produces a
        skills-match score in one of five fit categories: very strong,
        strong, possible, weak, very weak. When the fit is strong or
        better (score &ge; {threshold}), a 2-page r&eacute;sum&eacute;
        custom built for the uploaded JD is generated as a locked PDF.
      </p>

      <section className="mt-12 grid gap-4 border-l-2 border-line pl-5 text-sm leading-relaxed text-ink-2">
        <p>
          <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            what happens
          </span>
        </p>
        <ol className="ml-4 list-decimal space-y-1.5 text-sm">
          <li>
            The full JD text lands in Roger&rsquo;s inbox with the score
            and every requirement verdict.
          </li>
          <li>
            <span className="text-ink">Strong or very strong (score &ge; {threshold}):</span>{" "}
            a 2-page résumé is generated from the master corpus, tailored
            to the posting, and emailed to you as a locked PDF.
          </li>
          <li>
            <span className="text-ink">Possible or weak (score &ge; {weakEdge}):</span>{" "}
            no résumé yet. Roger will look at the posting himself and
            reply personally.
          </li>
          <li>
            <span className="text-ink">Very weak:</span> the review shows
            what was and was not evidenced, and no further action is
            taken.
          </li>
        </ol>
        <p className="text-xs text-ink-3">
          Scoring is requirement by requirement: the posting is broken
          into checkable asks, each is judged in its own call against
          Roger&rsquo;s career corpus, and the score is computed from
          those verdicts rather than guessed by a model. A review runs 30
          to 60 minutes and sometimes longer; you can close the page and
          reopen it from your
          submissions below, and the outcome is emailed. See{" "}
          <a
            href="/how-ask-roger-works"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            how this works
          </a>
          .
        </p>
      </section>

      <div className="mt-14">
        <JdForm threshold={threshold} quota={quota} />
      </div>

      <MySubmissions />

      <p className="mt-14 border-t border-line pt-6 text-sm text-ink-3">
        Rather email?{" "}
        <a
          href="/contact"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          Use the contact form
        </a>
        .
      </p>
    </div>
  );
}
