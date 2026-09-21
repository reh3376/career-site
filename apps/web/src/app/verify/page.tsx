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
        <Kicker tone="signal">missing token</Kicker>
        <Heading>Nothing to verify.</Heading>
        <p className="mt-6 text-base leading-relaxed text-ink-2">
          This page needs a verification token in the URL. Use the link from
          the email you were sent, or{" "}
          <Link
            href="/register"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
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
        <Kicker tone="signal">verification failed</Kicker>
        <Heading>That link didn&rsquo;t work.</Heading>
        <p className="mt-6 text-base leading-relaxed text-ink-2">{result.error}</p>
        <p className="mt-8">
          <Link
            href="/register"
            className="inline-flex items-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
          >
            Register again
          </Link>
        </p>
      </Wrapper>
    );
  }

  return (
    <Wrapper>
      <Kicker tone="accent">email verified</Kicker>
      <Heading>
        {result.name ? `Thanks, ${result.name.split(" ")[0]}.` : "Thanks."}
      </Heading>
      <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">
        Your email is verified. Roger has been notified and will review your
        request, usually within a day.
      </p>
      <p className="mt-3 text-sm text-ink-3">
        You&rsquo;ll receive an email either way. Nothing else you need to do
        right now.
      </p>
      <p className="mt-10">
        <Link
          href="/"
          className="text-sm text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          Back to home
        </Link>
      </p>
    </Wrapper>
  );
}

function Wrapper({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      {children}
    </div>
  );
}

function Kicker({
  tone,
  children,
}: {
  tone: "accent" | "signal";
  children: React.ReactNode;
}) {
  const color = tone === "signal" ? "text-signal" : "text-accent";
  return (
    <p className={`font-mono text-[11px] uppercase tracking-[0.14em] ${color}`}>
      {children}
    </p>
  );
}

function Heading({ children }: { children: React.ReactNode }) {
  return (
    <h1
      className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
      style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
    >
      {children}
    </h1>
  );
}
