---
id: project-career-site
type: project
title: rogerhenley.dev
slug: career-site
summary: This site: a working system built in the open, argued for by running rather than by describing.
role: Sole designer and developer
status: active
technologies: Go, Python, Next.js, Postgres, pgvector, Docker, Caddy
started: 2026
repo: https://github.com/reh3376/career-site
order: 10
visibility: public
---

## The problem

A CV asserts. A reader has no way to check an assertion, so the ones
that matter get discounted along with the ones that do not.

## The approach

Build the claim instead of writing it. The site runs a job-description
reviewer, a grounded assistant and a timed decision-quality experiment,
all on one small server, with the specification, the architecture
decisions and every diff public.

The reviewer reads a posting requirement by requirement against a
corpus of real records and returns met, partial or unmet with the
evidence it used. The score is arithmetic over those verdicts, computed
in code: the model never emits a number.

## Outcomes

In the open and running. The specification is a living document with
every requirement tracked to its state, and the gap between what it
claims and what is built is itself published rather than tidied away.
