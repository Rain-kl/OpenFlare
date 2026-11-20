#!/usr/bin/env python3
"""Audit a repository's documentation set: broken references, drift, gaps, and duplicates.

Usage:
    doc_audit.py REPO [options]
    doc_audit.py REPO --changed-since REF

Examples:
    doc_audit.py .
    doc_audit.py . --docs docs README.md --json > /tmp/audit.json
    doc_audit.py . --changed-since origin/main --fail-on error

Whole-set mode reports, for each doc: reference problems (from verify_refs), the AI-writing
score (from slop_scan), staleness, and structure problems. Across the set it reports missing
document types that the repo's own contents call for, repeated passages, and docs whose
referenced code changed after the doc did.

--changed-since mode answers one question for a pull request: which docs need to change with
this diff? It lists docs that mention a changed file, and docs that still mention a name the
diff removed from the code.

Findings are leads for a human or agent to confirm, ranked by severity. Read-only. Never
edits a file and never contacts the network.

Exit codes: 0 finished; 1 findings at or above --fail-on; 2 bad usage.
Standard library only; git is optional and enables the drift checks.
"""
import argparse
import json
import os
import re
import sys
import time

import doclib
import slop_scan
import verify_refs

# Findings printed by default; --limit 0 prints all. Keeps output within agent context limits.
DEFAULT_LIMIT = 40
DEFAULT_STALE_DAYS = 365
# A passage must be this long to count as duplication; shorter repeats are normal boilerplate.
DUPLICATE_MIN_WORDS = 30
SEVERITIES = {"error": 0, "warning": 1, "info": 2}
SECONDS_PER_DAY = 86400
# Names this short or common cause false matches in the removed-name check.
REMOVED_NAME_MIN_LENGTH = 5

# Each row: document type, patterns that satisfy it, and the repo signal that makes it expected.
EXPECTED_DOCS = [
    ("Root README", r"^README[^/]*$", "always", "every repo needs a front door"),
    ("CONTRIBUTING guide", r"(?:^|/)CONTRIBUTING[^/]*$|(?:^|/)docs?/.*contribut", "tests", "repo has tests, so contributors need to know how to run them"),
    ("Architecture overview", r"ARCHITECTURE|(?:^|/)docs?/.*(?:architecture|overview|design)", "services", "repo has several deployable parts"),
    ("Runbook", r"(?:runbook|playbook|operations?|on-?call|incident)", "deployable", "repo deploys to an environment, so on-call needs a procedure"),
    ("API reference", r"(?:openapi|swagger|asyncapi|\.proto$|schema\.graphql|(?:^|/)docs?/.*api)", "routes", "repo exposes HTTP routes"),
    ("CLI reference", r"(?:^|/)docs?/.*(?:cli|command|usage)|(?:^|/)man/", "cli", "repo defines a command-line interface"),
    ("Decision records", r"(?:^|/)(?:adrs?|decisions|rfcs?)(?:/|$)|(?:^|/)docs?/.*(?:adr|decision|rfc)", "services", "several parts usually mean decisions worth recording"),
    ("Changelog or release notes", r"(?:^|/)(?:CHANGELOG|HISTORY|RELEASES|NEWS)[^/]*$", "versioned", "repo publishes versions"),
    ("SECURITY policy", r"(?:^|/)SECURITY[^/]*$", "public-package", "repo publishes a package people depend on"),
    ("Agent instructions", r"(?:^|/)(?:AGENTS|CLAUDE|GEMINI)\.md$|copilot-instructions|\.cursor/rules|\.cursorrules", "non-standard", "repo has several task runners or a custom toolchain, and agents need the exact commands"),
]


CONTRACT_FILE_RE = re.compile(r"(?:^|/)(?:openapi|swagger|asyncapi)\.(?:ya?ml|json)$|(?:^|/)schema\.graphql$|\.proto$")


# Agent instructions, test data, and templates are not the project's documentation.
EXCLUDED_DIRS_RE = re.compile(r"(?:^|/)(?:\.claude|\.cursor|\.agents|\.github|\.windsurf|evals?|fixtures?|__fixtures__|testdata|test_?data|tests?|templates?)(?:/|$)", re.I)


def is_excluded(repo, path):
    """True for files a doc audit should ignore, including anything inside an agent skill."""
    if EXCLUDED_DIRS_RE.search(path):
        return True
    # A folder holding SKILL.md is an agent skill. Its paths are relative to the skill, not the repo.
    if "__skill_dirs" not in repo._cache:
        repo._cache["__skill_dirs"] = [os.path.dirname(f) + "/" for f in repo.files if f.rsplit("/", 1)[-1] == "SKILL.md" and "/" in f]
    return any(path.startswith(d) for d in repo._cache["__skill_dirs"])


def find_docs(repo, wanted):
    """Return doc paths relative to the repo, from --docs or by discovery."""
    def is_prose(f):
        return f.lower().endswith((".md", ".mdx", ".markdown"))

    if wanted:
        docs = []
        for w in wanted:
            rel = os.path.relpath(os.path.abspath(w), repo.root).replace(os.sep, "/")
            if os.path.isdir(w):
                docs.extend(f for f in repo.files if is_prose(f) and (rel == "." or f.startswith(rel + "/")))
            elif os.path.isfile(w):
                docs.append(rel)
        return sorted(set(docs))
    return [f for f in repo.files if is_prose(f) and not is_excluded(repo, f)]


def repo_signals(repo):
    files = repo.files
    names = {f.rsplit("/", 1)[-1] for f in files}
    code = [f for f in files if not doclib.is_doc_file(f)]
    route_re = re.compile(r"@(?:app|router)\.(?:get|post|put|delete|patch)\(|\bapp\.(?:get|post)\(|\[Http(?:Get|Post)|\.Map(?:Get|Post)\(|@(?:Get|Post)Mapping|\.HandleFunc\(|BaseHTTPRequestHandler")
    cli_re = re.compile(r"argparse\.ArgumentParser|@click\.command|typer\.Typer|cobra\.Command|clap::|System\.CommandLine|from ['\"]commander['\"]")
    routes = cli = False
    for f in code[:3000]:
        if not f.endswith((".py", ".js", ".ts", ".go", ".java", ".kt", ".cs", ".rs")) or re.search(r"(?:^|/)tests?/", f):
            continue
        t = doclib.read_text(repo.abspath(f), limit=200000)
        routes = routes or bool(route_re.search(t))
        # Helper scripts often use argparse, but they are not a product CLI.
        if not re.search(r"(?:^|/)(?:scripts?|tools?|hack|ci|local-environment)/", f):
            cli = cli or bool(cli_re.search(t))
        if routes and cli:
            break
    deploy = any(n.endswith(".tf") or n in ("serverless.yml", "template.yaml", "Chart.yaml", "kustomization.yaml", "Procfile", "fly.toml") for n in names)
    dockerfiles = sum(1 for f in files if f.rsplit("/", 1)[-1].startswith("Dockerfile"))
    compose = [v for v in repo.compose_services().values()]
    services = dockerfiles >= 2 or any(len(s) >= 2 for s in compose) or sum(1 for n in files if n.rsplit("/", 1)[-1] in ("package.json", "pyproject.toml", "go.mod", "Cargo.toml")) >= 3
    # A manifest alone does not mean anything is published. A root license and tags do suggest it.
    has_license = any(n.upper().startswith(("LICENSE", "COPYING")) for n in (f for f in files if "/" not in f))
    tagged = bool((doclib.run_git(repo.root, "tag", "--list") or "").strip())
    manifest = any(n in names for n in ("pyproject.toml", "setup.py", "Cargo.toml", "package.json", "go.mod", "pom.xml"))
    pkg = manifest and has_license
    runners = sum(1 for n in ("Makefile", "justfile", "Taskfile.yml", "package.json", "tox.ini", "noxfile.py") if n in names)
    return {
        "always": True,
        "tests": any(re.search(r"(?:^|/)tests?/|_test\.|\.test\.|\.spec\.|Tests?\.(?:cs|java|kt)$", f) for f in files),
        "services": services,
        "deployable": deploy or dockerfiles >= 1,
        "routes": routes,
        "cli": cli,
        "versioned": manifest and (tagged or pkg),
        "public-package": pkg,
        "non-standard": runners >= 2,
    }


def coverage(repo, signals, docs):
    rows = []
    for name, pattern, signal, why in EXPECTED_DOCS:
        rx = re.compile(pattern, re.I)
        found = [f for f in repo.files if rx.search(f) and (doclib.is_doc_file(f) or CONTRACT_FILE_RE.search(f) or ".cursor" in f or ".github" in f)
                 and (not is_excluded(repo, f) or name == "Agent instructions")]
        expected = bool(signals.get(signal))
        rows.append({"type": name, "found": found[:3], "expected": expected, "why": why})
    return rows


def structure_findings(rel, text, heads):
    out = []
    h1s = [h for h in heads if h["level"] == 1]
    if not heads:
        out.append(("info", "structure", 1, "no headings", "add a title that says what this document is"))
    elif len(h1s) > 1:
        out.append(("info", "structure", h1s[1]["line"], "%d H1 headings" % len(h1s), "keep one H1 per file"))
    front, _ = doclib.strip_front_matter(text)
    verified = doclib.front_matter_value(front, "last_verified") or (re.search(r"[Ll]ast verified:?\s*(\d{4}-\d{2}-\d{2})", text) or [None, None])[1]
    if verified and verified.lower().startswith("not run"):
        out.append(("info", "freshness", 1, "last_verified: not run", "the steps were never run; follow them on a non-production system"))
    elif verified:
        try:
            age = (time.time() - time.mktime(time.strptime(verified[:10], "%Y-%m-%d"))) / SECONDS_PER_DAY
            if age > 180:
                out.append(("warning", "freshness", 1, "last verified %s" % verified, "%d days old; re-verify and update the date" % age))
        except ValueError:
            out.append(("info", "freshness", 1, verified, "use an ISO date, YYYY-MM-DD"))
    elif re.search(r"runbook|playbook|on-?call", rel, re.I):
        out.append(("info", "freshness", 1, "no last-verified date", "runbooks need a date and the commit they were tested against"))
    return out


def git_times(repo):
    """Map each path to the Unix time of its latest commit, from one git log pass."""
    out = doclib.run_git(repo.root, "log", "--format=%x00%ct", "--name-only", "--no-renames", "HEAD", timeout=60)
    if out is None:
        return {}
    times = {}
    current = None
    for line in out.split("\n"):
        if line.startswith("\0"):
            current = int(line[1:] or 0)
        elif line.strip() and current is not None and line not in times:
            times[line] = current
    return times


def referenced_paths(repo, text):
    prose, _ = doclib.split_code(doclib.strip_front_matter(text)[1])
    found = set()
    for tok in re.findall(r"(?<!`)`([^`\n ]+)`(?!`)", prose) + re.findall(r"\]\(([^)#\s]+)", prose):
        tok = re.sub(r":\d+(?:-\d+)?$", "", tok).lstrip("./")
        if tok in repo.fileset and not doclib.is_doc_file(tok):
            found.add(tok)
    return found


def duplicates(repo, docs):
    """Find long paragraphs repeated across docs. Each repeat is a future contradiction."""
    seen, out = {}, []
    for rel in docs:
        prose = doclib.mask_inline_code(doclib.split_code(doclib.strip_front_matter(repo.text_of(rel))[1])[0])
        for para in re.split(r"\n\s*\n", prose):
            norm = " ".join(re.findall(r"[a-z0-9']+", para.lower()))
            if len(norm.split()) < DUPLICATE_MIN_WORDS or para.lstrip().startswith(("|", "#")):
                continue
            first = seen.setdefault(norm, rel)
            if first != rel:
                line = repo.text_of(rel).find(para.strip()[:60])
                out.append(("info", "duplicate", repo.text_of(rel).count("\n", 0, max(line, 0)) + 1, rel,
                            "passage also appears in %s; keep one and link to it" % first))
    return out


def audit(repo, wanted=None, stale_days=DEFAULT_STALE_DAYS):
    docs = find_docs(repo, wanted)
    times = git_times(repo)
    now = time.time()
    per_doc, findings = [], []

    def note(sev, kind, rel, line, item, msg):
        findings.append({"severity": sev, "kind": kind, "doc": rel, "line": line, "item": item, "message": msg})

    for rel in docs:
        text = repo.text_of(rel)
        checker = verify_refs.verify_doc(repo.abspath(rel), repo)
        for p in checker.problems:
            note(p["severity"], p["kind"], rel, p["line"], p["item"], p["message"])
        scan = slop_scan.scan(text)
        band = slop_scan.band_of(scan["score"])
        if scan["score"] >= slop_scan.BAND_PASS:
            note("warning", "ai-prose", rel, 1, "score %.1f" % scan["score"], "reads as unedited AI or template text; rewrite from evidence")
        elif scan["score"] >= slop_scan.BAND_CLEAN:
            note("info", "ai-prose", rel, 1, "score %.1f" % scan["score"], "needs a prose pass; run slop_scan.py on it")
        for h in scan["hits"]:
            if h["category"] in ("placeholder", "leaked-artifact"):
                note("error", h["category"], rel, h["line"], h["match"], h["fix"])
        for m in scan["unverified_markers"]:
            note("warning", "unverified-marker", rel, m["line"], m["text"], "resolve before publishing")
        for sev, kind, line, item, msg in structure_findings(rel, text, doclib.parse_headings(text)):
            note(sev, kind, rel, line, item, msg)

        doc_time = times.get(rel)
        age_days = int((now - doc_time) / SECONDS_PER_DAY) if doc_time else None
        newer = []
        if doc_time:
            for path in referenced_paths(repo, text):
                t = times.get(path)
                if t and t > doc_time:
                    newer.append((t, path))
        if newer:
            newer.sort(reverse=True)
            sample = ", ".join(p for _, p in newer[:3])
            note("warning", "drift", rel, 1, "%d referenced files changed after this doc" % len(newer),
                 "compare the doc with: %s" % sample)
        if age_days is not None and age_days > stale_days:
            note("info", "stale", rel, 1, "untouched for %d days" % age_days, "confirm it is still true, or delete it")
        per_doc.append({"doc": rel, "words": scan["words"], "score": scan["score"], "band": band,
                        "days_since_commit": age_days, "referenced_files_changed_since": len(newer),
                        "errors": sum(1 for f in findings if f["doc"] == rel and f["severity"] == "error"),
                        "warnings": sum(1 for f in findings if f["doc"] == rel and f["severity"] == "warning")})

    for sev, kind, line, rel, msg in duplicates(repo, docs):
        note(sev, kind, rel, line, "repeated passage", msg)
    signals = repo_signals(repo)
    cover = coverage(repo, signals, docs)
    for row in cover:
        if row["expected"] and not row["found"]:
            # Only a missing front door is a defect. The others are suggestions for the plan.
            note("warning" if row["type"] == "Root README" else "info", "missing-doc", "(repo)", 0, row["type"], row["why"])
    findings.sort(key=lambda f: (SEVERITIES[f["severity"]], f["doc"], f["line"]))
    return {"repo": repo.root, "docs_checked": len(docs), "git_available": bool(times), "documents": per_doc,
            "coverage": cover, "findings": findings,
            "counts": {s: sum(1 for f in findings if f["severity"] == s) for s in SEVERITIES}}


# ---------------------------------------------------------------- changed-since mode

DEF_RE = re.compile(
    r"^-(?!--)\s*(?:export\s+)?(?:async\s+)?(?:public\s+|private\s+|protected\s+|internal\s+|static\s+)*"
    r"(?:def|class|function|func|fn|const|let|var|interface|type|struct|enum|void|int|string|bool)\s+\*?([A-Za-z_][A-Za-z0-9_]*)")
ENV_REMOVED_RE = re.compile(r"""^-(?!--).*(?:environ(?:\.get)?[\[(]\s*|getenv\(\s*|process\.env\.|GetEnvironmentVariable\(\s*)['"]?([A-Z][A-Z0-9_]{2,})""")
FLAG_REMOVED_RE = re.compile(r"""^-(?!--).*['"](--[a-z][a-z0-9-]+)['"]""")


def changed_since(repo, ref):
    if doclib.run_git(repo.root, "rev-parse", "--verify", ref + "^{commit}") is None:
        return None, "cannot resolve git ref: %s" % ref
    # Three dots compares with the merge base, which is what a pull request changes.
    names = doclib.run_git(repo.root, "diff", "--name-status", "--no-renames", ref + "...HEAD", timeout=60) or ""
    diff = doclib.run_git(repo.root, "diff", "--unified=0", ref + "...HEAD", timeout=120) or ""
    uncommitted = doclib.run_git(repo.root, "diff", "--name-status", "--no-renames", "HEAD") or ""
    uncommitted_diff = doclib.run_git(repo.root, "diff", "--unified=0", "HEAD") or ""
    changed, doc_changed = {}, set()
    for line in (names + uncommitted).splitlines():
        parts = line.split("\t")
        if len(parts) >= 2:
            (doc_changed.add(parts[1]) if doclib.is_doc_file(parts[1]) else changed.__setitem__(parts[1], parts[0]))
    removed = {}
    in_doc_file = False
    for line in (diff + "\n" + uncommitted_diff).splitlines():
        if line.startswith("diff --git"):
            m = re.search(r" b/(.+)$", line)
            in_doc_file = bool(m and doclib.is_doc_file(m.group(1)))
            continue
        if in_doc_file:
            continue
        for rx in (DEF_RE, ENV_REMOVED_RE, FLAG_REMOVED_RE):
            m = rx.match(line)
            if m and len(m.group(1)) >= REMOVED_NAME_MIN_LENGTH:
                removed[m.group(1)] = True
    gone = {n for n in removed if not (repo.has_flag(n) if n.startswith("--") else repo.has_name(n))}

    docs = find_docs(repo, None)
    impacted = []
    for rel in docs:
        text = repo.text_of(rel)
        mentions = sorted(p for p in changed if p in text or (p.count("/") == 0 and re.search(r"\b%s\b" % re.escape(p), text)))
        stale_names = sorted(n for n in gone if re.search(r"(?<![\w-])%s(?![\w-])" % re.escape(n), text))
        if mentions or stale_names:
            impacted.append({"doc": rel, "already_changed": rel in doc_changed, "mentions_changed_files": mentions[:8], "mentions_removed_names": stale_names[:8]})
    impacted.sort(key=lambda d: (d["already_changed"], -len(d["mentions_removed_names"]), -len(d["mentions_changed_files"])))
    return {"repo": repo.root, "ref": ref, "code_files_changed": len(changed), "docs_changed": sorted(doc_changed),
            "names_removed_from_code": sorted(gone), "impacted_docs": impacted}, None


def render_changed(res):
    print("Compared with %s: %d code files changed, %d docs changed." % (res["ref"], res["code_files_changed"], len(res["docs_changed"])))
    if res["names_removed_from_code"]:
        print("Names removed from code: %s" % ", ".join(res["names_removed_from_code"][:20]))
    todo = [d for d in res["impacted_docs"] if not d["already_changed"] or d["mentions_removed_names"]]
    if not todo:
        print("No unchanged doc mentions a changed file or a removed name.")
        return
    print("Docs to check:")
    for d in todo:
        state = "changed in this diff, still mentions removed names" if d["already_changed"] else "not changed in this diff"
        print("  %s (%s)" % (d["doc"], state))
        if d["mentions_removed_names"]:
            print("    still mentions removed: %s" % ", ".join(d["mentions_removed_names"]))
        if d["mentions_changed_files"]:
            print("    mentions changed files: %s" % ", ".join(d["mentions_changed_files"]))


def render(res, limit):
    c = res["counts"]
    print("Audited %d docs in %s: %d errors, %d warnings, %d info.%s" % (
        res["docs_checked"], res["repo"], c["error"], c["warning"], c["info"], "" if res["git_available"] else " Git history unavailable, so drift checks did not run."))
    print("\nCoverage (expected means the repo's contents call for it):")
    for row in res["coverage"]:
        state = "found: %s" % ", ".join(row["found"]) if row["found"] else ("MISSING" if row["expected"] else "not expected")
        print("  %-28s %s" % (row["type"], state))
    print("\nDocuments, worst first (score is AI-writing signal; days since last commit):")
    print("  %-6s %-5s %-5s %-6s %-6s %s" % ("Errors", "Warn", "Score", "Days", "Drift", "Doc"))
    ranked = sorted(res["documents"], key=lambda d: (-d["errors"], -d["warnings"], -d["score"]))
    for d in ranked[:25]:
        days = "-" if d["days_since_commit"] is None else str(d["days_since_commit"])
        print("  %-6d %-5d %-5.1f %-6s %-6d %s" % (d["errors"], d["warnings"], d["score"], days, d["referenced_files_changed_since"], d["doc"]))
    if len(ranked) > 25:
        print("  ... %d more docs" % (len(ranked) - 25))
    shown = res["findings"] if limit == 0 else res["findings"][:limit]
    print("\nFindings (leads to confirm, most severe first):")
    for f in shown:
        loc = "%s:%d" % (f["doc"], f["line"]) if f["line"] else f["doc"]
        print("  %-7s %-16s %s  %s -> %s" % (f["severity"], f["kind"], loc, f["item"], f["message"]))
    if len(shown) < len(res["findings"]):
        print("  ... %d more; rerun with --limit 0 or --json" % (len(res["findings"]) - len(shown)))


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("repo", help="repository root")
    ap.add_argument("--docs", nargs="*", help="files or directories to audit (default: discover prose files)")
    ap.add_argument("--changed-since", metavar="REF", help="list docs affected by the diff from REF to HEAD and the working tree")
    ap.add_argument("--stale-days", type=int, default=DEFAULT_STALE_DAYS, help="flag docs untouched this long (default %(default)s)")
    ap.add_argument("--fail-on", choices=["error", "warning", "never"], default="never", help="exit 1 at or above this severity")
    ap.add_argument("--json", action="store_true", help="emit JSON on stdout")
    ap.add_argument("--limit", type=int, default=DEFAULT_LIMIT, help="max findings to print; 0 for all (default %(default)s)")
    args = ap.parse_args(argv)
    if not os.path.isdir(args.repo):
        print("error: not a directory: %s" % args.repo, file=sys.stderr)
        return 2
    repo = doclib.RepoIndex(args.repo)
    if args.changed_since:
        res, err = changed_since(repo, args.changed_since)
        if err:
            print("error: %s. Check the ref name, or fetch it first." % err, file=sys.stderr)
            return 2
        if args.json:
            print(json.dumps(res, indent=2))
        else:
            render_changed(res)
        missed = [d for d in res["impacted_docs"] if d["mentions_removed_names"]]
        return 1 if (missed and args.fail_on != "never") else 0
    res = audit(repo, args.docs, args.stale_days)
    if res["docs_checked"] == 0 and args.docs:
        print("error: no Markdown files found in: %s" % ", ".join(args.docs), file=sys.stderr)
        return 2
    if args.json:
        print(json.dumps(res, indent=2))
    else:
        render(res, args.limit)
    if args.fail_on == "error" and res["counts"]["error"]:
        return 1
    if args.fail_on == "warning" and (res["counts"]["error"] or res["counts"]["warning"]):
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
