import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Decision",
};

// One-click Accept / Decline landing. The email link Roger clicks lands here
// with ?token=SIGNED. This server component calls the API server-side to
// consume the token, then renders the result. Because the fetch happens
// inside the RSC, the browser never touches the API directly and there is
// no CSRF surface here (the token is the auth).
export const dynamic = "force-dynamic";

type DecisionResult =
  | { status: "approved"; email?: string; message?: string }
  | { status: "declined"; email?: string; message?: string }
  | { status: "already_decided"; email?: string; message?: string }
  | { status: "user_gone"; message?: string }
  | { status: "error"; detail?: string; message?: string };

async function processToken(token: string): Promise<DecisionResult> {
  const apiBase =
    process.env.API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
  try {
    const resp = await fetch(`${apiBase}/api/admin/decision`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
      cache: "no-store",
    });
    return (await resp.json()) as DecisionResult;
  } catch (err) {
    return {
      status: "error",
      detail: err instanceof Error ? err.message : String(err),
    };
  }
}

export default async function DecisionPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string }>;
}) {
  const { token } = await searchParams;
  if (!token) {
    return (
      <Wrapper>
        <Kicker tone="signal">missing token</Kicker>
        <Heading>Nothing to decide.</Heading>
        <Body>
          This page needs a signed decision token. Open the link from the
          approval email you were sent.
        </Body>
      </Wrapper>
    );
  }

  const result = await processToken(token);

  switch (result.status) {
    case "approved":
      return (
        <Wrapper>
          <Kicker tone="accent">approved</Kicker>
          <Heading>Access granted.</Heading>
          <Body>
            {result.email ? (
              <>
                <span className="font-mono text-ink">{result.email}</span> can
                now sign in.
              </>
            ) : (
              "The applicant can now sign in."
            )}
          </Body>
          {result.message ? (
            <p className="mt-3 text-sm text-ink-3">{result.message}</p>
          ) : null}
          <p className="mt-8 text-sm text-ink-3">
            They&rsquo;ve been emailed a sign-in link. Different TTL wanted?
            Open the review console (Task 5) and re-approve with a custom
            period.
          </p>
        </Wrapper>
      );
    case "declined":
      return (
        <Wrapper>
          <Kicker tone="signal">declined</Kicker>
          <Heading>Request declined.</Heading>
          <Body>
            {result.email ? (
              <>
                <span className="font-mono text-ink">{result.email}</span> has
                been notified.
              </>
            ) : (
              "The applicant has been notified."
            )}
          </Body>
          {result.message ? (
            <p className="mt-3 text-sm text-ink-3">{result.message}</p>
          ) : null}
        </Wrapper>
      );
    case "already_decided":
      return (
        <Wrapper>
          <Kicker tone="signal">already decided</Kicker>
          <Heading>Already handled.</Heading>
          <Body>
            {result.message ?? "This request has already been resolved."}
          </Body>
          <p className="mt-8 text-sm text-ink-3">
            To change it, open the review console (coming in Task 5).
          </p>
        </Wrapper>
      );
    case "user_gone":
      return (
        <Wrapper>
          <Kicker tone="signal">request gone</Kicker>
          <Heading>This request no longer exists.</Heading>
          <Body>{result.message ?? "The applicant's account was removed."}</Body>
          <p className="mt-8 text-sm text-ink-3">
            If you approved them earlier, no action is needed. If not, they can
            register again.
          </p>
          <p className="mt-8">
            <Link
              href="/"
              className="text-sm text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              Back to home
            </Link>
          </p>
        </Wrapper>
      );
    case "error":
    default:
      return (
        <Wrapper>
          <Kicker tone="signal">error</Kicker>
          <Heading>Something went wrong.</Heading>
          <Body>
            {result.detail ?? "Unknown error."}
            {result.message ? `, ${result.message}` : null}
          </Body>
          <p className="mt-8">
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

function Body({ children }: { children: React.ReactNode }) {
  return (
    <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">
      {children}
    </p>
  );
}
