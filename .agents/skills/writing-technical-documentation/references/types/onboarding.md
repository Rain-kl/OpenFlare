# Onboarding guide

Reader: a new teammate in their first week. The goal is a first merged change, so order the guide by what they need next.

Order: what the system does and why, access to request and from whom, dev environment setup with tested commands, running the tests, a small real first change, a map of the code, who owns what, and a glossary. Say how long each step takes.

Walk the guide on a clean machine or a fresh clone before calling it done, and fix drift as you go. Do not invent access requests, owners, or channels: ask, or mark `[unverified: ...]`.

Done when: a new hire can reach a passing test run from the text alone.

````markdown
# Onboarding: <team or service>

Time to first merged change: about <n> hours.

## What this is

<Purpose and users in a short paragraph.>

## Get access

| System | Request from | Time |
|---|---|---|
| <system> | <who or how> | <time> |

## Set up

```bash
<tested commands>
```

Expected: <result>.

## Run the tests

```bash
<command>
```

## Make your first change

<Small real task with steps.>

## Map of the code

| Path | What lives there |
|---|---|
| `<path>` | <purpose> |

## Who owns what

<Roles and channels.>

## Glossary

<Project terms.>
````
