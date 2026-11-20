# CLI reference

Reader: someone at a terminal looking up a flag. Verify everything against `--help` output and the argument parser, because invented flags are the most common documentation hallucination.

Include: name and one-line purpose, synopsis, arguments and flags (name, short form, type, default, env var equivalent), exit codes, what goes to stdout versus stderr, config file locations and precedence, and failure messages with fixes.

Lead with common, realistic examples, then list the rest. The usual precedence, highest first, is flags, environment variables, project config, user config, system config. State this tool's actual order from the code.

Prefer generating this page from the parser (a `--help` dump or a tool's own docs generator), so it cannot drift from the code.

Done when: every flag appears in the parser, and the page lists every flag the parser has.

````markdown
## `<tool> <subcommand>`

<One-line purpose.>

```text
<tool> <subcommand> [options] <args>
```

Examples:

```bash
<most common case>
<next most common case>
```

| Option | Default | Env var | Description |
|---|---|---|---|
| `--<flag>` | `<default>` | `<ENV>` | <meaning, units> |

Exit codes: `0` success; `<n>` <meaning>.

Output: <what goes to stdout and what goes to stderr>.

| Error | Cause | Fix |
|---|---|---|
| `<exact message>` | <cause> | <fix> |
````
