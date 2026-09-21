---
title: "Better Business Decisions Require Better Information — Part 1"
subtitle: "Why Acting on Partial Data Feels Right and Often Is Not"
author: "Roger E. Henley II"
date: 2026
series: "Better Business Decisions Require Better Information"
part: 1
tags:
  - decision-making
  - data-governance
  - operations
  - engineering
  - manufacturing
---

# Better Business Decisions Require Better Information
## Part 1 — Why Acting on Partial Data Feels Right and Often Is Not

*Roger E. Henley II · 2026*

## Purpose

This paper is intended to improve **business decision-making in complex industrial operations** — not to comment on personal decision-making or individual judgment in private life. Personal decisions and business decisions are not governed by the same constraints, the same consequences, or the same obligation to preserve evidence, context, and repeatability. Business decisions operate inside shared systems, affect many people at once, and can create delayed consequences that are costly, difficult to reverse, and sometimes existential if they are not detected early.

The purpose here is to show why intelligent teams can still make poor business decisions when the information available to them is incomplete, fragmented, badly normalized, or missing critical context. It is meant to clarify that many decision failures are not primarily failures of effort or intent, but failures of information structure. When an organization cannot see base rates, conditioning variables, hidden interactions, lineage, or system state, even reasonable people will draw the wrong conclusion with confidence.

This paper is therefore meant to support a more disciplined view of digital transformation in manufacturing. The goal is not more dashboards, more disconnected tools, or more local optimization. The goal is to improve the quality of enterprise decisions by strengthening the information architecture beneath them. Reliable, governed, connected data is not an IT preference; it is part of the operating foundation required for sound judgment, faster learning, and lower-cost correction. That is why early engineering engagement, architecture review, and data governance matter. They are not administrative friction. They are mechanisms for reducing preventable decision error and preserving the context required for better decisions across the business.

This purpose also aligns with the intent of ISO-based management systems. At their best, such systems do not exist simply to satisfy audits or produce documentation. They formalize the conditions required for better business decisions: clear requirements, controlled change, traceable information, defined ownership, evidence-based evaluation, risk-based planning, and continual improvement. In that sense, stronger information architecture is not separate from compliance discipline. It is one of the practical ways the business supports it.

This is the **first part of a five-part series**. Part 1 focuses on the information side of decision quality: how incomplete or poorly structured information causes reasonable conclusions to become wrong. Part 2 builds on that foundation by addressing the human side: how bias, framing, assumptions, and premature consensus distort interpretation even when data exists. Part 3 addresses the exchange in which a correct analysis is presented and either acted on or set aside. Part 4 specifies the technical review as a control. Part 5 specifies the information infrastructure that control depends on. Together they describe a single discipline — surfacing weak signals early and testing conclusions before action hardens, so problems are caught while they are still inexpensive to correct.

## Introduction

Most of us make personal decisions with incomplete information every day. That is normal. In personal life, many of those decisions are local, reversible, and limited in consequence. Business decisions are different. They often affect multiple functions, persist long after the original decision-maker has moved on, and can create delayed consequences that are expensive to unwind.

That difference matters.

When important context is missing, when information is incomplete or unreliable, or when the structure of a problem is hidden behind partial data, the intuitive answer can feel entirely reasonable and still be wrong. Not because the people involved are careless. Not because they lack intelligence or commitment. Because the information available made the wrong answer look like the right one.

Business decision-making is therefore not simply "personal judgment at larger scale." It requires a more deliberate framework. We have to treat decision quality as something that can be designed for or designed against. When the information architecture is weak, even capable teams will misread patterns, overreact to noise, normalize the wrong variables, and act on incomplete stories. When the information architecture is strong, the business is far more likely to preserve the structure required for sound inference.

There is another reason this is more serious in business than in everyday life: feedback is often delayed. A process change can appear viable today, quietly affect every batch produced afterward, and only reveal its true cost years later in sensory drift, quality loss, customer dissatisfaction, or margin erosion. In that environment, decision quality cannot depend on intuition alone.

This paper examines how that happens, why it matters in manufacturing and distilling, and what it means for digital transformation in industrial operations. The goal is not to assign blame for past decisions. The goal is to understand the mechanics of decision quality so an organization can build systems that make good decisions more likely and bad decisions harder to reach.

## When Intuition Misleads: Three Examples

There are well-known problems in probability and statistics where the overwhelming majority of people, including trained professionals, arrive at the wrong answer. These are not trick questions. They are ordinary cases in which the intuitive answer is wrong because the framing leaves out structure that matters.

Each one maps directly onto the kinds of decisions an enterprise makes every day.

## Example 1: The False Positive Trap

A quality test is 95% accurate. A batch tests positive for contamination. How confident should we be that the batch is contaminated?

Most people answer, "very confident — around 95%." That feels natural. The test is accurate. The result is positive. The math seems obvious.

But the answer depends heavily on how common contamination actually is.

Suppose genuine contamination occurs in about 1 out of every 1,000 batches. Now run the numbers across 100,000 batches:

- 100 batches are genuinely contaminated. A 95% accurate test correctly flags about 95 of them.
- 99,900 batches are clean. But a 5% false-positive rate means roughly 4,995 of those also test positive.
- Total positive results: about 5,090. Of those, only 95 are real.

That means a positive result in this scenario is correct less than 2% of the time. The intuitive answer of 95% confidence is not just slightly wrong. It is wrong by roughly a factor of fifty.

**Enterprise parallel:** This same logic applies any time the business uses a predictive model, anomaly detector, quality flag, or vendor-supplied algorithm. If the event being predicted is uncommon and the false-positive rate is not extremely low, most alerts will be false. Without governed historical data and correct base-rate assumptions, a system can look mathematically impressive while driving wasteful holds, unnecessary interventions, and misallocated attention.

> **Key point:** The lesson is not that tests or models are useless. The lesson is that performance claims detached from context are easy to over-trust.

## Example 2: When Every Subgroup Tells One Story and the Total Tells Another

Imagine two production processes, A and B. Process A outperforms Process B on small batches. Process A also outperforms Process B on large batches. But when all the data is combined, Process B appears better overall.

That sounds impossible, but it is a real and well-documented statistical phenomenon called **Simpson's Paradox**. It happens because the two processes run a different mix of batch sizes. If Process B runs far more large batches, which are inherently higher yield regardless of process, the aggregate number tilts in its favor even though Process A is genuinely better in every comparable condition.

The aggregate result reverses because the weighting structure changed, and the weighting structure was hidden inside the combined view.

**Enterprise parallel:** This is exactly what happens when the business compares plants, shifts, operators, mash bills, raw material suppliers, or product families without proper normalization. One run appears to have worse yield, but it runs a lower-yield substrate. One shift appears slower, but it inherits more changeovers and startups. One supplier looks better overall, but only because its material was used during easier operating conditions. Without curated enterprise data and shared definitions, leadership can draw the wrong conclusions from aggregated views. That leads to distorted incentives, weak capital decisions, misguided root-cause analysis, wasted time, and increased total cost of production.

> **Key point:** The problem is not that people are irrational. The problem is that the underlying structure is not visible in the report they are using.

## Example 3: Correlation Does Not Ensure Cause

Ice cream sales and drowning deaths rise together every summer. Nobody concludes that ice cream causes drowning. The hidden variable — hot weather — drives both.

That example is obvious in a textbook. In a business, it is much less obvious because the hidden variables are buried inside systems that do not talk to each other.

- A process setting appears to increase yield, but the real driver is a change in feedstock composition that happened at the same time.
- A maintenance intervention appears to reduce downtime, but it was deployed first on the newest and easiest assets, so the comparison is not apples-to-apples.
- A quality deviation appears tied to a specific operator, but the hidden variable is upstream raw material moisture or CIP effectiveness on the batches that operator happens to run.
- A commercial promotion appears to improve margin, but the apparent gain came from inventory liquidation and short-term mix changes while operations absorbed lower throughput, higher coordination burden, and more changeover inefficiency.

Finding truly meaningful relationships — as opposed to patterns that merely appear in partial data — requires linking multi-departmental context across the enterprise so that hidden variables can be identified, measured, and accounted for. That means connecting process conditions, equipment states, operator actions, maintenance history, laboratory data, lot genealogy, scheduling context, inventory state, and commercial outcomes into a shared, queryable foundation.

Without that integration, the enterprise is doing pattern matching rather than causal learning. It finds associations and acts on them as if they were causes. That is how well-intentioned decisions go wrong.

## The Common Lesson

The same pattern appears in every example above:

- The intuitive answer is wrong because the available information is incomplete or badly framed.
- The correct answer emerges only when the full structure of the problem is preserved — base rates, segmentation, conditioning context, and hidden variables.
- Decision quality improves when that structure is made visible to the decision-maker.

This is not primarily a talent problem. It is a **systems problem**. Intelligent, experienced people will reach wrong conclusions with surprising consistency when the information presented to them omits the structure needed for sound inference. The fix is not better intuition. The fix is better information architecture.

That is the role of enterprise data infrastructure. Reliable, governed, curated, well-connected data does not merely make information easier to access. It preserves the structure required for good judgment. It keeps the base rates, the segmentation, the context, the lineage, and the process history intact so that decisions are grounded in the full picture rather than a convenient fragment.

## What This Looks Like in Practice: A Distillery Example

Consider a plausible scenario in a bourbon distillery. The specifics are illustrative rather than drawn from a single real incident. Over a three-week production run, operations notices a 1.2% drop in alcohol yield, sporadic sensory concerns, and a rise in residual starch on selected batches. The loss is large enough to matter financially but small enough that it could be dismissed as normal variation.

### What each department sees in isolation

- **Production dashboard:** Lower yield appears concentrated on second shift.
- **LIMS:** Several batches show elevated residual starch and slightly higher acidity.
- **Maintenance system:** No critical alarms, but one steam control valve shows intermittent position drift below the alarm threshold.
- **ERP / procurement:** A new corn lot with higher moisture content has entered normal use.
- **Historian / utilities:** Steam pressure is softer during certain post-CIP and startup windows.
- **Scheduling:** Second shift is more likely to inherit the higher-moisture grain lots during restart periods.

### The reasonable-looking but wrong decision

Leadership sees "second shift plus lower yield" and concludes the issue is operator execution. That is a reasonable inference from the data available in any single system. The business responds by tightening shift instructions, increasing supervision, adding more yeast and enzymes, lengthening cook time, and placing more batches on hold.

Those actions consume money, capacity, and employee goodwill. The yield problem persists. Worse, the organization has now framed a system interaction as a people problem, which damages trust and misdirects future investigation.

### What the integrated view reveals

When historian data, LIMS data, maintenance history, procurement lot information, and scheduling context are linked together, the actual pattern becomes visible. Yield loss occurs primarily when three conditions coincide: a higher-moisture corn lot, a startup window after CIP, and steam valve position drift during the cook stage. Second shift looks worse only because it disproportionately inherits that combination. Residual starch, acidity drift, and yield loss are not separate problems. They are different observations of the same underlying process instability.

The correct response is to normalize yield on a dry-solids basis, repair or replace the steam control valve, tighten startup validation after CIP, connect lot genealogy to process windows and quality outcomes, and update escalation logic so shift performance is evaluated in process context rather than in isolation.

> **Key point:** The business pays for this problem twice: once for the original process loss, and again for the cost of the wrong response — extra chemicals, lost tank capacity, delayed shipments, avoidable holds, repeated troubleshooting, and erosion of trust between shifts and management. The right answer was in the data. It just was not visible in any one system.

## Why Local Fixes Cannot Solve a System-Wide Problem

There is a well-established principle in systems engineering called **Amdahl's Law**. It states that the total improvement achievable in a system is limited by the fraction of the system that remains unimproved. If a dramatic improvement is applied to a small piece of the whole while the rest stays the same, the overall gain is modest.

The math is simple. If a department improves its reporting workflow by a factor of ten, but that workflow represents only 10% of the end-to-end decision cycle, and the remaining 90% is still spent searching for data, reconciling conflicting numbers, correcting quality issues, and working around disconnected systems, the enterprise-wide improvement is roughly 1.1x. A tenfold local improvement produces barely a 10% overall gain.

That is **not** transformation. It is a fast tool operating inside a slow system.

This is the fundamental reason well-intentioned department-level digital initiatives — each solving a real problem for its immediate users — so often fail to produce enterprise-wide improvement. Each one optimizes a fragment. None of them addresses the shared infrastructure that determines whether the decision is correct in the first place.

The consequences are predictable:

- **Multiple versions of reality.** Each department builds its own metrics, its own pipeline, and its own definitions. Yield, margin, downtime, quality loss, and forecast accuracy mean different things in different systems. Reconciliation meetings replace decision meetings. Many organizations see versions of this pattern during major system transitions, where mismatched definitions and data lineage turn reconciliation into a prerequisite for action.
- **Hidden relationships stay hidden.** The most valuable enterprise insights live at the intersection of departments: how maintenance practices affect quality drift, how procurement variability affects process stability, how scheduling decisions affect working capital. Local systems cannot see these connections.
- **Costs compound silently.** A proof of concept looks cheap because it externalizes system cost. The bill arrives later as integration rework, duplicate infrastructure, person-dependent tribal knowledge, security gaps, and the inability to support new use cases without starting over.
- **Signal-to-action latency increases.** People still need to validate, reformat, reinterpret, and manually bridge gaps between systems before they can act. Departmental patches add steps instead of removing them.

## The Hidden Costs of Fragmented Information

The financial impact of acting on incomplete information does not appear on any single line item. It is distributed across departments and across time. That is precisely why it persists: each individual cost looks small or gets attributed to something else. Collectively, these costs represent a recurring tax on the business.

| Source of Loss | Illustrative Annual Cost |
|---|---:|
| Avoidable yield loss, giveaway, or rework from incomplete process context | $500,000 |
| Slow root-cause analysis and misdirected action on bottleneck assets | $600,000 |
| Manual reconciliation and duplicate analysis across departments | $300,000 |
| Excess inventory or safety stock driven by low visibility and low signal trust | $250,000 |
| Expedites, schedule disruption, and premium logistics from late or wrong decisions | $350,000 |
| **Estimated Recurring Annual Cost** | **$2,000,000** |

These are illustrative order-of-magnitude figures, not precise forecasts. The exact numbers will differ for any given business. But the structure of the problem does not change: the enterprise pays for fragmented information continuously, in small amounts, across many functions, and the total is substantial.

The comparison with a one-time foundational investment is worth considering. An enterprise data program — integration, governance, storage, curation, and service delivery — typically requires concentrated upfront investment. But it lowers the marginal cost of every future analytics, AI, optimization, and application effort built on top of it. The patchwork path distributes cost invisibly. The foundational path concentrates it visibly. Over a five-year horizon, the foundational path is usually cheaper, and the decisions it enables are usually better.

## What This Means in Practice

An enterprise that has an approved digital strategy has already defined the architecture, the governance model, the integration standards, and the minimum technical requirements for systems that enter the environment. That strategy exists to ensure every digital investment contributes to a coherent foundation rather than adding to a fragmented patchwork.

The strategy works when the organization follows it collectively. It weakens when individual departments bypass it to solve immediate problems independently. Not because those departments are wrong to want faster solutions. Because the downstream cost of fragmentation usually exceeds the upfront cost of doing it right.

The practical implications are straightforward.

### Engage engineering early

When a department identifies a digital need, the right time to involve engineering is before evaluating vendors, signing contracts, or launching pilots. Early engagement makes it possible to meet the need within the existing architecture or to design new capability that extends the platform rather than fragmenting it.

### Respect the review process

The requirement that new tools pass architecture, security, and data governance review before entering production exists because the cost of integrating, maintaining, and eventually replacing a non-compliant system is almost always higher than the cost of the review. This is not overhead. It is protection against compounding technical debt.

This is also one reason formal management-system disciplines exist. Requirements around documented information, management of change, corrective action, management review, and risk-based planning are not paperwork for their own sake. They are safeguards that make evidence, ownership, review, and escalation explicit.

### Invest in the foundation, not just the applications

Do not promise a roof tomorrow if the foundation will not be laid until next week. Applications are visible and easy to justify. The data platform underneath them is less visible but far more valuable. Budget decisions should reflect that the platform is the enabling investment that makes every application cheaper, faster, and more trustworthy.

### Treat data governance as a business responsibility

In practical terms, **governance** means agreeing on what key data means, who owns it, how quality is verified, how changes are controlled, and how conflicts are resolved when systems disagree.

Governance requires cross-functional ownership of data definitions, quality rules, and stewardship. Engineering can build the machinery, but governance only works when every department commits to canonical definitions, consistent lineage, and shared accountability for data quality. This is a business responsibility, not only a technology responsibility.

### Expect infrastructure before dashboards

Foundational data work does not produce visible applications immediately. It produces the conditions under which every future application is trustworthy, maintainable, and extensible. A sound digital strategy acknowledges that progress will be slow at first and accelerate as infrastructure matures. That acceleration depends on the foundation being built correctly, which depends on organizational patience and commitment.

## Closing Thought

The goal of digital transformation in industrial operations is not more tools. It is better decisions: decisions about production, quality, maintenance, inventory, energy, scheduling, procurement, sales, forecasting, and commercial performance that are faster, more accurate, more consistent, and grounded in the full context of the business.

That goal is achievable. But it requires the organization to internalize a simple principle: **the quality of a decision can never exceed the quality of the information behind it.** When information is fragmented, incomplete, or disconnected, smart and experienced people will reach wrong conclusions on a regular basis — not because of personal failure, but because the system failed to preserve what they needed to see clearly.

Building the system that gives people what they need is the transformation. The right strategy is not to buy isolated answers forever. It is to build a common foundation that gives teams the tools, context, and trust required to solve their own problems on purpose rather than by workaround. Everything else is incremental.

---

*Roger E. Henley II — Part 1 of a five-part series on decision quality in complex operations. Continued in Part 2: "Why and How We Become Confidently Wrong," Part 3: "When Being Right Is Not Enough," Part 4: "The Technical Review Standard," and Part 5: "The Decision Infrastructure Standard."*
*github.com/reh3376 · linkedin.com/in/rogerehenley*
