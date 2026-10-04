# 0030. Collect in Postgres, analyse in a graph later

**Status:** Accepted 2026-10-04.

## Context

The decision test (`docs/fsd-decision-test.md`) exists to produce a governed, curated dataset and the means to find correlations in it. The owner proposed storing it in a graph database, on the reasoning that edges make relationships easy to uncover with simple queries. He has grounds: MDEMG is his own Go and Neo4j platform, so this is an informed preference rather than a reach for a fashionable tool.

Three facts about the data decided it the other way, for collection.

**A statistical correlation is not an edge.** An edge records a known relationship between two entities and traversal finds it. "Education correlates with calibration" is stored nowhere and cannot be traversed to; it is computed by aggregating across many rows. Graph databases are strong at variable-depth traversal over sparse heterogeneous relationships, and comparatively weak at aggregation and statistics. The questions this dataset exists to answer, education against calibration, baseline attention against susceptibility, load against the confidence gap, are `GROUP BY` and regression. Cypher handles those awkwardly and Neo4j has no native regression.

**The shape is a star schema.** One fact table of answers, thirty rows per session, with four dimensions: participant, item, session, block. Fixed grain, fixed joins, all known before any data exists. The joins are not the question here, which is what makes traversal valuable elsewhere; they are fixed plumbing underneath the question.

**The asymmetry.** Relational data projects into a graph cheaply. Relational rigour cannot be recovered from a graph built without it. Getting this wrong costs a re-collection, and participants are the scarcest input this project has.

Memory was considered and is not the deciding factor. The box has 9.3 GB available, Ollama sits at 4.9 GB against its 7 GB cap, Postgres uses 125 MB; Neo4j would fit. The operational cost is more concrete than the memory: Postgres is already backed up nightly with a weekly automated restore that verifies row counts, already has the read-only role behind `/admin/db`, already has migration discipline. A second datastore means a second backup story, a second migration story, a second security story, and the restore drill that currently passes would stop covering everything.

## Decision

**Collection is relational, in the existing Postgres.** Sessions, answers, recalls, items and participants, with the provenance and flags in FSD §7b.

**Analysis in a graph is expected and planned for, not precluded.** Once there is data worth exploring, and particularly if test results are to be linked across the rest of the site, event stream, JD submissions, chat, articles read, a graph is the better tool for that exploration. That is a genuinely graph-shaped problem in a way the collection is not.

**The projection boundary is kept clean deliberately**, so the graph load is a mechanical export rather than a redesign:

- **Relationships are named explicitly** in the schema and its comments, not left implicit in foreign-key conventions. Each one states the edge it becomes: a participant *took* a session, a session *contains* an answer, an answer *is of* an item and *sits in* a block.
- **The grain stays one row per answer.** Aggregates are derived, never stored as the source of truth, per `docs/metrics.md`.
- **The flat analysis view is the projection boundary.** It is what analysis reads and what a graph load exports from. Nothing else should need to know the table layout.
- **Entities carry stable identifiers** that survive an export, so a node in the graph can be traced back to the row that produced it.

## Consequences

The dataset is immediately queryable with the tooling already in place: `/admin/db`, metric views, CSV export, the nightly backup and its verified restore. No new service, no new failure mode, nothing added to the deploy.

The graph work is deferred rather than designed away, and it starts from clean data rather than from data shaped by a collection-time guess about which edges would matter. Which edges matter is usually clearer after seeing the data than before.

The cost is that cross-surface exploration waits. That is the right trade while the dataset is empty: there is nothing to explore yet, and the thing that would be irreversible is collecting badly.

If the graph projection is built and proves itself, this record gets a status update pointing at it. It is not superseded by that: collection stays relational either way.
