---
id: project-mdemg
type: project
title: MDEMG
slug: mdemg
summary: A graph memory and reasoning substrate for AI agents: multi-dimensional, emergent, and reinforced by use.
role: Sole developer
status: active
technologies: Go, vector retrieval, knowledge graphs, RAG, LLM re-ranking
started: 2024
repo: https://github.com/reh3376/mdemg
order: 20
visibility: public
---

## The problem

An agent with a vector store can find text that resembles a question.
It cannot easily say what relates to what, or why one association
should matter more than another after a thousand uses.

## The approach

A multi-dimensional emergent memory graph: vector retrieval and
knowledge graph structure together, with agent memory and state
management on top, LLM re-ranking over the candidates, and Hebbian
reinforcement so associations that keep proving useful strengthen and
the rest decay.

## Outcomes

Two years of work and still active, with runners and clients across
several platforms: a self-contained sidecar runner, a Linux installer,
a macOS menu-bar app, a Windows build, an observability sidebar and a
Neovim plugin.

The public repository dates from 2026; the work began in 2024.
