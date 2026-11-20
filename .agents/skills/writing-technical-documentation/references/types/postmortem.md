# Postmortem (blameless)

Write one when an incident crossed a threshold the team set in advance: user-visible downtime or degradation, any data loss, on-call intervention such as a rollback, long resolution time, or a monitoring failure. Any stakeholder may request one.

Rules, from the Google SRE book:

- Focus on contributing causes and assume everyone acted in good faith with the information they had. "Human error" is never a root cause: ask why the system allowed it.
- Leave out anything that could identify an end user.
- Give impact in numbers, and timestamps in UTC.
- Every action item has an owner, a priority, and a due date.
- Facts first, then analysis.

Take the timeline from logs, deploy history, alerts, and chat, and mark anything reconstructed from memory. Never invent a time or a number. If you lack them, leave a marked gap for the incident lead.

Reviewers check that key data was collected, impact is complete, the root cause goes deep enough, and action items fit the problem. Publish to the widest audience that benefits, and store it with past incidents.

```markdown
# Postmortem: <incident title> (<YYYY-MM-DD>)

Status: <draft | final>. Severity: <level>.

## Summary

<Impact, duration, detection, resolution.>

## Impact

<Users affected, requests failed, data lost, money.>

## Timeline (UTC)

| Time | Event |
|---|---|
| <hh:mm> | <fact, with source> |

## Root cause and contributing factors

<System and process causes.>

## Detection and response

- What worked: <>
- What did not: <>
- Where we got lucky: <>

## Action items

| Action | Owner | Priority | Due | Link |
|---|---|---|---|---|
| <action> | <owner> | <P0-P3> | <date> | <issue> |

## Lessons

<Specific to this incident.>
```
