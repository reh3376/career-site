import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { deleteQaEntryAction, setQaEnabledAction } from "./actions";
import { QaEntryForm, type Source } from "./entry-form";
import { Phrasings, type Phrasing } from "./phrasings";

export const metadata: Metadata = { title: "Admin · Q&A bank" };
export const dynamic = "force-dynamic";

type Entry = {
  id: string;
  question?: string;
  answer?: string;
  sources?: Source[];
  tags?: string[];
  covers_restricted?: boolean;
  coversRestricted?: boolean;
  enabled?: boolean;
  updated_at?: string;
  updatedAt?: string;
  phrasings?: {
    id: string;
    text?: string;
    canonical?: boolean;
    embedded?: boolean;
  }[];
};

type ListResp = {
  entries?: Entry[];
  awaiting_embedding?: number | string;
  awaitingEmbedding?: number | string;
};

export default async function QaBankPage() {
  const cookie = await getSessionCookie();
  let entries: Entry[] = [];
  let awaiting = 0;
  let error = "";

  if (!cookie) {
    error = "Not signed in";
  } else {
    const resp = await callApi({
      path: "/api/career.v1.AdminService/ListQaEntries",
      body: { includeDisabled: true },
      cookie,
    });
    if (!resp.ok) {
      error = `HTTP ${resp.status}`;
    } else {
      const data = (await resp.json()) as ListResp;
      entries = data.entries ?? [];
      awaiting = Number(data.awaiting_embedding ?? data.awaitingEmbedding ?? 0);
    }
  }

  const live = entries.filter((e) => e.enabled).length;
  const restricted = entries.filter(
    (e) => e.covers_restricted ?? e.coversRestricted,
  );

  return (
    <div className="max-w-3xl">
      <h1 className="text-2xl text-ink">Q&amp;A bank</h1>
      <p className="mt-2 text-sm leading-relaxed text-ink-2">
        Your own answers, served word for word. Ask Roger checks this before it
        does anything else, and a match comes back at once with no model
        involved. On this hardware that is the difference between an instant
        answer and a fifteen second wait, so the questions people actually ask
        belong here.
      </p>
      <p className="mt-2 text-sm leading-relaxed text-ink-2">
        Nothing here is ever rewritten by a model. That is what lets an entry
        answer a question the assistant would otherwise decline.
      </p>

      {error ? (
        <p className="mt-6 border-l-2 border-danger bg-paper-2 px-4 py-3 text-sm text-ink">
          {error}
        </p>
      ) : null}

      <p className="mt-6 font-mono text-[11px] text-ink-3">
        {entries.length} {entries.length === 1 ? "entry" : "entries"} · {live}{" "}
        approved
        {restricted.length
          ? ` · ${restricted.length} answering otherwise-refused topics`
          : ""}
      </p>

      {/* A phrasing with no vector never matches, so this is the
          difference between the bank being written and the bank
          working. It clears itself within five minutes. */}
      {awaiting > 0 ? (
        <p className="mt-2 border-l-2 border-accent bg-paper-2 px-4 py-3 text-sm text-ink">
          {awaiting} {awaiting === 1 ? "phrasing is" : "phrasings are"} waiting
          to be embedded and cannot match until that runs, which happens every
          five minutes. If this number does not fall, the sidecar is not
          reachable.
        </p>
      ) : null}

      {entries.length === 0 && !error ? (
        <p className="mt-6 border border-line bg-paper-2 px-4 py-4 text-sm leading-relaxed text-ink-2">
          The bank is empty, so every question goes to the model and waits. The
          fastest thing you can do for the assistant is write the five questions
          you are asked most, in your own words.
        </p>
      ) : null}

      <section className="mt-8 border border-line p-5">
        <h2 className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          new entry
        </h2>
        <div className="mt-4">
          <QaEntryForm />
        </div>
      </section>

      <ul className="mt-8 space-y-5">
        {entries.map((e) => {
          const isRestricted = e.covers_restricted ?? e.coversRestricted;
          const phrasings: Phrasing[] = (e.phrasings ?? []).map((p) => ({
            id: p.id,
            text: p.text ?? "",
            canonical: Boolean(p.canonical),
            embedded: Boolean(p.embedded),
          }));
          return (
            <li key={e.id} className="border border-line p-5">
              <div className="flex flex-wrap items-baseline justify-between gap-3">
                <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                  #{e.id} · {e.enabled ? "approved" : "not approved"}
                  {isRestricted ? " · otherwise refused" : ""}
                </p>
                <div className="flex items-center gap-4">
                  <form action={setQaEnabledAction}>
                    <input type="hidden" name="id" value={e.id} />
                    <input
                      type="hidden"
                      name="enabled"
                      value={e.enabled ? "false" : "true"}
                    />
                    <button
                      type="submit"
                      className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 underline underline-offset-2 hover:text-accent"
                    >
                      {e.enabled ? "withdraw" : "approve"}
                    </button>
                  </form>
                  <form action={deleteQaEntryAction}>
                    <input type="hidden" name="id" value={e.id} />
                    <button
                      type="submit"
                      className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 underline underline-offset-2 hover:text-danger"
                    >
                      delete
                    </button>
                  </form>
                </div>
              </div>

              <details className="mt-3">
                <summary className="cursor-pointer text-base leading-snug text-ink">
                  {e.question || "(no question)"}
                </summary>
                <div className="mt-4">
                  <QaEntryForm
                    id={e.id}
                    question={e.question}
                    answer={e.answer}
                    sources={e.sources}
                    tags={e.tags}
                    coversRestricted={isRestricted}
                    enabled={e.enabled}
                  />
                </div>
              </details>

              <div className="mt-3 border-l-2 border-line bg-paper-2 px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap text-ink-2">
                {e.answer}
              </div>

              <Phrasings entryId={e.id} phrasings={phrasings} />
            </li>
          );
        })}
      </ul>
    </div>
  );
}
