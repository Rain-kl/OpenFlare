# Troubleshooting guide

Readers arrive with an error message, so organize by symptom as they see it, using the exact error text as the heading. A reader searches for what the screen says, and internal cause names mean nothing to them yet.

Each entry: symptom, likely causes in order of probability, how to confirm each, the fix. Copy error messages from source or a run. "Check the logs" is only useful with the log location and the pattern to look for. End with what to collect and where to ask when nothing matches.

Find real entries in issue trackers, support channels, `except` and `catch` blocks, and test names for failure paths. Do not invent failure modes.

````markdown
### `<exact error message>`

Cause: <most likely cause>.

Confirm:

```bash
<command>
```

Fix: <steps>.

If it persists, collect <logs or output> and ask in <place>.
````
