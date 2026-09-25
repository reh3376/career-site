// Pandoc Typst template for Roger's résumé archive.
//
// The typography is lifted from the site's own renderer
// (services/sidecar/src/career_sidecar/templates/resume.typ) so an
// archive PDF and a site-generated PDF look like the same document:
// US Letter, Libertinus Serif at 10.5pt, ruled small-caps section
// headings, tight leading.
//
// It differs from that template in one deliberate way. The site
// renderer takes structured JSON with a fixed set of sections and no
// contact block, because the site generates a résumé from verified
// fields. These are Roger's own documents: they carry a phone number,
// an email, profile links, citizenship and veteran status, and they
// have sections the JSON schema has no slot for. So this one pours the
// whole Markdown body through rather than reading named fields, and
// nothing is dropped.
//
// Density was set by measurement, not taste. At the site renderer's
// 10.5pt with 0.55em leading, 8 of 12 résumés spilled onto a page
// their hand-made PDF did not need, and a résumé that runs to three
// pages when the author wrote two has been changed, not converted.
// These values bring 10 of 12 back to their original page count.
//
// Used by scripts/render_resume_pdf.py.

#set page(
  paper: "us-letter",
  margin: (x: 0.7in, top: 0.55in, bottom: 0.55in),
)
#set text(font: "Libertinus Serif", size: 10pt)
#set par(justify: false, leading: 0.5em)
#set list(indent: 0.6em, body-indent: 0.5em, spacing: 0.35em)
#set enum(indent: 0.6em, body-indent: 0.5em, spacing: 0.35em)

// Level 1 is the name: the document title, set once at the top.
#show heading.where(level: 1): it => block(above: 0em, below: 0.3em)[
  #text(size: 21pt, weight: "bold")[#it.body]
]

// Level 2 is a section: upper case, letter-spaced, underscored by a
// hairline rule the full measure of the page.
#show heading.where(level: 2): it => block(above: 0.8em, below: 0.4em)[
  #text(size: 11pt, weight: "bold", tracking: 0.06em)[#upper(it.body)]
  #v(-0.55em)
  #line(length: 100%, stroke: 0.5pt + luma(60))
]

// Level 3 and below are job entries and sub-points inside a section.
#show heading.where(level: 3): it => block(above: 0.7em, below: 0.2em)[
  #text(size: 10.5pt, weight: "bold")[#it.body]
]

#show link: it => text(fill: luma(60))[#it]

$if(smart)$
$else$
#set smartquote(enabled: false)

$endif$
$for(header-includes)$
$header-includes$

$endfor$
$body$
