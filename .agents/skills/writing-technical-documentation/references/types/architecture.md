# Architecture overview

Reader: a new engineer or reviewer who needs the map before reading code. This document describes the system as it runs today. Plans belong in a design doc, and a design doc is a record of intent, so it stops being current once the code ships.

Cover what the system has, and delete the rest:

- Purpose and main constraint, in two sentences.
- Context: users, upstream and downstream systems, trust boundaries.
- Components: one sentence of responsibility each, main technology, path or owner.
- Two or three key flows as numbered paths with file references, each with its failure handling.
- Data: stores, key entities, ownership, retention, consistency.
- Deployment: environments, how a change ships, how it rolls back.
- Cross-cutting: auth, observability, scaling limits with numbers, cost drivers.
- Decisions, as links to ADRs, and known gaps.

Diagrams follow the C4 model, which is notation-independent. Its authors state that a system context diagram and a container diagram are enough for most teams, so draw those two and stop unless a reader needs more. Add a sequence diagram for a flow that crosses services. See `references/diagrams.md`.

For a large system going through formal review, use the arc42 section list as a completeness check, and keep only sections with project-specific content.

Evidence: take the component list from config, IaC, or the deployed service list, because an old diagram or README often shows services that no longer exist.

Done when: each component maps to a real path, each arrow has a real caller, and a new engineer can find where to make a change.

````markdown
# <System> architecture

Last verified: <YYYY-MM-DD> against <commit or version>.

<Purpose and main constraint in two sentences.>

## Context

<Users and systems that interact with it. Trust boundaries.>

## Components

| Component | Responsibility | Tech | Location |
|---|---|---|---|
| <name> | <one sentence> | <language, runtime, service> | `<path>` |

## Flow: <name of main flow>

1. <Actor> sends <what> to <component> (`<file:line>`).
2. <Component> validates <what> and writes <what> to <store>.

Failure handling: <what happens on error, retry, timeout>.

## Data

<Stores, key entities, ownership, retention, consistency.>

## Deployment

<Environments, regions, pipeline, rollback.>

## Cross-cutting concerns

| Concern | How it works here |
|---|---|
| Auth | <mechanism> |
| Observability | <logs, metrics, traces, where> |
| Limits and cost drivers | <numbers> |

## Decisions

- [<ADR title>](decisions/0003-<slug>.md)

## Known gaps

- <gap, with issue link if any>
````
