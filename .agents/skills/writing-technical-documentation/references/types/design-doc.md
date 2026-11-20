# Design doc and RFC

Reader: reviewers deciding whether to approve a plan, before the code exists. Its value is the trade-offs, so a document with no trade-offs or alternatives is an implementation manual, and the code would serve better.

Length: 10 to 20 pages at most for a large project; past that, split the problem. A 1 to 3 page mini design doc suits an incremental change. Skip the document when the solution is unambiguous. Write one when the design is uncertain, senior input would help, the choice is contentious, or cross-cutting concerns are easy to miss.

Rules that make it useful:

- Keep context objective. It is background facts, with the evidence that the problem exists (numbers, incidents).
- A non-goal is something that could reasonably be a goal and is deliberately excluded. "Will not crash" is a negated goal, so it does not count.
- Show enough of the design to implement and to disagree with. Use a system context diagram and the relevant parts of the API and storage, and show code only for novel algorithms.
- Alternatives considered is one of the most important sections: each option, its strengths, and why it lost.
- Take a position. A doc that lists options without choosing is a survey.
- Mark what would change the decision.

Lifecycle: iterate with close colleagues, then review, then implement. Update the doc while reality corrects it before shipping, and link amendments. After shipping it is a record of the decision, so current behavior belongs in the architecture overview.

An RFC is a design doc with a formal process. Add status, reviewers, a decision date, and a decision summary at the top.

````markdown
# <Title>

- Author: <name>
- Reviewers: <names>
- Status: <draft | in review | approved | rejected | implemented>
- Last updated: <YYYY-MM-DD>

## Summary

<Problem, proposed solution, and the decision needed, in four sentences.>

## Context and problem

<What is wrong now, with evidence.>

## Goals

- <measurable goal>

## Non-goals

- <something a reader could reasonably expect that this will not do>

## Design

<Overview first, then data model, interfaces, and failure handling as needed.>

## Alternatives considered

| Option | Strength | Why not |
|---|---|---|
| <A> | <what it does well> | <reason> |

## Cross-cutting concerns

<Security, privacy, cost, observability, testing, rollout, rollback, compatibility. Only those that change the design.>

## Risks and open questions

- <risk and mitigation>
- <question, owner>

## Milestones

1. <deliverable>
````
