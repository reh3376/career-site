"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { chatClient } from "@/lib/chat-client";

import { Message, type ChatMessage } from "./message";
import { Waiting, WaitingNote } from "./waiting";

// The conversation, shared by the side panel and the full page at /ask.
//
// One component for both because they are the same thing at two sizes,
// and because the alternative is two implementations of the stream
// handling that drift apart. The page passes `roomy` to open the
// spacing up; nothing else differs.
//
// The stream is real even though the server currently sends the answer
// as a single delta: `start` arrives as soon as the question is stored,
// which is what lets the wait be shown honestly against a ten to
// twenty-five second answer. When the sidecar gains a streaming RPC the
// deltas become many and nothing here changes.

type Props = {
  roomy?: boolean;
  /** Content item the panel was opened from, for suggestions. */
  contextContentId?: string;
};

export function Thread({ roomy = false, contextContentId }: Props) {
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [draft, setDraft] = useState("");
  const [waiting, setWaiting] = useState(false);
  const [error, setError] = useState("");
  const [asked, setAsked] = useState(0);
  const endRef = useRef<HTMLDivElement>(null);
  const startedRef = useRef(false);

  // Open a conversation once, on first mount, and keep it for the life
  // of the panel. The disclosure comes back with it (FR-CHAT-02) and is
  // rendered as a real message in the thread rather than as chrome
  // above it, because chrome is what a reader scrolls past.
  useEffect(() => {
    if (startedRef.current) return;
    startedRef.current = true;
    (async () => {
      try {
        const res = await chatClient.createConversation({
          contextContentId: contextContentId ?? "",
        });
        setConversationId(res.conversation?.id ?? null);
        if (res.disclosure) {
          setMessages([
            {
              role: "assistant",
              text: res.disclosure.text,
              ephemeral: true,
            },
          ]);
        }
      } catch (e) {
        setError(friendly(e));
      }
      try {
        const s = await chatClient.getSuggestions({
          contentId: contextContentId ?? "",
        });
        setSuggestions(s.questions ?? []);
      } catch {
        // Suggestions are a garnish. Losing them is not worth an error
        // in front of someone who came here to ask something.
      }
    })();
  }, [contextContentId]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [messages, waiting]);

  const send = useCallback(
    async (text: string) => {
      const question = text.trim();
      if (!question || waiting) return;
      if (!conversationId) {
        setError("The conversation has not opened yet. Try again in a moment.");
        return;
      }
      setError("");
      setDraft("");
      setAsked((n) => n + 1);
      setMessages((m) => [...m, { role: "user", text: question }]);
      setWaiting(true);

      try {
        let assistantText = "";
        let started = false;
        for await (const res of chatClient.sendMessage({
          conversationId,
          text: question,
          contextContentId: contextContentId ?? "",
        })) {
          const e = res.event;
          if (e.case === "delta") {
            assistantText += e.value.text;
            if (!started) {
              started = true;
              setWaiting(false);
              setMessages((m) => [
                ...m,
                { role: "assistant", text: assistantText },
              ]);
            } else {
              setMessages((m) =>
                replaceLast(m, { role: "assistant", text: assistantText }),
              );
            }
          } else if (e.case === "done") {
            const msg = e.value.message;
            if (msg) {
              setMessages((m) =>
                replaceLast(m, {
                  id: msg.id,
                  role: "assistant",
                  text: msg.text,
                  citations: msg.citations?.map((c) => ({
                    chunkId: c.chunkId,
                    title: c.title,
                    path: c.path,
                    heading: c.heading,
                    rank: c.rank,
                  })),
                  proposedAction: msg.proposedAction
                    ? {
                        action: msg.proposedAction.action,
                        arg: msg.proposedAction.arg,
                      }
                    : null,
                  flags: msg.flags
                    ? {
                        outOfScope: msg.flags.outOfScope,
                        noSupport: msg.flags.noSupport,
                        degraded: msg.flags.degraded,
                        qaMatch: msg.flags.qaMatch,
                      }
                    : undefined,
                }),
              );
            }
          }
        }
      } catch (e) {
        setError(friendly(e));
      } finally {
        setWaiting(false);
      }
    },
    [conversationId, contextContentId, waiting],
  );

  const showSuggestions = suggestions.length > 0 && asked === 0 && !waiting;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div
        className={`min-h-0 flex-1 overflow-y-auto ${roomy ? "space-y-6 py-2" : "space-y-5"}`}
      >
        {messages.map((m, i) => (
          <Message key={m.id ?? `${m.role}-${i}`} message={m} />
        ))}

        {asked === 0 && !waiting ? <WaitingNote /> : null}
        {waiting ? <Waiting /> : null}

        {showSuggestions ? (
          <div>
            <p className="font-mono text-[11px] tracking-[0.12em] text-ink-3 uppercase">
              Things people ask
            </p>
            <ul className="mt-2 space-y-2">
              {suggestions.map((q) => (
                <li key={q}>
                  <button
                    type="button"
                    onClick={() => send(q)}
                    className="text-left text-sm text-accent underline underline-offset-2 hover:text-accent-hover"
                  >
                    {q}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        ) : null}

        {error ? (
          <p className="border-l-2 border-danger bg-paper-2 px-3 py-2 text-sm text-ink">
            {error}
          </p>
        ) : null}

        <div ref={endRef} />
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          send(draft);
        }}
        className="mt-3 shrink-0 border-t border-line pt-3"
      >
        <label htmlFor="ask-input" className="sr-only">
          Ask a question about Roger
        </label>
        <textarea
          id="ask-input"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            // Enter sends, shift+enter makes a new line. A question is
            // usually one line and a send button is a second target to
            // find on a phone.
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              send(draft);
            }
          }}
          rows={roomy ? 3 : 2}
          maxLength={4000}
          disabled={waiting}
          placeholder="Ask about his work, what he has built, or what he has written."
          className="w-full resize-none border border-line bg-canvas px-3 py-2 text-sm leading-relaxed text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none disabled:opacity-60"
        />
        <div className="mt-2 flex items-center justify-between gap-3">
          <p className="font-mono text-[11px] tracking-[0.1em] text-ink-3 uppercase">
            AI assistant
          </p>
          <button
            type="submit"
            disabled={waiting || !draft.trim()}
            className="border border-accent px-4 py-1.5 font-mono text-[11px] tracking-[0.12em] text-accent uppercase transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
          >
            {waiting ? "Reading" : "Ask"}
          </button>
        </div>
      </form>
    </div>
  );
}

function replaceLast(list: ChatMessage[], next: ChatMessage): ChatMessage[] {
  if (list.length === 0) return [next];
  return [...list.slice(0, -1), next];
}

// Connect errors carry a code and a server message. The server messages
// here are written for a reader ("sign in to ask a question"), so they
// are shown as-is; anything else gets a plain sentence rather than a
// stack of protocol vocabulary.
function friendly(e: unknown): string {
  const msg = e instanceof Error ? e.message : "";
  if (/unauthenticated|sign in/i.test(msg)) {
    return "Sign in to ask a question.";
  }
  if (/unavailable|not available/i.test(msg)) {
    return "The assistant is not available right now. Try again shortly.";
  }
  const cleaned = msg.replace(/^\[[a-z_ ]+\]\s*/i, "").trim();
  return cleaned || "Something went wrong asking that.";
}
