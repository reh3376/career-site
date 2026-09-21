# From Data to Decisions: Why Your Manufacturing AI Keeps Dying in the Pilot

Most industrial AI programs don’t fail because the model was bad. They fail because the context the model needed was never built — or never governed once it was.

That’s the argument at the center of a reference architecture I’ve been putting together for the **ontology-driven manufacturing hub**. The shorthand: *the ontology is the ammunition for decision-making, but manufacturing the ammunition is the war.* Vendors love to sell the gun — the “AI layer” — and quietly assume the supply line feeding it already exists. It usually doesn’t. A 2026 industrial readiness survey found roughly 68% of AI efforts stuck in pilots, with more than half of respondents naming data quality as the top blocker. Ambition is running about six times ahead of readiness.

The piece pulls apart a distinction the word “AI” is most often used to hide: **contextualized data is not knowledge.** “Pump P-12 vibration 0.8 mm/s during Run 4471’s cure phase” is a beautifully labeled *fact* — but it’s still one fact, about one thing, at one moment. Knowledge is the typed, relational, generalized structure that lets you reason about a reading you’ve never seen. Crossing that gap takes four moves — type it, relate it, generalize it, formalize it — and not one of them is contextualization. That’s the part your SMEs own, and the part no vendor can ship in a box.

From there it lays out three levers that decide everything:

- **Decision quality** is a function of *context*, not data volume.
- **Decision latency** is a function of *where* context gets assembled — once at capture (cheap to query forever) or reconstructed at query time (the latency that kills the pilot-to-production jump).
- **Decision trustworthiness** is a function of whether the loop is *closed and governed* — because a grounded-*looking* answer from a stateless retriever is the most dangerous output in the building: it joins two real entities through the wrong edge, cites a real provenance trail, and is confidently wrong.

The architecture itself is a six-layer stack — from OT and edge capture, through a contextualization layer (the thing the industry usually calls a “Unified Namespace”), up to a formal semantic layer built on ISA-95/88 and IOF/BFO, and finally a reasoning-and-governance layer that closes the action-to-outcome loop. Along the way it gets specific about the modeling choices that make or break the whole thing: the continuant-vs-occurrent discipline (model the *run* as a thing in its own right, not as properties bolted onto a pump), decision lineage as first-class nodes, and why GraphRAG is necessary but nowhere near sufficient.

That last point is the load-bearing one. Grounded retrieval is table stakes. The hard, mostly-unsolved part is everything you have to build *around* it: runtime constraint enforcement, contradiction detection against standing knowledge, a persistent memory of what was decided and why, and a learning signal that turns SME overrides into measurably better decisions over time. The document argues — bluntly — that this is the part the market does not yet hand you finished, which is exactly why it’s the moat.

The back half is practical: a market map (Palantir Foundry, Cognite, HighByte/Litmus, open-source, pure DIY), a build-vs-buy decision lens, an “abstraction-treadmill test” for sniffing out layers that just relabel a stable standard without adding context, and a phased build sequence designed so *every* phase ships a measurably better decision — instead of 18 months of “data foundation” with nothing to show for it.

One thing it is *not*: a product pitch. It’s a map and an evaluation lens — a way to tell durable capability from marketing, because the parts vendors gloss over are usually the parts you can’t buy your way across. A pilot that works in a demo and dies in production almost always died at one of those glossed-over layers — the knowledge transformation, or the closed loop — not at the model.

If you’re trying to get manufacturing AI out of the pilot graveyard — or trying to evaluate a platform that promises to do it for you — this is the framework I wish I’d had at the start.

The full reference architecture goes layer by layer. Happy to share it with anyone working this problem.