# Audit, plan, and maintain a documentation set

Read this when asked to review or audit docs, to plan what a repo needs, to update docs for a code change, or to set up upkeep.

Contents

1. Audit existing docs
2. Plan a documentation set
3. Update docs for a code change
4. Prune and maintain

## 1. Audit existing docs

An audit reports. It does not change files, so wait for approval before fixing anything.

Run the script, then use its findings as leads:

```bash
python3 scripts/doc_audit.py <repo-path>
python3 scripts/doc_audit.py <repo-path> --json > /tmp/audit.json
```

It reports broken references, AI-writing scores, stale and drifting docs, repeated passages, and document types the repo's contents call for. It only checks that references resolve, so it cannot tell whether a claim is true. That needs the second step.

Then verify claims by hand for the docs that matter most: the README, setup steps, and anything a reader will act on in an incident. Pick the facts most likely to rot and check them against code: ports, env vars, commands, versions, defaults, service names, flow descriptions. Each false claim you confirm is a finding, with both sides quoted.

Assess each doc on what DORA's research measures, which is whether readers find it clear, findable, and reliable:

| Dimension | Question | Evidence |
|---|---|---|
| Reliable | Is it true and current? | Claims checked against code, last commit date, drift |
| Clear | Can the reader act on it? | One type per page (the Diataxis compass), first screen, steps with expected results |
| Findable | Will the reader get here? | Linked from the README, descriptive titles, one canonical page per topic |

Report in this form, most severe first, and rank by what would hurt a reader. A wrong command in the quick start outranks a hundred style notes. Do not lead with AI-pattern scores, and do not accuse an author of using AI. Describe a prose problem plainly ("marketing language with no facts"), and leave tool names and scores out of a report that will reach a client. Report what you notice beyond the docs when it affects the reader, such as a committed secret or a missing test suite, and label it as outside the docs.

```markdown
# Documentation audit: <repo> (<YYYY-MM-DD>, commit <hash>)

<Two sentences: overall state, and the one thing to fix first.>

## Findings

| # | Severity | Doc | Finding | Evidence | Fix |
|---|---|---|---|---|---|
| 1 | Blocker | `README.md:12` | Says port 8080; the server binds 8081 | `src/server.py:14` | Change to 8081 |

Severity: Blocker (a reader following it fails or is misled), Major (a wrong or missing fact with a workaround), Minor (style, thin prose).

## Missing documents

<Each with the repo evidence that calls for it, in priority order.>

## Not checked

<What you could not verify, and why.>

## Suggested order of work

1. <One change at a time, smallest high-value first.>
```

## 2. Plan a documentation set

A small set of correct docs beats a large set that drifts, so propose the minimum and let structure grow. Diataxis warns against creating empty tutorial, how-to, reference, and explanation sections up front: improve one page, publish it, and repeat.

Default order for a repo with little documentation:

1. README, as the front door.
2. Setup that works, in the README or an onboarding guide.
3. Architecture overview, if there are several deployable parts.
4. A runbook for the riskiest production flow, if it runs in production.
5. Reference generated from contracts: OpenAPI, CLI help, schemas, type annotations.
6. ADRs from this point on, for new decisions.

If a request is "document everything" or "prepare the technical documentation", the default is one overall technical document: `references/types/technical-document.md`. Offer this order only when the developer wants a doc set. Build one document at a time and show each before starting the next.

Layout, once there are several pages:

- One page, one type, one purpose. Link instead of repeating, because each repeated fact is a future contradiction. Accept small repeats a reader needs in place.
- Keep docs near the code they describe, so one change updates both. A monorepo package gets its own README.
- A landing page is navigation: a clear purpose and links, not several screens of content.
- Link text names the destination, as in "see the retry configuration reference". Link to existing guides for common technology.

## 3. Update docs for a code change

Change docs in the same pull request as the code. It keeps docs current and gives reviewers context.

```bash
python3 scripts/doc_audit.py <repo-path> --changed-since origin/main
```

This lists docs that mention a changed file, and docs that still mention a name the diff removed. Then:

1. Read the diff, and find behavior users see: defaults, flags, errors, routes, env vars, setup steps.
2. Fix only what changed. Keep the author's structure and voice, and show a short diff summary.
3. Add a changelog entry if the project keeps one.
4. Run `verify_refs.py` on each changed doc, and run the commands you changed.

## 4. Prune and maintain

Outdated docs are worse than missing ones: they mislead readers and waste time. Google's guidance for a neglected set is to improve it gradually, delete what is clearly wrong first, and when in doubt delete, since version control can bring a page back. Ask before deleting anything.

- Give each doc an owner, and track doc bugs where code bugs live.
- Add `last_verified` and `verified_against` front matter to runbooks, setup guides, and architecture overviews. `doc_audit.py` flags ones older than 180 days.
- Generate what can be generated, and make examples executable, so they fail in CI when they drift.
- Consolidate competing pages into one, and redirect or mark the rest as replaced.
- After shipping, mark design docs as records of a decision, so readers do not take them as current behavior.
- Suggest docs checks in CI when the team wants them: see `references/docs-ci.md`.

To measure over time, DORA's approach is a short survey asking whether docs are clear, findable, and reliable. A useful task metric is time from clean clone to first passing test.
