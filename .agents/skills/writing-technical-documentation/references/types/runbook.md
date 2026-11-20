# Runbook

Reader: an on-call engineer under stress, possibly at 2 a.m., with no time to read prose. Each line must be an action or the meaning of a result.

Rules:

- Start with the trigger: the exact alert name or symptom.
- State impact and urgency in one line.
- Mitigate first and find root cause later. Include the rollback.
- Give diagnosis as commands with expected output and what each result means, as a decision path.
- Mark dangerous steps, and put the blast radius before the command.
- Name escalation by team and channel. Do not invent these: take them from the repo or ask, and mark gaps with `[unverified: who is on call for this]`.
- Give dashboards and logs by exact name or query.
- End with how to confirm the fix.
- Test the runbook by following it on a non-production system, then record the date and the commit. If you could not run it, say so at the top and name the commands that did not run. A date there tells the reader the steps worked.

The temptation here is to fill gaps with plausible names. An invented dashboard or Slack channel costs minutes during an incident, so leave a marked gap. Trace the failure path in code first: what raises the alert, what the retry and backoff settings are, what the queue depth limit is.

Done when: someone who did not write it can run each step from the text alone, every command, query, and name was checked, and the top says what was only read and not run.

````markdown
---
title: "Runbook: <alert or symptom>"
last_verified: <YYYY-MM-DD, or "not run">
verified_against: <commit or version>
owner: <team>
---

# Runbook: <alert or symptom>

Trigger: <exact alert name or symptom>.
Impact: <who or what is affected>.
Urgency: <page now | business hours>.

## Mitigate

1. <Action.> Expected: <result>.

## Diagnose

1. Check <thing>:

   ```bash
   <command>
   ```

   If the output shows <A>, go to [Fix: cause A](#fix-cause-a). If <B>, go to [Fix: cause B](#fix-cause-b).

## Fix: cause A

Warning: <blast radius> before running the next command.

1. <Step.>

## Roll back

1. <Step.>

## Confirm the fix

<Command or dashboard, and the expected value.>

## Escalate

If not resolved in <time>, contact <team or channel>.

## Links

- Dashboard: <exact name>
- Logs: <query or log group>
````
