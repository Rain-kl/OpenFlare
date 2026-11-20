# Documentation brief

Edit any field, delete what does not apply, then tell the agent "go". Lines starting with `>` are guidance. Anything marked (inferred) was guessed from the repo, so check it.

## 1. Title and destination

- Title:
- Save to: (path or folder)

## 2. Readers

> Who reads this, and what will they do with it? One line per reader. Example: "on-call engineer, restarting the worker at night".

-

## 3. Scope

- In scope:
- Out of scope:
- Systems or repos covered:

## 4. Type or style

> Default is one overall technical document. Other choices: README, architecture (C4, arc42), HLD, LLD, design doc, ADR, API or SDK reference, data dictionary, runbook, handover document, test document, threat model, user guide, or any style you name.

- Type or style: overall technical document

## 5. Outline

> Choose one.
> A) You have a table of contents or template: paste it below and keep the line "Use exactly".
> B) You do not: the agent fills in a proposed outline. Edit it freely: add, remove, rename, reorder.

- Use exactly: yes / no
- Outline:

1.

## 6. Diagrams

> Choose one: I have diagrams (list the files) / draw new ones (say which, for example system context, one sequence) / none / you decide.

- Choice:
- Existing diagram files:

## 7. Leave out

> Topics, folders, services, or details that must not appear. Secrets are never copied.

-

## 8. Tone and house style

- Style guide or glossary to follow: (path or link, or "none")
- Tone: plain and direct

## 9. Output format

- Format: Markdown / Word / PDF / HTML
- Length target: (optional)

## 10. Facts only you know

> The agent will not guess these. Fill in what you can. Anything left blank becomes "To confirm" in the document.

- Owners and contacts:
- Support terms, SLAs, or response times:
- Environments and who can access them:
- Known gaps, open work, and technical debt:
- Decisions and their reasons the code does not show:

## 11. Working rules

- Scripts allowed (Python, pandoc): yes / no
- Commands the agent may run: reads, tests, and `--help` only / ask first / anything
- Review: show the outline first / go straight to a draft
