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
        <h1 className="mb-4 text-3xl font-semibold text-ink">Missing token</h1>
        <p className="text-ink-2">
          This page needs a signed decision token. Open the link from the approval email you were
          sent.
        </p>
      </Wrapper>
    );
  }

  const result = await processToken(token);

  switch (result.status) {
    case "approved":
      return (
        <Wrapper>
          <Badge color="green">Approved</Badge>
          <h1 className="mt-3 mb-4 text-3xl font-semibold text-ink">Access granted</h1>
          <p className="mb-2 text-ink-2">
            {result.email ? (
              <>
                <span className="font-medium text-ink">{result.email}</span> can now sign in.
              </>
            ) : (
              "The applicant can now sign in."
            )}
          </p>
          {result.message ? <p className="text-sm text-ink-3">{result.message}</p> : null}
          <p className="mt-6 text-sm text-ink-3">
            They&rsquo;ve been emailed a sign-in link. Different TTL wanted? Open the review console
            (Task 5) and re-approve with a custom period.
          </p>
        </Wrapper>
      );
    case "declined":
      return (
        <Wrapper>
          <Badge color="red">Declined</Badge>
          <h1 className="mt-3 mb-4 text-3xl font-semibold text-ink">Request declined</h1>
          <p className="mb-2 text-ink-2">
            {result.email ? (
              <>
                <span className="font-medium text-ink">{result.email}</span> has been notified.
              </>
            ) : (
              "The applicant has been notified."
            )}
          </p>
          {result.message ? <p className="text-sm text-ink-3">{result.message}</p> : null}
        </Wrapper>
      );
    case "already_decided":
      return (
        <Wrapper>
          <Badge color="amber">Already decided</Badge>
          <h1 className="mt-3 mb-4 text-3xl font-semibold text-ink">Already handled</h1>
          <p className="text-ink-2">{result.message ?? "This request has already been resolved."}</p>
          <p className="mt-6 text-sm text-ink-3">
            To change it, open the review console (coming in Task 5).
          </p>
        </Wrapper>
      );
    case "user_gone":
      return (
        <Wrapper>
          <Badge color="amber">Request gone</Badge>
          <h1 className="mt-3 mb-4 text-3xl font-semibold text-ink">This request no longer exists</h1>
          <p className="text-ink-2">
            {result.message ?? "The applicant's account was removed."}
          </p>
          <p className="mt-6 text-sm text-ink-3">
            If you approved them earlier, no action is needed. If not, they can register again.
          </p>
          <Link
            href="/"
            className="mt-6 inline-flex text-sm text-accent underline underline-offset-2 hover:text-accent-hover"
          >
            ← Back to home
          </Link>
        </Wrapper>
      );
    case "error":
    default:
      return (
        <Wrapper>
          <Badge color="red">Error</Badge>
          <h1 className="mt-3 mb-4 text-3xl font-semibold text-ink">Something went wrong</h1>
          <p className="text-ink-2">
            {result.detail ?? "Unknown error."}
            {result.message ? ` — ${result.message}` : null}
          </p>
          <Link href="/" className="mt-6 inline-flex text-sm text-accent underline underline-offset-2">
            ← Back to home
          </Link>
        </Wrapper>
      );
  }
}

function Wrapper({ children }: { children: React.ReactNode }) {
  return <div className="mx-auto max-w-lg px-6 py-16">{children}</div>;
}

function Badge({ color, children }: { color: "green" | "red" | "amber"; children: React.ReactNode }) {
  const bg = { green: "bg-green-50 text-green-800", red: "bg-red-50 text-red-900", amber: "bg-amber-50 text-amber-900" }[color];
  return (
    <span
      className={`inline-block rounded-full px-3 py-1 text-xs font-medium uppercase tracking-widest ${bg}`}
    >
      {children}
    </span>
  );
}
