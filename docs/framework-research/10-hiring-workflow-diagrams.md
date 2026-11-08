# Hiring Workflow Diagrams (Mermaid source)

Revision 2, 2026-11-08. Node numbers match `09-hiring-workflows-draft.md`. Rendered copy: <https://claude.ai/artifact/AJWsmPtYodKEfbcbBRbieL>, or open `diagrams/workflow-viewer.html` locally for pan and zoom. GitHub renders the blocks below directly; the `.mmd` sources in `diagrams/` are the same text.

Legend: diamond = decision point · dashed grey border = open pending the pilot (C-14, K-04, K-09) · dashed amber = hold/deferred state, dated, escalates when the date passes unreviewed · blue = system or system touchpoint · green = outcome data.

## 1. Company perspective (swimlanes by owner)

```mermaid
flowchart TB
  classDef decision fill:#FFF3D6,stroke:#8A5A00,stroke-width:2px
  classDef hold fill:#F3F3F0,stroke:#8A5A00,stroke-dasharray:5 3
  classDef open fill:#fff,stroke:#6B6B6B,stroke-dasharray:6 4
  classDef sys fill:#E6EEF9,stroke:#1F4E8C
  classDef outcome fill:#E7F2EB,stroke:#1E5A3A

  subgraph HM[Hiring manager]
    C01[C-01 Need identified] --> C02[C-02 Manpower request]
    C06[C-06 HM drafts role document<br/>+ structured requirements]
    C15[C-15 Interviews]
    C16[C-16 Assessment / work sample]
    C17{C-17 Hire decision}
    C23[C-23 30-day check-in]
    C24[C-24 90-day review]
  end

  subgraph CMT[Committee / budget owner]
    C03[C-03 Request review<br/>finance · HR · dept head · HM mgr]
    C04{C-04 Approval decision}
    H04[Deferred: resubmission date<br/>past date → escalate]
  end

  subgraph HR[HR]
    C05[(C-05 Role-definition template)]
    C07{C-07 Conformance + comp check}
    H07[Hold with revisit date]
    C18[C-18 Offer prepared]
    C21[C-21 Pre-boarding + verification]
    C22[C-22 Start / onboarding]
  end

  subgraph REC[Recruiter]
    C08[C-08 JD rendered + posted]
    C09[C-09 Applications / sourcing]
    C10[C-10 Knockout filter<br/>hard constraints only]
    C12{C-12 Screen decision}
    H12[Hold with revisit date<br/>past date → escalate]
    C13[C-13 Screen call<br/>+ remaining clarifications]
    C14{C-14 Slate to HM}
    C19{C-19 Offer outcome}
    C20[C-20 Notify non-selected<br/>with substantive reason]
    C26[C-26 Requisition closed<br/>lessons captured]
  end

  subgraph SYS[System]
    C11a[C-11a Evidence gathered]
    C11b[C-11b Requirements judged<br/>three-number coverage]
    C11c[C-11c System clarification round]
  end

  subgraph OUT[Outcome data]
    C25[C-25 6–12 month retention / performance]
  end

  C02 --> C03 --> C04
  C04 -- Approved --> C06
  C04 -- Rejected --> X04((end))
  C04 -- Deferred --> H04 --> C03
  C05 -.template.-> C06
  C06 --> C07
  C07 -- Conforms --> C08
  C07 -- Non-conforming --> C06
  C07 -- Hold --> H07 --> C07
  C08 --> C09 --> C10 --> C11a --> C11b
  C11b -- "unresolved weight" --> C11c --> C11b
  C11b --> C12
  C12 -- Advance --> C13
  C12 -- Reject --> C20
  C12 -- Hold --> H12 --> C12
  C12 -- "Needs HM input" --> C06x[HM audits flagged judgments] --> C12
  C13 --> C14
  C14 -- Accept slate --> C15
  C14 -- Reject individuals --> C20
  C14 -- Reject slate --> C09
  C15 --> C16 --> C17
  C17 -- Select --> C18
  C17 -- Re-source --> C09
  C17 -- Redefine --> C06
  C17 -- Defer --> H17[Defer with revisit date] --> C17
  C18 --> C19
  C19 -- Accepted --> C21
  C19 -- Negotiate --> C18
  C19 -- "Declined or no response by date" --> C17
  C21 --> C22 --> C23 --> C24 --> C25 --> C26
  C20 --> C26

  class C04,C07,C12,C14,C17,C19 decision
  class H04,H07,H12,H17 hold
  class C14 open
  class C11a,C11b,C11c sys
  class C23,C24,C25 outcome
```

## 2. Candidate perspective

```mermaid
flowchart TB
  classDef decision fill:#FFF3D6,stroke:#8A5A00,stroke-width:2px
  classDef open fill:#fff,stroke:#6B6B6B,stroke-dasharray:6 4
  classDef touch fill:#E6EEF9,stroke:#1F4E8C
  classDef outcome fill:#E7F2EB,stroke:#1E5A3A

  K01[K-01 Trigger, decide to look] --> K02[K-02 Evidence preparation<br/>depth + audience chosen]
  K02 --> K03[K-03 Role discovery]
  K03 --> K04{K-04 Apply or not}
  K04 -- Skip --> K03
  K04 -- Apply --> K05[K-05 Application submitted]
  K05 --> K06[K-06 Knockout filter · C-10]
  K06 -- "fails hard constraint" --> K06x[Told which constraint, where policy allows]
  K06 --> K07[K-07 Initial screen · C-11<br/>candidate sees own match record]
  K07 -- "unresolved items" --> K08{K-08 Clarification questions · C-11c}
  K08 -- "Answer / not applicable" --> K07
  K08 -- Decline --> K09
  K08 -- Withdraw --> KW((withdrawn))
  K07 --> K09{K-09 Screen outcome · C-12}
  K09 -- "Rejected, reason given" --> K09d[Request feedback<br/>Dispute a judgment: OPEN]
  K09 -- "Hold, revisit date shown" --> K09h[Wait or withdraw]
  K09h -- "date passes" --> K09s[Status change / escalation visible]
  K09 -- Advanced --> K10[K-10 Recruiter call · C-13<br/>remaining clarifications asked live]
  K10 -- misaligned --> KW
  K10 --> K11[K-11 Interviews · C-15/16<br/>knows which requirements will be probed]
  K11 -- withdraw --> KW
  K11 --> K12{K-12 Decision outcome · C-17}
  K12 -- "Rejected, reason given" --> K12d[Request feedback]
  K12 -- Selected --> K13{K-13 Offer · C-18/19}
  K13 -- Negotiate --> K13
  K13 -- "Ask for time until DATE" --> K13
  K13 -- Decline --> K03
  K13 -- Accept --> K14[K-14 Pre-boarding + start · C-21/22<br/>self-attested items verified]
  K14 --> K15[K-15 30/90-day check-in · C-23/24<br/>own fit rating recorded against pre-hire record]
  K16[K-16 Parallel applications<br/>one evidence record, many roles] -. "runs alongside K-04 to K-13" .-> K04
  K13 -- Accept --> K16x[Withdraw other applications]

  class K04,K08,K09,K12,K13 decision
  class K04,K09d open
  class K06,K07,K10,K11,K14 touch
  class K15 outcome
```
