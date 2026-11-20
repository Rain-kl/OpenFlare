# Docs for coding agents: AGENTS.md, CLAUDE.md, llms.txt, and retrieval

## AGENTS.md and CLAUDE.md

Measured evidence favors less. Gloaguen and colleagues (arXiv 2602.11988, ICLR 2026) tested coding agents on real tasks and found that context files generally did not improve success rates and raised inference cost by over 20 percent on average. Agents followed the instructions well, and repository overviews did not help them find files. The authors conclude these files are useful for non-standard practices, and they advise human-written files with minimal requirements. Every line adds a requirement the agent will obey at a cost.

So, in this order of value:

1. Exact commands the agent cannot guess, with flags, including how to run one test. Agents use explicitly named tools far more when a file names them.
2. Non-standard conventions that tooling does not enforce, each with a short reason.
3. Boundaries in three tiers, from GitHub's analysis of over 2,500 files: always do, ask first, never do. Name the files and directories to leave alone. Never commit secrets.
4. A small real code snippet when a convention is hard to describe. One example beats paragraphs.
5. Pointers to deeper docs, with when to read them.

Leave out: repository overviews and directory maps (the study found they do not help, so name only a location an agent would not find by search), anything in the README, anything a linter enforces, generic advice, and personas such as "you are a helpful assistant". Do not generate this file from the codebase and commit it unedited: the study found generated files redundant with existing docs. Before writing a line, ask whether an agent would get this wrong without it. Grow the file from real mistakes, and delete lines that stop mattering.

Mechanics: AGENTS.md is plain Markdown with no required fields, and the nearest file to the edited code wins, so a monorepo can nest them. Chat instructions override all of them. CLAUDE.md is the same idea for Claude Code, so one file can point to the other.

```markdown
# <Project>

## Commands

- Test all: `<cmd>`
- Test one: `<cmd> <path>::<name>`
- Lint and format: `<cmd>`

## Conventions

- <non-obvious rule tooling does not enforce>, because <reason>.

## Boundaries

- Always: <rule>
- Ask first: <schema changes, new dependencies>
- Never: <touch these paths>, <commit secrets>

## More

- Read `docs/architecture.md` before changing <area>.
```

## llms.txt

A Markdown index that helps a model find the right page. Order: H1 with the project name (the only required part), a blockquote summary, optional detail without headings, then H2 sections of links as `- [name](url): notes`. An H2 named `Optional` holds links an agent can skip. Link to clean Markdown versions of pages, at the same URL with `.md` added. Keep it small enough to fit in context, and test it by asking an agent questions with only this file as a starting point.

## Docs that a retrieval system will index

For a knowledge base feeding RAG, each chunk gets read alone, so write sections that stand alone:

- Use clear headings and split large documents into smaller ones with descriptive titles.
- Repeat the subject in each section. "It" and "as described above" mean nothing in isolation.
- Define abbreviations and company terms, because models lack your internal context.
- Describe the content of diagrams in text.

AWS's RAG guidance also recommends a short summary after each heading and replacing tables with flat lists. Apply those two only when retrieval is the document's main use, since for human readers they conflict with this skill's rules against fractal summaries and for tables. If a document serves both, keep the table and state its key facts in a sentence too.
