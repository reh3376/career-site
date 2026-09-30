"use client";

import Link from "next/link";
import { useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { chatClient } from "@/lib/chat-client";

export type Citation = {
  chunkId?: string;
  title?: string;
  path?: string;
  heading?: string;
  rank?: number;
};

export type ChatMessage = {
  id?: string;
  role: "user" | "assistant" | "owner";
  text: string;
  citations?: Citation[];
  flags?: {
    outOfScope?: boolean;
    noSupport?: boolean;
    degraded?: boolean;
    qaMatch?: boolean;
  };
  /** Local-only: the disclosure, which is never stored and never rated. */
  ephemeral?: boolean;
};

// One message.
//
// Assistant text is Markdown, rendered by react-markdown, which does
// not emit raw HTML unless rehype-raw is added and it is not
// (NFR-SEC-07). The persona is told to write paragraphs, short lists
// and links and nothing else, so the renderer is deliberately given a
// small component map rather than left to style arbitrary output.
export function Message({ message }: { message: ChatMessage }) {
  if (message.role === "user") {
    return (
      <div className="flex justify-end">
        <p className="max-w-[85%] rounded-md bg-accent-soft px-4 py-2 text-sm leading-relaxed whitespace-pre-wrap text-ink">
          {message.text}
        </p>
      </div>
    );
  }

  const isOwner = message.role === "owner";
  return (
    <div>
      {isOwner ? (
        <p className="mb-1 font-mono text-[11px] tracking-[0.12em] text-signal uppercase">
          Roger, replying himself
        </p>
      ) : null}

      <div className="text-sm leading-relaxed text-ink">
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={MARKDOWN}>
          {message.text}
        </ReactMarkdown>
      </div>

      <Flags flags={message.flags} />
      <Citations citations={message.citations} />
      {message.id && !message.ephemeral && !isOwner ? (
        <Rating messageId={message.id} />
      ) : null}
    </div>
  );
}

// A link an answer writes is only ever followed if it points at this
// site.
//
// The assistant is grounded in retrieved passages, and a passage is
// untrusted data. An external link in generated text is therefore a way
// for corpus content to put a destination in front of a reader under
// Roger's name, which is the one thing a grounded answer must not be
// able to do. Anything that is not a site-relative path renders as
// plain text, so the words survive and the destination does not.
const MARKDOWN = {
  a({ href, children }: { href?: string; children?: React.ReactNode }) {
    const internal =
      typeof href === "string" &&
      href.startsWith("/") &&
      !href.startsWith("//") &&
      !href.startsWith("/\\");
    if (!internal) return <>{children}</>;
    return (
      <Link href={href} className="text-accent underline underline-offset-2">
        {children}
      </Link>
    );
  },
  p({ children }: { children?: React.ReactNode }) {
    return <p className="mb-3 last:mb-0">{children}</p>;
  },
  ul({ children }: { children?: React.ReactNode }) {
    return <ul className="mb-3 list-disc space-y-1 pl-5 last:mb-0">{children}</ul>;
  },
  ol({ children }: { children?: React.ReactNode }) {
    return (
      <ol className="mb-3 list-decimal space-y-1 pl-5 last:mb-0">{children}</ol>
    );
  },
  code({ children }: { children?: React.ReactNode }) {
    return <code className="font-mono text-[0.9em]">{children}</code>;
  },
};

// Why an answer looks the way it does.
//
// Four situations render as a short reply and mean entirely different
// things, so the surface says which. Without this, "I don't have
// anything in my records about that" and "the model is down" are the
// same paragraph, and a reader draws the wrong conclusion about how
// much Roger has actually written.
function Flags({ flags }: { flags?: ChatMessage["flags"] }) {
  if (!flags) return null;
  if (flags.degraded) {
    return (
      <Note tone="warning">
        The model behind this is unavailable right now, so this is not a full
        answer.
      </Note>
    );
  }
  if (flags.qaMatch) {
    return (
      <Note tone="quiet">
        Roger wrote this answer himself, word for word.
      </Note>
    );
  }
  if (flags.noSupport) {
    return (
      <Note tone="quiet">
        Nothing in his records covered this, so nothing was invented.
      </Note>
    );
  }
  return null;
}

function Note({
  tone,
  children,
}: {
  tone: "warning" | "quiet";
  children: React.ReactNode;
}) {
  return (
    <p
      className={`mt-2 font-mono text-[11px] leading-relaxed tracking-[0.06em] ${
        tone === "warning" ? "text-warning" : "text-ink-3"
      }`}
    >
      {children}
    </p>
  );
}

// Sources. A private-corpus source has a title and no path: it may
// inform an answer and must never be linked or quoted, so it is shown
// as a name without a destination rather than hidden, which would make
// the answer look less supported than it is.
function Citations({ citations }: { citations?: Citation[] }) {
  if (!citations?.length) return null;
  return (
    <ol className="mt-3 space-y-1 border-t border-line pt-2">
      {citations.map((c, i) => (
        <li key={`${c.rank ?? i}-${c.chunkId ?? i}`} className="text-[13px]">
          <span className="font-mono text-[11px] text-ink-3">
            [{c.rank ?? i + 1}]
          </span>{" "}
          {c.path ? (
            <Link
              href={c.path}
              className="text-accent underline underline-offset-2"
            >
              {c.title || c.path}
            </Link>
          ) : (
            <span className="text-ink-2">
              {c.title || "an unpublished note"}
              <span className="ml-2 text-ink-3">not published</span>
            </span>
          )}
        </li>
      ))}
    </ol>
  );
}

// FR-CHAT-14. The rating is a queue signal for Roger's own review, not
// a grade: it says an answer landed badly with someone, which is the
// row worth opening first, and says nothing about why.
function Rating({ messageId }: { messageId: string }) {
  const [sent, setSent] = useState<"up" | "down" | null>(null);
  const [comment, setComment] = useState("");
  const [showComment, setShowComment] = useState(false);
  const [error, setError] = useState("");

  async function rate(up: boolean, withComment = "") {
    try {
      await chatClient.rateMessage({
        messageId,
        rating: up ? 1 : 2,
        comment: withComment,
      });
      setSent(up ? "up" : "down");
      setShowComment(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not save that.");
    }
  }

  if (sent && !showComment) {
    return (
      <p className="mt-2 font-mono text-[11px] tracking-[0.12em] text-ink-3 uppercase">
        {sent === "up" ? "Marked helpful" : "Marked unhelpful"}
      </p>
    );
  }

  return (
    <div className="mt-2">
      <div className="flex items-center gap-4">
        <button
          type="button"
          onClick={() => rate(true)}
          className="font-mono text-[11px] tracking-[0.12em] text-ink-3 uppercase underline underline-offset-2 hover:text-accent"
        >
          Helpful
        </button>
        <button
          type="button"
          onClick={() => setShowComment(true)}
          className="font-mono text-[11px] tracking-[0.12em] text-ink-3 uppercase underline underline-offset-2 hover:text-accent"
        >
          Not helpful
        </button>
      </div>
      {showComment ? (
        <div className="mt-2">
          <textarea
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            rows={2}
            maxLength={1000}
            placeholder="What was wrong with it? Optional."
            className="w-full border border-line bg-canvas px-3 py-2 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
          />
          <button
            type="button"
            onClick={() => rate(false, comment)}
            className="mt-2 border border-line px-3 py-1 font-mono text-[11px] tracking-[0.12em] text-ink-2 uppercase hover:border-accent hover:text-accent"
          >
            Send
          </button>
        </div>
      ) : null}
      {error ? <p className="mt-1 text-[13px] text-danger">{error}</p> : null}
    </div>
  );
}
