#!/usr/bin/env python3
"""Shared helpers for this skill's scripts: repo file listing and Markdown parsing.

Not a command. Imported by repo_inventory.py, verify_refs.py, slop_scan.py, and
doc_audit.py, which find it because Python puts a script's own directory on
sys.path. Standard library only.
"""
import json
import os
import re
import subprocess

SKIP_DIRS = {
    ".git", ".hg", ".svn", "node_modules", "vendor", ".venv", "venv", "env", "__pycache__",
    "target", "build", "dist", "out", ".next", ".nuxt", ".svelte-kit", ".gradle", "Pods",
    ".idea", ".vscode", ".tox", ".nox", ".mypy_cache", ".pytest_cache", ".ruff_cache",
    "coverage", "htmlcov", ".terraform", "cdk.out", ".serverless", ".aws-sam", ".cache",
    "bower_components", ".dart_tool", "site-packages", ".eggs", ".turbo", ".parcel-cache",
    "__snapshots__", ".vs",
}
# .NET writes build output under bin/ and obj/, but bin/ also holds real scripts in
# Ruby and Node projects, so these two are skipped only when they contain build output.
BUILD_OUTPUT_MARKERS = ("Debug", "Release", "project.assets.json")
SKIP_FILES = {
    "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "poetry.lock", "Cargo.lock", "go.sum",
    "Gemfile.lock", "composer.lock", "Pipfile.lock", "uv.lock", "bun.lockb", "packages.lock.json",
    ".DS_Store", "Thumbs.db",
}
SKIP_SUFFIXES = (
    ".min.js", ".min.css", ".map", ".pb.go", "_pb2.py", ".generated.ts", ".g.dart", ".snap",
    ".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp", ".woff", ".woff2", ".ttf", ".eot",
    ".zip", ".tar", ".gz", ".tgz", ".jar", ".class", ".pyc", ".so", ".dylib", ".dll", ".exe",
    ".bin", ".mp4", ".mov", ".pdf", ".docx", ".pptx", ".xlsx",
)
DOC_SUFFIXES = (".md", ".mdx", ".markdown", ".rst", ".adoc", ".txt")
DOC_DIR_NAMES = {"docs", "doc", "documentation", "wiki"}
# Files this large are generated or data. Reading them slows the index without adding names.
MAX_INDEX_BYTES = 1_500_000
MAX_LISTED_FILES = 60000


def _in_skipped_dir(rel):
    return any(part in SKIP_DIRS or part.endswith(".egg-info") for part in rel.split("/")[:-1])


def _is_build_output_dir(path):
    return any(os.path.exists(os.path.join(path, m)) for m in BUILD_OUTPUT_MARKERS)


def run_git(root, *args, timeout=30):
    """Return stdout of a git command, or None if git is missing or the command fails."""
    try:
        r = subprocess.run(["git", "-C", root] + list(args), capture_output=True, text=True,
                           errors="replace", timeout=timeout)
    except (OSError, subprocess.TimeoutExpired):
        return None
    return r.stdout if r.returncode == 0 else None


def list_repo_files(root, max_files=MAX_LISTED_FILES):
    """List repo files as forward-slash relative paths, skipping vendored and generated ones.

    Prefers git so .gitignore applies, which removes build output in every ecosystem.
    Returns (files, truncated, source) where source is "git" or "walk".
    """
    root = os.path.abspath(root)
    out = run_git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
    files, source = [], "walk"
    if out is not None:
        source = "git"
        for rel in out.split("\0"):
            if rel and os.path.isfile(os.path.join(root, rel)):
                files.append(rel)
    else:
        for dp, dn, fn in os.walk(root):
            dn[:] = sorted(
                d for d in dn
                if d not in SKIP_DIRS and not d.endswith(".egg-info")
                and not (d in ("bin", "obj") and _is_build_output_dir(os.path.join(dp, d)))
            )
            for f in sorted(fn):
                files.append(os.path.relpath(os.path.join(dp, f), root).replace(os.sep, "/"))
    kept = []
    for rel in sorted(files):
        base = rel.rsplit("/", 1)[-1]
        if _in_skipped_dir(rel) or base in SKIP_FILES or base.endswith(SKIP_SUFFIXES):
            continue
        kept.append(rel)
        if len(kept) >= max_files:
            return kept, True, source
    return kept, False, source


def read_text(path, limit=None):
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            return fh.read() if limit is None else fh.read(limit)
    except OSError:
        return ""


def is_doc_file(rel):
    """True for prose files, which must not count as evidence that a name exists in code."""
    parts = rel.split("/")
    return rel.lower().endswith(DOC_SUFFIXES) or any(p.lower() in DOC_DIR_NAMES for p in parts[:-1])


# ---------------------------------------------------------------- Markdown parsing

_FENCE = re.compile(r"^[ \t>]*(`{3,}|~{3,})(.*)$")


def split_code(text):
    """Return (prose with code lines blanked, [(lang, body, first_body_line_number)]).

    Follows CommonMark fence rules: a fence closes only with the same character and at
    least the same length, so four-backtick blocks can hold three-backtick examples.
    """
    blocks, out = [], []
    fence, lang, buf, start = None, "", [], 0
    for i, line in enumerate(text.split("\n"), 1):
        m = _FENCE.match(line)
        if fence is None:
            if m and not (m.group(1)[0] == "`" and "`" in m.group(2)):
                info = m.group(2).strip()
                lang = re.sub(r"[{}.]", " ", info).split()[0].lower() if info.strip("{}. ") else ""
                fence, buf, start = m.group(1), [], i + 1
                out.append("")
            else:
                out.append(line)
        else:
            closes = (m and m.group(1)[0] == fence[0] and len(m.group(1)) >= len(fence)
                      and not m.group(2).strip())
            if closes:
                blocks.append((lang, "\n".join(buf), start))
                fence = None
            else:
                buf.append(line)
            out.append("")
    if fence is not None:
        blocks.append((lang, "\n".join(buf), start))
    return "\n".join(out), blocks


def mask_inline_code(text):
    """Blank inline code spans, keeping length and newlines so positions stay valid."""
    def blank(m):
        return re.sub(r"[^\n]", " ", m.group(0))
    return re.sub(r"(?<!`)(`+)(?!`)[^\n]*?(?<!`)\1(?!`)", blank, text)


def strip_front_matter(text):
    """Return (front_matter_text, text with front matter blanked so line numbers hold)."""
    m = re.match(r"---[ \t]*\n(.*?\n)?(?:---|\.\.\.)[ \t]*(?:\n|$)", text, flags=re.S)
    if not m:
        return "", text
    return m.group(1) or "", re.sub(r"[^\n]", " ", m.group(0)) + text[m.end():]


def front_matter_value(front_matter, key):
    m = re.search(r"^%s\s*:\s*(.+?)\s*$" % re.escape(key), front_matter, flags=re.M)
    return m.group(1).strip("'\"") if m else None


def slugify(heading):
    """Build the anchor GitHub generates for a heading.

    GitHub replaces each space with a hyphen without collapsing, so "A & B" gives "a--b".
    """
    h = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", heading.strip())
    h = re.sub(r"<[^>]+>", "", h)
    h = re.sub(r"[`*~]", "", h)
    h = re.sub(r"(?<!\w)_(.+?)_(?!\w)", r"\1", h)
    h = re.sub(r"[^\w\- ]", "", h.lower(), flags=re.U)
    return h.replace(" ", "-")


def parse_headings(text):
    """Return heading dicts (level, title, line, slug) from ATX and setext headings."""
    prose, _ = split_code(strip_front_matter(text)[1])
    lines = prose.split("\n")
    found, counts = [], {}

    def add(level, title, line):
        custom = re.search(r"\{[^}]*#([\w:.-]+)[^}]*\}\s*$", title)
        clean = re.sub(r"\s*\{[^}]*\}\s*$", "", title).strip()
        slug = custom.group(1) if custom else slugify(clean)
        n = counts.get(slug, 0)
        counts[slug] = n + 1
        found.append({"level": level, "title": clean, "line": line,
                      "slug": slug if n == 0 else "%s-%d" % (slug, n)})

    for i, line in enumerate(lines):
        m = re.match(r"^ {0,3}(#{1,6})[ \t]+(.+?)[ \t]*#*[ \t]*$", line)
        if m:
            add(len(m.group(1)), m.group(2), i + 1)
            continue
        if i + 1 < len(lines) and line.strip() and not line.lstrip().startswith(("|", ">", "-", "*", "+")):
            if re.match(r"^ {0,3}=+[ \t]*$", lines[i + 1]):
                add(1, line.strip(), i + 1)
            elif re.match(r"^ {0,3}-{2,}[ \t]*$", lines[i + 1]) and (i == 0 or not lines[i - 1].strip()):
                add(2, line.strip(), i + 1)
    return found


def anchors_of(text):
    slugs = {h["slug"] for h in parse_headings(text)}
    for m in re.finditer(r"""<(?:a|div|span|section|h[1-6])\b[^>]*\b(?:name|id)=["']([^"']+)["']""", text):
        slugs.add(m.group(1))
    return slugs


# ---------------------------------------------------------------- repo index

_IDENT = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")
_LONG_FLAG = re.compile(r"(?<![\w-])--[a-zA-Z][\w-]*")


class RepoIndex:
    """File list plus a lazy index of the names and flags that appear in non-doc files."""

    def __init__(self, root):
        self.root = os.path.abspath(root)
        self.files, self.truncated, self.source = list_repo_files(self.root)
        self.fileset = set(self.files)
        self.dirs = set()
        self.by_basename = {}
        for f in self.files:
            self.by_basename.setdefault(f.rsplit("/", 1)[-1], []).append(f)
            parts = f.split("/")
            for i in range(1, len(parts)):
                self.dirs.add("/".join(parts[:i]))
        self._names = None
        self._flags = None
        self._cache = {}

    def abspath(self, rel):
        return os.path.join(self.root, rel)

    def norm(self, rel):
        return os.path.normpath(rel.strip()).replace(os.sep, "/")

    def exists(self, rel):
        """True if the path exists in the index or on disk, including ignored paths."""
        rel = self.norm(rel).rstrip("/")
        if rel in (".", ""):
            return True
        if rel.startswith("../"):
            return os.path.exists(os.path.join(self.root, rel))
        return rel in self.fileset or rel in self.dirs or os.path.exists(self.abspath(rel))

    def glob_exists(self, pattern):
        rx = re.escape(pattern.lstrip("./"))
        rx = rx.replace(r"\*\*/", "(?:.*/)?").replace(r"\*\*", ".*").replace(r"\*", "[^/]*").replace(r"\?", "[^/]")
        rx = re.sub(r"\\\{[^}]*\\\}", ".*", rx)
        r = re.compile("^" + rx + "/?$")
        return any(r.match(f) for f in self.files)

    def similar(self, path, limit=3):
        base = path.rstrip("/").rsplit("/", 1)[-1]
        return self.by_basename.get(base, [])[:limit]

    def text_of(self, rel):
        if rel not in self._cache:
            self._cache[rel] = read_text(self.abspath(rel))
        return self._cache[rel]

    def _build_source_index(self):
        names, flags = set(), set()
        for f in self.files:
            if is_doc_file(f):
                continue
            p = self.abspath(f)
            try:
                if os.path.getsize(p) > MAX_INDEX_BYTES:
                    continue
            except OSError:
                continue
            t = read_text(p)
            names.update(_IDENT.findall(t))
            flags.update(_LONG_FLAG.findall(t))
        self._names, self._flags = names, flags

    def has_name(self, name):
        """True if the identifier appears in any non-doc file."""
        if self._names is None:
            self._build_source_index()
        return name in self._names

    def has_flag(self, flag):
        if self._flags is None:
            self._build_source_index()
        return flag in self._flags

    # Task runner definitions, keyed by the directory that holds them.

    def _files_named(self, *names):
        return [f for f in self.files if f.rsplit("/", 1)[-1] in names]

    def make_targets(self):
        if "__make" not in self._cache:
            res = {}
            for f in self._files_named("Makefile", "makefile", "GNUmakefile"):
                t = self.text_of(f)
                targets = set(re.findall(r"^([A-Za-z0-9_][A-Za-z0-9_./-]*)[ \t]*(?:\s[^:=\n]*)?:(?![=])", t, flags=re.M))
                for m in re.finditer(r"^\.PHONY\s*:\s*(.+)$", t, flags=re.M):
                    targets.update(m.group(1).split())
                res[os.path.dirname(f)] = (f, targets)
            self._cache["__make"] = res
        return self._cache["__make"]

    def just_recipes(self):
        if "__just" not in self._cache:
            res = {}
            for f in self._files_named("justfile", "Justfile", ".justfile"):
                res[os.path.dirname(f)] = (f, set(re.findall(r"^@?([A-Za-z0-9_-]+)[^:=\n]*:(?!=)", self.text_of(f), flags=re.M)))
            self._cache["__just"] = res
        return self._cache["__just"]

    def task_names(self):
        if "__task" not in self._cache:
            res = {}
            for f in self._files_named("Taskfile.yml", "Taskfile.yaml", "taskfile.yml"):
                res[os.path.dirname(f)] = (f, set(re.findall(r"^ {2}([A-Za-z0-9_:-]+):", self.text_of(f), flags=re.M)))
            self._cache["__task"] = res
        return self._cache["__task"]

    def package_scripts(self):
        if "__npm" not in self._cache:
            res = {}
            for f in self._files_named("package.json"):
                try:
                    data = json.loads(self.text_of(f))
                    res[os.path.dirname(f)] = (f, set((data.get("scripts") or {}).keys()))
                except ValueError:
                    continue
            self._cache["__npm"] = res
        return self._cache["__npm"]

    def compose_services(self):
        if "__compose" not in self._cache:
            res = {}
            for f in self.files:
                base = f.rsplit("/", 1)[-1]
                if re.match(r"(?:docker-)?compose[\w.-]*\.ya?ml$", base):
                    services = set()
                    in_services = False
                    for line in self.text_of(f).split("\n"):
                        if re.match(r"^services\s*:", line):
                            in_services = True
                            continue
                        if in_services and re.match(r"^\S", line) and not line.lstrip().startswith("#"):
                            in_services = False
                        m = re.match(r"^ {2}([A-Za-z0-9_.-]+)\s*:", line) if in_services else None
                        if m:
                            services.add(m.group(1))
                    res[f] = services
            self._cache["__compose"] = res
        return self._cache["__compose"]
