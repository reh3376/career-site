import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";

import { AdminQueries } from "@/components/ask/admin-queries";
import { Thread } from "@/components/ask/thread";
import { getSessionUser } from "@/lib/session-user";

export const metadata: Metadata = { title: "Ask Roger" };
export const dynamic = "force-dynamic";

// The full-page assistant (FR-CHAT-01).
//
// Members only, and gated here as well as in the API. The proxy lets a
// request through on cookie presence alone, which is an optimistic
// check rather than a session, so a page that assumes it has a member
// is a page that will one day render for someone who is not.
export default async function AskPage() {
  const me = await getSessionUser();
  if (!me) redirect("/login?next=/ask");

  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-10 sm:py-14">
      <header>
        <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
          AI assistant
        </p>
        <h1 className="mt-2 font-display text-3xl leading-tight text-ink">
          Ask Roger
        </h1>
        <p className="mt-3 text-base leading-relaxed text-ink-2">
          Ask about his work, what he has built, or what he has written. Answers
          come from his own records and show you where each one came from.{" "}
          <Link
            href="/how-ask-roger-works"
            className="text-accent underline underline-offset-2"
          >
            How this works
          </Link>
        </p>
      </header>

      {/* Admin only, and it renders nothing for anyone else: the RPC
          refuses a member and the component stays empty. Above the
          thread because a live number is a different kind of thing
          from a conversation, and burying one in a scrollback is how
          it gets missed. */}
      <div className="mt-8">
        <AdminQueries />
      </div>

      {/* A fixed height rather than a growing page, so the composer
          stays where it was put and the reader is not chasing it down
          the screen after every answer. */}
      <div className="h-[min(70vh,44rem)]">
        <Thread roomy />
      </div>
    </div>
  );
}
