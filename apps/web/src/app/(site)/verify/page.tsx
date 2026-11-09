import type { Metadata } from "next";
import Link from "next/link";
import { create } from "@bufbuild/protobuf";
import { ConnectError } from "@connectrpc/connect";

import { VerifyRequestSchema } from "@/gen/career/v1/auth_pb";
import { authClient } from "@/lib/api";

import { CodeForm } from "./code-form";
import { Verified } from "./verified";

export const metadata: Metadata = {
  title: "Verify your email",
};

// Two ways in, as the email says: the link, which carries a token and
// is verified here on the server; or the six-digit code, entered on
// this page with the address, which a server action verifies. Without
// a token the page is the code form, not an error.
export const dynamic = "force-dynamic";

type VerifyResult = { ok: true; name: string; status: string; pilot: boolean } | { ok: false; error: string };

async function verify(token: string): Promise<VerifyResult> {
  try {
    const resp = await authClient.verify(
      create(VerifyRequestSchema, {
        credential: { case: "token", value: token },
      }),
    );
    return { ok: true, name: resp.me?.name ?? "", status: statusName(resp.me?.status), pilot: resp.me?.pilotApproved === true };
  } catch (err) {
    const message =
      err instanceof ConnectError ? err.rawMessage : err instanceof Error ? err.message : "Verification failed.";
    return { ok: false, error: message };
  }
}

// The generated client gives the enum as a number; the code path gets
// the JSON name. Verified compares the name.
function statusName(v: number | undefined): string {
  switch (v) {
    case 2:
      return "MEMBER_STATUS_ACTIVE";
    case 3:
      return "MEMBER_STATUS_PENDING_APPROVAL";
    default:
      return "";
  }
}

export default async function VerifyPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string; email?: string }>;
}) {
  const { token, email } = await searchParams;

  if (!token) {
    return (
      <Wrapper>
        <Kicker tone="accent">verify your email</Kicker>
        <Heading>Enter the code from your email.</Heading>
        <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">
          The email has a link and a six-digit code; either one works. If you did not get it, check your spam folder or{" "}
          <Link
            href="/register"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            register again
          </Link>
          .
        </p>
        <CodeForm initialEmail={email ?? ""} />
      </Wrapper>
    );
  }

  const result = await verify(token);

  if (!result.ok) {
    return (
      <Wrapper>
        <Kicker tone="signal">verification failed</Kicker>
        <Heading>That link didn&rsquo;t work.</Heading>
        <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">{result.error}</p>
        <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">
          If you still have the email, the six-digit code in it works here. Otherwise{" "}
          <Link
            href="/register"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            register again
          </Link>
          .
        </p>
        <CodeForm initialEmail={email ?? ""} />
      </Wrapper>
    );
  }

  return (
    <Wrapper>
      <Verified name={result.name} status={result.status} pilot={result.pilot} />
    </Wrapper>
  );
}

function Wrapper({ children }: { children: React.ReactNode }) {
  return <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">{children}</div>;
}

function Kicker({ tone, children }: { tone: "accent" | "signal"; children: React.ReactNode }) {
  return (
    <p className={`font-mono text-[11px] uppercase tracking-[0.14em] ${tone === "signal" ? "text-signal" : "text-accent"}`}>
      {children}
    </p>
  );
}

function Heading({ children }: { children: React.ReactNode }) {
  return (
    <h1
      className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
      style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
    >
      {children}
    </h1>
  );
}
