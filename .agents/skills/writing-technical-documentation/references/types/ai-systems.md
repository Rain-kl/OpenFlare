# Docs for AI, ML, and agentic systems

Behavior here hides in prompts, tool schemas, and configuration, so read `references/reading-order.md` section 5 first. Say which parts vary run to run and which behaviors tests pin down. Give model IDs and parameters exactly as code sets them.

For each LLM flow, answer: what goes in, what the model is told, what tools it can call, what comes out, how output is validated, what happens on failure or low confidence, and what it costs per run when that is knowable. Missing evals are a finding.

Model or system card:

```markdown
# <Model or system> card

| Field | Value |
|---|---|
| Version | <id> |
| Intended use | <> |
| Out of scope | <> |
| Data | <sources, size, dates, licenses> |
| Evaluation | <datasets, metrics, numbers, date> |
| Limitations and failure modes | <> |
| Safety and bias findings | <> |
| Cost and latency | <numbers, setup> |
| Update policy | <> |
```

Agent or LLM flow:

```markdown
# <Flow name>

Goal: <>
Input: <>
Model: <provider, model ID, region, temperature, max tokens>
System prompt: `<path>` (version <>)

| Tool | Schema | Side effects | Permissions |
|---|---|---|---|
| <name> | `<path>` | <> | <> |

Loop: <steps, max iterations, stop conditions>
Validation: <output checks>
Failure handling: <>
Human approval: <where>
Evaluation: <dataset, metric, threshold, CI>
Cost per run: <number and basis>
Non-deterministic parts: <>
```

Prompt docs: purpose, inputs and variables, expected output format, versioning, tests, known failure cases. Retrieval docs: sources, chunking, embedding model, index settings, refresh schedule, and how retrieval quality is measured.
