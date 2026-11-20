# Changelog and release notes

Changelog: for developers, following Keep a Changelog 1.1.0.

- Written for people, and a commit log dump does not qualify: commits record steps, while an entry describes a noteworthy difference for users.
- Latest version first, with one entry per version and an ISO 8601 date.
- Group by Added, Changed, Deprecated, Removed, Fixed, Security.
- Keep an `Unreleased` section at the top.
- Make versions linkable, and state whether the project follows Semantic Versioning.
- Mark a pulled release: `## [0.0.5] - 2014-12-13 [YANKED]`.

Build entries from merged PRs and tags, and read the actual diff when a title is vague. If the repo uses Conventional Commits, `git-cliff` can generate a first draft to edit.

Release notes: for users. Highlights first, breaking changes prominently with upgrade steps, then known issues.

```markdown
# Changelog

All notable changes to this project are documented here. This project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [<version>] - <YYYY-MM-DD>

### Added

- <what the user can now do> (#<issue>)

### Changed

- <behavior change>

### Fixed

- <what no longer breaks>

[<version>]: <compare or tag URL>
```
