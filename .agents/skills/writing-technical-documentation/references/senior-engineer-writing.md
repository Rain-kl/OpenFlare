# Writing like a senior engineer

Contents
1. What senior engineers do differently
2. Answer first
3. Claims need evidence
4. Decisions, trade-offs, alternatives
5. Numbers and precision
6. Say what you do not know
7. Terms and consistency
8. Explaining mechanisms
9. Examples that earn their place
10. Diagrams
11. Structure at document level
12. Reviewing and being reviewed
13. Before and after rewrites
14. Worked example: a README opening
15. Worked example: an ADR paragraph
16. Quality tests
17. Common failure modes

## 1. What senior engineers do differently

- They write for a specific reader with a specific next action, and they know what that action is.
- They lead with the conclusion, the decision, or the command.
- They are specific: names, numbers, paths, error strings, versions.
- They explain why, with the constraint that forced the choice, and what they rejected.
- They state what they do not know and how to find out.
- They document the sharp edges and failure modes, not only the happy path.
- They keep documents short because they respect the reader's time, and they cut before they add.
- They use plain words. Complexity is in the system, not in the sentences.
- They do not sell. Documentation informs; it does not persuade the reader the work is good.
- They treat docs like code: one source of truth, reviewed, tested, versioned, deleted when wrong.
- They choose the right place for each fact. Google's guidance runs from meaningful names, to comments for what code cannot say, to method docs for the contract, to a README for a directory's purpose, to `docs/` for setup and release, to design docs as records of decisions.

## 2. Answer first

Put the answer in the first one or two sentences. Then give supporting detail in descending order of importance. A busy reader who stops after the first paragraph should still have the main point.

Two readers arrive at every page. A seeker knows what they want and scans, so keep structure consistent. A stumbler arrived from search or a link and wonders if this page applies to them, so the opening must say who it is for and who it is not for. For anything likely to be read out of context, the first paragraphs should settle who, what, when (a date or version), where, and why.

<!-- slop-scan: off -->
Weak: "This document provides an overview of the authentication system and its various components, and discusses design considerations."
<!-- slop-scan: on -->

Strong: "Requests are authenticated with short-lived JWTs issued by the `auth` service (15-minute expiry) and verified at the gateway. Refresh tokens live in Redis for 30 days."

For decisions: state the decision, then the reason, then the alternatives. For procedures: state the goal and prerequisites, then steps. For incidents: impact, cause, fix, then timeline.

## 3. Claims need evidence

For each factual claim, be able to point to the line, the output, or the test. Prefer to include that pointer in the doc when the reader may want to check it ("see `src/retry.py:41`").

Separate three kinds of statements:
- Observed: you saw it in code or output. State plainly.
- Inferred: you reasoned from the code. Say "the code suggests" or "appears to" once, and say what would confirm it.
- Unknown: you could not tell. Say so and name what is needed.

Adjectives are claims. "Fast", "scalable", "secure", "reliable", "simple", "lightweight" need a number, a mechanism, or a comparison, or they go.

## 4. Decisions, trade-offs, alternatives

A decision record has four parts: the situation, the choice, what it cost, and what else was possible. Name the constraint that decided it (team size, deadline, existing system, compliance, latency target). Name what gets harder. "We chose DynamoDB over Postgres because access is by key only, p99 must stay under 20 ms at 5k requests per second, and the team does not run databases. Cost: no ad hoc joins; reporting goes through a nightly export to S3."

Do not present a trade-off as a win-win. If there is no downside listed, the analysis is incomplete.

Non-goals prevent scope fights. Write them as things a reasonable reader might assume are in scope.

## 5. Numbers and precision

- Use the number that exists: "retries 3 times with exponential backoff starting at 200 ms, capped at 5 s", not "retries a few times".
- Use units always. Name the percentile, the sample, and the date for measurements.
- Derive numbers from source; do not do mental arithmetic you have not checked. Run the calculation.
- Do not give false precision. If you estimate, say it is an estimate and how.
- Keep defaults and limits in one place and link to them.
- If a number may change, say where it is defined (`config/default.yaml`, key `timeout_ms`).

## 6. Say what you do not know

Honest gaps are more useful than smooth text. Mark with a consistent convention, for example `[unverified: needs a staging run to confirm the timeout]`, and collect them in the closing report. Never fill a gap with a plausible guess. Never invent a flag, endpoint, version, quote, owner, SLA, or link. If the reader is going to rely on a sentence, it must be true.

## 7. Terms and consistency

Run a term inventory before finishing: list every noun that names a thing in the system (service, queue, job, record, user type). For each concept, one name. If the code calls it `Order` and the doc says "purchase", "transaction", and "order", pick `order`. Use the code's casing for identifiers in code format and normal words in prose. Define a term when first needed. Avoid the same word meaning two things.

Do not invent labels for ideas ("the Trust Triangle", "the three pillars") unless the project already uses them. Invented named concepts are a slop signal and a maintenance burden.

## 8. Explaining mechanisms

Explain how something works by following the data or control. Use verbs with actors: "The scheduler enqueues one message per tenant. The worker polls the queue, locks the tenant row, runs the sync, and writes the result to `sync_runs`." Then add the failure path: "If the worker dies mid-run, the lock expires after 10 minutes and another worker retries."

Prefer concrete nouns over abstractions ("a Lambda function triggered by an SQS queue", not "an event-driven component"). Name the file or service.

Explain at the level the reader needs. A reference page does not explain what HTTP is. An onboarding page may.

## 9. Examples that earn their place

An example should be the smallest real case that shows the thing, tested, with expected output. Good: one common case, one edge case, one failure. Bad: five variations of the same call, toy domains with no relation to the project, examples that cannot run.

Put the example near the sentence it supports. Say what to notice in it, in one line, if it is not obvious.

## 10. Diagrams

Use a diagram when the relationships are the point: components and data flow, a sequence across services, a state machine, a deployment topology. Do not draw a diagram of a list. Rules:
- Real names from the code. Arrows mean one thing and are labeled with what flows.
- One question per diagram. Split rather than shrink text.
- Keep source (Mermaid, PlantUML, D2) in the repo so it diffs and updates.
- Add a text description for accessibility and for readers who skim.
- Date or version it if it can drift, and check it against code.

## 11. Structure at document level

- Title says what the document is. First paragraph says who it is for and what they get.
- Order sections by reader need, not by the order you discovered things.
- Group by task or question. Keep sections the same shape when they hold the same kind of thing.
- Put the most-used material at the top and the rarely-used material at the bottom.
- Use lists for parallel items and tables for attributes. Use prose for reasons.
- Give each document one purpose. If a section serves a different reader, it belongs on a different page, linked.
- Do not duplicate content. Link. Accept a small repeat a reader needs in place.
- End when the content ends. No conclusion that repeats the doc. A closing section is for next steps, open questions, or contacts, if there are any.

## 12. Reviewing and being reviewed

Review your own doc as a skeptical reader: pick five claims at random and verify them; follow the procedure on a clean machine; ask whether a new hire could act on it. Ask a second reader who was not in the context. When reviewing others, comment on what the reader cannot do after reading, not on taste.

## 13. Before and after rewrites

<!-- slop-scan: off -->
1. Before: "Our robust, scalable platform seamlessly integrates with a wide range of services, empowering teams to unlock new insights."
   After: "The platform ingests events from Kafka, S3, and HTTP webhooks, and writes aggregated results to Postgres. It handles 12k events per second on a single c6i.2xlarge (load test, 2026-09-30)."
   Why: replaced adjectives and verbs with the inputs, outputs, and a measurement.

2. Before: "It's important to note that configuration is crucial for proper functioning."
   After: "The service fails to start if `DATABASE_URL` is unset."
   Why: the real fact.

3. Before: "This isn't just a cache; it's a performance layer that transforms how data flows."
   After: "The cache stores query results for 60 seconds, keyed by the SQL string and tenant ID. It cut p95 latency of `/reports` from 1.8 s to 240 ms."
   Why: negative parallelism and a metaphor replaced by behavior and a result.

4. Before: "Retries are handled gracefully to ensure a smooth user experience."
   After: "On a 5xx response the client retries up to 3 times with delays of 200 ms, 400 ms, and 800 ms, then raises `UpstreamError`."
   Why: "gracefully" and "smooth" say nothing; the policy says everything.

5. Before: "In conclusion, by following these best practices, you can ensure your deployments are reliable and efficient."
   After: (delete)
   Why: fails the delete test.

6. Before: "Simply run the install script and you're good to go!"
   After: "Run `./scripts/install.sh`. It takes about 2 minutes and prints `Install complete` when it finishes."
   Why: removed condescension and filler, added expected result.

7. Before: "The module leverages advanced techniques to handle errors."
   After: "The module catches `TimeoutError` and `ConnectionError`, logs them with the request ID, and returns HTTP 503."
   Why: specific exceptions and behavior.

8. Before: "Whether you're a beginner or an expert, this guide has something for everyone."
   After: "This guide assumes you can run Docker and have an AWS account."
   Why: states the real prerequisites.

## 14. Worked example: a README opening

Weak:
> # AwesomeFlow
> AwesomeFlow is a powerful, flexible workflow engine that empowers teams to orchestrate complex pipelines with ease. Whether you are building small scripts or enterprise systems, AwesomeFlow has you covered.

Strong:
> # pipeflow
> pipeflow runs YAML-defined task graphs on a single machine. It handles dependencies, retries, and log capture, and has no external services to run. Use it for build and data jobs that fit on one host. For multi-host scheduling, use Airflow or Argo.
>
> ```bash
> pip install pipeflow==1.4.2
> pipeflow run examples/hello.yaml
> ```
> Expected output: `hello: success (0.2s)`

The strong version says what it does, its limit, an alternative, and a command with expected output. Every claim is checkable.

## 15. Worked example: an ADR paragraph

Weak: "After careful consideration of many options, we decided to adopt Kafka as it is a best-in-class solution that provides numerous benefits."

Strong: "We will use Kafka for order events. The consumers need replay of up to 7 days to rebuild search indexes, which SQS cannot do. Kafka adds operational load: we will use managed MSK and accept about $1,400 per month. Considered and rejected: SQS (no replay), Kinesis (shard management, team has no experience), and polling the orders table (lag above 30 s at peak)."

<!-- slop-scan: on -->

## 16. Quality tests

Run these on every document before delivery.

**Delete test.** For each sentence, ask what the reader loses if it is deleted. If nothing, delete it.

**Interchangeability test.** Could this sentence appear unchanged in docs for a different project? If yes, it is generic. Replace it with a project-specific fact or delete it.

**So-what test.** After each claim, ask "so what should the reader do or know?" If the sentence does not answer, add the consequence or cut the claim.

**False-work test.** Does the sentence describe effort, care, or quality ("carefully designed", "thoroughly tested") instead of results? Replace with the result or remove.

**Evidence test.** Can you point to the source of every fact? If not, verify or mark unverified.

**Cold-reader test.** Could someone who was not here follow it from a clean machine?

**Term inventory.** One name per concept.

**Skim test.** Read only headings and first sentences. Do they tell a coherent, true story?

**Specificity density.** Count concrete items (names, numbers, paths, commands) per paragraph. Paragraphs with zero need rewriting.

## 17. Common failure modes

- Documenting what the code does line by line instead of what the system is for.
- Writing the happy path only.
- Copying the old README's claims without checking them.
- Putting rationale in comments nobody reads and omitting it from the design doc.
- Describing the intended architecture instead of the actual one.
- Mixing tutorial, reference, and explanation in one page.
- Over-structuring: headings and bullets over thin content.
- Hedging everything so that no claim can be wrong, and none is useful.
- Using adjectives in place of measurements.
- Long preambles and recaps.
- Leaving placeholders and TODOs in delivered docs.
- Documenting a plan as if it were shipped.
