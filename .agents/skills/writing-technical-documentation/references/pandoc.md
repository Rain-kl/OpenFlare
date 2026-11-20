# Converting documents with pandoc

Write in Markdown in the repo, and treat Word, PDF, and HTML as build outputs. Do not hand-edit an exported file, because the next export overwrites the edit. When someone returns a marked-up Word file, read their comments, change the Markdown, and export again.

Contents

1. Default path: the export script
2. Which output to choose
3. Word templates
4. PDF engines
5. Importing old documents
6. Multi-file documents
7. A pipeline without the script
8. Gotchas
9. Check the output

## 1. Default path: the export script

`scripts/export_doc.py` wraps pandoc and handles what usually goes wrong: engine choice, image paths, title handling, version differences, and silent warnings. It never installs anything.

```bash
python3 scripts/export_doc.py doctor
python3 scripts/export_doc.py export docs/design.md --to docx --title "Payments design" --author "Platform team"
python3 scripts/export_doc.py export docs/design.md --to pdf
python3 scripts/export_doc.py export docs/intro.md docs/ops.md --to pdf -o handover.pdf --toc yes
python3 scripts/export_doc.py export docs/design.md --to html --link-base https://github.com/org/repo/blob/main/docs
```

Run `doctor` first. If pandoc is missing, it prints the install command for this system: ask the user before installing anything. Exit code 3 means a tool is missing, and 4 means the file was written but has a problem such as a missing image, so read the message before calling it done. `--dry-run` prints the exact pandoc command, which you can hand to a user or put in CI.

## 2. Which output to choose

| Output | Choose it when | Note |
|---|---|---|
| Word | A client or reviewer will comment or edit | Most common for client handover. Needs no PDF engine |
| PDF | The document is final, or layout must not shift | Needs an engine; see section 4 |
| HTML | One file to email or host, viewable anywhere | Images and CSS are embedded |
| Markdown | Everything else | The source stays the best reading format in a repo |

## 3. Word templates

Pandoc takes styles and page setup from a reference document and ignores its content. The script generates one with wrapped code, callouts, and page numbers. It also writes borders, header shading, and content-sized column widths directly into each table, because Pages, Google Docs, and macOS preview ignore Word table styles. To commit a template for a team, or to change the look:

```bash
python3 scripts/export_doc.py reference-doc -o docs/reference.docx --font Arial --accent 0B6E4F --paper letter
python3 scripts/export_doc.py export docs/design.md --to docx --reference-doc docs/reference.docx
```

To match a company template, start from that reference file, open it in Word, and change the named styles. Do not paste a company template's content into it, since pandoc ignores content. If a template lacks a style, pandoc adds a plain one. The styles that matter:

| Style | Used for |
|---|---|
| Title, Subtitle, Author, Date | Title block |
| Heading 1 to Heading 9 | Headings |
| Body Text, First Paragraph, Compact | Paragraphs; Compact is for tight list items |
| Table | All tables, unless a table sets `custom-style`. With your own reference doc, the script leaves tables to this style |
| Source Code, Verbatim Char | Code blocks and inline code |
| Block Text, Hyperlink, Caption, Table Caption, Image Caption | Quotes, links, captions |
| Callout, Callout Title | GitHub alerts, added by this skill's filter |

For styles on a specific paragraph, use `--from commonmark_x`, which adds fenced divs that `gfm` lacks. An unknown style name is created automatically and inherits from Normal:

```markdown
::: {custom-style="Disclaimer"}
This estimate expires on 2026-12-31.
:::
```

Page break in any output format:

```markdown
<!-- pagebreak -->
```

## 4. PDF engines

Default to Typst. The script picks the first installed engine in this order.

| Engine | Install size | Unicode | Style with | Use when |
|---|---|---|---|---|
| Typst | One small binary | Full | `assets/pandoc/typst-style.typ`, variables | The default; also the simplest in CI |
| WeasyPrint | Python package plus system libraries | Full | CSS (`assets/pandoc/doc.css`) | The PDF must match a web page or a CSS brand |
| XeLaTeX or LuaLaTeX | TeX install, often gigabytes | Full, with a suitable font | LaTeX variables | The project already has TeX, or needs LaTeX packages |
| pdflatex | TeX install | Fails on arrows, CJK, and much else | LaTeX variables | Never as a default |

Pandoc's own default is pdflatex, so `pandoc in.md -o out.pdf` fails on a machine with no TeX, and on many documents with TeX. Always name an engine.

## 5. Importing old documents

```bash
python3 scripts/export_doc.py import spec.docx -o docs/spec.md
python3 scripts/export_doc.py import third-party.html -o docs/vendor.md --sandbox
python3 scripts/export_doc.py import reviewed.docx -o /tmp/reviewed.md --track-changes all
```

This writes GitHub-flavored Markdown with unwrapped lines, ATX headings, and images extracted to a folder. Then:

1. Run `scripts/slop_scan.py` and `scripts/verify_refs.py` on the result.
2. Treat every statement as a claim from an old document. Verify it against the code like any other doc.
3. Fix what conversion cannot: tables with merged cells or lists inside cells come through as raw HTML, and visual emphasis often stands in for missing headings.

`--track-changes all` keeps insertions, deletions, and comments as marked spans, so you can read a reviewer's feedback. The default accepts all changes silently.

## 6. Multi-file documents

List files in reading order. The defaults files set `--file-scope`, so duplicate footnote names do not collide and a link such as `ops.md#rollback` between input files becomes an internal link. Pandoc only recognizes the link when it matches the path given on the command line, so keep files that link to each other in one folder, and run from it. The script does that for you. Only the first file can carry YAML front matter when reading `gfm`. Pass the title with `--title`.

A relative link to a file outside the export set is dead in a Word or PDF file. Without `--link-base`, the filter keeps the text and drops the link, and it reports each one, so a reader never hits a broken link.

## 7. A pipeline without the script

For a project that wants plain pandoc in a Makefile or CI, copy `assets/pandoc/` into the repo, for example to `docs/pandoc/`, and call the defaults files. Paths inside them resolve relative to the defaults file. Commit the reference doc, so that exports match.

```bash
python3 scripts/export_doc.py reference-doc -o docs/pandoc/reference.docx
pandoc --defaults docs/pandoc/defaults/docx.yaml --reference-doc docs/pandoc/reference.docx -o build/design.docx docs/design.md
pandoc --defaults docs/pandoc/defaults/pdf-typst.yaml --variable papersize=a4 -o build/design.pdf docs/design.md
```

In CI, `pandoc/core` and `pandoc/latex` are official Docker images. Pin a version tag, because output can change between pandoc releases. Set `SOURCE_DATE_EPOCH` for byte-identical Word files; the script sets it from the input's last commit.

## 8. Gotchas

- Reader: use `--from gfm`, so the file reads the same on GitHub and in the output. The `markdown` reader adds behavior most repo docs do not expect: it sizes pipe table columns by the dash counts in the separator row once a line passes 72 characters, turns a lone image into a captioned figure, and converts `--` and straight quotes. `gfm` does treat `$...$` as math, as GitHub does.
- Repeated values combine: `--variable`, `--metadata`, and `--css` on the command line add to the same key in a defaults file. Set a value in one place only. Two `papersize` values become a list, and Typst rejects it.
- Renamed flags: `--self-contained` became `--standalone --embed-resources` (2.19). `--highlight-style` and `--no-highlight` became `--syntax-highlighting` (3.8), and the old forms print a deprecation warning. Check `pandoc --version` before copying a recipe from the web.
- Old packages: distro packages lag, and alerts need 3.1.10 or later. Without them, `> [!NOTE]` prints as a plain quote.
- Word contents page: `--toc` in Word writes a field that fills in when updated. Tell the recipient to accept Word's update prompt, or press F9 on the table.
- Resource path: setting `--resource-path` removes the working directory from the search unless you list it.
- Reference doc command: `-o` must come before `--print-default-data-file`.
- Security: a Markdown line such as `![x](/etc/hosts)` embeds that local file in the output, so converting someone else's document can leak files. Use `--sandbox` on third-party input. It blocks this, and it also drops the document's own local images. It does not limit Lua filters or PDF engines, which can do anything your account can, so do not run a filter from an untrusted repo. With `--embed-resources`, absolute image URLs are downloaded.
- Fonts: Typst bundles Latin fonts and falls back to system fonts. On a minimal CI image, install a font for any other script, or that text can render as empty boxes.
- Mermaid: this skill renders to PNG, because Mermaid's SVG uses `foreignObject`, which Word and Typst do not render reliably. Without `mmdc`, the source stays as a code block. Headless Chromium in CI often needs a Puppeteer config file, set through `MERMAID_PUPPETEER_CONFIG`.
- Accessibility: PDFs are untagged by default. Typst 0.12 or later can produce PDF/A-2b, and LuaLaTeX with TeX Live 2025 can produce PDF/UA. If a recipient requires accessible files, Word export from `reference.docx` keeps real headings and alt text.

## 9. Check the output

A successful exit does not prove the file looks right.

1. Read the script's problems and notes. For Word and HTML it compares word counts with the source and reports dropped text.
2. Extract the text with `pandoc out.docx --to plain | head -40` and confirm the title, the first heading, and the first table.
3. If you can view images, render the first page and a page with a wide table. On macOS, `qlmanage -t -s 1000 -o /tmp out.docx` makes a thumbnail. Check that code wraps and tables fit the page.
4. Tell the user what you could not check, such as how the file looks in Word itself.
