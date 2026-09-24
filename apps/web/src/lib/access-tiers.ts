// What a visitor can do before and after an account, in one place.
//
// Two landing surfaces render this, the editorial one and the plant HMI
// one, and a third copy of the same claims would be a third chance to
// be wrong. The list is also the site's promise, so it has to describe
// what the software actually does today rather than what it is meant to
// do later.

export type AccessItem = {
  // Short enough to scan down a column.
  label: string;
  // What it actually gets you. One sentence.
  detail: string;
};

// Open to anyone, no account and nothing asked.
export const OPEN_TO_ANYONE: AccessItem[] = [
  {
    label: "The writing",
    detail: "Every article in full, on control systems, manufacturing data and applied AI.",
  },
  {
    label: "The work photos",
    detail: "A selection from the plant floor: the sites, the panels and the equipment behind the record.",
  },
  {
    label: "How the reviewer works",
    detail:
      "Which model reads a posting, that it was never trained on the career it judges, and how far it currently disagrees with Roger. The numbers are live, including the ones he is failing.",
  },
  {
    label: "A direct line",
    detail: "The contact form reaches him without an account.",
  },
];

// What an account adds. Access is granted by Roger rather than issued
// automatically, which is stated here because a visitor who expects an
// instant login and gets a wait feels misled by the difference.
export const WITH_AN_ACCOUNT: AccessItem[] = [
  {
    label: "Have a posting reviewed",
    detail:
      "Paste a job description. It is split into individual requirements and each one is judged against his record, with the evidence shown for every verdict and the score computed in code.",
  },
  {
    label: "A résumé for that posting",
    detail:
      "When the fit is strong or better, a two-page résumé written for that specific posting, where every line has to be carried by a source. Delivered as a PDF and emailed to you.",
  },
  {
    label: "The full gallery",
    detail: "The complete set of work photos rather than the public selection.",
  },
  {
    label: "Your reviews, kept",
    detail:
      "Reviews reopen from your submissions list, so you can close the tab. Three reviews a day, because each one is close to an hour of work on one machine.",
  },
];

// The one thing a visitor most needs to know before clicking, and the
// thing most likely to annoy them if it is discovered afterwards.
export const ACCESS_CAVEAT =
  "Access is approved by Roger himself, so a request is a short wait rather than an instant login.";
