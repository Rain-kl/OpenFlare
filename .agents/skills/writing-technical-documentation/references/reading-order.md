# Reading a codebase before documenting it

Contents
1. Principles
2. Scope first
3. The eight tiers, in order
4. Ecosystem file map
5. AI, LLM, and agentic projects
6. AWS and infrastructure projects
7. Data and ML pipelines
8. Monorepos
9. What to skip and why
10. Tracing one execution path
11. Questions every document must be able to answer
12. Git archaeology
13. Reading tests as documentation
14. Existing docs are claims
15. Large files, generated code, secrets, dynamic behavior
16. Stopping criteria and notes

## 1. Principles

- Codebases are not books. Read by boundaries and execution paths, not by file order or directory order.
- Map before territory. Learn where the doors are (inputs, outputs, entry points, contracts) before reading internals. Most of the logic that matters sits in 10 to 20 percent of the files. The rest is glue.
- Read the contract before the implementation: interfaces, types, schemas, route definitions, CLI definitions.
- Tag as you go. Core: business logic, models, handlers. Support: helpers, middleware, utilities. Config: environment, build, deploy. Noise: generated, vendored, build output. Read core first. Support serves it. Config can wait until a claim depends on it, with the exception of the files in tier 4 below that encode architecture.
- Ask what problem each odd thing solved. Strange code usually records a past failure. Read decisions, not only syntax.
- Source and tests are truth. Prose decays. Verify prose against them.
- Keep notes with file paths and line numbers from the first minute. You will not remember them.

## 2. Scope first

Before reading, fix the scope from the request: whole system, one service, one flow, one module, one decision. Then read only what that scope needs. A README for a repo needs tiers 1 to 5 and a skim of 6 and 7. A runbook for one failure needs the specific flow, its config, its logs, and its alerts. A design doc for a new feature needs the touched boundaries and the code it will change.

## 3. The eight tiers, in order

### Tier 1: Instructions for humans and agents about the repo
`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.claude/`, `.cursor/rules/`, `.cursorrules`, `.windsurfrules`, `.github/copilot-instructions.md`, `CONTRIBUTING.md`, `CODEOWNERS`, `SECURITY.md`, `docs/CONTRIBUTING*`, `.editorconfig`, style guides.
Read fully. These tell you conventions, commands, and what the maintainers consider important. They also tell you how docs are expected to look. Note the doc tooling too (`mkdocs.yml`, `docusaurus.config.*`, `conf.py`, `.vale.ini`), which decides where a page goes and which rules it must pass. The inventory script reports these.

### Tier 2: Existing documentation
`README*`, `docs/`, `doc/`, `wiki/`, `ADR/`, `adr/`, `docs/decisions/`, `rfcs/`, `design/`, `CHANGELOG*`, `UPGRADING*`, `MIGRATION*`, diagrams (`*.drawio`, `*.mmd`, `*.puml`, `*.excalidraw`), OpenAPI and AsyncAPI specs, `openapi.yaml`, `swagger.json`, `schema.graphql`, `*.proto`.
Read to learn intent, vocabulary, and what already exists so you do not duplicate it. Treat every factual statement as unverified until checked against code. Record disagreements between docs and code. They are findings.

### Tier 3: Manifests and build definitions
Dependencies, scripts, entry points, versions, supported runtimes, build commands. Scan; do not read lockfiles. Pull out: runtime and version constraints, the commands for install, build, test, lint, run, the declared entry points (`main`, `bin`, `scripts`, `console_scripts`), and the dependencies that reveal the architecture (web framework, ORM, queue client, cloud SDK, LLM SDK, vector store).

### Tier 4: Runtime and deploy configuration
`Dockerfile`, `docker-compose*.yml`, `.env.example`, `.env.sample`, `config/`, `settings.*`, CI definitions (`.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, `azure-pipelines.yml`, `.circleci/`, `buildspec.yml`), IaC (`*.tf`, `cdk.json`, `template.yaml`, `serverless.yml`, `Chart.yaml`, `k8s/`, `kustomization.yaml`, `pulumi`), process definitions (`Procfile`, `supervisord.conf`, `systemd` units), gateway and proxy config (`nginx.conf`, `envoy.yaml`).
Config files encode architectural decisions: which services exist, how they connect, which secrets and env vars are needed, what is deployed where, what runs on a schedule. A docker-compose file often lists every service in the system in one screen.

### Tier 5: Boundaries
The surface where the system touches the outside:
- Inbound: HTTP routes, gRPC services, GraphQL schema, CLI commands and flags, message consumers, webhooks, scheduled jobs, Lambda handlers, event triggers, UI entry points.
- Outbound: databases, queues, object stores, third-party APIs, LLM or model endpoints, email and notification services, file system writes.
- Contracts: request and response schemas, event schemas, DB schema and migrations, config schema.
- Exit behavior for CLIs: exit codes, stdout versus stderr, side effects.

Useful searches (adapt to the language):
```
grep -rnE "@(app|router|blueprint)\.(get|post|put|delete|route)|app\.(get|post|put|delete)\(|@(Get|Post|Put|Delete)Mapping|router\.(GET|POST)|\[Http(Get|Post|Put|Delete)|\.Map(Get|Post)\(" --include=*.{py,js,ts,java,go,kt,cs}
grep -rnE "os\.environ|process\.env|getenv|System\.getenv|Environment\.GetEnvironmentVariable|viper\.|config\." .
grep -rnE "boto3\.client|new \w+Client\(|requests\.(get|post)|fetch\(|axios\." .
grep -rnE "open\(|fs\.(read|write)|writeFile|readFile" .
```

### Tier 6: Tests
Read test names first, then fixtures, then a handful of assertions around the core flows. Tests show intended behavior, accepted inputs, edge cases someone cared about enough to pin, and how components are wired for real. If there are integration tests, they often show the end-to-end flow better than any doc. Missing or empty tests are a finding.

### Tier 7: Core logic, one path at a time
Pick one concrete scenario (one request, one command, one job, one message) and follow it from entry to exit. Note each hop: file, function, what it reads, what it writes, what it can fail on. Repeat for the next most important scenario. Stop when the document's claims are covered.

### Tier 8: History
`git log --oneline -30`, `git log -S"<symbol>"`, `git blame` on odd lines, merged PR titles, recent churn. Use it to learn why, to date decisions, and to detect abandoned areas. Do not write history into the doc unless the reader needs it (an ADR or migration guide).

## 4. Ecosystem file map

| Ecosystem | Manifest | Entry points | Config and extras |
|---|---|---|---|
| Node and TypeScript | `package.json` (`main`, `bin`, `scripts`, `engines`), `tsconfig.json` | `src/index.*`, `src/main.*`, `server.*`, `bin/*`, framework pages and routes folders | `.nvmrc`, `.env.example`, `next.config.*`, `vite.config.*`, `jest.config.*`, workspaces |
| Python | `pyproject.toml`, `setup.cfg`, `setup.py`, `requirements*.txt`, `Pipfile` | `__main__.py`, `manage.py`, `app.py`, `main.py`, `wsgi.py`, `asgi.py`, `[project.scripts]` | `tox.ini`, `pytest.ini`, `conftest.py`, `alembic/`, `.python-version` |
| Go | `go.mod` | `cmd/*/main.go`, `main.go` | `Makefile`, `internal/`, `pkg/`, `*.proto` |
| Rust | `Cargo.toml` (`[[bin]]`, `[features]`, workspace) | `src/main.rs`, `src/lib.rs`, `src/bin/` | `build.rs`, `rust-toolchain.toml` |
| Java and Kotlin | `pom.xml`, `build.gradle(.kts)`, `settings.gradle` | `@SpringBootApplication`, `public static void main`, `application.yml` | `src/main/resources`, Flyway or Liquibase migrations |
| .NET | `*.csproj`, `*.sln`, `global.json` | `Program.cs`, `Startup.cs` | `appsettings*.json` |
| Ruby | `Gemfile`, `*.gemspec` | `config.ru`, `bin/`, `config/routes.rb` | `config/`, `db/schema.rb` |
| PHP | `composer.json` | `public/index.php`, `artisan`, `routes/` | `.env.example`, `config/` |
| Mobile | `Podfile`, `Package.swift`, `build.gradle`, `AndroidManifest.xml`, `Info.plist` | `AppDelegate`, `MainActivity`, `App.swift` | entitlements, signing config (never copy secrets) |
| Infrastructure | `*.tf`, `cdk.json`, `template.yaml`, `Chart.yaml` | root modules, stacks, `main.tf`, `app.py` for CDK | `variables.tf`, `outputs.tf`, `terraform.tfvars.example`, `values.yaml` |
| Data and ML | `pyproject.toml`, `environment.yml`, `dvc.yaml`, `MLproject` | pipelines, DAGs, notebooks, training scripts | `params.yaml`, `config/`, experiment tracking config |

## 5. AI, LLM, and agentic projects

These projects hide their behavior in prompts, tool schemas, and configuration that conventional reading misses. Add these to the read:

- Prompts and templates: `prompts/`, `*.prompt`, `*.jinja`, system prompt strings, few-shot examples. Quote them exactly when documenting behavior. Note variables injected into them and where those come from.
- Model configuration: provider, model ID or ARN, region, temperature and other sampling parameters, max tokens, stop sequences, guardrail IDs, inference profile, timeouts, retries, fallbacks. Record exact values from code or config.
- Tool and function definitions: names, JSON schemas, descriptions, which handler runs, permissions, side effects, error returns.
- Orchestration: agent loops, graph definitions (LangGraph, Step Functions state machines, Bedrock Agents action groups, custom loops), routing logic, max iterations, termination conditions, human-in-the-loop checkpoints.
- Retrieval: chunking strategy and sizes, embedding model, vector store and index settings, top-k, reranking, filters, ingestion pipeline and refresh schedule.
- Memory and state: where conversation or agent state lives, TTLs, what is persisted.
- Evaluation: eval datasets, metrics, thresholds, judge prompts, regression scripts, CI gates. Missing evals are a finding.
- Safety and cost: guardrails, PII handling, input and output validation, rate limiting, token budgets, caching, logging of prompts and outputs (and whether it logs sensitive data).
- Observability: tracing, token and latency metrics, prompt versioning.
- Non-determinism: say which parts vary run to run, and which behaviors are pinned by tests.

For each LLM-backed flow, the document should answer: what goes in, what the model is told, what tools it can call, what comes out, how output is validated, what happens on failure or low confidence, and what it costs per call or per run when that is knowable.

## 6. AWS and infrastructure projects

- Identify the compute (Lambda, ECS, EKS, EC2, Step Functions, Glue, Batch), triggers (API Gateway, EventBridge, SQS, SNS, S3 events, schedules), data stores (DynamoDB, RDS, S3, OpenSearch, S3 Vectors), and AI services (Bedrock models, agents, knowledge bases, guardrails, Textract, Comprehend, SageMaker).
- Record resource names via naming conventions and variables, not hardcoded account IDs or ARNs from the repo. Use placeholders such as `<ACCOUNT_ID>` and `<REGION>` in docs.
- IAM: which roles and policies exist, least-privilege gaps, cross-account access. Summarize permissions that matter for operating the system. Do not paste policy documents wholesale.
- Networking: VPC, subnets, endpoints, security groups, public versus private exposure.
- Environments: how dev, staging, prod differ in config and in pipeline.
- Deployment: how a change reaches production, who approves, how to roll back.
- Cost drivers and quotas that can bite: concurrency limits, token quotas, provisioned throughput, data transfer.

## 7. Data and ML pipelines

Sources and their schemas, ingestion schedule and freshness, transformation steps, outputs and consumers, data quality checks, lineage, backfill procedure, training and evaluation entry points, artifacts and where they are stored, model and dataset versioning, reproducibility steps, and known data issues.

## 8. Monorepos

Identify the workspace definition first (`pnpm-workspace.yaml`, `lerna.json`, `nx.json`, `turbo.json`, `go.work`, Cargo workspace, Bazel `WORKSPACE`, Gradle `settings.gradle`). List packages and their dependency edges. Pick the package in scope and read its tiers 1 to 7 plus the shared packages it imports. Write a root README that is a map (what each package is, how they connect, how to run all), and per-package READMEs that stand alone.

## 9. What to skip and why

| Skip | Reason | Read only when |
|---|---|---|
| `node_modules/`, `vendor/`, `.venv/`, `venv/`, `target/`, `build/`, `dist/`, `out/`, `.next/`, `.nuxt/`, `__pycache__/`, `.gradle/`, `Pods/` | Third-party or build output | A claim depends on a dependency's behavior (then read that dependency's docs, not the vendored copy) |
| Lockfiles: `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `poetry.lock`, `Cargo.lock`, `go.sum`, `Gemfile.lock` | Machine-generated | You must state an exact resolved version |
| Generated code: `*.pb.go`, `*_pb2.py`, `*.generated.*`, `*.g.dart`, GraphQL codegen output, OpenAPI client output | Derived from a source | Documenting the generator, not its output |
| Minified and bundled assets, source maps, images, fonts, binaries, archives | No prose value | Never, except to note their existence |
| Snapshots and large fixtures (`__snapshots__`, `*.snap`, data dumps) | Noisy | A test's expected output is the claim |
| `.git/` internals | Use git commands instead | Never read directly |
| Coverage reports, logs, caches | Ephemeral | Debugging a specific claim |
| Deep utility and helper code | Serves the core | A traced path touches it |

## 10. Tracing one execution path

1. Choose the scenario in one sentence ("A client POSTs /orders and the order is stored and an event is published").
2. Find the entry (route definition, handler registration, CLI command, trigger).
3. Walk the call chain. At each hop note: file and line, input shape, validation, external calls, state changes, errors raised or returned, logging.
4. Note configuration read along the way and where it comes from.
5. Note async hand-offs (queues, events, background jobs) and follow them to their consumers.
6. Write the path as an ordered list with file references. That list is the raw material for an architecture or flow explanation.
7. Check the path against a test or a run when possible.

For dynamic behavior (dependency injection, reflection, plugin loading, config-driven wiring, decorators that register handlers), find the registration point and say in notes how you resolved it. If you cannot, mark the claim `unverified`.

## 11. Questions every document must be able to answer

Collect answers to these during reading. Not every document includes every answer, but any gap is a visible one.

1. What is this and who is it for?
2. How do I install, configure, and run it? Which versions?
3. What are the entry points and boundaries?
4. What does it depend on at runtime (services, data, credentials)?
5. What are the key data types and where is state kept?
6. What are the main flows from input to output?
7. How is it tested and deployed?
8. How does it fail, and how do I tell?
9. What decisions shaped it, and what was rejected?
10. What is deliberately out of scope or known to be broken?
11. Who owns it and where do questions go?

## 12. Git archaeology

```
git log --oneline -30
git log --stat -10
git log -S"<string>" --oneline
git log --follow -p -- <path>
git blame -L <start>,<end> <path>
git shortlog -sn --no-merges | head
git log --since="6 months ago" --name-only --pretty=format: | sort | uniq -c | sort -rn | head -20
```

Churn hot spots show where the real work is. Old untouched areas may be dead or stable. Commit messages and PR titles explain decisions. Never write personal blame into docs. Cite decisions by ADR or PR number when it helps.

## 13. Reading tests as documentation

- Test names are claims about behavior. List them for the module in scope.
- Fixtures reveal valid inputs and shapes.
- Parametrized cases show boundaries.
- Failing-path tests show the intended error behavior and messages.
- A test that mocks heavily shows what the authors consider a boundary.
- Tests that are skipped, xfailed, or commented out show known gaps.
Use test code as the source for examples in docs when it is minimal and correct. Cite it.

## 14. Existing docs are claims

For each factual statement in existing docs: find its evidence, then mark verified, outdated, or unverifiable. Typical drift: renamed flags, removed endpoints, changed defaults, wrong versions, install steps that no longer work, diagrams that show deleted services. Record drift as a finding and fix it in the doc you write or report it if out of scope. Preserve any rationale or history in existing docs that code cannot supply, such as why a decision was made, and mark it as sourced from the old doc.

## 15. Large files, generated code, secrets, dynamic behavior

- Large files: grep for the symbol, then read a range with line numbers. Do not paste large blocks into notes.
- Generated code: find the generator and the input. Document those.
- Secrets: never copy values from `.env`, key files, tokens, connection strings, or cloud credentials into notes or docs. Use `<PLACEHOLDER>` names. If you find a committed secret, tell the user and do not repeat it.
- Personal data in fixtures or logs: do not reproduce it.
- Environment-dependent behavior: say which environment you observed.
- Feature flags: list flags and defaults, and say what each changes.

## 16. Stopping criteria and notes

Stop reading a tier when every claim the planned document needs has evidence. Stop reading the repo when the outline's sections are all covered. More reading beyond that produces more text, not better documentation.

Keep a scratch notes file outside the repo with: scope, reader, outline, ledger (claim, evidence, status), open questions, findings. The ledger drives the document and the final report.
