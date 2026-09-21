// Two-page résumé layout for the verified résumé JSON the API produces.
// Data is read from /data.json in the compile root; strings inserted
// with # render literally, so model output cannot inject markup.
#let data = json("/data.json")

#set page(paper: "us-letter", margin: (x: 0.75in, top: 0.65in, bottom: 0.65in))
#set text(font: "Libertinus Serif", size: 10.5pt)
#set par(justify: false, leading: 0.55em)
#set list(indent: 0.6em, body-indent: 0.5em, spacing: 0.45em)

#show heading.where(level: 1): it => block(above: 0em, below: 0.3em)[
  #text(size: 21pt, weight: "bold")[#it.body]
]
#show heading.where(level: 2): it => block(above: 1.0em, below: 0.45em)[
  #text(size: 11pt, weight: "bold", tracking: 0.06em)[#upper(it.body)]
  #v(-0.55em)
  #line(length: 100%, stroke: 0.5pt + luma(60))
]

= Roger E. Henley II
#text(size: 11.5pt, fill: luma(40))[#data.headline]

== Summary
#data.summary

== Core competencies
#columns(2, gutter: 1.4em)[
  #for c in data.competencies [
    - #c.text
  ]
]

== Selected experience
#for e in data.experience [
  #block(above: 0.75em, below: 0.25em)[
    #text(weight: "bold")[#e.role]#if e.organisation != "" [, #e.organisation]
    #h(1fr)
    #text(size: 9.5pt, fill: luma(60))[#e.dates]
  ]
  #for b in e.bullets [
    - #b.text
  ]
]

#if data.education.len() > 0 [
  == Education and credentials
  #for ed in data.education [
    - #ed.text
  ]
]
