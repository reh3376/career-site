---
id: project-forge
type: project
title: Forge
slug: forge
summary: An industrial data hub that collects, governs, stores, curates and serves plant data, in the form each consumer asks for.
role: Sole developer
status: active
technologies: Python, industrial data governance, plugin architecture
started: 2026
repo: https://github.com/reh3376/forge
order: 30
visibility: public
---

## The problem

Plant data is produced by systems that were never designed to share it
and is wanted by systems that cannot read it. The usual answer is a
point-to-point integration per pair, which multiplies.

## The approach

Forge Core, Forge DBs and Forge Plugins: a hub that takes data from the
system that created it, governs and curates it in one place, and serves
it to the systems that need it in the shape each one requests, rather
than in the shape it happened to arrive in.

## Outcomes

Active development. The architecture is the argument: governance in the
middle rather than at every edge.

Active, though the public repository lags the work: development has
continued outside it, so the last push date is not the last day spent
on this.
