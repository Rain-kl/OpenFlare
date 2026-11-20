#!/usr/bin/env python3
"""Inventory a repository in documentation reading order.

Usage:
    repo_inventory.py REPO [--json] [--all] [--limit N] [--max-files N]

Examples:
    repo_inventory.py .
    repo_inventory.py ../service --all
    repo_inventory.py . --json > /tmp/inventory.json

Lists, in reading order: languages, doc tooling, instruction files, existing docs,
manifests, runtime and deploy config, entry points, route, CLI, env var, and dependency
hints, tests, source layout, and a git summary. Long sections collapse repeated files
into one line so the output fits an agent's context; --all or --json shows every path.

Uses git to list files when possible, so .gitignore applies, and always skips vendored,
generated, and build directories. Read-only. Prints only the names of secret and
environment files, never their values.

Exit codes: 0 finished; 2 not a directory. Standard library only.
"""
import argparse
import json
import os
import re
import sys

import doclib

# Keeps default output readable; agent harnesses often truncate tool output past 10,000 to
# 30,000 characters, and a large monorepo would otherwise exceed that.
DEFAULT_SECTION_LIMIT = 20
MAX_SCANNED_SOURCE_FILES = 4000
MAX_SCANNED_FILE_BYTES = 400000

INSTRUCTION_NAMES = {
    "AGENTS.md", "CLAUDE.md", "GEMINI.md", ".cursorrules", ".windsurfrules", ".clinerules", "CONVENTIONS.md",
    "CONTRIBUTING.md", "CODEOWNERS", "SECURITY.md", "CODE_OF_CONDUCT.md", ".editorconfig", "STYLEGUIDE.md",
    "STYLE_GUIDE.md", "PULL_REQUEST_TEMPLATE.md", "pull_request_template.md",
}
INSTRUCTION_DIRS = (".claude/agents", ".claude/commands", ".claude/skills", ".agents/skills", ".cursor/rules", ".github/instructions", ".windsurf/rules")
DOC_TOOLING = {
    "mkdocs.yml": "MkDocs", "mkdocs.yaml": "MkDocs", "docusaurus.config.js": "Docusaurus", "docusaurus.config.ts": "Docusaurus",
    "conf.py": "Sphinx", "antora.yml": "Antora", ".readthedocs.yaml": "Read the Docs", ".readthedocs.yml": "Read the Docs",
    "book.toml": "mdBook", "astro.config.mjs": "Astro", "docs.json": "Mintlify", "mint.json": "Mintlify", "typedoc.json": "TypeDoc",
    ".vale.ini": "Vale", ".markdownlint.json": "markdownlint", ".markdownlint.yaml": "markdownlint",
    ".markdownlint-cli2.jsonc": "markdownlint", ".markdownlint-cli2.yaml": "markdownlint", "lychee.toml": "lychee",
    "cspell.json": "CSpell", ".cspell.json": "CSpell", "docfx.json": "DocFX", "Doxyfile": "Doxygen", "llms.txt": "llms.txt",
    "defaults.yaml": "pandoc defaults (possible)", "reference.docx": "pandoc reference doc (possible)",
}
DOC_NAMES = {"README", "CHANGELOG", "HISTORY", "UPGRADING", "MIGRATION", "ARCHITECTURE", "DESIGN", "RUNBOOK", "OPERATIONS", "ROADMAP", "GLOSSARY"}
DOC_DIRS = {"docs", "doc", "documentation", "wiki", "adr", "adrs", "rfcs", "rfc", "design", "decisions", "runbooks"}
DOC_FILE_SUFFIXES = (".md", ".mdx", ".rst", ".adoc", ".txt", ".mmd", ".puml", ".drawio", ".excalidraw", ".d2")
CONTRACT_NAMES = {"openapi.yaml", "openapi.yml", "openapi.json", "swagger.json", "swagger.yaml", "asyncapi.yaml", "asyncapi.yml", "schema.graphql"}
MANIFESTS = {
    "package.json", "pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile", "environment.yml", "go.mod",
    "Cargo.toml", "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "Gemfile",
    "composer.json", "Makefile", "justfile", "Taskfile.yml", "tox.ini", "noxfile.py", "pytest.ini", "tsconfig.json",
    "turbo.json", "nx.json", "lerna.json", "pnpm-workspace.yaml", "go.work", "dvc.yaml", "MLproject", ".nvmrc",
    ".python-version", ".tool-versions", "global.json", "Package.swift", "Podfile", "Directory.Build.props", "WORKSPACE", "MODULE.bazel",
}
MANIFEST_SUFFIXES = (".csproj", ".fsproj", ".vbproj", ".sln", ".gemspec")
CONFIG_NAMES = {
    "Dockerfile", ".env.example", ".env.sample", ".env.template", "Procfile", "serverless.yml", "template.yaml", "template.yml",
    "cdk.json", "samconfig.toml", "buildspec.yml", "Jenkinsfile", ".gitlab-ci.yml", "azure-pipelines.yml", "Chart.yaml", "values.yaml",
    "kustomization.yaml", "nginx.conf", "envoy.yaml", "app.yaml", "vercel.json", "netlify.toml", "fly.toml", "render.yaml", "wrangler.toml",
}
CONFIG_DIRS = (".github/workflows", ".circleci", "k8s", "kubernetes", "helm", "terraform", "infra", "deploy", "config", "configs", "ansible", "pulumi")
CONFIG_DIR_SUFFIXES = (".yml", ".yaml", ".json", ".toml", ".ini", ".tf", ".conf", ".env", ".hcl")
ENTRY_NAMES = {
    "main.py", "__main__.py", "app.py", "manage.py", "wsgi.py", "asgi.py", "server.py", "cli.py", "index.js", "index.ts", "main.ts", "main.js",
    "server.js", "server.ts", "app.js", "app.ts", "main.go", "main.rs", "lib.rs", "Program.cs", "Startup.cs", "Main.java", "Application.java",
    "handler.py", "lambda_function.py", "handler.js", "handler.ts", "config.ru", "App.swift", "AppDelegate.swift", "MainActivity.kt",
}
TEST_DIR_NAMES = {"test", "tests", "__tests__", "spec", "specs", "e2e", "integration", "cypress"}
TEST_FILE_RE = re.compile(r"(?:^test_.*\.py$|.*_test\.(?:py|go)$|.*\.(?:test|spec)\.(?:js|ts|jsx|tsx)$|.*(?:Test|Tests)\.(?:java|kt|cs)$|.*_spec\.rb$)")
AI_HINTS = {
    "prompts": re.compile(r"(?:^|/)prompts?(?:/|$)|\.prompt$|\.jinja2?$|system_prompt", re.I),
    "evals": re.compile(r"(?:^|/)(?:evals?|evaluation|benchmarks?)(?:/|$)", re.I),
    "agents_tools": re.compile(r"(?:^|/)(?:agents?|tools|mcp)(?:/|$)", re.I),
    "retrieval": re.compile(r"(?:^|/)(?:rag|retriev\w*|embeddings?|vectorstores?|indexing)(?:/|$)", re.I),
    "notebooks": re.compile(r"\.ipynb$"),
}
ROUTE_RES = [
    re.compile(r"@(?:app|router|bp|blueprint|api)\.(?:get|post|put|patch|delete|route)\(\s*['\"]([^'\"]+)"),
    re.compile(r"\b(?:app|router|server)\.(?:get|post|put|patch|delete|use)\(\s*['\"`]([^'\"`]+)"),
    re.compile(r"@(?:Get|Post|Put|Patch|Delete|Request)Mapping\(\s*(?:value\s*=\s*)?\{?\s*['\"]([^'\"]+)"),
    re.compile(r"@(?:Get|Post|Put|Patch|Delete)\(\s*['\"`]([^'\"`]*)"),
    re.compile(r"\.(?:GET|POST|PUT|PATCH|DELETE|HandleFunc|Handle)\(\s*\"([^\"]+)"),
    re.compile(r"\.Map(?:Get|Post|Put|Patch|Delete|Group)\(\s*\"([^\"]+)"),
    re.compile(r"\[(?:Http(?:Get|Post|Put|Patch|Delete)|Route)\(\s*\"([^\"]*)\""),
    re.compile(r"\bpath\(\s*['\"]([^'\"]*)['\"]\s*,"),
]
ENV_RES = [
    re.compile(r"os\.environ(?:\.get)?\(?\[?\s*['\"]([A-Z][A-Z0-9_]{2,})['\"]"),
    re.compile(r"\bgetenv\(\s*['\"]([A-Z][A-Z0-9_]{2,})['\"]"),
    re.compile(r"process\.env\.([A-Z][A-Z0-9_]{2,})"),
    re.compile(r"process\.env\[\s*['\"]([A-Z][A-Z0-9_]{2,})['\"]"),
    re.compile(r"(?:System\.getenv|os\.Getenv|os\.LookupEnv|std::env::var|env::var|ENV\.fetch|Environment\.GetEnvironmentVariable)\(\s*\"([A-Z][A-Z0-9_]{2,})\""),
    re.compile(r"ENV\[\s*['\"]([A-Z][A-Z0-9_]{2,})['\"]\s*\]"),
]
CLI_HINTS = {
    "argparse": r"argparse\.ArgumentParser", "click": r"@click\.(?:command|group)", "typer": r"typer\.Typer", "cobra": r"cobra\.Command",
    "commander": r"from ['\"]commander['\"]|require\(['\"]commander['\"]\)", "yargs": r"from ['\"]yargs|require\(['\"]yargs",
    "clap": r"clap::", "picocli": r"picocli\.", "System.CommandLine": r"System\.CommandLine",
}
BOUNDARY_HINTS = {
    "postgres": r"psycopg|asyncpg|\bpg\b|Npgsql|postgresql://", "mysql": r"pymysql|mysql2|MySqlConnection", "sqlite": r"sqlite3",
    "orm": r"sqlalchemy|prisma|typeorm|sequelize|EntityFramework|DbContext|gorm\.", "redis": r"\bredis\b|StackExchange\.Redis|ioredis",
    "dynamodb": r"dynamodb", "s3": r"\bs3\b|S3Client|AmazonS3", "sqs_sns": r"\bsqs\b|\bsns\b|SQSClient|SnsClient|AmazonSQS", "kafka": r"kafka",
    "rabbitmq": r"\bpika\b|amqp|rabbitmq", "celery": r"\bcelery\b", "http_client": r"\brequests\.(?:get|post)|httpx\.|\baxios\b|\bfetch\(|HttpClient",
    "grpc": r"\bgrpc\b", "bedrock": r"bedrock", "openai": r"\bopenai\b", "anthropic": r"\banthropic\b", "langchain": r"langchain|langgraph",
    "scheduler": r"@Scheduled|BackgroundService|\bcron\b|APScheduler|EventBridge",
    "http_server": r"BaseHTTPRequestHandler|\bFastAPI\(|\bFlask\(|\bexpress\(\)|http\.ListenAndServe|WebApplication\.CreateBuilder",
}
LANGUAGES = {
    ".py": "Python", ".js": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript", ".jsx": "JavaScript", ".ts": "TypeScript", ".tsx": "TypeScript",
    ".go": "Go", ".rs": "Rust", ".java": "Java", ".kt": "Kotlin", ".kts": "Kotlin", ".rb": "Ruby", ".php": "PHP", ".cs": "C#", ".fs": "F#", ".swift": "Swift",
    ".c": "C", ".h": "C/C++", ".cc": "C++", ".cpp": "C++", ".hpp": "C++", ".scala": "Scala", ".sh": "Shell", ".tf": "Terraform", ".sql": "SQL",
    ".dart": "Dart", ".ex": "Elixir", ".exs": "Elixir", ".lua": "Lua", ".vue": "Vue", ".svelte": "Svelte", ".ps1": "PowerShell", ".ipynb": "Notebook",
}
CODE_SUFFIXES = tuple(k for k in LANGUAGES if k not in (".tf", ".sql", ".ipynb"))
# Files named anything can be programs, so also look for the language's own start-up marker.
MAIN_MARKER_RE = re.compile(
    r"""^if __name__ == ['"]__main__['"]:|^func main\(\)|^fn main\(\)|\bstatic void [Mm]ain\(|@SpringBootApplication|\brequire\.main === module""",
    re.M)


def under(rel, prefix):
    return rel == prefix or rel.startswith(prefix.rstrip("/") + "/") or ("/" + prefix.rstrip("/") + "/") in ("/" + rel)


def size_of(repo, rel):
    try:
        return os.path.getsize(os.path.join(repo, rel))
    except OSError:
        return 0


def package_summary(repo):
    p = os.path.join(repo, "package.json")
    if not os.path.isfile(p):
        return {}
    try:
        data = json.loads(doclib.read_text(p))
    except ValueError:
        return {}
    return {
        "name": data.get("name"), "version": data.get("version"), "main": data.get("main"), "bin": data.get("bin"),
        "engines": data.get("engines"), "scripts": data.get("scripts", {}), "workspaces": data.get("workspaces"),
        "dependencies": sorted((data.get("dependencies") or {}).keys())[:40],
    }


def pyproject_summary(repo):
    p = os.path.join(repo, "pyproject.toml")
    if not os.path.isfile(p):
        return {}
    t = doclib.read_text(p)
    info = {}
    m = re.search(r"^name\s*=\s*\"([^\"]+)\"", t, re.M)
    if m:
        info["name"] = m.group(1)
    m = re.search(r"requires-python\s*=\s*\"([^\"]+)\"", t)
    if m:
        info["requires_python"] = m.group(1)
    m = re.search(r"\[project\.scripts\](.*?)(?:\n\[|\Z)", t, re.S)
    if m:
        info["console_scripts"] = [ln.strip() for ln in m.group(1).strip().splitlines() if "=" in ln]
    return info


def makefile_targets(repo):
    p = os.path.join(repo, "Makefile")
    if not os.path.isfile(p):
        return []
    targets = set(re.findall(r"^([A-Za-z0-9_][A-Za-z0-9_.-]*)\s*:(?!=)", doclib.read_text(p), re.M))
    return sorted(targets)[:40]


def env_example_names(repo, files):
    names = {}
    for f in files:
        base = f.rsplit("/", 1)[-1]
        if base in (".env.example", ".env.sample", ".env.template"):
            for m in re.finditer(r"^\s*(?:export\s+)?([A-Z][A-Z0-9_]{2,})\s*=", doclib.read_text(os.path.join(repo, f)), re.M):
                names.setdefault(m.group(1), []).append(f)
    return names


def inventory(repo, max_files=doclib.MAX_LISTED_FILES):
    files, truncated, source = doclib.list_repo_files(repo, max_files)
    inv = {"repo": os.path.abspath(repo), "truncated": truncated, "file_count": len(files), "file_source": source}

    langs = {}
    for f in files:
        lang = LANGUAGES.get(os.path.splitext(f)[1].lower())
        if lang:
            langs[lang] = langs.get(lang, 0) + 1
    inv["languages"] = dict(sorted(langs.items(), key=lambda x: -x[1])[:8])
    inv["doc_tooling"] = sorted({"%s (%s)" % (label, f) for f in files for name, label in DOC_TOOLING.items() if f.rsplit("/", 1)[-1] == name and f.count("/") <= 2})

    inv["1_instructions"] = sorted(
        {f for f in files if f.rsplit("/", 1)[-1] in INSTRUCTION_NAMES and f.count("/") <= 3}
        | {f for f in files if f == ".github/copilot-instructions.md"}
        | {f for f in files if any(under(f, d) for d in INSTRUCTION_DIRS) and f.lower().endswith((".md", ".mdc"))}
    )

    docs = set()
    for f in files:
        base = f.rsplit("/", 1)[-1]
        stem = os.path.splitext(base)[0].upper()
        if stem in DOC_NAMES and f.count("/") <= 3 and base.lower().endswith((".md", ".rst", ".adoc", ".txt", "")):
            docs.add(f)
        if any(p.lower() in DOC_DIRS for p in f.split("/")[:-1]) and f.lower().endswith(DOC_FILE_SUFFIXES):
            docs.add(f)
        if base.lower().endswith((".proto", ".graphql")) or base in CONTRACT_NAMES:
            docs.add(f)
    inv["2_existing_docs"] = sorted(docs)

    inv["3_manifests"] = sorted(f for f in files if f.rsplit("/", 1)[-1] in MANIFESTS or f.endswith(MANIFEST_SUFFIXES))
    inv["3_manifest_summary"] = {"package_json": package_summary(repo), "pyproject": pyproject_summary(repo), "makefile_targets": makefile_targets(repo)}

    cfg = set()
    for f in files:
        base = f.rsplit("/", 1)[-1]
        if base in CONFIG_NAMES or base.startswith("Dockerfile.") or re.match(r"(?:docker-)?compose.*\.ya?ml$", base) or base.endswith((".tf", ".tfvars.example")):
            cfg.add(f)
        if any(under(f, d) for d in CONFIG_DIRS) and f.lower().endswith(CONFIG_DIR_SUFFIXES):
            cfg.add(f)
    inv["4_runtime_and_deploy_config"] = sorted(cfg)
    # Git may hide ignored secret files, so check the disk too; names only, never contents.
    secrets = {f for f in files if re.match(r"\.env(\..+)?$", f.rsplit("/", 1)[-1]) and f.rsplit("/", 1)[-1] not in (".env.example", ".env.sample", ".env.template")}
    if os.path.isfile(os.path.join(repo, ".env")):
        secrets.add(".env")
    inv["4_secret_files_names_only"] = sorted(secrets)

    entries = set()
    for f in files:
        base = f.rsplit("/", 1)[-1]
        if (base in ENTRY_NAMES and f.count("/") <= 5) or re.match(r"cmd/[^/]+/main\.go$", f) or (f.startswith("bin/") and f.count("/") == 1):
            entries.add(f)
    routes, envs, cli, boundary = {}, {}, {}, {}
    cli_res = {k: re.compile(v) for k, v in CLI_HINTS.items()}
    boundary_res = {k: re.compile(v, re.I) for k, v in BOUNDARY_HINTS.items()}
    scanned = 0
    for f in files:
        if not f.endswith(CODE_SUFFIXES) or size_of(repo, f) > MAX_SCANNED_FILE_BYTES:
            continue
        if TEST_FILE_RE.match(f.rsplit("/", 1)[-1]) or any(part in TEST_DIR_NAMES for part in f.split("/")[:-1]):
            continue
        scanned += 1
        if scanned > MAX_SCANNED_SOURCE_FILES:
            break
        text = doclib.read_text(os.path.join(repo, f))
        if MAIN_MARKER_RE.search(text) and f.count("/") <= 6:
            entries.add(f)
        for rx in ROUTE_RES:
            for m in rx.finditer(text):
                routes.setdefault(m.group(1), set()).add(f)
        for rx in ENV_RES:
            for m in rx.finditer(text):
                envs.setdefault(m.group(1), set()).add(f)
        for name, rx in cli_res.items():
            if rx.search(text):
                cli.setdefault(name, set()).add(f)
        for name, rx in boundary_res.items():
            if rx.search(text):
                boundary.setdefault(name, set()).add(f)
    inv["5_entry_points"] = sorted(entries)
    inv["5_route_hints"] = {k: sorted(v)[:3] for k, v in sorted(routes.items())[:80]}
    inv["5_env_vars_read"] = {k: sorted(v)[:3] for k, v in sorted(envs.items())[:80]}
    inv["5_env_vars_in_examples"] = sorted(env_example_names(repo, files))
    inv["5_cli_hints"] = {k: sorted(v)[:3] for k, v in sorted(cli.items())}
    inv["5_boundary_hints"] = {k: {"files": len(v), "sample": sorted(v)[:2]} for k, v in sorted(boundary.items(), key=lambda x: -len(x[1]))}

    tests = [f for f in files if TEST_FILE_RE.match(f.rsplit("/", 1)[-1]) or any(p in TEST_DIR_NAMES for p in f.split("/")[:-1])]
    inv["6_tests"] = {"count": len(tests), "sample": sorted(tests)[:25]}

    layout = {}
    for f in files:
        key = f.split("/")[0] if "/" in f else "."
        layout[key] = layout.get(key, 0) + 1
    inv["7_source_layout"] = dict(sorted(layout.items(), key=lambda x: -x[1])[:30])

    ai = {}
    for label, rx in AI_HINTS.items():
        m = [f for f in files if rx.search(f)]
        if m:
            ai[label] = sorted(m)[:15]
    inv["7_ai_ml_hints"] = ai
    large = sorted(((size_of(repo, f), f) for f in files if f.endswith(CODE_SUFFIXES)), reverse=True)[:8]
    inv["7_largest_source_files"] = [{"path": f, "bytes": s} for s, f in large]

    head = doclib.run_git(repo, "rev-parse", "--short", "HEAD")
    if head:
        churn = {}
        for f in (doclib.run_git(repo, "log", "--since=6 months ago", "--name-only", "--pretty=format:") or "").splitlines():
            if f:
                churn[f] = churn.get(f, 0) + 1
        inv["8_git"] = {
            "branch": (doclib.run_git(repo, "rev-parse", "--abbrev-ref", "HEAD") or "").strip(),
            "head": head.strip(),
            "recent_commits": (doclib.run_git(repo, "log", "--oneline", "-15") or "").splitlines(),
            "top_authors": (doclib.run_git(repo, "shortlog", "-sn", "--no-merges", "HEAD") or "").splitlines()[:5],
            "churn_last_6_months": sorted(churn.items(), key=lambda x: -x[1])[:12],
        }
    else:
        inv["8_git"] = {}
    return inv


def collapse(paths, limit):
    """Merge paths that repeat a file name or fill a directory, then cap the list."""
    if limit == 0 or len(paths) <= limit:
        return paths, 0
    by_base, by_dir = {}, {}
    for p in paths:
        by_base.setdefault(p.rsplit("/", 1)[-1], []).append(p)
    lines, used = [], set()
    for base, group in sorted(by_base.items(), key=lambda x: -len(x[1])):
        if len(group) >= 4:
            dirs = [os.path.dirname(g) or "." for g in group]
            lines.append("%s (%d copies): %s" % (base, len(group), summarize_dirs(dirs)))
            used.update(group)
    by_suffix = {}
    for p in paths:
        if p not in used:
            by_suffix.setdefault(os.path.splitext(p)[1], []).append(p)
    for suffix, group in by_suffix.items():
        if suffix and len(group) >= 8:
            lines.append("*%s (%d files): %s" % (suffix, len(group), summarize_dirs([os.path.dirname(g) or "." for g in group], 3)))
            used.update(group)
    for p in paths:
        if p not in used:
            by_dir.setdefault(os.path.dirname(p) or ".", []).append(p)
    for d, group in sorted(by_dir.items()):
        if len(group) >= 4:
            exts = sorted({os.path.splitext(g)[1] or g.rsplit("/", 1)[-1] for g in group})
            lines.append("%s/ (%d files: %s)" % (d, len(group), ", ".join(exts[:5])))
        else:
            lines.extend(group)
    lines = sorted(lines)
    hidden = max(0, len(lines) - limit)
    return lines[:limit], hidden


def summarize_dirs(dirs, show=3):
    head = ", ".join(sorted(dirs)[:show])
    return head + (", and %d more" % (len(dirs) - show) if len(dirs) > show else "")


def render(inv, limit):
    o = ["Repo: %s   files: %d%s   listed by: %s" % (inv["repo"], inv["file_count"], " (truncated)" if inv["truncated"] else "", inv["file_source"])]
    o.append("Languages: " + (", ".join("%s %d" % kv for kv in inv["languages"].items()) or "none detected"))
    o.append("Doc tooling: " + (", ".join(inv["doc_tooling"]) or "none detected"))

    def section(title, items, collapsible=True):
        o.append("")
        o.append(title)
        if not items:
            o.append("  (none found)")
            return
        lines, hidden = collapse(items, limit) if collapsible else (items, 0)
        o.extend("  " + str(i) for i in lines)
        if hidden:
            o.append("  ... %d more lines; rerun with --all" % hidden)

    section("1. Instructions for humans and agents (read fully)", inv["1_instructions"])
    section("2. Existing docs (treat as claims; verify)", inv["2_existing_docs"])
    section("3. Manifests and build definitions", inv["3_manifests"])
    ms = inv["3_manifest_summary"]
    if ms["package_json"]:
        pj = ms["package_json"]
        o.append("  package.json: name=%s version=%s main=%s engines=%s" % (pj.get("name"), pj.get("version"), pj.get("main"), pj.get("engines")))
        for k, v in (pj.get("scripts") or {}).items():
            o.append("    npm run %s -> %s" % (k, v))
    if ms["pyproject"]:
        o.append("  pyproject: %s" % ms["pyproject"])
    if ms["makefile_targets"]:
        o.append("  root make targets: %s" % ", ".join(ms["makefile_targets"]))
    section("4. Runtime and deploy config", inv["4_runtime_and_deploy_config"])
    if inv["4_secret_files_names_only"]:
        o.append("  Secret files (names only; never copy values): %s" % ", ".join(inv["4_secret_files_names_only"]))
    section("5. Entry points", inv["5_entry_points"])
    def capped(items):
        return items if not limit or len(items) <= limit else items[:limit] + ["... %d more; rerun with --all" % (len(items) - limit)]

    section("5b. Route hints (verify in code)", capped(["%s  (%s)" % (k, v[0]) for k, v in inv["5_route_hints"].items()]), collapsible=False)
    section("5c. Env vars read in code", capped(["%s  (%s)" % (k, v[0]) for k, v in inv["5_env_vars_read"].items()]), collapsible=False)
    if inv["5_env_vars_in_examples"]:
        o.append("  In .env examples: %s" % ", ".join(inv["5_env_vars_in_examples"]))
    if inv["5_cli_hints"]:
        section("5d. CLI framework hints", ["%s: %s" % (k, ", ".join(v)) for k, v in inv["5_cli_hints"].items()], collapsible=False)
    if inv["5_boundary_hints"]:
        section("5e. Boundary hints (keyword matches in source; verify)", ["%s: %d files, for example %s" % (k, v["files"], ", ".join(v["sample"])) for k, v in inv["5_boundary_hints"].items()], collapsible=False)
    t = inv["6_tests"]
    section("6. Tests (%d files)" % t["count"], t["sample"], collapsible=False)
    section("7. Source layout (files per top-level dir)", ["%s: %d" % kv for kv in inv["7_source_layout"].items()], collapsible=False)
    if inv["7_ai_ml_hints"]:
        section("7b. AI and ML hints", ["%s: %s" % (k, ", ".join(v[:6])) for k, v in inv["7_ai_ml_hints"].items()], collapsible=False)
    section("7c. Largest source files (grep, then read ranges)", ["%s (%d bytes)" % (x["path"], x["bytes"]) for x in inv["7_largest_source_files"]], collapsible=False)
    g = inv["8_git"]
    if g:
        hot = "hot files (6 months): " + ", ".join("%s(%d)" % tuple(x) for x in g["churn_last_6_months"][:8])
        section("8. Git", ["branch %s @ %s" % (g["branch"], g["head"])] + g["recent_commits"][:8] + [hot], collapsible=False)
    return "\n".join(o)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("repo", help="path to the repository root")
    ap.add_argument("--json", action="store_true", help="emit full JSON")
    ap.add_argument("--all", action="store_true", help="list every path instead of collapsing long sections")
    ap.add_argument("--limit", type=int, default=DEFAULT_SECTION_LIMIT, help="max lines per section (default %(default)s)")
    ap.add_argument("--max-files", type=int, default=doclib.MAX_LISTED_FILES, help="stop listing after this many files")
    args = ap.parse_args(argv)
    if not os.path.isdir(args.repo):
        print("error: not a directory: %s" % args.repo, file=sys.stderr)
        return 2
    inv = inventory(args.repo, args.max_files)
    print(json.dumps(inv, indent=2) if args.json else render(inv, 0 if args.all else args.limit))
    return 0


if __name__ == "__main__":
    sys.exit(main())
