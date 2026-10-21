---
id: project-pro-manage
type: project
title: pro-manage
slug: pro-manage
summary: An owner-side capital project management system that applies TPM to the project itself, treating delay, rework and waiting on approvals as losses to be priced.
role: Sole author of the specification
status: active
technologies: Go, Python, Next.js, Postgres, TimescaleDB, gRPC
started: 2026
repo: https://github.com/reh3376/pro-manage
order: 70
visibility: public
---

## The problem

A plant running Total Productive Maintenance names its losses, prices
them in dollars, and ranks them on a Pareto chart to decide what to fix
first. It does this rigorously for every machine on the floor.

It does none of it for the capital projects that build those machines.
Delay, rework, interface clashes and waiting on information or approvals
are real losses with real costs, and they disappear into a single line
called "schedule slip". Nobody can rank them, so nobody fixes the
biggest one.

## The idea

Treat the capital project as a piece of equipment, and manage it with
TPM. Project losses and plant losses get priced in the same dollars on
the same Pareto, which makes them comparable for the first time. Losses
are captured as a by-product of work people already do, from requests
for information, submittals, work packages and hold points, rather than
as a reporting task nobody has time for.

The system carries a project from the capital approval request through
front-end loading and early equipment management to vertical startup and
benefits realization, and it closes the loop that TPM is actually about:
what the running assets teach goes back into the next design.

The organizing question, in the words a plant manager would use: *did
the asset start up clean, and did Operations and Maintenance inherit
something they could run?*

## Where it stands

**Specification, not software.** The functional specification, domain
model, roadmap and open-items list are written and public. No
application code is committed yet, and the repository says so on its
front page.

That ordering is the point rather than an apology for it. The design
covers 100 to 500 users across 200-plus concurrent projects ranging from
twenty-five thousand dollar refurbishments to quarter-billion-dollar
greenfields, and the expensive mistakes in a system like that are made
in the domain model, not the syntax.

## Shape

A Go core with a Python sidecar and a Next.js front end, over Postgres
for the source of truth and TimescaleDB for metric readings and loss
events.

The part worth describing is the connector framework. It does not
replace the ERP, the maintenance system or the plant historian; it
connects to them through nine pluggable module types, each a container
behind a stable gRPC contract, so changing a vendor is a configuration
change rather than a rewrite. Where the ERP already owns capital
projects, it stays the system of record and this adds the TPM and
governance layer on top.

[MDEMG](/projects/mdemg) runs alongside it as the institutional memory,
with its model hosted locally, so no project data leaves the company's
own network.
