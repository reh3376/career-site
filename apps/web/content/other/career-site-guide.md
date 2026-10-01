---
title: How this site works
visibility: public
---

# How rogerhenley.dev works

A guide to what is on this site and how to use it. Written so the
assistant can answer questions about the site itself, which it
previously could not: asked how to upload a job description, it said it
had nothing in its records, because nothing in the corpus described the
site.

Roger built all of it. The site is itself a work sample.

## What anyone can see without signing in

- **The landing page** at `/`. It ships in two visual modes, an
  editorial one and an HMI/SCADA one that reads like a control screen.
  A toggle switches between them and the choice is remembered.
- **Articles** at `/articles`. Long-form writing on manufacturing data,
  decision-making and industrial systems.
- **Gallery** at `/gallery`. Professional portraits and on-the-floor
  work photos, with context.
- **Contact** at `/contact`. Reach Roger about the site, a bug, a
  feature idea, or an interview.
- **How Ask Roger works** at `/how-ask-roger-works`. The AI disclosure:
  what the assistant is, what it draws on, and what it will not do.
- **Privacy** at `/privacy` and **terms** at `/terms`.
- **Register** at `/register` and **sign in** at `/login`.

Everything else redirects to sign-in.

## What members can do

Registration is open; an account is approved before the member-only
tools unlock.

### Ask a job description how well it fits

`/jd-upload`. Paste a job description and Roger's reviewer scores it
against his experience, requirement by requirement. It extracts the
requirements from the posting, retrieves evidence from his records,
judges each one as met, partial or unmet, and returns a score with the
reasoning behind every verdict.

When the fit comes back strong or better it also writes a résumé
tailored to that specific posting and returns it as a PDF. Every line
of that résumé is drawn from his actual records with the sources
recorded; nothing is invented.

There is a daily submission limit per member.

### Book time with Roger

`/meetings`. Pick a length (15, 30 or 45 minutes), pick a day, pick a
time. The slots come from Roger's real calendar, so anything offered is
genuinely free, and booking writes the meeting to it. Fifteen minutes
of clearance is enforced between meetings.

Choose video or phone. For video, pick Google Meet, Microsoft Teams or
Zoom and you host the room. For phone, give the number you will call
from. Both sides get an email with the details, and a member can cancel
their own booking from the same page.

Times are shown in Eastern Time and follow daylight saving.

### Ask Roger

`/ask` for the full page, or the panel that opens from any page once
signed in. Ask about his background, what he has built, or what he has
written. Answers come from his own records and show their sources.

Answers take ten to thirty seconds. Everything runs on one small
server, and the assistant reads his actual records before it says
anything. Common questions he has answered himself come back instantly.

### Member home and settings

`/home` shows what you have looked at and what is new since last time.
`/settings` is your account.

## What the assistant will and will not do

It answers about Roger's professional background, his work, the views
he has published, and this site.

It will not discuss compensation or salary expectations, references,
anything confidential to an employer, or his personal life. Those are
better asked of Roger directly, through `/contact` or by booking time.

It answers only from his records. Asked something they do not cover, it
says so rather than guessing. Sources are listed under each answer;
some of his material is unpublished and is shown as a title with no
link, because it informs an answer without being quotable.

## How it is built

Go and Python behind a Next.js front end, on Docker, on one small
server. Postgres with pgvector for retrieval. The language model runs on
the same box rather than a hosted API.

The specification, the decision log and the engineering notes are in
the repository, including the things that went wrong.
