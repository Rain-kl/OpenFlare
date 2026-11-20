# Docs checks in CI

Suggest these when a team wants docs to stay correct. Do not add them without approval, since a new failing check blocks other people's work. Start with the checks that catch broken things, and add prose rules at warning level only after the existing docs pass.

Ready-to-copy files are in `assets/ci/`. Pin tool versions, because a new lint rule can fail a build with no change in the repo.

| Check | Catches | Tool | Start at |
|---|---|---|---|
| Broken links | Dead internal and external links | lychee | Internal: fail. External: schedule, report only |
| References | Missing paths, make targets, scripts, removed names | `verify_refs.py`, `doc_audit.py --changed-since` | Report only |
| Markdown structure | Heading levels, code fences | markdownlint-cli2 | Fail, after fixing existing issues |
| Prose rules | AI patterns, banned terms, house style | Vale, or `slop_scan.py` | Warning |
| Code samples | Examples that stopped running | `pytest --doctest-glob`, `cargo test --doc`, Go `Example` functions, or a script over extracted blocks | Fail |
| API contract | Invalid OpenAPI, code that drifts from it | Spectral or Redocly | Fail |
| Spelling | Typos and term variants | codespell or CSpell, with a project dictionary | Warning |
| Secrets | Tokens pasted into docs | gitleaks | Fail |
| Clean setup | Install steps that no longer work | A scheduled job running the README on a clean runner | Report only |

## GitHub Actions

Copy `assets/ci/docs-checks.yml` to `.github/workflows/docs-checks.yml`. It runs markdownlint-cli2, Vale with the bundled AISlop style, and lychee. To add this skill's scripts, vendor the skill's `scripts/` folder into the repo, then uncomment the marked step. `slop_scan.py --format github` prints annotations that show on the pull request.

## Vale

Vale checks prose with YAML rules and no Python. Copy `assets/ci/.vale.ini` to the repo root and the contents of `assets/ci/vale/` to `.vale/styles/`, then fetch the Google style with:

```bash
vale sync
vale README.md docs
```

`assets/ci/vale/AISlop/` holds the strongest patterns from `references/slop-catalog.md` as rules. It cannot do the scanner's density, cluster, or structure checks, so the two complement each other.

Put project terms in the vocabulary files, so Vale stops flagging real names and starts flagging wrong ones. Vale requires the folder named by `Vocab` to exist, and the kit includes it:

- `.vale/styles/config/vocabularies/Project/accept.txt`: one term per line, regular expressions allowed.
- `.vale/styles/config/vocabularies/Project/reject.txt`: terms that always fail.

Commit `.vale/styles/AISlop` and the vocabulary. Add downloaded packages such as `.vale/styles/Google` to `.gitignore`, because `vale sync` restores them.

## Pandoc in CI

See `references/pandoc.md` section 7.
