# Verification

Contents
1. Why verification is the main defense
2. Evidence by claim type
3. What you may run
4. Hallucination checklist
5. Evidence notes and the ledger
6. Marking unknowns: To confirm
7. Links and references
8. Staleness and drift
9. Reader test and reviews
10. Final verification report
11. Working without scripts

## 1. Why verification is the main defense

AI-written documentation fails in two ways: it sounds generic, and it states things that are false with full confidence. The second does more damage. A fluent wrong flag name costs a reader an hour. Verify before polishing. A well-polished wrong sentence is worse than a rough true one.

## 2. Evidence by claim type

| Claim | Acceptable evidence | Not acceptable |
|---|---|---|
| Command works | Ran it, saw output; or source of the CLI definition plus `--help` | Memory, similar tools |
| Flag, option, parameter | CLI definition, `--help`, schema, function signature | Docs of a different version |
| File or path exists | `ls`, glob, the inventory script | Assumed convention |
| Function, class, endpoint | grep result with file and line | Name that sounds right |
| Default value | Config schema or code line | README |
| Version, runtime | Manifest, CI config, lockfile for exact resolved version | "Latest" |
| Env var | Code reading it, or `.env.example` | Naming convention |
| Behavior | Code path trace, test, or run | Reading a function name |
| Performance number | Benchmark output with date and setup | Estimate presented as fact |
| Cost number | Pricing page seen today, with date and region | Memory |
| Security property | Code and config evidence for each part | "Follows best practices" |
| Dependency behavior | Official docs for the pinned version | Training memory |
| Architecture diagram | Config, IaC, or deployed service list | Old diagram |
| Quote or citation | The source text seen | Paraphrase presented as quote |
| URL | Fetched and seen | Constructed from a pattern |
| Ownership, SLA, contact | The user or an owner file | Invented |
| Date or history | Git log, PR, ADR | Guess |

## 3. What you may run

`scripts/verify_refs.py <doc> --repo <repo> --commands` lists a draft's shell commands with a risk class: `safe`, `ask`, or `never`. It is a first pass, so read the commands yourself too.

Normally safe without asking: `ls`, `cat`, grep, `git log`/`blame`/`show`, `--help`, `--version`, listing tests, linting, type-checking, running unit tests that do not touch external systems, running the doc scripts in this skill.

Ask first: installs, builds that download artifacts, starting servers or containers, running integration tests, any command that contacts external systems, writes outside scratch space, or uses credentials, migrations, scripts of unknown effect, anything that costs money (cloud APIs, LLM calls at scale).

Never: run commands that delete, deploy, publish, or modify production; use secrets you found; paste secret values into output.

Say in the final report what you ran and in what environment. If you could not run something, say so and mark dependent claims `unverified`. A command that passes on your machine can still depend on state you set up earlier, so the setup steps in a README need the cold-start test in section 9.

## 4. Hallucination checklist

Check each of these in every draft:
- Invented flags, options, subcommands, config keys, env vars
- Invented functions, classes, modules, files, directories
- Packages that do not exist, or exist under a different name (typosquat risk when suggesting installs)
- Wrong versions, wrong default values, wrong units
- Endpoints, status codes, or error messages that do not appear in code
- Features from a similar tool attributed to this one
- Behavior from a different version
- Example output that was written from imagination, not captured
- Fabricated statistics, benchmarks, dates, quotes, citations, URLs
- Fabricated ownership, contacts, SLAs, compliance claims (SOC 2, HIPAA, GDPR)
- Claims about security or reliability without mechanisms
- Described architecture that does not match the deployed one
- Steps that skip a prerequisite you had set up on your machine
- Planned work described as shipped

## 5. Evidence notes and the ledger

Default, for most documents: a light note per fact, kept in your scratch space outside the repo. One line each: the claim, and its source (`path:line`, command and output, test name, or the developer's answer). Anything with no source becomes a `To confirm` item, section 6.

```
Server listens on 8081 | src/server.py:14
Worker retries 3 times | src/client.py:52 (MAX_RETRIES = 3)
p99 under 50 ms | no source: To confirm
```

Use the full ledger below for audits, handovers, and documents others will rely on without reading code. It adds a status per row.

Keep a scratch file outside the repo:

```
| # | Claim | Evidence | Status |
|---|---|---|---|
| 1 | Service listens on port 8080 | src/server.ts:14 | verified |
| 2 | Retries 3 times | src/client.ts:52 (MAX_RETRIES = 3) | verified |
| 3 | Uses exponential backoff | inferred from delay calc at client.ts:58 | inferred |
| 4 | p99 under 50 ms | README only | unverified |
```

Only verified claims appear as plain statements. Inferred claims use careful wording once. Unverified claims are verified, marked, or removed.

Check the ledger before drafting, because an invented citation is as bad as an invented fact. This confirms that each cited `path:line` exists, and flags `verified` rows that cite nothing:

```bash
python3 scripts/verify_refs.py <draft.md> --repo <repo> --ledger <ledger.md>
```

## 6. Marking unknowns: To confirm

Inline: `[To confirm: <what would settle it>]`. The older form `[unverified: ...]` means the same and the scripts accept both. Keep them few and specific. Collect them in the final report. A doc delivered for publication should have none, or each should be agreed with the user. Do not use hedge words as a substitute for marking.

## 7. Links and references

- `scripts/verify_refs.py` checks relative links, anchors, repo paths and `path:line` citations, code identifiers, env vars, flags passed to local scripts, and make, just, task, npm, and compose targets. It searches non-doc files, so a doc cannot vouch for itself.
- It only shows that a name exists. It cannot show that a sentence about that name is true, so a clean run is necessary and not sufficient.
- For external URLs: `--check-urls` fetches each link, or use your web tools and confirm the content matches the claim. Report broken or unverifiable links. Never include a URL you did not see.
- For citations: confirm title, author, and date from the source. If you cannot, cite less.

## 8. Staleness and drift

- Add `last_verified: YYYY-MM-DD` and `verified_against: <commit or version>` front matter to runbooks, setup guides, and architecture overviews. Get the date from the system, not from memory. The date claims a run, so if you only read the code, write `last_verified: not run; read against <commit>`.
- Prefer generated reference (OpenAPI, CLI help dumps, schema docs, TypeDoc, pdoc) over hand-copied tables.
- Prefer executable docs: doctests, tested code blocks, notebooks that run in CI.
- Say what rots fastest in the final report: version numbers, screenshots, step lists, diagrams, hardcoded names.
- For drift checks and pruning, see `references/audit-and-maintain.md`.

## 9. Reader test and reviews

The author is the worst judge of a document, because they already know what it means. Run these in proportion to the stakes.

Reader test, for anything longer than a page:

1. Write 5 to 10 questions a real reader of this document would ask, including "what do I run first?" and "what happens when this fails?"
2. Answer them using only the document, with no memory of the code. If your environment has subagents and the user is fine with the extra cost, give a fresh one only the draft and the questions, because it has no context to leak. Otherwise switch roles deliberately and read only the doc.
3. Ask the reader to list ambiguities, unstated assumptions, and contradictions.
4. Every wrong or missing answer is a gap in the document. Fix it and repeat until the answers are right and no new gaps appear.

Cold-start test, for setup steps and tutorials. Ask first, because this runs installs:

```bash
git clone --depth 1 <repo> /tmp/doc-check && cd /tmp/doc-check
```

Follow the document literally and fix each step that assumed something on your machine. Use what the document tells the reader to use, not a scratch database or variables you set yourself, because those hide the failures this test exists to find. If you cannot run it, trace each step against the code, say so, and mark the setup steps unverified.

Second-pass check, for high-stakes documents such as production runbooks, security pages, and public API reference. A reviewer who did not write the draft picks 15 claims, including every command, number, and path on the first screen, verifies each, and reports failures with evidence. The reviewer does not rewrite, and the author fixes the findings.

Three kinds of review catch different faults, and even one reviewer beats none: technical (is it accurate), audience (is it clear to someone unfamiliar), and writing (is it consistent). Scale rigor to how widely the document will be read.

## 10. Final verification report

Short, in the closing message:
- Verified by: commands run and files traced
- Unverified: each item with what would confirm it
- Not run, and why
- Drift found: old doc claims that the code contradicts
- Fast-rotting content: what to regenerate or retest, and when

## 11. Working without scripts

Every check here can be done by reading and searching.

- Paths and names: search the repo for each one the draft cites. No match means it is wrong or must be marked.
- Commands and flags: read the argument parser or `--help`. Run only what section 3 allows.
- Links: open the file or fetch the page. Never include a URL you did not see.
- Numbers and defaults: read the line of code or config that sets them.
- Prose: read the draft once against the patterns in `references/slop-catalog.md`, then again for terms that change name between sections.

Say in the report which checks you did by hand and which you skipped.
