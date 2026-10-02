import Link from "next/link";

// Why the site exists, high on the page, in both modes.
//
// Everything below this on the landing page is evidence. Without this
// section a visitor has no reason to read it as anything other than an
// elaborate portfolio, and the JD reviewer looks like a gimmick rather
// than the answer to a specific problem.
//
// The argument is Roger's own, from the decision not to publish a
// generic résumé: a long career does not compress into a document
// someone scans for six keywords, the terms that matter are usually
// equivalent rather than identical, and the screen happens before any
// conversation. The reviewer inverts that. It reads the posting's
// requirements and goes looking for evidence, instead of reading his
// document and looking for strings.
//
// Written to survive scrutiny rather than to sell: every claim here is
// something /how-ask-roger-works already publishes, including the
// disagreement rate and the fact that it is slow.

export function WhyThisExists() {
  return (
    <section
      id="why"
      aria-labelledby="why-heading"
      className="scroll-mt-20 border-y border-line bg-paper"
    >
      <div className="mx-auto max-w-5xl px-6 py-20 sm:px-10 sm:py-24">
        <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
          why this site exists
        </p>
        <h2
          id="why-heading"
          className="font-display mt-3 max-w-3xl text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
          style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
        >
          The screen happens before the conversation.
        </h2>

        <div className="mt-8 grid gap-8 md:grid-cols-2">
          <div className="space-y-5 text-base leading-relaxed text-ink-2">
            <p>
              Thirty years across oil and gas, power, mining, telecom,
              manufacturing, industrial automation and applied AI does not
              compress into a document somebody scans for twelve keywords.
              The terms that matter are usually equivalent rather than
              identical. A historian is a time-series database. A control
              narrative is a specification. Model predictive control is
              applied ML, optimisation under constraints.
            </p>
            <p>
              Someone who does not recognise the equivalence screens the
              candidate out, and it happens before anyone has spoken. That
              is what makes it expensive: there is no conversation in
              which to correct it.
            </p>
          </div>

          <div className="space-y-5 text-base leading-relaxed text-ink-2">
            <p>
              So this site does not hand you a résumé and hope. Paste the
              posting instead. The{" "}
              <Link
                href="/jd-upload"
                className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 transition-colors hover:decoration-accent"
              >
                reviewer
              </Link>{" "}
              breaks it into requirements, goes looking through my records
              for evidence of each one, and returns met, partial or unmet
              with the passage behind every verdict. The score is
              arithmetic on those verdicts, computed in code, not a number
              a model felt like producing.
            </p>
            <p className="text-ink">
              It reads your requirements and looks for evidence. It does
              not read my document and look for keywords. That inversion
              is the whole argument.
            </p>
          </div>
        </div>

        {/* The caveats are load-bearing. An argument about honest
            evidence that hid its own numbers would be making the
            opposite case by example. */}
        <p className="mt-10 max-w-3xl text-sm leading-relaxed text-ink-3">
          It is slow, it runs on one small server, and it disagrees with me
          often enough to be worth measuring.{" "}
          <Link
            href="/how-ask-roger-works"
            className="underline decoration-line decoration-1 underline-offset-4 transition-colors hover:text-accent hover:decoration-accent"
          >
            All three numbers are published
          </Link>
          , including which direction the disagreements run.
        </p>
      </div>
    </section>
  );
}

// The same argument as an HMI panel.
export function WhyThisExistsOt() {
  return (
    <div className="border border-line-strong bg-paper-2 p-5">
      <p className="font-mono text-[13px] leading-relaxed text-ink">
        Thirty years across oil and gas, power, mining, telecom,
        manufacturing, automation and applied AI does not compress into a
        document scanned for twelve keywords. The terms that matter are
        equivalent, not identical: historian equals time-series database,
        control narrative equals specification, MPC equals applied ML,
        optimisation under constraints. Miss the equivalence and the
        candidate is screened out before anyone speaks.
      </p>
      <p className="mt-3 font-mono text-[13px] leading-relaxed text-ink-2">
        So: paste the posting. The reviewer breaks it into requirements,
        retrieves evidence for each, and returns MET / PARTIAL / UNMET with
        the passage behind every verdict. Score computed in code, not
        guessed.
      </p>
      <p className="mt-3 font-mono text-[12px] leading-relaxed text-accent">
        It reads your requirements and looks for evidence. It does not read
        my document and look for keywords.
      </p>
      <p className="mt-3 font-mono text-[11px] tracking-[0.08em] text-ink-3 uppercase">
        slow · one small server · disagreement rate published at{" "}
        <Link
          href="/how-ask-roger-works"
          className="text-accent underline underline-offset-4"
        >
          /how-ask-roger-works
        </Link>
      </p>
    </div>
  );
}
