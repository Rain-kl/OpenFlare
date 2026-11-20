---
name: writing-technical-documentation
description: 'Writes, updates, audits, and exports technical documentation for software projects from evidence in the code: one overall technical document with a table of contents, or any single type such as README, architecture overview, HLD or LLD, design doc, ADR, API, SDK or CLI reference, data dictionary, runbook, handover document, test or threat-model document, postmortem, migration guide, docstrings, and AGENTS.md. Follows the developer''s own table of contents, template, or named style when one is given. Use when the user asks to prepare, write, or document a project, repo, system, API, decision, or incident, even in a vague request like "prepare the technical documentation for this project"; to check docs for stale or wrong claims; to make text sound less like AI; or to convert docs to or from Word, PDF, or HTML. Not for marketing copy, blog posts, or essays.'
---

# Writing technical documentation

Every sentence must carry a fact, a decision, an instruction, or a warning the reader can act on or check. AI-sounding prose comes from having no specific facts, so the order of work is: gather evidence, write only what it supports, then scan for leftover patterns. A sentence that would still be true of a different project is empty: give it a name, number, path, command, or failure mode from this project, or delete it.

This skill assists the developer. It does not impose a structure. The developer's table of contents, template, and style guide win over every default here.

## Pick the path

| The user wants | Path |
|---|---|
| Vague: "document this project", "prepare the technical documentation" | Overall document: `references/types/technical-document.md` |
| A named type or style (arc42, HLD, SRS, handover, runbook, and so on) | Find it in `references/doc-styles.md`, then use the type file it names |
| To use their own table of contents, template, or house style | Follow it exactly: `references/outlines.md` section 1 |
| One docstring, one section, a fix | Small edit: read the code it touches, change it, run the checks in stage 4 |
| To know what is wrong with existing docs | Audit, read-only: `references/audit-and-maintain.md` section 1 |
| Docs brought in line with a code change | Update: `references/audit-and-maintain.md` section 3 |
| Prose that sounds less like AI | Clean: see "Cleaning a draft" |
| Word, PDF, or HTML, or old files turned into Markdown | Convert: `references/pandoc.md` |
| Docs checks in CI | Suggest from `references/docs-ci.md`, and add only with approval |

## The flow

Five stages. The developer can skip or merge any of them, and a small edit needs only stages 2 to 4. Do not add gates of your own.

### 1. Intake

Infer what you can from the request and a quick look at the repo. Settle the type or style, the readers and what they will do with it, the scope, the outline, the diagrams, and where the document goes.

- If the request is vague, write `docs-brief.md` from `assets/templates/docs-brief.md`, next to where the document will go. Fill every field you can infer, propose an outline, and stop. Tell the developer to edit the file and say "go". This is the one stop in the flow.
- If the developer prefers chat, ask the same questions, at most three at a time.
- If the request already gives type, readers, and scope, skip the brief and state your assumptions in two lines.
- If nobody is there to answer, take the most reasonable reading, say which one at the top of your report, and continue.
- A table of contents, template, or style guide from the developer goes into the brief unchanged and is marked "use exactly".

### 2. Evidence

Read the repo in the order in `references/reading-order.md`, and stop each tier once you have what the document needs. Treat the README and old docs as claims to verify, never as facts.

For each fact you plan to use, keep a one-line note: the claim and where it came from (`path:line`, a command and its output, a test name, or the developer's answer). A fact you cannot source is not written as fact. Verify it, mark it `[To confirm: <what would settle it>]` in the text, or drop it. Collect every `To confirm` for the final report. Never fill a gap with a plausible guess. See `references/verification.md` for what counts as evidence and for the fuller ledger used in audits.

Stay read-only by default. Tests and `--help` are normally fine. Ask before installs, servers, migrations, or anything with side effects. Never copy secret values into notes or docs.

### 3. Draft

Take the outline from the brief. If there is none, use the default for the project kind in `references/outlines.md` and show it first when the document runs past a page. Read the type file for each chapter and `references/style-rules.md`. For explanations, design docs, and ADRs, read `references/senior-engineer-writing.md` too.

- Templates are starting points. Delete sections with no project-specific content. Never fill a section to satisfy a template, except where the developer's own outline requires the heading: then write one line saying why it does not apply.
- Answer first. Say what the document is, who it is for, and what to do or know.
- Give exact values: versions, defaults, limits, units, ports, variable names, error strings, exit codes.
- State decisions with the reason and the rejected alternative. Show what breaks and how it looks.
- Prefer the real thing to a description: a command with its output, a config excerpt with its path.
- Write timeless text. Use versions or dates in place of "currently", "new", or "soon".

### 4. Check

Facts first, then prose.

1. Re-derive every number, default, path, command, flag, and name from the source. Run what is safe to run. For the rest, check against `--help` or the code. Walk the setup path as the reader will, in the order written.
2. Read the prose against the patterns below and `references/slop-catalog.md`. Fix each by supplying the missing fact, because swapping one stock word for another keeps the emptiness. Check for restated points and for terms that change name between sections.
3. For anything over a page, write 5 to 10 questions a reader would ask and answer them from the document alone. Each wrong answer is a gap. Then cut every sentence that fails the test "what does the reader lose without this?".

Scripts can do parts of this faster. They are optional: see "Working without scripts".

### 5. Deliver

Confirm the path and filename unless given. Create new files. Do not restructure existing docs or edit source comments without approval. When updating a doc, change what is wrong or missing, keep the author's structure and voice, and summarize the diff. Close with these lines, dropping any that do not apply:

```
Wrote: <path> (<type, approximate length>)
Checked by: <what you ran or traced>
To confirm: <each unknown, with what would settle it>
Found in code or old docs: <problems worth the developer's attention>
```

## Outlines and styles

`references/doc-styles.md` lists about 35 documentation types and standards, and which type files build each. `references/outlines.md` has the rules for a developer's own table of contents and default outlines by project kind. A published standard is described in this skill's words: write "structured after <standard>", and do not claim conformance unless the developer asks for that check and has the standard text. For a style that is not listed, ask for a sample or template, or propose the closest entry. Never invent a named standard's section list.

## Diagrams

Ask at intake which case applies, and read `references/diagrams.md`.

- The developer has diagrams: embed them with a caption and alt text, check each against the code, and report mismatches. Do not redraw them.
- New diagrams are needed: write diagram-as-code, only where relationships are the point, with real names taken from the repo.
- No diagrams: write none, and describe structure in a table.

## Patterns to avoid while drafting

Models write these by default, so keep them in mind from the first sentence. The full catalog is `references/slop-catalog.md`.

<!-- slop-scan: off -->
1. Stock vocabulary: delve, tapestry, landscape, realm, testament, pivotal, crucial, robust, seamless, comprehensive, leverage, harness, showcase, underscore, foster, streamline, unlock, empower, ecosystem, journey, cutting-edge.
2. Negative parallelism: "It's not X, it's Y", "not just X but Y". State the true thing.
3. Self-answered questions and dramatic fragments.
4. Trailing participle filler: ", ensuring robust performance".
5. Significance inflation: "plays a crucial role", "marks a pivotal moment".
6. Copula avoidance: use "is" and "has" in place of "serves as", "stands as", "boasts".
7. Preamble and recap: "This guide covers", "In summary". Start with content and end when it ends.
8. Filler openers and hedge stacks: "It's worth noting", "can potentially help".
9. Vague attribution: "experts say", "industry best practice". Name the source or own the claim.
10. Rule of three by default. Use the number of items that exist.
11. Formatting tells: bold-lead bullets, bold in running prose, emojis, title case headings, em dashes, unicode arrows, curly quotes.
12. Synonym cycling. One name per concept, and use the code's name.
13. Unproven adjectives: fast, scalable, secure, elegant, very. Give the number or the mechanism.
14. Condescension: simply, just, easy, obviously.
15. Leaked assistant text: "I hope this helps", "Would you like me to".
16. Things doing human verbs ("the config decides"), analogies in place of mechanisms, and tie-back endings.
<!-- slop-scan: on -->

Do not over-police. Correct grammar, a formal register, a table, or one transition word is not a defect. Clusters are the signal.

## Working without scripts

The skill works with plain file reading and search. Use the scripts only if Python is available and the developer has not said otherwise. Each task below has a by-hand equivalent.

| Script | By hand |
|---|---|
| `scripts/repo_inventory.py` | List the repo and read in the order of `references/reading-order.md` |
| `scripts/verify_refs.py` | Search the repo for each path, command, flag, and name the draft cites |
| `scripts/slop_scan.py` | Read the draft against the patterns above |
| `scripts/doc_audit.py` | Follow `references/audit-and-maintain.md` section 1 |
| `scripts/export_doc.py` | Run pandoc as in `references/pandoc.md` |

Paths are relative to this skill's folder. From a project, run `python3 <skill-dir>/scripts/repo_inventory.py .`. The scripts use only the standard library, are read-only unless a script says otherwise, and explain themselves with `--help`.

## Gotchas

- Test the reader's path, not your shortcut. A scratch database or a variable you set yourself hides a broken quick start. Follow the steps in order, using the config file the doc tells the reader to copy.
- A date or "last verified" line means you ran it. If you only read the code, say `not run; read against <commit>` and name the commands that did not run.
- The README is usually the least reliable file in the repo. Check its ports, commands, variables, and service names against code, and report each contradiction, even when the user asked only for a rewrite.
- Finding a name in a doc is not evidence. A second document repeating a claim is the same claim.
- Example output must be captured. If you cannot run the command, label the block illustrative or leave the output out.
- Runbooks, handover documents, SECURITY files, and onboarding guides tempt you to invent owners, channels, dashboards, SLAs, and contacts. Mark the gap and ask: a false contact costs more than a blank.
- Reverse-engineered requirements are observed behavior, not what the system was required to do. Label them so.
- Never paste secrets. Name variables and leave values out, and tell the developer if you find a committed secret.
- Say which directory each command runs from.
- Get today's date from the system before writing a date.
- Keep your process out of the document. "Checked by" and `To confirm` go in the report. The document keeps marked gaps and one last-verified line.
- A low `slop_scan` score does not make a document good. Never accuse an author of using AI.

## Push back or limit scope

- If the code does not support a claim the user wants documented, say so and document what is true.
- If the repo has no tests, no clear entry point, or contradicting config, record that as a finding.
- A document for one reader and one purpose beats one for everyone. If the brief lists several readers with different needs, propose separate chapters or documents.

## Cleaning a draft

For "make this sound less like AI", or editing text someone wrote or generated:

1. Read the draft against the patterns and `references/slop-catalog.md`, or run `scripts/slop_scan.py` on it. If there is a repo, check facts against it too.
2. Find the vague claim behind each pattern. Ask for missing facts when the repo cannot supply them. Never invent them.
3. Rewrite with specifics. Keep the meaning and every supported claim, keep the voice where it is human, and leave quotations, names, and identifiers alone.
4. Show before and after for the largest changes, and list the facts still missing.

## Reference files

Read a file when its trigger applies. Do not read them all.

| File | Read it when |
|---|---|
| `references/doc-styles.md` | The developer names a style or standard, or asks which fits |
| `references/outlines.md` | Building the outline; the developer supplies a table of contents or template |
| `references/reading-order.md` | Stage 2; unfamiliar stack, monorepo, AI or AWS project; deciding what to skip |
| `references/verification.md` | Stages 2 and 4; deciding what you may run; audits |
| `references/style-rules.md` | Drafting; procedures, code samples, headings, placeholders |
| `references/senior-engineer-writing.md` | Any explanation, design doc, or ADR; before-and-after rewrites |
| `references/slop-catalog.md` | Stage 4; cleaning a draft; auditing prose |
| `references/diagrams.md` | Embedding or adding a diagram; Mermaid syntax and alt text |
| `references/audit-and-maintain.md` | Auditing, planning a doc set, updating for a code change, pruning |
| `references/pandoc.md` | Word, PDF, or HTML output; importing old documents |
| `references/docs-ci.md` | A team wants docs checks in CI |

Document types. Read the one you are writing:

| Type | File |
|---|---|
| Overall technical document | `references/types/technical-document.md` |
| README | `references/types/readme.md` |
| Architecture overview | `references/types/architecture.md` |
| Design doc, RFC, HLD, LLD | `references/types/design-doc.md` |
| ADR | `references/types/adr.md` |
| Runbook | `references/types/runbook.md` |
| API reference | `references/types/api-reference.md` |
| CLI reference | `references/types/cli-reference.md` |
| Onboarding guide | `references/types/onboarding.md` |
| Tutorial | `references/types/tutorial.md` |
| How-to guide | `references/types/how-to.md` |
| Explanation or concept page | `references/types/explanation.md` |
| Troubleshooting guide | `references/types/troubleshooting.md` |
| Changelog, release notes | `references/types/changelog.md` |
| Migration or upgrade guide | `references/types/migration.md` |
| Postmortem | `references/types/postmortem.md` |
| CONTRIBUTING, SECURITY | `references/types/contributing-security.md` |
| Glossary | `references/types/glossary.md` |
| Docstrings, code comments | `references/types/docstrings.md` |
| AGENTS.md, CLAUDE.md, llms.txt | `references/types/agent-instructions.md` |
| Model card, LLM or agent flow | `references/types/ai-systems.md` |
| Infrastructure and operations | `references/types/infrastructure.md` |

Scripts, all optional: `scripts/repo_inventory.py` lists a repo in reading order. `scripts/verify_refs.py` checks a doc against a repo. `scripts/slop_scan.py` scans prose for AI patterns. `scripts/doc_audit.py` audits a doc set or lists docs affected by a diff. `scripts/export_doc.py` converts with pandoc. Assets: `assets/templates/docs-brief.md`, `assets/pandoc/`, `assets/ci/`.
