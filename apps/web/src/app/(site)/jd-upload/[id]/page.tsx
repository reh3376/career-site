import type { Metadata } from "next";
import Link from "next/link";
import { notFound, redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { getJdBands } from "@/lib/jd-bands";
import { getSessionCookie } from "@/lib/session";
import { getSessionUser } from "@/lib/session-user";

import { JdResult } from "../result";

export const metadata: Metadata = { title: "JD review" };
export const dynamic = "force-dynamic";

// One submission's review, reopenable from any signed-in session of
// the member who submitted it. The API releases the verdicts, résumé
// and PDF link to the owner without the tab-bound result token, so the
// link in the acknowledgement and in the finished-review email works
// after the original tab is gone.
export default async function JdReviewPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  if (!/^\d{1,18}$/.test(id)) notFound();
  const me = await getSessionUser();
  if (!me) redirect(`/login?next=/jd-upload/${id}`);
  const cookie = await getSessionCookie();
  if (!cookie) redirect(`/login?next=/jd-upload/${id}`);

  // Ownership check up front so a wrong id is a clean 404 rather than
  // a panel that polls a NotFound forever.
  const probe = await callApi({
    path: "/api/career.v1.JdService/GetJdResult",
    body: { submissionId: id },
    cookie,
  });
  if (probe.status === 404) notFound();

  const threshold = (await getJdBands(cookie)).strong.toFixed(2);
  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        jd review <span className="text-ink-4">·</span> #{id}
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Your review.
      </h1>
      <p className="mt-6 max-w-2xl text-base leading-relaxed text-ink-2">
        The posting was broken into requirements and each was checked against
        Roger&rsquo;s records. Above the {threshold} gate a two-page résumé is
        written for the posting; below it you still see exactly what was and was
        not evidenced.
      </p>

      <div className="mt-12">
        <JdResult submissionId={id} resultToken="" threshold={threshold} />
      </div>

      <p className="mt-14 border-t border-line pt-6 text-sm text-ink-3">
        <Link
          href="/jd-upload"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          All your submissions
        </Link>
        {" · "}
        <Link
          href="/contact"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          Reach Roger directly
        </Link>
      </p>
    </div>
  );
}
