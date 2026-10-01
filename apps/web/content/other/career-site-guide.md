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

The site is in beta. The core functions all work; some of it is still
rough, and the header says so on every page rather than leaving anyone
to find out by hitting an edge.

## What anyone can see without signing in

- **The landing page** at `/`. It ships in two visual modes, an
  editorial one and an HMI/SCADA one that reads like a control screen.
  A toggle switches between them and the choice is remembered.
  The hero carries the three core functions, JD review, booking a
  meeting, and Ask Roger, each with a "What is this?" that opens an
  explanation. All three need an account, so pressing one while signed
  out goes to the sign-in page and then on to the thing that was
  pressed.
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
signed in. Ask about his background, what he has built, what he has
written, or about this site and how to use it. Answers come in his
voice, from his own records, and show where they came from.

**What happens when you ask.** Four steps, and the first one is the
reason some answers are instant.

1. The question is compared against a bank of questions Roger wrote the
   answers to himself. A close enough match, and far enough ahead of the
   next candidate, returns his words straight away. No model runs, so
   the answer is his rather than a reconstruction of his.
2. Otherwise the question is turned into a vector and the nearest
   passages in his records are retrieved, alongside a short sheet of
   career facts that every question gets: tenure, titles, dates,
   credentials. Those are what retrieval is worst at and what questions
   most often turn on.
3. The model is given the facts, the passages and the conversation so
   far, and answers briefly in the first person. It streams back a
   sentence at a time.
4. Published passages that fed the answer are listed underneath and link
   to the piece they came from.

**How long it takes.** A few tens of seconds for anything the model has
to answer. Everything runs on one small server with no graphics card,
shared with the JD reviewer, and most of that time is the machine
reading the question's context rather than writing the reply. Questions
answered from the bank come back immediately. A question asked while a
JD review is running will be slower still, because the review has the
machine.

That is slower than a hosted assistant and it is the honest trade: the
records never leave the server, and the cost of running it is a box
Roger already pays for.

**Which model.** `qwen3:4b-q8_0`, open weights, served on the same
server as the reviewer. Retrieval uses a second, smaller model,
`nomic-embed-text`. The model has never been trained on Roger's career.
It is handed the relevant passages at the moment of the question, which
is why correcting it means fixing a document rather than retraining
anything.

**For admins.** An admin signed in at `/ask` also gets a dropdown of
fixed, read-only database queries: members, submissions, how answers
were produced, corpus and model usage, and the review queue. The model
neither writes nor chooses the SQL. A person picks from a list and a
fixed statement runs, so results are exact and arrive in milliseconds.

### Member home and settings

`/home` lists the five live surfaces, the JD review, booking a meeting,
Ask Roger, the articles and the gallery, and shows what you have looked
at and what is new since last time. `/settings` is your account.

## What the assistant will and will not do

It answers about Roger's professional background, his work, the views
he has published, and this site.

It will not discuss compensation or salary expectations, references,
anything confidential to an employer, or his personal life. Those are
better asked of Roger directly, through `/contact` or by booking time.

It answers only from his records. Asked something they do not cover, it
says so rather than guessing.

Sources are listed under each answer, and only published material
appears there. Roger's records include unpublished material, and the
assistant reads it: that is what lets it answer about work that never
became an article. But unpublished material is never quoted at length,
never linked, and never named, not even as a bare title, because a
title is itself something he did not publish. So an answer can be
informed by more than its source list shows. The rule is enforced where
the source list is built rather than by asking the model to behave, and
anything he does not want speaking for him at all is kept away from the
assistant entirely.

This is one way the assistant differs from the JD reviewer: a posting
submitted to the reviewer cannot reach the unpublished half of the
records at all. A question here can, because it is Roger's own record
answering about Roger.

## How it is built

Go and Python behind a Next.js front end, on Docker, on one small
server. Postgres with pgvector for retrieval. The language model runs on
the same box rather than a hosted API.

The specification, the decision log and the engineering notes are in
the repository, including the things that went wrong.
