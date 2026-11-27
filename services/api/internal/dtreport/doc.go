// Package dtreport builds the decision test's individual results report:
// one participant's run, described on its own terms and, where a fair
// comparison exists, set against the other first-time participants.
//
// It is pure. It takes rows that the users package read from the
// decision test views and returns a Document, which is the whole report
// as data: every figure worked out, every sentence chosen, every chart
// reduced to the numbers it draws. The sidecar renders that Document to
// two PDFs and decides nothing.
//
// The split is the same one the results email already follows. Which
// true thing a run supports is a decision about the research, so it is
// made here, where it is unit-tested, rather than in a template that
// branches on the sign of a number. A sentence in the report is selected
// by a rule that holds for every run, or it is not in the report.
//
// Two copies come out of one Document. A block marked for the owner
// appears only in the owner's copy, which is the one place the questions
// and their answers are printed (FSD FR-DT-04, FR-DT-21). Everything
// else is described by kind, never by content, so the participant's
// copy can be sent without carrying any part of the answer key.
package dtreport
