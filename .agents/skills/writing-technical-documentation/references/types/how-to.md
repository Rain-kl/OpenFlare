# How-to guide

Purpose: a competent user completes one specific task. The title starts with a verb or "How to". Assume knowledge, skip background, and link to reference and explanation.

Rules, mostly from the Google developer documentation style guide:

- Put prerequisites before step 1, and conditions before instructions: "If the queue is empty, skip step 4."
- One action per step, starting with an imperative verb. Keep steps to about seven; split longer tasks with a verification point between parts.
- Order a complex step this way: describe the action, give the command, explain placeholders, then show output and explain the result.
- State the expected result when a step can fail silently.
- Mark optional steps with "Optional:" at the start of the step.
- Give the best method, one method. Reference repeated procedures instead of copying them.
- Do not write "run the following command" or point "above" and "below". Link instead.

````markdown
# How to <task>

<One sentence: what you will have at the end.>

## Before you begin

- <prerequisite with version>

## Steps

1. <Action.> Expected: <result>.

   ```bash
   <command>
   ```

   Replace `<PLACEHOLDER>` with <what it is>.

2. <Action.>

## Verify

<Command and expected output.>

## Troubleshooting

| Error | Fix |
|---|---|
| `<error text>` | <cause and fix> |
````
