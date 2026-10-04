import type { Metadata } from "next";
import Link from "next/link";

import { getJdBands } from "@/lib/jd-bands";
import { getReviewerStatus } from "@/lib/reviewer-status";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = {
  title: "How Ask Roger works",
  description:
    "How the assistant answers a question and the reviewer scores a posting: which model, why it is not trained on Roger's career, what each one is allowed to read, and how both are made better.",
};
export const dynamic = "force-dynamic";

export default async function HowAskRogerWorksPage() {
  const bands = await getJdBands(await getSessionCookie());
  const status = await getReviewerStatus();
  const veryStrong = bands.veryStrong.toFixed(2);
  const strong = bands.strong.toFixed(2);
  const possible = bands.possible.toFixed(2);
  const weak = bands.weak.toFixed(2);
  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        the assistant
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        How Ask Roger works.
      </h1>
      <p className="mt-6 inline-flex items-center gap-2 border border-success bg-success-soft/40 px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.14em] text-success">
        <span className="pilot text-success" aria-hidden="true" />
        status: live
        <span className="text-ink-4">·</span>
        <span className="text-ink-2">members only</span>
      </p>
      <div className="mt-10 space-y-6 text-base leading-relaxed text-ink-2">
        <p>
          Ask Roger answers questions about Roger in his own voice, from
          his own records, and shows you where each answer came from. It
          runs at{" "}
          <Link
            href="/ask"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            /ask
          </Link>{" "}
          and from the button in the corner of every page.
        </p>
        <p>
          It is the same machinery as the JD reviewer below, pointed at a
          different job, and the two sections of this page share most of
          their answer. One model, open weights, on Roger&rsquo;s own
          server. Nothing is sent to a hosted API, and the model has never
          been trained on his career: it is handed the relevant passages
          at the moment of the question.
        </p>
      </div>

      <section className="mt-14 border-t border-line pt-10">
        <h2
          className="font-display text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          What happens when you ask.
        </h2>
        <ol className="mt-6 ml-4 list-decimal space-y-3 text-sm leading-relaxed text-ink-2">
          <li>
            <span className="text-ink">A question Roger has answered.</span>{" "}
            Your question is compared against a bank of questions Roger
            wrote the answers to himself. A close enough match, and far
            enough ahead of the next candidate, returns his words
            immediately. No model runs, and the answer is his rather than
            a reconstruction of his.
          </li>
          <li>
            <span className="text-ink">Otherwise, retrieval.</span> The
            question is turned into a vector and the nearest passages in
            the career corpus are pulled out, alongside a short sheet of
            career facts that every question gets: tenure, titles, dates,
            credentials. Those are the things retrieval is worst at and
            questions most often turn on.
          </li>
          <li>
            <span className="text-ink">Then the answer.</span> The model is
            given the facts, the passages and the conversation so far, and
            asked to answer briefly in the first person. It streams back a
            sentence at a time.
          </li>
          <li>
            <span className="text-ink">Sources.</span> Published passages
            that fed the answer are listed under it and link to the piece
            they came from, so you can read the thing itself rather than
            the summary of it.
          </li>
        </ol>
        <div className="mt-6 space-y-4 text-base leading-relaxed text-ink-2">
          <p>
            <span className="text-ink">
              It will tell you when it does not know.
            </span>{" "}
            Asked something the records do not cover, it says so instead of
            producing something plausible. It will not discuss
            compensation, references, anything confidential to an employer,
            or Roger&rsquo;s personal life, and points you at him directly
            for those.
          </p>
          <p>
            <span className="text-ink">
              It reads more than the reviewer does.
            </span>{" "}
            A posting submitted to the reviewer cannot reach the private
            half of the corpus. A question here can: it is Roger&rsquo;s
            own record answering about Roger. Private material may inform
            an answer and can never be quoted at length, named, or listed
            as a source, which is enforced where the sources are built
            rather than by instructing the model. Anything he does not want
            speaking for him at all is excluded from the assistant outright.
          </p>
          <p>
            <span className="text-ink">It is slow, and that is real.</span>{" "}
            An answer takes a few tens of seconds. The whole thing runs on
            one small server with no graphics card, shared with the
            reviewer, and the honest reason it is not faster is that it is
            not worth renting a bigger machine to shave off twenty seconds.
            Questions from the bank return instantly; a question asked while
            a JD review is running will be slower still, because the review
            has the machine.
          </p>
          <p className="text-sm text-ink-3">
            Conversations are kept so Roger can read what was asked and
            grade the answers, which is how the assistant is corrected. The
            mechanism is the one described under &ldquo;How it gets
            better&rdquo; below, and it works the same way here: when an
            answer is wrong, the usual fix is that no document says the
            thing plainly, and the document gets written.
          </p>
        </div>
      </section>

      <section className="mt-14 border-t border-line pt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
          the other half
        </p>
        <h2
          className="font-display mt-3 text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          The JD reviewer.
        </h2>
        <p className="mt-4 text-base leading-relaxed text-ink-2">
          The review behind{" "}
          <Link
            href="/jd-upload"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            /jd-upload
          </Link>{" "}
          reads a posting rather than a question, and draws on the same
          career corpus. It works like this.
        </p>
        <ol className="mt-6 ml-4 list-decimal space-y-3 text-sm leading-relaxed text-ink-2">
          <li>
            <span className="text-ink">Requirements.</span> The posting is
            broken into checkable requirements, each marked must or
            preferred and weighted 1 to 3. Clauses like &ldquo;or
            equivalent&rdquo; are kept as written.
          </li>
          <li>
            <span className="text-ink">Evidence.</span> For each
            requirement the reviewer retrieves the most relevant passages
            from Roger&rsquo;s career corpus and adds a short sheet of
            career facts, so the judge always sees the basics.
          </li>
          <li>
            <span className="text-ink">Verdicts.</span> A judge model reads
            that evidence and returns one verdict per requirement: met,
            partial or unmet, with a one-sentence reason. Each requirement
            gets its own call, so the model never has to hold the whole
            posting at once.
          </li>
          <li>
            <span className="text-ink">Score.</span> The score is computed
            in code from those verdicts, not guessed by the model. Met
            counts 1, partial 0.5, unmet 0, each weighted. Must-have items
            always count against the total; preferred items only count
            when they are evidenced.
          </li>
          <li>
            <span className="text-ink">Fit category.</span> The score
            lands in one of five categories: very strong (&ge; {veryStrong}),
            strong (&ge; {strong}), possible (&ge; {possible}), weak
            (&ge; {weak}), and very weak below that. Roger can adjust the
            bands; the numbers here are the ones in force.
          </li>
          <li>
            <span className="text-ink">R&eacute;sum&eacute;.</span> When
            the fit is strong or better, a second model writes a two-page
            r&eacute;sum&eacute; for that posting. Every claim in it carries
            a source id that is checked in code against the corpus before
            the r&eacute;sum&eacute; is rendered as a PDF. The PDF opens
            without a password but is locked against editing.
          </li>
        </ol>
        <p className="mt-6 text-sm leading-relaxed text-ink-2">
          Reviews run one at a time on Roger&rsquo;s own server and usually
          take 30 to 60 minutes, sometimes longer. You can close the page:
          your submissions
          list on /jd-upload reopens a review, and the outcome is emailed
          to you.
        </p>
      </section>

      <section className="mt-14 border-t border-line pt-10">
        <h2
          className="font-display text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          Which model, and what it is not.
        </h2>
        <div className="mt-4 space-y-4 text-base leading-relaxed text-ink-2">
          <p>
            One model does the reading:{" "}
            <span className="font-mono text-sm text-ink">qwen3:4b-q8_0</span>,
            four billion parameters at eight-bit precision, open weights,
            served through Ollama on Roger&rsquo;s own server with an
            8,192-token context. Retrieval uses a second, smaller model,{" "}
            <span className="font-mono text-sm text-ink">nomic-embed-text</span>,
            which turns text into 768-dimension vectors stored in Postgres
            with pgvector. Nothing is sent to a hosted API. No posting you
            submit leaves that machine.
          </p>
          <p>
            <span className="text-ink">
              The model is not trained on Roger&rsquo;s career.
            </span>{" "}
            It has never seen it in training and nothing about it has been
            fine-tuned. It is a general open-weight model that is handed
            the relevant passages at the moment of the question and asked
            to judge one requirement against them. That is a deliberate
            choice rather than a shortcut: a model trained on a career can
            only be corrected by training it again, while a model that
            reads a corpus is corrected by fixing the corpus, and you can
            see which passage produced which verdict.
          </p>
          <p>
            It is also not allowed to decide the outcome. The model
            returns a verdict per requirement and a sentence of reasoning.
            Every number, the score, the weighting, the threshold, is
            computed in code from those verdicts. When the model has been
            asked to reason about something it is reliably bad at, that
            judgment has been moved out of the prompt and into code: how
            long a span of years is, whether a named company actually
            appears in the evidence, and whether a r&eacute;sum&eacute;
            line is carried by the source it cites.
          </p>
        </div>
      </section>

      <section className="mt-14 border-t border-line pt-10">
        <h2
          className="font-display text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          How it gets better.
        </h2>
        <div className="mt-4 space-y-4 text-base leading-relaxed text-ink-2">
          <p>
            Four mechanisms, in the order they matter.
          </p>
          <ol className="ml-4 list-decimal space-y-3 text-sm leading-relaxed">
            <li>
              <span className="text-ink">The corpus gets fixed.</span>{" "}
              Most wrong verdicts are not the model reasoning badly. They
              are the model failing to find something because no document
              says it plainly. When that happens the answer is to write
              the missing document, not to adjust the prompt.
            </li>
            <li>
              <span className="text-ink">
                Roger grades the verdicts himself.
              </span>{" "}
              Every verdict is logged with the exact evidence the model
              saw and the model and prompt version that produced it. He
              marks each one agree, disagree or not enough evidence to
              judge, in his own words. Disagreements are counted in two
              directions, because a reviewer that is too harsh and one
              that is too generous need opposite fixes.
            </li>
            <li>
              <span className="text-ink">
                A fixed set of postings guards every change.
              </span>{" "}
              A group of real job descriptions, some Roger applied for and
              some drawn at random from job boards, each carrying one
              claim: he can do this job, or he cannot. Any change to a
              prompt, a model or the corpus is scored against the whole
              set before and after. A change that improves one posting and
              quietly breaks another shows up as a number, not as a
              feeling.
            </li>
            <li>
              <span className="text-ink">
                Rules move from prose into code.
              </span>{" "}
              When instructing the model in words fails twice, the
              judgment is taken away from it and written as a rule that
              runs the same way every time.
            </li>
          </ol>
          <p className="text-sm text-ink-3">
            A fine-tuned adapter, trained on the graded verdicts, is
            planned rather than built. It is listed here as a plan so that
            nothing above reads as a claim about something that exists.
          </p>
        </div>

        {status ? (
          <div className="mt-8 border-l-2 border-line pl-5">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              where that stands today
            </p>
            <dl className="mt-4 space-y-4 text-sm leading-relaxed text-ink-2">
              <div>
                <dt className="text-ink">
                  {status.agreementPct.toFixed(1)}% agreement, over{" "}
                  {status.graded} verdicts Roger has graded by hand
                </dt>
                <dd className="mt-1 text-ink-3">
                  {status.hardDisagreements === 0
                    ? "No outright disagreements."
                    : `${status.hardDisagreements} outright ${
                        status.hardDisagreements === 1
                          ? "disagreement"
                          : "disagreements"
                      }, of which ${status.tooHarsh} were the reviewer refusing to credit work he can evidence and ${status.tooGenerous} were it crediting work he cannot.`}{" "}
                  The second number is the one that would matter to you, and
                  it is published whatever it says.
                </dd>
              </div>
              <div>
                <dt className="text-ink">
                  {status.gateCorrect} of {status.scored} postings on the
                  expected side of the gate
                  {status.inversions === 0
                    ? ", with no inversions"
                    : `, with ${status.inversions} inversions`}
                </dt>
                <dd className="mt-1 text-ink-3">
                  From a set of {status.postings} job descriptions,{" "}
                  {status.postingsRandom} of them drawn at random from job
                  boards rather than picked.
                  {status.margin !== undefined
                    ? ` The closest the two groups came was ${status.margin.toFixed(3)}.`
                    : ""}
                  {status.evaluatedAt
                    ? ` Last run ${new Date(status.evaluatedAt).toLocaleDateString(undefined, { day: "numeric", month: "long", year: "numeric" })}`
                    : ""}
                  {status.model ? ` on ${status.model}.` : "."}
                </dd>
              </div>
            </dl>
            <p className="mt-4 text-sm leading-relaxed text-ink-3">
              These are read live from the same tables Roger reads. They are
              not a good look on purpose; they are the numbers, and a claim
              to be honest about evidence is worth less than the figures it
              is currently failing on.
            </p>
          </div>
        ) : null}
      </section>

      <section className="mt-14 border-t border-line pt-10">
        <h2
          className="font-display text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          What your posting is read against.
        </h2>
        <div className="mt-4 space-y-4 text-base leading-relaxed text-ink-2">
          <p>
            The corpus has a public part and a private part. The private
            part holds client work and material under agreement, and a
            posting submitted through this site cannot reach it: the
            restriction is enforced in the database query, not by asking
            the model nicely. Your review is drawn from the public
            documents, plus two files about Roger himself that every
            review uses, a sheet of career facts and his master
            r&eacute;sum&eacute;.
          </p>
          <p>
            That is a real limit and worth stating plainly: a review here
            sees less than Roger does when he runs the same posting for
            himself. It is the reason a strong result comes with a
            r&eacute;sum&eacute; and a weak one comes with Roger reading
            the posting personally.
          </p>
          <p className="text-sm text-ink-3">
            Your posting is stored so the review can be reopened and so
            Roger can see what was asked. It is never used to train
            anything and never shown to anyone else.
          </p>
        </div>
      </section>

      <section className="mt-14 border-t border-line pt-10">
        <h2
          className="font-display text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          Ask it twice and the answer is the same.
        </h2>
        <div className="mt-4 space-y-4 text-base leading-relaxed text-ink-2">
          <p>
            Submit the same posting against the same corpus and you get the
            same verdicts and the same score. Not close to it. The same
            number. The table below is the check: the postings scored in
            the two most recent evaluations, before and after the
            reviewer&rsquo;s prompt was rewritten, to three decimal places.
          </p>
          <p>
            Four things make that true, and none of them is the model being
            reliable. The judge runs at temperature zero, so it is not
            sampling. Its answer is constrained to a fixed shape. That answer
            is one of three words, met, partial or unmet, not a number. And
            the score is arithmetic performed in code on those three words.
          </p>
          <p>
            <span className="text-ink">
              The model itself is not deterministic and this does not claim
              it is.
            </span>{" "}
            Its intermediate reasoning does vary between runs on identical
            input. What the design does is put the decision somewhere that
            variation cannot reach: it would have to be large enough to move
            a requirement from met to partial before it could move a score at
            all.
          </p>
          <p>
            Which matters for a practical reason. If you disagree with a
            result, it points at one requirement and the passage behind it,
            and you can run it again and get the same thing to argue with.
            And when a number does move, something really changed: the
            corpus, the prompt, or the model. All three are recorded with
            every run.
          </p>
        </div>

        {status && status.comparison.length > 0 ? (
          <div className="mt-8">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              the same postings, scored before and after a prompt rewrite
            </p>
            <table className="mt-4 w-full border-collapse text-sm">
              <thead>
                <tr className="border-b border-line text-left">
                  <th className="py-2 pr-4 font-normal text-ink-3">posting</th>
                  <th className="py-2 pr-4 text-right font-normal text-ink-3">
                    before
                  </th>
                  <th className="py-2 pr-4 text-right font-normal text-ink-3">
                    after
                  </th>
                  <th className="py-2 text-right font-normal text-ink-3"></th>
                </tr>
              </thead>
              <tbody>
                {status.comparison.map((row, i) => (
                  <tr key={`${row.label}-${i}`} className="border-b border-line">
                    <td className="py-2 pr-4 text-ink-2">{row.label}</td>
                    <td className="py-2 pr-4 text-right font-mono text-ink">
                      {row.previous === undefined
                        ? "not in that run"
                        : row.previous.toFixed(3)}
                    </td>
                    <td className="py-2 pr-4 text-right font-mono text-ink">
                      {row.score.toFixed(3)}
                    </td>
                    <td className="py-2 text-right font-mono text-[11px] uppercase tracking-[0.14em]">
                      {row.previous === undefined ? (
                        <span className="text-ink-4">new</span>
                      ) : row.unchanged ? (
                        <span className="text-success">same</span>
                      ) : (
                        <span className="text-danger">moved</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <p className="mt-4 text-sm leading-relaxed text-ink-3">
              Read live from the evaluation records. The two postings Roger
              applied for are not named: telling those employers he applied,
              and what the reviewer scored him at, is his to disclose and is
              not something the evidence needs. The rest are public job
              advertisements drawn at random. A posting appears here only if
              both evaluations reached it.
            </p>
          </div>
        ) : null}
      </section>
    </div>
  );
}
