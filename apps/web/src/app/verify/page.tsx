import type { Metadata } from "next";
import Link from "next/link";
import { create } from "@bufbuild/protobuf";
import { ConnectError } from "@connectrpc/connect";

import { VerifyRequestSchema } from "@/gen/career/v1/auth_pb";
import { authClient } from "@/lib/api";

export const metadata: Metadata = {
  title: "Verify your email",
};

// Server-rendered so the verify call runs on the server (no token in the
// browser tab's fetch tree). On success we show the pending-approval message
// inline; on failure we render an actionable error.
export const dynamic = "force-dynamic";

type VerifyResult =
  | { ok: true; name: string }
  | { ok: false; error: string };

async function verify(token: string): Promise<VerifyResult> {
  try {
    const resp = await authClient.verify(
      create(VerifyRequestSchema, {
        credential: { case: "token", value: token },
      }),
    );
    return { ok: true, name: resp.me?.name ?? "" };
  } catch (err) {
    const message =
      err instanceof ConnectError
        ? err.rawMessage
        : err instanceof Error
          ? err.message
          : "Verification failed.";
    return { ok: false, error: message };
  }
}

export default async function VerifyPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string }>;
}) {
  const { token } = await searchParams;
  if (!token) {
    return (
      <Wrapper>
        <h1 className="mb-4 text-3xl font-semibold text-ink">Missing token</h1>
        <p className="text-ink-2">
          This page needs a verification token in the URL. Use the link from the email you were
          sent, or{" "}
          <Link href="/register" className="text-accent underline underline-offset-2">
            register again
          </Link>
          .
        </p>
      </Wrapper>
    );
  }

  const result = await verify(token);

  if (!result.ok) {
    return (
      <Wrapper>
        <h1 className="mb-4 text-3xl font-semibold text-ink">Verification failed</h1>
        <p className="mb-6 text-ink-2">{result.error}</p>
        <Link
          href="/register"
          className="inline-flex items-center rounded-md bg-accent px-5 py-3 text-sm font-medium text-white hover:bg-accent-hover"
        >
          Register again
        </Link>
      </Wrapper>
    );
  }

  return (
    <Wrapper>
      <h1 className="mb-4 text-3xl font-semibold text-ink">
        {result.name ? `Thanks, ${result.name.split(" ")[0]}.` : "Thanks."}
      </h1>
      <p className="mb-4 leading-relaxed text-ink-2">
        Your email is verified. Roger has been notified and will review your request &mdash; usually
        within a day.
      </p>
      <p className="text-sm text-ink-3">
        You&rsquo;ll receive an email either way. Nothing else you need to do right now.
      </p>
      <Link
        href="/"
        className="mt-8 inline-flex text-sm text-accent underline underline-offset-2 hover:text-accent-hover"
      >
        ← Back to home
      </Link>
    </Wrapper>
  );
}

function Wrapper({ children }: { children: React.ReactNode }) {
  return <div className="mx-auto max-w-lg px-6 py-16">{children}</div>;
}
