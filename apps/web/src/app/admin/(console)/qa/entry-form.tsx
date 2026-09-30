"use client";

import { useActionState, useState } from "react";
import { useFormStatus } from "react-dom";

import {
  createQaEntryAction,
  updateQaEntryAction,
  type QaState,
} from "./actions";

export type Source = { title?: string; path?: string };

type Props = {
  /** Absent for the new-entry form. */
  id?: string;
  question?: string;
  answer?: string;
  sources?: Source[];
  tags?: string[];
  coversRestricted?: boolean;
  enabled?: boolean;
};

const SOURCE_ROWS = 3;

// Writing and editing one bank entry.
//
// The whole point of the bank is that what comes back is what Roger
// wrote, word for word, with no model in between. So this form is a
// plain editor and nothing on it offers to draft, improve or rewrite an
// answer: the moment an entry goes through generation it loses the one
// property that lets it answer the topics the assistant otherwise
// refuses.
export function QaEntryForm({
  id,
  question = "",
  answer = "",
  sources = [],
  tags = [],
  coversRestricted = false,
  enabled = false,
}: Props) {
  const isNew = !id;
  const [state, action] = useActionState<QaState, FormData>(
    isNew ? createQaEntryAction : updateQaEntryAction,
    {},
  );
  const [restricted, setRestricted] = useState(coversRestricted);
  const [answerText, setAnswerText] = useState(answer);

  return (
    <form action={action} className="space-y-4">
      {id ? <input type="hidden" name="id" value={id} /> : null}

      <label className="block">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          the question, as you would put it
        </span>
        <span className="mt-1 block text-sm text-ink-3">
          This is also shown as a suggested question, and it is the first
          wording the bank matches on. Changing it later stops the old wording
          matching straight away.
        </span>
        <input
          name="question"
          defaultValue={question}
          required
          maxLength={500}
          className="mt-2 w-full border border-line bg-transparent px-3 py-2 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
          placeholder="Are you open to relocating?"
        />
      </label>

      <label className="block">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          your answer, word for word
        </span>
        <span className="mt-1 block text-sm text-ink-3">
          Served exactly as written, in the first person. No model ever rewrites
          it, which is why this is the only place the assistant can speak about
          things it otherwise refuses.
        </span>
        <textarea
          name="answer"
          value={answerText}
          onChange={(e) => setAnswerText(e.target.value)}
          required
          rows={6}
          maxLength={8000}
          className="mt-2 w-full border border-line bg-transparent px-3 py-2 text-sm leading-relaxed text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
          placeholder="I am, for the right role."
        />
        <span className="mt-1 block font-mono text-[11px] text-ink-3">
          {answerText.length} of 8000 characters
        </span>
      </label>

      {isNew ? (
        <label className="block">
          <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            other ways people ask it
          </span>
          <span className="mt-1 block text-sm text-ink-3">
            One per line. Matching runs over every phrasing, so variants are how
            one answer covers the several ways the same thing gets asked. You
            can add more later.
          </span>
          <textarea
            name="phrasings"
            rows={3}
            className="mt-2 w-full border border-line bg-transparent px-3 py-2 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            placeholder={"would you move?\nare you willing to relocate"}
          />
        </label>
      ) : null}

      <fieldset>
        <legend className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          sources shown under the answer
        </legend>
        <span className="mt-1 block text-sm text-ink-3">
          A path on this site, starting with a single slash. A title with no
          path shows as a source with no link, which is how something
          unpublished is credited.
        </span>
        <div className="mt-2 space-y-2">
          {Array.from({ length: SOURCE_ROWS }).map((_, i) => (
            <div key={i} className="flex flex-col gap-2 sm:flex-row">
              <input
                name={`source_title_${i}`}
                defaultValue={sources[i]?.title ?? ""}
                maxLength={200}
                placeholder="Title"
                className="w-full border border-line bg-transparent px-3 py-2 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none sm:w-1/2"
              />
              <input
                name={`source_path_${i}`}
                defaultValue={sources[i]?.path ?? ""}
                maxLength={256}
                placeholder="/about"
                className="w-full border border-line bg-transparent px-3 py-2 font-mono text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none sm:w-1/2"
              />
            </div>
          ))}
        </div>
      </fieldset>

      <label className="block">
        <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          tags
        </span>
        <input
          name="tags"
          defaultValue={tags.join(", ")}
          placeholder="logistics, availability"
          className="mt-1 w-full border border-line bg-transparent px-3 py-2 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
      </label>

      <label className="flex items-start gap-3">
        <input
          type="checkbox"
          name="covers_restricted"
          defaultChecked={coversRestricted}
          onChange={(e) => setRestricted(e.target.checked)}
          className="mt-1 accent-accent"
        />
        <span className="text-sm text-ink">
          This answers something the assistant otherwise refuses
          <span className="mt-1 block text-ink-3">
            Compensation, references, an employer matter, or something personal.
            Tick this so every such statement can be listed later, and so
            opening one of those doors is a decision rather than a side effect.
          </span>
        </span>
      </label>

      <label className="flex items-start gap-3">
        <input
          type="checkbox"
          name="enabled"
          defaultChecked={enabled}
          className="mt-1 accent-accent"
        />
        <span className="text-sm text-ink">
          Approved, use it
          <span className="mt-1 block text-ink-3">
            Ticking this is the approval. Until it is ticked the entry is never
            matched and never served.
          </span>
        </span>
      </label>

      {restricted ? (
        <p className="border-l-2 border-danger bg-paper-2 px-4 py-3 text-sm text-ink">
          Approving this lets the assistant answer a question it would otherwise
          decline, using these exact words and nothing else. It will not
          improvise around them.
        </p>
      ) : null}

      <div className="flex flex-wrap items-center gap-4">
        <Submit isNew={isNew} />
        {state.error ? (
          <p className="text-sm text-danger">{state.error}</p>
        ) : null}
        {state.saved ? (
          <p className="text-sm text-signal">
            Saved. It can match once the embedding job has run, within five
            minutes.
          </p>
        ) : null}
      </div>
    </form>
  );
}

function Submit({ isNew }: { isNew: boolean }) {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex items-center border border-accent px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-accent transition-colors hover:bg-accent hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Saving..." : isNew ? "Add entry" : "Save changes"}
    </button>
  );
}
