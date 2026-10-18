// Pandoc Typst template for an interview reference document.
//
// Companion to scripts/templates/resume-pandoc.typ, with the opposite
// goal. That template is tuned for density, because a résumé that
// gains a page has been changed rather than converted. This one is
// tuned for retrieval under pressure: the reader is in a thirty-minute
// call and needs to be on the right page in two seconds, so it spends
// space on navigation and legibility instead of saving it.
//
// What that buys, concretely:
//
//   - a clickable contents page, and PDF bookmarks, so the reader's
//     viewer sidebar is a usable index
//   - every "§6.3" and "Story D" in the prose is a live internal link,
//     wired by render_interview_pdf.py, so a cross-reference is one
//     click and not a hunt
//   - a running header naming the current section, because a reader who
//     has just jumped somewhere needs to know where they landed
//   - links in a visible colour, since a link nobody can see is not a
//     link
//
// Used by scripts/render_interview_pdf.py.

// Helpers pandoc's Typst writer emits calls to, taken from its own
// default template (`pandoc -D typst`). A custom template replaces that
// one wholesale, so anything it defined has to be defined here or the
// compile aborts on an unknown variable: `---` in the Markdown becomes
// `#horizontalrule`, and a definition list becomes a `terms.item`.
#let horizontalrule = align(center)[
  #v(0.3em)
  #line(length: 32%, stroke: 0.5pt + rgb("#c8c8c8"))
  #v(0.3em)
]

#show terms.item: it => block(breakable: false)[
  #text(weight: "bold")[#it.term]
  #block(inset: (left: 1.5em, top: -0.4em))[#it.description]
]

#show figure.where(kind: table): set figure.caption(position: top)
#show figure.where(kind: image): set figure.caption(position: bottom)

// Straight quotes, as the source wrote them. Smart quotes would turn
// the inch marks and the code samples into curly characters a reader
// cannot type into a search box.
#set smartquote(enabled: false)

#set page(
  paper: "us-letter",
  margin: (x: 0.7in, top: 0.8in, bottom: 0.7in),
  header: context {
    // Name the section the reader is currently in. A jump from the
    // contents page or from a cross-reference lands mid-document with
    // no context; this is that context.
    //
    // Compared by page rather than with .before(here()). The header is
    // laid out at the top of the page, above the heading that starts
    // there, so "everything before this point" is the *previous*
    // section on every section's first page, which is precisely the
    // page where the reader most needs to be told where they are.
    let here_page = counter(page).at(here()).first()
    let current = query(heading.where(level: 1)).filter(h => (
      counter(page).at(h.location()).first() <= here_page
    ))
    set text(size: 8pt, fill: rgb("#6b6b6b"))
    grid(
      columns: (1fr, auto),
      align: (left, right),
      if current.len() > 0 { current.last().body } else { [] },
      [$if(header)$$header$$else$$if(title)$$title$$endif$$endif$],
    )
    v(-0.4em)
    line(length: 100%, stroke: 0.4pt + rgb("#d4d4d4"))
  },
  footer: context {
    set text(size: 8pt, fill: rgb("#6b6b6b"))
    align(center)[#counter(page).display("1")]
  },
)

// Document metadata, so the reader's viewer and their operating system
// can both find this file by name and by subject rather than only by
// filename.
#set document(
  title: $if(title)$"$title$"$else$"Interview notes"$endif$,
  author: $if(author)$"$author$"$else$"Roger Henley"$endif$,
$if(keywords)$
  keywords: ($for(keywords)$"$keywords$",$endfor$),
$endif$
)

#set text(
  font: ("Libertinus Serif", "Times New Roman", "serif"),
  size: $if(fontsize)$$fontsize$$else$10.5pt$endif$,
  lang: "en",
  // Hyphenation off, deliberately, and this is a search requirement
  // rather than a typographic preference. A hyphenated break splits a
  // word across two lines, and in several PDF viewers a find for
  // "traceability" then misses the one on the line that broke it. The
  // whole point of this artifact is that the reader can find a word in
  // it during a live call, so ragged right is the cheaper price.
  hyphenate: false,
)
#set par(justify: false, leading: $if(leading)$$leading$$else$0.62em$endif$)

// Headings carry their own weight and spacing. Level 1 is a section
// (§0 to §13), level 2 a subsection, level 3 a story or a case.
#show heading.where(level: 1): it => {
$if(toc)$
  pagebreak(weak: true)
$endif$
  block(above: 0em, below: 0.7em)[
    #grid(
      columns: (1fr, auto),
      align: (left + bottom, right + bottom),
      text(size: 15pt, weight: "bold")[#it.body],
      // Back to the index from any section. A reader who arrived here
      // by following a cross-reference has no back button in a printed
      // or a full-screen PDF, and jumping twice through the contents
      // page is the whole navigation loop.
$if(toc)$      text(size: 8pt, fill: rgb("#0b4f9e"))[#link(<contents>)[Contents]],$else$      [],$endif$
    )
    #v(-0.45em)
    #line(length: 100%, stroke: 0.9pt + rgb("#1a1a1a"))
  ]
}
#show heading.where(level: 2): it => block(above: 1.1em, below: 0.5em)[
  #set text(size: 12pt, weight: "bold")
  #it.body
]
#show heading.where(level: 3): it => block(above: 0.95em, below: 0.4em)[
  #set text(size: 11pt, weight: "bold", fill: rgb("#333333"))
  #it.body
]
#show heading.where(level: 4): it => block(above: 0.8em, below: 0.3em)[
  #set text(size: 10.5pt, weight: "bold", style: "italic")
  #it.body
]

// A link has to look like one. Internal cross-references and source
// URLs are both coloured; the reader is going to be clicking these
// mid-call and cannot afford to guess.
#show link: it => text(fill: rgb("#0b4f9e"))[#it]

#set list(indent: 0.7em, body-indent: 0.45em, spacing: 0.5em)
#set enum(indent: 0.7em, body-indent: 0.45em, spacing: 0.5em)

// Tables are reference material, so they are set smaller and are
// allowed to break across pages rather than being shrunk to fit.
#show table: it => block(breakable: true)[
  #set text(size: 9pt)
  #it
]
#set table(
  stroke: (x, y) => (
    bottom: if y == 0 { 0.8pt + rgb("#1a1a1a") } else { 0.35pt + rgb("#d4d4d4") },
  ),
  inset: (x: 5pt, y: 3.5pt),
)

#show raw: it => text(font: ("DejaVu Sans Mono", "Menlo", "monospace"), size: 9pt)[#it]

// Block quotes are the spoken answers, the lines read aloud in the
// room, so they are set apart rather than merely indented.
#show quote.where(block: true): it => block(
  width: 100%,
  inset: (left: 0.7em, right: 0.5em, top: 0.5em, bottom: 0.5em),
  fill: rgb("#f6f4ef"),
  stroke: (left: 2pt + rgb("#8a7a52")),
  it.body,
)

$if(title)$
#align(center)[
  #block[
    #set text(size: 17pt, weight: "bold")
    $title$
  ]
]
$if(subtitle)$
#align(center)[
  #set text(size: 10pt, fill: rgb("#555555"))
  $subtitle$
]
$endif$
#v(0.6em)
$endif$

$if(toc)$
// `title: none` and the heading written by hand, because outline()'s own
// title is a level-1 heading, and a level-1 heading here carries a
// page-break rule that Typst refuses to apply inside a container.
#block[
  #text(size: 12pt, weight: "bold")[Contents]
  #metadata("contents") <contents>
  #v(0.35em)
  #line(length: 100%, stroke: 0.8pt + rgb("#1a1a1a"))
  #v(0.5em)
  #set text(size: 9.5pt)
  // depth 2 after the heading shift, which is sections plus their
  // subsections and the ten stories: what a reader actually jumps to,
  // and the same tree the viewer's bookmark sidebar gets.
  // A little air between entries; at depth 2 this list is long and a
  // solid block of it is hard to scan.
  #show outline.entry: it => block(above: 0.4em)[#it]
  #outline(title: none, depth: $if(toc-depth)$$toc-depth$$else$2$endif$, indent: 1em)
]
$endif$

$body$
