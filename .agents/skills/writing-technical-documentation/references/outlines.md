# Outlines and tables of contents

Read this when building the outline of a document, or when the developer supplies a table of contents, template, or house style.

Contents
1. When the developer supplies their own outline
2. When there is none: propose and let them edit
3. Default outlines by project kind
4. Merging, splitting, and numbering

## 1. When the developer supplies their own outline

The developer's outline is the specification. Follow it as given.

- Keep their headings, their order, and their numbering. Do not rename, merge, split, or reorder. Do not add top-level headings.
- Do not drop a section because the repo has nothing for it. Write what the evidence supports, and mark the rest `To confirm` with what would settle it. If the section truly does not apply, write one line saying why.
- If the outline has a section the code cannot answer (business case, schedule, staffing), do not invent it. Ask for the content, or mark it `To confirm`.
- Content that fits no heading goes under the closest one, or in your report. Suggest a new heading in the report and leave the outline unchanged.
- A template with placeholder text: replace the placeholders, keep the rest. Delete example text that was left in the template.
- A house style guide (terms, voice, headings in sentence case or title case) overrides this skill's style rules. Where they differ, follow the guide and note the difference once in the report.
- If the outline has an obvious problem (a duplicate heading, a chapter the evidence contradicts), say so in the report. Do not fix it silently.
- Reflect the structure in the table of contents exactly. Numbering in the outline stays numbering in the document.

## 2. When there is none: propose and let them edit

1. Choose the default outline for the project kind below, or the section list of the style the developer named in `references/doc-styles.md`.
2. Cut chapters with no project-specific content before showing it. A short document with the chapters that apply beats a long one that fills all of them.
3. Put the outline in the brief (`assets/templates/docs-brief.md`, field 5). One line per chapter: the title, and the evidence that fills it.
4. The developer edits it. Their edit is now their own outline, and section 1 applies.
5. For a document under a page, skip the proposal and write.

Never present the default as required. Say that it is a starting point.

## 3. Default outlines by project kind

Each chapter lists what fills it. Drop any chapter the repo cannot support. These are for the overall technical document: see `references/types/technical-document.md`.

### Web service or API

1. Overview: purpose, users, main constraint (README, entry points)
2. Architecture: context and components (config, IaC, imports)
3. Interfaces: endpoints, events, and authentication (route definitions, OpenAPI file)
4. Data: stores, entities, retention (migrations, models)
5. Configuration: settings and environment variables (code that reads them)
6. Build, run, and deploy (build files, CI, deploy config)
7. Operations: monitoring, scaling limits, backup, common failures (config, alert rules, runbooks)
8. Security: authentication, authorization, secrets handling (middleware, config)
9. Testing: what is covered and how to run it (tests, CI)
10. Known gaps and decisions (issues, ADRs, `To confirm` list)
11. Glossary

### Library or SDK

1. Overview and supported platforms and versions (manifest, CI matrix)
2. Install and quickstart that produces a visible result
3. Concepts: the main types and how they fit (public API)
4. Task guides for common jobs (examples, tests)
5. Reference: public API, generated if possible
6. Error handling and retries
7. Configuration
8. Versioning and upgrade notes (changelog, tags)
9. Contributing and testing

### CLI tool

1. Overview and install
2. Quickstart with real output
3. Commands and flags: synopsis, examples first (`--help`, argument parser)
4. Configuration files, environment variables, precedence
5. Exit codes and errors
6. Recipes
7. Development: build, test, release

### Data or ML pipeline

1. Overview: what goes in, what comes out, who uses it
2. Architecture and data flow (orchestration config, entry points)
3. Sources and schemas (loaders, schema files)
4. Transformations and quality checks (pipeline code, tests)
5. Models and artifacts: versions, storage, reproduction steps (training scripts, config)
6. Evaluation: datasets, metrics, thresholds, dates (evaluation scripts, CI gates)
7. Scheduling, backfill, and failure handling (scheduler config)
8. Operations and cost drivers
9. Data dictionary (schemas, migrations)
10. Known issues

### AI or LLM application

1. Overview and intended use
2. Architecture: components, model providers, data flow
3. Prompts, tools, and agent flow (prompt files, tool definitions, graph code)
4. Retrieval and data sources (index config, loaders)
5. Evaluation and guardrails, with numbers and dates (eval scripts, CI)
6. Configuration, cost, and limits (model IDs, token limits, quotas)
7. Deployment and operations
8. Security and data handling
9. Known failure cases and limits

For model cards and agent flows, also use `references/types/ai-systems.md`.

### Infrastructure repository

1. Overview: what it provisions, which environments
2. Topology and diagram (IaC files)
3. Modules and variables (module inputs and outputs)
4. How a change ships and rolls back (pipeline config)
5. Access and permissions as roles, never credentials
6. Backup, restore, and disaster recovery (config, scripts)
7. Monitoring and alerts
8. Routine operations and runbooks
9. Cost drivers

### Monorepo

1. Overview and a map of packages: one row each with purpose, owner path, and entry point
2. Shared tooling and build
3. Cross-package architecture and dependencies (workspace config, imports)
4. One chapter per package that needs more than its own README, following the kind above
5. Release and versioning
6. Contributing and testing

## 4. Merging, splitting, and numbering

- One document, one reader and one purpose. If the brief lists readers with different needs, such as operators and API users, split into parts with their own openings, or into separate documents that link to each other.
- A document over about 30 pages usually wants a short summary document and a separate reference. Ask before splitting.
- Number headings only when the developer's outline does, or when the document will be cited by section.
- Put a table of contents at the top of any document with more than five headings or longer than a few screens. Skip it on shorter ones.
- Keep chapter titles plain: the thing or the task ("Retry behavior", "Deploy to staging"). Avoid "What is" and "Why" headings.
