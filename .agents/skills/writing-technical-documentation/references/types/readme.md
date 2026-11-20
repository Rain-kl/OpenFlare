# README

Reader: someone who landed on the repo and has 30 seconds to decide whether it applies to them, then wants one working command. A README is a front door and a map, so depth moves to `docs/`.

Answer these in order:

1. What it is, in one or two sentences with concrete nouns, and its main limit.
2. When to use something else.
3. The fastest working path: prerequisites with versions, install, run, expected output.
4. A minimal example that does something real.
5. Where to go next: configuration, architecture, contributing.
6. Status and support: maturity, supported versions, license, how to get help. State only what the repo shows.

Every command must run as written from a fresh clone. That is what readers try first, and it is where stale READMEs fail. Say which directory to run from when a repo has several. Walk the quick start in order, with the config file it names: check that an example value works where the reader runs it, and that whatever loads the config happens before the command that needs it. If a service needs a second process, say so in the same block. Do not carry over a support channel or contact from the old README unless you can confirm it: mark it `[unverified: ...]`.

Variants:

| Repo kind | What changes |
|---|---|
| Library | Install line, then the smallest import-and-call example; link the API reference |
| Service | How to run it locally with its dependencies, the port, a request that proves it works |
| CLI | Install, then the most common command and its output; link the full reference |
| Monorepo | A map: each package with one line, how they connect, how to run all. Each package gets its own README that stands alone |

Leave out adjective lists, decorative badges, a table of contents on short files, and a repeated file tree.

Done when: a stranger on a clean machine gets the expected output by following the first screen, and every link resolves.

````markdown
# <project-name>

<What it does, for whom, and the main constraint or limit.>
<When not to use it, and the alternative.>

## Quick start

Prerequisites: <runtime and version>, <tool and version>.

```bash
<install command>
<run command>
```

Expected output:

```text
<captured output>
```

## Usage

<Smallest real example, tested.>

## Configuration

| Key | Type | Default | Description |
|---|---|---|---|
| `<KEY>` | <type> | `<default>` | <what it controls, units> |

## Documentation

- [Architecture](docs/architecture.md)
- [API reference](docs/api.md)

## Development

```bash
<test command>
<lint command>
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Status and support

<Maturity, supported versions, license, where to ask.>
````
