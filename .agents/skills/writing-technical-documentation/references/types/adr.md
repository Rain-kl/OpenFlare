# Architecture decision record (ADR)

One decision per file, written so a reader a year later understands why without asking anyone. Write one when a decision is hard to reverse, costly, or likely to be questioned.

Follow the repo's existing convention. Otherwise use MADR (Markdown Any Decision Records) 4.0: files in `docs/decisions/` named `NNNN-title-with-dashes.md`, numbered consecutively.

An accepted ADR is not edited. A changed decision gets a new ADR, and the old one's status becomes `superseded by ADR-NNNN`. Status values: proposed, rejected, accepted, deprecated, superseded.

Common failures: no rejected alternatives, no costs, or a win-win with no downside. If nothing got harder, the analysis is incomplete. State the decision in active voice, and name the constraint that decided it: team size, deadline, existing system, compliance, or a latency target.

To record a decision already made, take the facts from code, PRs, and the people involved. Mark any reason you are inferring as inferred, because a plausible reason you invented is worse than a gap.

Done when: a reader can say what was chosen, what it cost, what lost, and how you would know it was followed.

```markdown
---
status: <proposed | accepted | rejected | deprecated | superseded by ADR-NNNN>
date: <YYYY-MM-DD>
decision-makers: <names or roles>
---

# <Short title naming the problem and the solution>

## Context and problem statement

<Facts and constraints that force a decision: numbers, deadlines, existing systems.>

## Decision drivers

- <force that matters>

## Considered options

- <option A>
- <option B>

## Decision outcome

Chosen option: <option>, because <reason tied to the drivers>.

### Consequences

- Good: <result>
- Bad: <cost, what becomes harder>

### Confirmation

<How to check it was followed: a test, a review, a lint rule.>

## Pros and cons of the options

### <Option B>

- Good: <argument>
- Bad: <argument>
```
