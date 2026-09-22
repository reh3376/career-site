import Link from "next/link";

// Every public page ends by naming the exchange: reading and looking
// are free, the reviewer is what an account is for. Shown only to
// anonymous visitors; a signed-in member already has it.
export function AccessNote({ variant = "read" }: { variant?: "read" | "gallery" }) {
  return (
    <aside className="mt-16 border-l-2 border-accent bg-paper-2 px-5 py-4">
      <p className="text-sm leading-relaxed text-ink-2">
        {variant === "gallery" ? "The rest of these, and the reviewer, are behind a free account. " : "Hiring for a role? "}
        <Link
          href="/register"
          className="text-accent underline decoration-accent/40 underline-offset-4"
        >
          Request access
        </Link>{" "}
        and you can put a job posting through it. It reads the posting
        requirement by requirement, checks each one against thirty years
        of records, and returns a two-page résumé written for that
        specific job.
      </p>
    </aside>
  );
}
