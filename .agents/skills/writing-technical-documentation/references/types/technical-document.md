# Overall technical document

Reader: the people who take the system over or review it: maintainers, another team's engineers, technical reviewers. This is the default when someone asks to document a project without naming a type. It is one document with a table of contents that covers the system as it runs today.

How to build it:

1. Settle the outline first: the developer's own, or the default for the project kind in `references/outlines.md`. Cut chapters with no project-specific content.
2. Build each chapter from the type file that matches it. Do not read all of them. Read only the ones the outline uses.
3. Link between chapters in place of repeating a fact. Each repeat is a future contradiction.
4. Keep chapters at the depth a reader needs to act. Move long reference tables to an appendix.

Which type file feeds which chapter:

| Chapter | Build from |
|---|---|
| Overview, quickstart | `references/types/readme.md` |
| Architecture, context, components, flows | `references/types/architecture.md` |
| Design choices and rejected alternatives | `references/types/design-doc.md`, `references/types/adr.md` |
| Interfaces and endpoints | `references/types/api-reference.md` |
| Commands and flags | `references/types/cli-reference.md` |
| Build, deploy, environments, operations | `references/types/infrastructure.md` |
| Failure handling and recovery | `references/types/runbook.md`, `references/types/troubleshooting.md` |
| Models, prompts, evaluation | `references/types/ai-systems.md` |
| Setup for a new maintainer | `references/types/onboarding.md` |
| Upgrades and breaking changes | `references/types/migration.md` |
| Terms | `references/types/glossary.md` |

For chapters that have no type file (data dictionary, security, testing, handover), use the outline in `references/doc-styles.md`.

Evidence: take the chapter list from what exists in the repo, not from the old README. A chapter that the code cannot support is cut or marked `To confirm`, never filled.

Done when: every chapter has a source for each fact, every `To confirm` is listed at the end, and a new maintainer can find where to make a change and how to ship it.

````markdown
# <System name>: technical documentation

Last verified: <YYYY-MM-DD> against <commit or version>, or "not run; read against <commit>".

## Contents

<Generated or written list of chapters, matching the outline.>

## 1. Overview

<What it does, who uses it, the main constraint. Two or three sentences.>

## 2. Architecture

<Context, components table, one or two key flows with file references, diagram if relationships are the point.>

## <Chapters from the outline>

## Known gaps

<Verified problems: dead code, drift, missing tests, missing monitoring.>

## To confirm

| # | Item | What would settle it | Who may know |
|---|---|---|---|
| 1 | <unknown> | <file, person, or command> | <role, if the developer named one> |

## Glossary
````
