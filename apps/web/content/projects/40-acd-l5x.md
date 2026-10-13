---
id: project-acd-l5x
type: project
title: acd-l5x-tool-lib
slug: acd-l5x-tool-lib
summary: Converts Rockwell .acd and .l5x both ways with validation, so PLC code can live in version control like any other source.
role: Sole developer
status: maintained
technologies: Python, Rockwell Allen-Bradley, Studio 5000, CI/CD
started: 2025
repo: https://github.com/reh3376/acd-l5x-tool-lib
order: 40
visibility: public
---

## The problem

PLC programs live in a binary project file. A binary cannot be
diffed, reviewed or merged, so the controls code running a plant sits
outside every practice that makes software changes safe, and "what
changed" is answered from memory.

## The approach

Convert `.acd` to `.l5x` and back again, with validation testing on the
round trip so a conversion that would lose something fails rather than
silently dropping it. Once the code is text, an enterprise can manage
versioning and development in standard GitHub repositories, with the
review and CI/CD practice that IT has had for twenty years.

## Outcomes

Maintained. The round-trip validation is the part that matters: a
one-way export would be a reporting tool, and this has to be safe
enough to put the result back on the controller.

The public repository has not moved since 2025; the work has
continued outside it.
