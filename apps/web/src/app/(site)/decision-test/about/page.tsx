import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "What the decision test was measuring",
  description:
    "What the test was doing, how the load escalated, and why the wrong answers feel so right.",
  robots: { index: false, follow: false },
};

// The debrief, and the obligation this whole instrument is built around.
//
// The test deliberately induces error and then tells people they were
// confidently wrong. Whether that lands as insight or as a gotcha is
// decided entirely here (FSD fsd-decision-test.md section 7).
//
// Two constraints shape every word on this page.
//
// **It reveals no answers.** Everyone sees the same thirty items, so one
// leaked answer spoils the instrument permanently for everyone who comes
// after. The effect is therefore taught through the published textbook
// demonstrations, which have been in print for decades and which a
// curious reader could find in an afternoon, rather than through this
// test's own items. That is a real constraint on the writing: it is
// harder to explain a trap without showing the trap.
//
// **It is reachable without an address.** The personal numbers require an
// email; the explanation does not. A participant who gave nothing and
// wanted nothing back is exactly who this page is for, and they are the
// people the gotcha outcome lands hardest on.
//
// Public in PUBLIC_PATHS so an anonymous participant can open it, and
// deliberately absent from INDEXABLE_PATHS with robots noindex, because
// somebody who reads this before taking the test is contaminated. Held
// back from discovery is not the same as held back from participants.
export default function DecisionTestAboutPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        the explanation
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        What that was measuring.
      </h1>

      <div className="mt-8 rounded-md border border-line bg-paper-2 px-6 py-5">
        <p className="text-base leading-relaxed text-ink-2">
          <strong className="text-ink">
            If you have not taken the test yet, stop here.
          </strong>{" "}
          Nothing below gives away an answer, but knowing what is being
          done to you changes how you do it, and then your run measures
          something other than what it was built to measure.
        </p>
      </div>

      <div className="mt-12 space-y-10 text-base leading-relaxed text-ink-2">
        <Section title="The short version">
          <p>
            You were asked to hold a number in mind while answering
            questions that have an obvious wrong answer. The number was
            the point. The questions were the measurement.
          </p>
          <p>
            Holding a number occupies the same limited machinery you use
            to check an answer that already feels right. As that load
            went up, the checking got worse. What the test is really
            looking for is whether your <em>confidence</em> went down
            with it, and for most people it does not.
          </p>
        </Section>

        <Section title="How the load escalated">
          <p>
            Five blocks, six questions each. The number you were given
            at the start of each block got harder to hold as you went:
          </p>
          <ol className="list-decimal space-y-2 pl-5">
            <li>Three digits, given back as they were.</li>
            <li>Four digits, given back as they were.</li>
            <li>Four digits, with one added to every digit.</li>
            <li>Four digits, with three added to every digit.</li>
            <li>Three digits again, given back as they were.</li>
          </ol>
          <p>
            Blocks three and four are the steep part, and not because
            four digits are harder than three. Adding to each digit
            moves the task from <em>storing</em> something to{" "}
            <em>operating on</em> it while storing it, and that second
            thing competes directly with the deliberate checking a
            trick question needs.
          </p>
          <p>
            <strong className="text-ink">
              The fifth block is the one that makes the rest readable.
            </strong>{" "}
            It returns to the difficulty of the first, about twelve
            minutes later. If performance drops through the middle and
            then recovers at the end, the middle was the load. If it
            drops and stays down, some of it was simply fifteen minutes
            of effort, which is a different finding and worth being able
            to tell apart. A test that only ever got harder could not
            distinguish the two, and &ldquo;they got tired&rdquo; would be an
            untestable excuse for any result at all.
          </p>
        </Section>

        <Section title="Why a wrong answer can feel so certain">
          <p>
            The questions were built around a well-documented habit:
            when a question is hard, people answer an easier one that
            came to mind instead, usually without noticing the swap. The
            substitute answer arrives fast, fluently and with no sense
            of effort, and that fluency is what gets read as confidence.
          </p>
          <p>
            That is why being wrong here does not feel like being
            unsure. There is no wobble to notice. The wrong answer
            presents itself as the answer, and checking it requires
            deciding to check, which is precisely what the number in
            your head was consuming.
          </p>
          <p>
            None of this is a defect peculiar to you. It is the ordinary
            behaviour of an efficient system that is right most of the
            time and is being shown the narrow class of problems built
            to exploit it.
          </p>
        </Section>

        <Section title="The four kinds of question">
          <p>
            Each family has a famous published counterpart, in print for
            decades. Those are described here rather than the test&rsquo;s own
            items, so you can see the mechanism without learning what to
            answer.
          </p>

          <Family
            name="Arithmetic with a lure"
            demo="The bat and ball"
            published="Frederick, 2005"
          >
            A bat and a ball cost $1.10 together, and the bat costs
            $1.00 more than the ball. Almost everyone first thinks ten
            cents. It is wrong, and noticing requires doing the small
            piece of arithmetic that the fluent answer already made feel
            unnecessary. Under load, far fewer people do it.
          </Family>

          <Family
            name="Base rates"
            demo="The taxicab problem"
            published="Kahneman and Tversky, 1972"
          >
            Given a vivid description of a specific case and a dull
            statistic about how common that case is, people lean on the
            description and discount the statistic. The description
            feels like evidence about this one; the base rate feels like
            background. It is usually the base rate that decides the
            answer.
          </Family>

          <Family
            name="Conjunctions"
            demo="The Linda problem"
            published="Tversky and Kahneman, 1983"
          >
            Adding a detail to a description makes it feel more likely,
            although every added condition can only make it less likely.
            A specific, coherent story beats a vague one that is
            strictly more probable, because coherence is what gets
            mistaken for probability.
          </Family>

          <Family
            name="Syllogisms"
            demo="Belief bias"
            published="Evans, Barston and Pollard, 1983"
          >
            People judge whether a conclusion <em>follows</em> by
            checking whether it is <em>true</em>. A valid argument with
            an implausible conclusion gets rejected; an invalid one with
            an agreeable conclusion gets accepted. This is why those
            questions said to assume both statements were true: the task
            was the logic, never the facts.
          </Family>
        </Section>

        <Section title="Why you are not shown your answers">
          <p>
            Everyone takes the same thirty questions. One answer getting
            out spoils the measurement for everyone who comes after, and
            an answer key only has to escape once. So no participant is
            told which questions they got wrong, during the test, in the
            emailed results, or here. If you gave an address you get an
            overall score and the comparison that matters, which is your
            accuracy against your confidence.
          </p>
          <p>
            That restriction ends when collection does. At that point
            the items and the full workings are published.
          </p>
        </Section>

        <Section title="What one run can and cannot say">
          <p>
            <strong className="text-ink">
              A single sitting proves nothing about you.
            </strong>{" "}
            Thirty questions is a short, noisy measurement. Have a bad
            block because a neighbour started drilling and the numbers
            move. Any individual result is as likely to reflect the
            fifteen minutes you happened to have as anything stable
            about how you think.
          </p>
          <p>
            What it does do is demonstrate a general effect on a
            specific person, which is a different and more useful thing
            than a verdict. The effect is real and well replicated
            across large samples. Your run is one data point in
            something that only becomes a finding in aggregate, which is
            exactly why you were asked.
          </p>
          <p>
            It is emphatically not an intelligence test and does not
            score anything that generalises. A high score means the
            trick questions did not catch you today.
          </p>
        </Section>

        <Section title="What happens to what you gave">
          <p>
            Your answers, timings and confidence ratings are kept as a
            research dataset and analysed together with everyone
            else&rsquo;s. The details you entered at the start were optional
            and are stored apart from the measurements, so they can be
            erased without destroying the data. Nothing is sold, shared
            or used for advertising.
          </p>
          <p>
            The{" "}
            <Link
              href="/privacy"
              className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              privacy page
            </Link>{" "}
            sets out what is collected, how long it is kept, and what
            asking for deletion does and does not remove.
          </p>
        </Section>

        <Section title="Where this came from">
          <p>
            This test supplements the series{" "}
            <Link
              href="/articles"
              className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              Better Business Decisions Require Better Information
            </Link>
            , which argues that the quality of a decision depends on the
            quality of the information behind it. The uncomfortable
            corollary is that attention is part of that information
            path, and attention is finite. A decision made with good
            data by someone with nothing left to spend on checking it is
            not a well-informed decision, and it will feel like one.
          </p>
          <p>
            Thank you for the fifteen minutes. There was no way to get
            this measurement without asking someone for real effort, and
            you gave it.
          </p>
        </Section>
      </div>

      <div className="mt-14 flex flex-wrap gap-4">
        <Link
          href="/"
          className="rounded-md bg-accent px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-canvas uppercase no-underline"
        >
          Back to the site
        </Link>
        <Link
          href="/articles"
          className="rounded-md border border-line px-8 py-3 font-mono text-[11px] tracking-[0.14em] text-ink-2 uppercase no-underline"
        >
          Read the series
        </Link>
      </div>
    </div>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <h2 className="font-display text-2xl tracking-tight text-ink">{title}</h2>
      <div className="mt-4 space-y-4">{children}</div>
    </section>
  );
}

// A published demonstration, named so a reader can go and look it up.
// The citation is the point: it says this is decades-old public
// knowledge rather than something invented for this page, which is what
// makes describing it compatible with never revealing an item.
function Family({
  name,
  demo,
  published,
  children,
}: {
  name: string;
  demo: string;
  published: string;
  children: React.ReactNode;
}) {
  return (
    <div className="border-l-2 border-line pl-5">
      <h3 className="font-display text-lg leading-tight text-ink">{name}</h3>
      <p className="mt-1 font-mono text-[11px] tracking-[0.12em] text-ink-3 uppercase">
        {demo} · {published}
      </p>
      <p className="mt-3 text-ink-2">{children}</p>
    </div>
  );
}
