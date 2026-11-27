---
id: project-v2c-translator
type: project
title: v2c translator
slug: v2c-translator
summary: Reads a veteran's DD-214, DD-215 and VMET and returns the civilian job titles, skills and keywords that record supports, at rogerhenley.dev/v2c.
role: Sole developer
status: active
technologies: Python, OCR, form layout analysis, O*NET, Next.js, TypeScript
started: 2026
repo: https://github.com/reh3376/v2c-translator
order: 80
visibility: public
---

## The problem

A separation document describes a career in the military's own codes:
a specialty number, a pay grade, a list of courses and awards. Job
applications are screened for civilian words. The translation between
the two is usually done by hand, by the veteran, at the moment they
have the least time for it.

## The approach

The translator reads the forms by their layout, block by block, so a
scanned DD-214 with a poor text layer still gives up its specialties,
service time, training and awards. It matches each specialty to
civilian occupations through the DoD and Department of Labor
crosswalks and O*NET, and returns job titles, transferable skills,
knowledge areas and a keyword string ready to paste into an
application.

Use it at [rogerhenley.dev/v2c](/v2c). No account is needed; uploads are
read in memory and deleted as soon as the report is written.

## Outcomes

Live. Every reading decision is tested against synthetic forms built
from real layouts, and the documents themselves never enter the
repository: the reader was calibrated from geometry alone.
