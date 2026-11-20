#!/usr/bin/env python3
"""Check Markdown docs against a repository: links, paths, names, flags, and commands.

Usage:
    verify_refs.py DOC_OR_DIR [DOC_OR_DIR ...] --repo REPO [options]

Examples:
    verify_refs.py README.md --repo .
    verify_refs.py docs --repo . --strict
    verify_refs.py draft.md --repo . --commands --json
    verify_refs.py draft.md --repo . --ledger /tmp/ledger.md

Checks:
    links and images    relative targets exist; anchors match headings; alt text present
    paths in backticks  exist relative to the doc or repo root; path:line is within the file
    commands            make, just, task, npm/yarn/pnpm/bun, compose services, local scripts,
                        python -m, and flags passed to local scripts; follows cd in a block
    names in backticks  env vars and identifiers appear outside docs, so a doc cannot
                        vouch for itself
    ledger              with --ledger, each path:line cited as evidence resolves
    URLs                with --check-urls only, because it uses the network

Severity: error means the reference is broken. warning means it could not be found and
needs a look. info is a style note or a weak signal. A hit means "could not find", so
confirm by hand before concluding the doc is wrong. This script never executes commands.

Exit codes: 0 finished; 1 errors or warnings found and --strict was given; 2 bad usage.
Standard library only.
"""
import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor

import doclib

# Output stays small enough that agent harnesses do not truncate it; --limit 0 shows all.
DEFAULT_LIMIT = 60
URL_TIMEOUT_SECONDS = 15
URL_WORKERS = 8
SEVERITY_ORDER = {"error": 0, "warning": 1, "info": 2}

EXTENSIONS = (
    "py|pyi|js|mjs|cjs|ts|tsx|jsx|go|rs|java|kt|kts|rb|php|cs|csproj|sln|fs|swift|c|h|cc|cpp|hpp|scala|"
    "sh|bash|zsh|ps1|bat|ya?ml|json|jsonc|toml|md|mdx|rst|adoc|txt|cfg|ini|conf|env|tf|tfvars|hcl|sql|"
    "proto|graphql|html|css|scss|xml|gradle|properties|lock|lua|ipynb|csv|mmd|puml|drawio|service|example|sample"
)
PATH_RE = re.compile(r"^(?:\./|\.\./)*[\w@.\-]+(?:/[\w@.\-*{}?]+)*/?$")
HAS_EXT_RE = re.compile(r"\.(?:%s)$" % EXTENSIONS, re.I)
PATH_LINE_RE = re.compile(r"^((?:\./)?[\w@.\-/]+\.\w+):(\d+)(?:[-:](\d+))?$")
IDENT_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*(?:(?:\.|::|->)[A-Za-z_][A-Za-z0-9_]*)*(?:\(\))?$")
ENV_RE = re.compile(r"^[A-Z][A-Z0-9_]{3,}$")
FLAG_RE = re.compile(r"^--[a-zA-Z][\w-]*$")
MIME_PREFIXES = ("application/", "text/", "image/", "audio/", "video/", "multipart/", "font/")
EXTERNAL_HOSTS = ("example.com", "example.org", "example.net", "localhost", "127.0.0.1", "0.0.0.0")
WEAK_LINK_TEXT = {"here", "click here", "this", "this link", "link", "read more", "more", "see here"}

COMMON_WORDS = {
    "true", "false", "null", "none", "nil", "undefined", "string", "str", "int", "integer", "bool", "boolean",
    "float", "list", "dict", "array", "object", "json", "yaml", "toml", "http", "https", "get", "post", "put",
    "patch", "delete", "main", "test", "tests", "src", "docs", "self", "this", "args", "kwargs", "id", "name",
    "type", "value", "key", "data", "error", "result", "response", "request", "config", "env", "master", "branch",
    "default", "status", "text", "body", "path", "file", "user", "users", "admin", "root",
}
KNOWN_TOOLS = {
    "git", "npm", "npx", "pnpm", "yarn", "bun", "node", "python", "python3", "pip", "pip3", "pipx", "uv", "uvx",
    "poetry", "pytest", "tox", "nox", "make", "just", "task", "docker", "podman", "kubectl", "helm", "terraform",
    "aws", "gcloud", "az", "curl", "wget", "grep", "sed", "awk", "jq", "bash", "zsh", "sudo", "ssh", "scp", "tar",
    "gzip", "go", "cargo", "rustc", "java", "javac", "mvn", "gradle", "dotnet", "ruby", "gem", "bundle", "rake",
    "php", "composer", "brew", "apt", "winget", "choco", "pandoc", "typst", "vale", "mmdc", "lychee", "gh", "postgres",
    "psql", "mysql", "redis", "sqlite", "nginx", "cron", "systemctl", "journalctl",
}
COMMON_ENV = {
    "PATH", "HOME", "USER", "PWD", "SHELL", "LANG", "TERM", "TMPDIR", "DEBUG", "EDITOR", "PYTHONPATH", "NODE_ENV",
    "POST", "PATCH", "DELETE", "HEAD", "JSON", "YAML", "HTTP", "HTTPS", "TODO", "NOTE", "WARNING", "SELECT", "FROM",
    "WHERE", "NULL", "TRUE", "FALSE", "README", "CHANGELOG", "LICENSE", "AGENTS", "CLAUDE", "CONTRIBUTING", "SECURITY",
    "NEVER", "ALWAYS", "ERROR", "INFO", "WARN", "CRITICAL", "FATAL", "TRACE", "UUID", "UTC", "AWS", "CORS", "CSRF",
    "OAUTH", "SAML", "REST", "GRPC", "HTML", "ASCII", "UTF8", "SIGTERM", "SIGKILL", "SIGINT", "SIGHUP", "MUST", "SHOULD",
    "SHALL", "OPTIONAL", "REQUIRED", "RECOMMENDED",
}
PY_MODULES = {
    "pytest", "pip", "venv", "unittest", "http.server", "json.tool", "doctest", "pydoc", "ensurepip", "build", "twine",
    "mypy", "ruff", "black", "flake8", "isort", "uvicorn", "gunicorn", "celery", "alembic", "flask", "django", "coverage",
}
NPM_BUILTINS = {
    "install", "i", "ci", "add", "init", "exec", "dlx", "create", "update", "up", "upgrade", "remove", "rm", "uninstall",
    "audit", "publish", "pack", "link", "ls", "list", "why", "outdated", "rebuild", "version", "login", "logout", "config",
    "cache", "help", "run", "workspace", "workspaces", "fund", "prune", "dedupe", "x", "bin", "root", "view", "info", "doctor",
}
SHELL_LANGS = {"bash", "sh", "shell", "zsh", "console", "terminal", "shell-session", "shellsession"}
SESSION_LANGS = {"console", "terminal", "shell-session", "shellsession"}
WRAPPERS = {"sudo", "time", "exec", "command", "nohup", "env"}
RUNNER_PREFIXES = (("uv", "run"), ("poetry", "run"), ("pipenv", "run"), ("pdm", "run"), ("hatch", "run"), ("rye", "run"), ("bundle", "exec"))
INTERPRETERS = {"python", "python3", "node", "bash", "sh", "zsh", "ruby", "perl", "php", "deno", "ts-node", "tsx"}
# docker compose flags that take a value, so the value is not read as a service name.
COMPOSE_VALUE_FLAGS = {
    "-f", "--file", "-p", "--project-name", "--profile", "--env-file", "--project-directory", "--tail", "-n", "--since", "--until",
    "--index", "-e", "--env", "-u", "--user", "-w", "--workdir", "--name", "--entrypoint", "--scale", "-t", "--timeout", "-l", "--label",
    "--ansi", "--parallel", "--progress",
}

# Each shell segment gets the first class that matches. Unknown commands are "ask" because a
# task runner target or script can do anything. "never" stays out of doc verification entirely.
NEVER_RE = re.compile(
    r"\brm\s+-[a-z]*[rf]|\bterraform\s+(?:apply|destroy)\b|\bkubectl\s+(?:delete|apply|replace|patch|scale|drain)\b"
    r"|\bhelm\s+(?:install|upgrade|uninstall)\b|\bgit\s+push\b|\b(?:npm|yarn|pnpm|twine|cargo)\s+publish\b"
    r"|\bdocker\s+push\b|\baws\s+\S+\s+(?:delete|terminate|remove|put|update)\S*|\bdrop\s+(?:database|table)\b"
    r"|\bdd\s+if=|\bmkfs\b|\b(?:re)?deploy\b|\bprod(?:uction)?\b", re.I)
PIPE_TO_SHELL_RE = re.compile(r"\b(?:curl|wget)\b.*\|\s*(?:sudo\s+)?(?:ba|z)?sh\b", re.I)
SAFE_RE = re.compile(
    r"^(?:ls|cat|grep|rg|find|head|tail|wc|echo|pwd|which|file|stat|diff|sort|uniq|cut|tree)\b"
    r"|^git\s+(?:log|show|status|diff|blame|shortlog|rev-parse|ls-files|branch|describe)\b"
    r"|(?:^|\s)(?:--help|--version|-h|-V)\s*$"
    r"|^(?:python3?\s+-m\s+)?(?:pytest|unittest|mypy|ruff\s+check|flake8|pyright)\b"
    r"|^(?:npm|pnpm|yarn|bun)\s+(?:run\s+)?(?:test|lint|typecheck|check)\b"
    r"|^(?:make|just|task)\s+(?:test|tests|lint|check|typecheck|help)\b"
    r"|^(?:go\s+(?:test|vet)|cargo\s+(?:test|check|clippy)|dotnet\s+test|mvn\s+test|gradle\s+test|tsc\s+--noEmit|eslint)\b"
    r"|^(?:cd|export|set|source|\.)\b", re.I)


def classify_risk(cmd):
    """Class a command line as safe, ask, or never; the worst shell segment decides."""
    if PIPE_TO_SHELL_RE.search(cmd) or NEVER_RE.search(cmd):
        return "never"
    for part in re.split(r"\s*(?:&&|\|\||;|\|)\s*", cmd):
        part = re.sub(r"^(?:[A-Za-z_]\w*=\S*\s+)+", "", part.strip())
        if part and not SAFE_RE.search(part):
            return "ask"
    return "safe"


def strip_shell_comment(line):
    """Remove a trailing # comment unless the # sits inside quotes."""
    quote = None
    for i, ch in enumerate(line):
        if quote:
            quote = None if ch == quote else quote
        elif ch in "'\"":
            quote = ch
        elif ch == "#" and (i == 0 or line[i - 1].isspace()):
            return line[:i].rstrip()
    return line


def is_placeholder(tok):
    return bool(re.search(r"[<>$]|\{\{|\bYOUR_|\bMY_|\.\.\.|^[A-Z][A-Z0-9_]*$", tok)) or tok.startswith(("path/to", "foo/", "bar/", "your-"))


def split_shell_words(s):
    return re.findall(r"""(?:[^\s"']+|"[^"]*"|'[^']*')+""", s)


def unquote(tok):
    return tok.strip("'\"")


class Checker:
    def __init__(self, repo):
        self.repo = repo
        self.doc_rel_dir = "."
        self.doc_text = ""
        self.problems = []
        self.commands = []
        self.urls = []
        self.tokens_checked = 0

    def add(self, severity, kind, line, item, message):
        self.problems.append({"severity": severity, "kind": kind, "line": line, "item": item, "message": message})

    def resolve(self, cwd, rel):
        return self.repo.norm(os.path.join(cwd, rel)) if cwd not in (".", "") else self.repo.norm(rel)

    # ------------------------------------------------------------ links

    def check_links(self, prose, doc_abs_dir):
        refs = dict(re.findall(r"^ {0,3}\[([^\]]+)\]:\s*<?([^\s>]+)>?", prose, flags=re.M))
        for i, line in enumerate(prose.split("\n"), 1):
            scan = doclib.mask_inline_code(line)
            for m in re.finditer(r"(!?)\[([^\]]*)\]\(\s*<?([^)\s>]+)>?(?:\s+[\"'][^\"']*[\"'])?\s*\)", scan):
                self.check_link(m.group(3), i, doc_abs_dir, m.group(1) == "!", m.group(2))
            for m in re.finditer(r"(!?)\[([^\]]+)\]\[([^\]]*)\]", scan):
                key = m.group(3) or m.group(2)
                if key in refs:
                    self.check_link(refs[key], i, doc_abs_dir, m.group(1) == "!", m.group(2))
                else:
                    self.add("error", "link", i, m.group(0), "reference-style link has no definition")
            for m in re.finditer(r"<img\b([^>]*)>", scan, flags=re.I):
                src = re.search(r"""\bsrc=["']([^"']+)""", m.group(1))
                if src:
                    self.check_link(src.group(1), i, doc_abs_dir, False, "")
                if not re.search(r"\balt=", m.group(1)):
                    self.add("warning", "alt-text", i, "<img>", "image has no alt attribute")
            for m in re.finditer(r"<a\b[^>]*\bhref=[\"']([^\"']+)", scan, flags=re.I):
                self.check_link(m.group(1), i, doc_abs_dir, False, "")

    def check_link(self, target, line, doc_abs_dir, is_image, text):
        if is_image and not text.strip():
            self.add("warning", "alt-text", line, target, "image has no alt text; describe the information it carries")
        if not is_image and text.strip().lower() in WEAK_LINK_TEXT:
            self.add("info", "link-text", line, text, "link text does not name the destination")
        if re.match(r"^(?:mailto:|tel:|data:|ftp:)", target):
            return
        if re.match(r"^https?://", target):
            self.urls.append((line, target))
            return
        path, _, frag = target.partition("#")
        path = path.split("?", 1)[0]
        if not path:
            if frag and frag not in doclib.anchors_of(self.doc_text):
                self.add("error", "anchor", line, target, "no heading in this document produces this anchor")
            return
        candidates = [os.path.normpath(os.path.join(doc_abs_dir, path))]
        if path.startswith("/"):
            candidates = [os.path.normpath(os.path.join(self.repo.root, path.lstrip("/")))]
        full = next((c for c in candidates if os.path.exists(c)), None)
        if full is None:
            similar = self.repo.similar(path)
            hint = " (similar: %s)" % ", ".join(similar) if similar else ""
            self.add("error", "link", line, target, "target does not exist" + hint)
            return
        if frag and os.path.isfile(full) and full.lower().endswith((".md", ".markdown", ".mdx")):
            if frag not in doclib.anchors_of(doclib.read_text(full)):
                self.add("error", "anchor", line, target, "anchor not found in linked file")

    # ------------------------------------------------------------ inline code

    def check_inline(self, prose):
        seen = set()
        for i, line in enumerate(prose.split("\n"), 1):
            for m in re.finditer(r"(?<!`)`([^`\n]+)`(?!`)", line):
                tok = m.group(1).strip()
                if tok in seen or not tok:
                    continue
                seen.add(tok)
                self.tokens_checked += 1
                self.check_token(tok, i)

    def looks_like_command(self, tok):
        words = tok.split()
        if len(words) < 2:
            return False
        first = words[0]
        return (first in ("make", "just", "task", "npm", "yarn", "pnpm", "bun", "docker", "docker-compose")
                or first in INTERPRETERS or first.startswith(("./", "scripts/", "bin/"))
                or (first, words[1]) in RUNNER_PREFIXES)

    def check_token(self, tok, line):
        if self.looks_like_command(tok):
            self.check_command(tok, ".", line, record=False)
            return
        if " " in tok:
            return
        m = PATH_LINE_RE.match(tok)
        if m:
            self.check_path_line(m.group(1), int(m.group(2)), int(m.group(3) or m.group(2)), line, "path-line")
            return
        if FLAG_RE.match(tok):
            if not self.repo.has_flag(tok):
                self.add("info", "flag", line, tok, "flag not found outside docs; confirm which tool it belongs to")
            return
        if ENV_RE.match(tok):
            if tok not in COMMON_ENV and not self.repo.has_name(tok):
                self.add("warning", "env-var", line, tok, "name not found outside docs")
            return
        if tok.startswith(("/", "http:", "https:", "-", "<", "$", "@")) or tok.startswith(MIME_PREFIXES):
            return
        # A bare ".csproj" names a file type, unless a file by that name exists.
        if re.match(r"^\.\w+$", tok) and not self.repo.exists(tok):
            return
        if ("/" in tok or HAS_EXT_RE.search(tok)) and PATH_RE.match(tok):
            self.check_path_token(tok, line)
            return
        if IDENT_RE.match(tok):
            self.check_identifier(tok, line)

    def check_path_token(self, tok, line):
        if is_placeholder(tok):
            return
        clean = tok.rstrip("/")
        if "*" in tok or "{" in tok or "?" in tok:
            if not self.repo.glob_exists(tok) and not self.repo.glob_exists(self.resolve(self.doc_rel_dir, tok)):
                self.add("warning", "path", line, tok, "no files match this pattern")
            return
        if self.repo.exists(clean) or self.repo.exists(self.resolve(self.doc_rel_dir, clean)):
            return
        similar = self.repo.similar(clean)
        hint = " (similar: %s)" % ", ".join(similar) if similar else ""
        if similar and "/" not in clean:
            self.add("info", "path", line, tok, "not at the repo root or next to this doc; give the full path" + hint)
        elif HAS_EXT_RE.search(clean) or clean.split("/")[0] in self.repo.dirs:
            self.add("error", "path", line, tok, "path not found in repo" + hint)
        else:
            # Two bare segments may be a package, owner/repo, or concept such as client/server.
            self.add("info", "path", line, tok, "looks like a path but is not in the repo" + hint)

    def check_path_line(self, path, start, end, line, kind):
        rel = self.repo.norm(path)
        if not self.repo.exists(rel):
            alt = self.resolve(self.doc_rel_dir, path)
            if not self.repo.exists(alt):
                similar = self.repo.similar(path)
                hint = " (similar: %s)" % ", ".join(similar) if similar else ""
                self.add("error", kind, line, "%s:%d" % (path, start), "file not found" + hint)
                return
            rel = alt
        total = self.repo.text_of(rel).count("\n") + 1
        if max(start, end) > total:
            self.add("error", kind, line, "%s:%d" % (path, start), "line is past the end of the file (%d lines)" % total)

    def check_identifier(self, tok, line):
        bare = tok.rstrip("()")
        if bare.lower() in COMMON_WORDS or bare in KNOWN_TOOLS or len(bare) < 4:
            return
        name = re.split(r"\.|::|->", bare)[-1]
        if self.repo.has_name(name):
            return
        distinctive = "(" in tok or "_" in bare or "." in bare or re.search(r"[a-z][A-Z]", bare) or bare[0].isupper()
        if distinctive:
            self.add("warning", "identifier", line, tok, "identifier not found outside docs")
        else:
            self.add("info", "term", line, tok, "word in code format not found outside docs; is it a real name?")

    # ------------------------------------------------------------ commands

    def check_blocks(self, blocks):
        for lang, body, start in blocks:
            if not lang:
                self.add("info", "code-fence", start - 1, "(fence)", "code block has no language tag")
                continue
            if lang not in SHELL_LANGS:
                continue
            cwd = "."
            logical, first_line = "", None
            heredoc = None
            for j, raw in enumerate(body.split("\n")):
                line_no = start + j
                text = raw.strip()
                if heredoc:
                    if text == heredoc:
                        heredoc = None
                    continue
                if lang in SESSION_LANGS:
                    if not text.startswith(("$ ", "> ", "% ")):
                        continue
                    text = text[2:]
                else:
                    text = re.sub(r"^\$\s+", "", text)
                text = strip_shell_comment(text)
                if not text:
                    continue
                if first_line is None:
                    first_line = line_no
                if text.endswith("\\"):
                    logical += text[:-1] + " "
                    continue
                logical += text
                hd = re.search(r"<<-?\s*['\"]?(\w+)['\"]?", logical)
                if hd:
                    heredoc = hd.group(1)
                cwd = self.check_command(logical, cwd, first_line)
                logical, first_line = "", None

    def check_command(self, full, cwd, line, record=True):
        """Check one logical shell line, returning the working directory after it runs."""
        if record:
            self.commands.append({"line": line, "cwd": cwd, "command": full, "risk": classify_risk(full)})
        for part in re.split(r"\s*(?:&&|\|\||;|\|)\s*", full):
            cwd = self.check_simple(part.strip(), cwd, line)
        return cwd

    def check_simple(self, cmd, cwd, line):
        words = split_shell_words(cmd)
        while words and re.match(r"^[A-Za-z_][A-Za-z0-9_]*=", words[0]):
            words = words[1:]
        while words and words[0] in WRAPPERS:
            words = words[1:]
        if len(words) >= 2 and (words[0], words[1]) in RUNNER_PREFIXES:
            words = words[2:]
        if not words:
            return cwd
        head = words[0]
        if head == "cd" and len(words) > 1:
            target = unquote(words[1])
            if is_placeholder(target) or target.startswith(("/", "~", "-")):
                return cwd
            new = self.resolve(cwd, target)
            if self.repo.exists(new):
                return new
            # Blocks often list independent commands, so retry from the repo root.
            if self.repo.exists(self.repo.norm(target)):
                return self.repo.norm(target)
            # A clone step creates the directory, so a cd after it cannot be checked.
            if not any(c["command"].startswith("git clone") for c in self.commands):
                self.add("error", "command-path", line, target, "cd target not found in repo")
            return "."
        if head == "make":
            self.check_runner(words, cwd, line, "make", self.repo.make_targets(), "-C")
        elif head == "just":
            self.check_runner(words, cwd, line, "just", self.repo.just_recipes(), "--justfile-dir")
        elif head == "task":
            self.check_runner(words, cwd, line, "task", self.repo.task_names(), "-d")
        elif head in ("npm", "yarn", "pnpm", "bun"):
            self.check_npm(words, cwd, line)
        elif head in ("docker", "docker-compose", "podman"):
            self.check_docker(words, cwd, line)
        elif head in ("cp", "cat", "source", ".", "less", "head", "tail") and len(words) > 1:
            self.check_arg_path([w for w in words[1:] if not w.startswith("-")][:1], cwd, line)
        elif head in INTERPRETERS:
            self.check_interpreter(words, cwd, line)
        elif head == "pytest":
            args = [w for w in words[1:] if not w.startswith("-")]
            self.check_arg_path([a.split("::")[0] for a in args], cwd, line)
        elif head == "dotnet":
            if "--project" in words:
                self.check_arg_path(words[words.index("--project") + 1:][:1], cwd, line)
            elif len(words) > 2 and words[1] in ("test", "build", "run", "publish") and not words[2].startswith("-"):
                self.check_arg_path(words[2:3], cwd, line)
        elif "/" in head or head.endswith((".sh", ".py", ".ps1", ".bat")):
            self.check_script(head, words[1:], cwd, line)
        for flag_file in re.findall(r"(?:^|\s)(?:-f|--file|--config|--values|--env-file)[ =]([\w./-]+\.(?:ya?ml|json|toml|ini|conf|env|hcl))", cmd):
            self.check_arg_path([flag_file], cwd, line)
        return cwd

    def check_arg_path(self, args, cwd, line):
        for a in args:
            a = unquote(a)
            if is_placeholder(a) or a.startswith(("/", "~", "http")) or not a:
                continue
            if not ("/" in a or "." in a):
                continue
            # Blocks often list independent commands, so a path may be relative to the root.
            if not self.repo.exists(self.resolve(cwd, a)) and not self.repo.exists(a):
                self.add("error", "command-path", line, a, "file not found relative to %s%s" % (
                    "the repo root" if cwd == "." else cwd, self.similar_hint(a)))

    def similar_hint(self, path):
        similar = self.repo.similar(path)
        return " (similar: %s)" % ", ".join(similar) if similar else ""

    def check_runner(self, words, cwd, line, tool, defs, chdir_flag):
        args = words[1:]
        where = cwd
        targets = []
        i = 0
        while i < len(args):
            a = args[i]
            if a == chdir_flag and i + 1 < len(args):
                where = self.resolve(cwd, args[i + 1])
                i += 2
                continue
            if a.startswith("-") or "=" in a:
                i += 1
                continue
            targets.append(a)
            i += 1
        key = "" if where in (".", "") else where
        here = defs.get(key)
        for t in targets:
            if is_placeholder(t):
                continue
            if here and t in here[1]:
                continue
            elsewhere = sorted(v[0] for k, v in defs.items() if t in v[1])
            if elsewhere:
                self.add("warning", "wrong-dir", line, "%s %s" % (tool, t),
                         "target exists in %s, not in %s; say which directory to run it from" % (
                             ", ".join(elsewhere[:3]), "the repo root" if not key else key))
            elif defs:
                known = ", ".join(sorted(here[1])[:8]) if here else "no %s file in this directory" % tool
                self.add("error", "%s-target" % tool, line, "%s %s" % (tool, t), "target not found (%s)" % known)
            else:
                self.add("error", "%s-target" % tool, line, "%s %s" % (tool, t), "repo has no %s definition file" % tool)

    def check_npm(self, words, cwd, line):
        args = words[1:]
        where = cwd
        if "--prefix" in args and args.index("--prefix") + 1 < len(args):
            where = self.resolve(cwd, args[args.index("--prefix") + 1])
        positional = [a for a in args if not a.startswith("-")]
        if not positional:
            return
        script = None
        if positional[0] == "run" and len(positional) > 1:
            script = positional[1]
        elif positional[0] == "test" or positional[0] == "start":
            script = positional[0]
        elif words[0] in ("yarn", "pnpm", "bun") and positional[0] not in NPM_BUILTINS:
            script = positional[0]
        if script is None or is_placeholder(script):
            return
        defs = self.repo.package_scripts()
        key = "" if where in (".", "") else where
        here = defs.get(key)
        if here and script in here[1]:
            return
        if script in ("test", "start") and not here:
            return
        elsewhere = sorted(v[0] for k, v in defs.items() if script in v[1])
        if elsewhere:
            self.add("warning", "wrong-dir", line, " ".join(words[:3]),
                     "script exists in %s, not in %s; say which directory to run it from" % (
                         ", ".join(elsewhere[:3]), "the repo root" if not key else key))
        elif defs:
            known = ", ".join(sorted(here[1])[:8]) if here else "no package.json in this directory"
            self.add("error", "npm-script", line, " ".join(words[:3]), "script not found (%s)" % known)

    def check_docker(self, words, cwd, line):
        args = words[1:]
        compose = words[0] == "docker-compose" or (args[:1] == ["compose"])
        if not compose:
            return
        args = args[1:] if words[0] != "docker-compose" else args
        files = self.repo.compose_services()
        chosen = None
        i = 0
        positional, after_service = [], []
        while i < len(args):
            a = args[i]
            # Everything after the service in exec or run is a command for the container.
            if len(positional) == 2 and positional[0] in ("exec", "run"):
                after_service = args[i:]
                break
            if a in ("-f", "--file") and i + 1 < len(args):
                chosen = self.resolve(cwd, args[i + 1])
                i += 2
                continue
            if a in COMPOSE_VALUE_FLAGS:
                i += 2
                continue
            if a.startswith("-"):
                i += 1
                continue
            positional.append(a)
            i += 1
        if after_service and after_service[0] in INTERPRETERS and "-m" in after_service:
            # Module names and flags are the same inside the container. Relative script paths may
            # not be, since the image sets its own working directory, so those are left alone.
            self.check_interpreter(after_service, cwd, line)
        if len(positional) < 2:
            return
        sub, names = positional[0], positional[1:]
        if sub not in ("up", "start", "stop", "restart", "logs", "build", "pull", "ps", "run", "exec", "rm", "kill", "top"):
            return
        names = names[:1] if sub in ("run", "exec") else names
        services = files.get(chosen) if chosen else None
        if services is None:
            prefix = "" if cwd in (".", "") else cwd + "/"
            local = [v for k, v in files.items() if os.path.dirname(k) + "/" == prefix or (not prefix and "/" not in k)]
            services = set().union(*local) if local else set().union(*files.values()) if files else None
        if not services:
            return
        for n in names:
            if not is_placeholder(n) and n not in services:
                self.add("error", "compose-service", line, n, "service not in compose file (found: %s)" % ", ".join(sorted(services)[:8]))

    def check_interpreter(self, words, cwd, line):
        args = words[1:]
        if "-m" in args and args.index("-m") + 1 < len(args):
            mod_at = args.index("-m") + 1
            self.check_py_module(args[mod_at], args[mod_at + 1:], cwd, line)
            return
        if "-c" in args or "-e" in args:
            return
        positional = [a for a in args if not a.startswith("-")]
        if positional:
            self.check_script(positional[0], args[args.index(positional[0]) + 1:], cwd, line)

    def check_py_module(self, module, rest, cwd, line):
        if is_placeholder(module):
            return
        rel = module.replace(".", "/")
        prefixes = ["" if cwd in (".", "") else cwd + "/"]
        prefixes += ["src/"] if prefixes[0] == "" else [prefixes[0] + "src/"]
        for p in prefixes:
            for cand in (p + rel + ".py", p + rel + "/__main__.py", p + rel + "/__init__.py"):
                if self.repo.exists(cand):
                    self.check_flags(cand, rest, line)
                    return
        if module.split(".")[0] not in {m.split(".")[0] for m in PY_MODULES}:
            self.add("info", "py-module", line, module, "module not found in repo; installed dependency or wrong name?")

    def check_script(self, script, rest, cwd, line):
        script = unquote(script)
        if is_placeholder(script) or script.startswith(("/", "~", "$", "%")):
            return
        rel = self.resolve(cwd, script)
        if not self.repo.exists(rel) and self.repo.exists(script):
            rel = self.repo.norm(script)
        if not self.repo.exists(rel):
            self.add("error", "script", line, script, "script not found relative to %s%s" % (
                "the repo root" if cwd in (".", "") else cwd, self.similar_hint(script)))
            return
        if os.path.isfile(self.repo.abspath(rel)):
            self.check_flags(rel, rest, line)

    def check_flags(self, rel, rest, line):
        text = self.repo.text_of(rel)
        for a in rest:
            flag = a.split("=", 1)[0]
            if FLAG_RE.match(flag) and flag not in ("--help", "--version") and flag not in text:
                # Argument parsers may build flags dynamically, so this stays a warning.
                self.add("warning", "flag", line, "%s %s" % (rel, flag), "flag does not appear in this script")

    # ------------------------------------------------------------ whole document

    def check_doc(self, doc_path):
        self.doc_text = doclib.read_text(doc_path)
        abs_doc = os.path.abspath(doc_path)
        rel_dir = os.path.relpath(os.path.dirname(abs_doc), self.repo.root).replace(os.sep, "/")
        self.doc_rel_dir = rel_dir
        _, body = doclib.strip_front_matter(self.doc_text)
        prose, blocks = doclib.split_code(body)
        prose = re.sub(r"<!--.*?-->", lambda m: re.sub(r"[^\n]", " ", m.group(0)), prose, flags=re.S)
        self.check_links(prose, os.path.dirname(abs_doc))
        self.check_inline(prose)
        self.check_blocks(blocks)


def check_urls(urls):
    """Fetch each unique external URL and return problems. Uses the network."""
    def fetch(url):
        host = re.sub(r"^https?://", "", url).split("/")[0].split(":")[0]
        if host in EXTERNAL_HOSTS or host.endswith((".example.com", ".example.org", ".local", ".internal")):
            return url, None, "skipped"
        req = urllib.request.Request(url, headers={"User-Agent": "doc-link-check/1.0"}, method="GET")
        try:
            with urllib.request.urlopen(req, timeout=URL_TIMEOUT_SECONDS) as resp:
                return url, resp.status, None
        except urllib.error.HTTPError as exc:
            return url, exc.code, None
        except (urllib.error.URLError, OSError, ValueError) as exc:
            return url, None, str(exc)[:80]

    unique = sorted({u for _, u in urls})
    with ThreadPoolExecutor(max_workers=URL_WORKERS) as pool:
        results = {u: (status, err) for u, status, err in pool.map(fetch, unique)}
    problems = []
    for line, url in urls:
        status, err = results[url]
        if err == "skipped":
            continue
        if err:
            problems.append({"severity": "warning", "kind": "url", "line": line, "item": url, "message": "could not fetch: %s" % err})
        elif status in (401, 403, 429):
            problems.append({"severity": "info", "kind": "url", "line": line, "item": url, "message": "HTTP %d; the host blocks scripts, so check by hand" % status})
        elif status >= 400:
            problems.append({"severity": "error", "kind": "url", "line": line, "item": url, "message": "HTTP %d" % status})
    return problems


def check_ledger(path, repo):
    """Check that evidence cited in the ledger points at real files and lines."""
    problems, counts = [], {"verified": 0, "inferred": 0, "unverified": 0}
    checker = Checker(repo)
    for i, line in enumerate(doclib.read_text(path).split("\n"), 1):
        low = line.lower()
        status = next((s for s in ("unverified", "verified", "inferred") if re.search(r"\b%s\b" % s, low)), None)
        if status is None or re.match(r"^\s*\|?\s*-{3,}", line) or "| status" in low:
            continue
        counts[status] += 1
        cited = re.findall(r"([\w@./-]+\.\w+):(\d+)(?:-(\d+))?", line)
        for path_, start, end in cited:
            checker.check_path_line(path_, int(start), int(end or start), i, "ledger")
        if status == "verified" and not cited and not re.search(r"`[^`]+`|\bran\b|\boutput\b|\btest\b|--help", low):
            checker.add("warning", "ledger", i, line.strip()[:60], "marked verified but cites no path:line, command, or test")
    problems.extend(checker.problems)
    return problems, counts


def expand_docs(paths, repo):
    docs, missing = [], []
    for p in paths:
        if os.path.isdir(p):
            base = os.path.abspath(p)
            prefix = os.path.relpath(base, repo.root).replace(os.sep, "/")
            for f in repo.files:
                if f.lower().endswith((".md", ".markdown", ".mdx")) and (prefix == "." or f.startswith(prefix + "/")):
                    docs.append(repo.abspath(f))
        elif os.path.isfile(p):
            docs.append(p)
        else:
            missing.append(p)
    return docs, missing


def verify_doc(doc, repo):
    """Run every offline check on one doc and return the Checker holding the results."""
    checker = Checker(repo)
    checker.check_doc(doc)
    return checker


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("docs", nargs="+", help="Markdown files or directories to check")
    ap.add_argument("--repo", required=True, help="repository root to check against")
    ap.add_argument("--json", action="store_true", help="emit JSON on stdout")
    ap.add_argument("--strict", action="store_true", help="exit 1 if any error or warning is found")
    ap.add_argument("--min-severity", choices=["info", "warning", "error"], default="info", help="hide findings below this level")
    ap.add_argument("--commands", action="store_true", help="list shell commands found, each with a risk class (safe, ask, never)")
    ap.add_argument("--check-urls", action="store_true", help="fetch external links (uses the network)")
    ap.add_argument("--ledger", help="evidence ledger file; checks each cited path:line")
    ap.add_argument("--limit", type=int, default=DEFAULT_LIMIT, help="max findings to print per doc; 0 for all (default %(default)s)")
    args = ap.parse_args(argv)
    if not os.path.isdir(args.repo):
        print("error: --repo is not a directory: %s" % args.repo, file=sys.stderr)
        return 2
    repo = doclib.RepoIndex(args.repo)
    docs, missing = expand_docs(args.docs, repo)
    if missing or not docs:
        print("error: no Markdown files found; not found: %s" % (", ".join(missing) or ", ".join(args.docs)), file=sys.stderr)
        return 2
    floor = SEVERITY_ORDER[args.min_severity]
    results, worst = [], 0
    for doc in docs:
        c = verify_doc(doc, repo)
        problems = list(c.problems)
        if args.check_urls:
            problems.extend(check_urls(c.urls))
        problems = [p for p in problems if SEVERITY_ORDER[p["severity"]] <= floor]
        problems.sort(key=lambda p: (SEVERITY_ORDER[p["severity"]], p["line"]))
        worst += sum(1 for p in problems if p["severity"] in ("error", "warning"))
        results.append({"doc": os.path.relpath(doc, repo.root), "problems": problems, "commands": c.commands,
                        "tokens_checked": c.tokens_checked, "external_urls": sorted({u for _, u in c.urls})})
    ledger = None
    if args.ledger:
        problems, counts = check_ledger(args.ledger, repo)
        ledger = {"file": args.ledger, "problems": problems, "counts": counts}
        worst += sum(1 for p in problems if p["severity"] in ("error", "warning"))
    if args.json:
        print(json.dumps({"repo": repo.root, "docs": results, "ledger": ledger}, indent=2))
    else:
        render(results, ledger, args)
    return 1 if (worst and args.strict) else 0


def render(results, ledger, args):
    for r in results:
        by = {s: sum(1 for p in r["problems"] if p["severity"] == s) for s in SEVERITY_ORDER}
        print("%s: %d errors, %d warnings, %d info (%d code tokens checked)" % (r["doc"], by["error"], by["warning"], by["info"], r["tokens_checked"]))
        shown = r["problems"] if args.limit == 0 else r["problems"][:args.limit]
        for p in shown:
            print("  L%-4d %-7s %-14s %s -> %s" % (p["line"], p["severity"], p["kind"], p["item"], p["message"]))
        if len(shown) < len(r["problems"]):
            print("  ... %d more; rerun with --limit 0 or --min-severity warning" % (len(r["problems"]) - len(shown)))
        if args.commands:
            print("  Commands found (risk: safe = run freely, ask = get approval, never = do not run for verification):")
            for c in r["commands"]:
                where = "" if c["cwd"] == "." else " [in %s]" % c["cwd"]
                print("    L%-4d %-5s %s%s" % (c["line"], c["risk"], c["command"], where))
        if r["external_urls"] and not args.check_urls:
            print("  %d external URLs not checked; pass --check-urls or fetch them yourself." % len(r["external_urls"]))
    if ledger:
        c = ledger["counts"]
        print("Ledger %s: %d verified, %d inferred, %d unverified; %d citation problems" % (ledger["file"], c["verified"], c["inferred"], c["unverified"], len(ledger["problems"])))
        for p in ledger["problems"]:
            print("  L%-4d %-7s %s -> %s" % (p["line"], p["severity"], p["item"], p["message"]))


if __name__ == "__main__":
    sys.exit(main())
