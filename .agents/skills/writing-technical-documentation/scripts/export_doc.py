#!/usr/bin/env python3
"""Convert documentation with pandoc: Markdown to Word, PDF, or HTML, and legacy docs to Markdown.

Usage:
    export_doc.py doctor [--json]
    export_doc.py export INPUT.md [MORE.md ...] --to docx|pdf|html [-o OUT] [options]
    export_doc.py import LEGACY.docx -o docs/legacy.md [--media-dir DIR]
    export_doc.py reference-doc -o reference.docx [--font NAME] [--accent HEX] [--paper a4|letter]

Examples:
    export_doc.py doctor
    export_doc.py export docs/design.md --to docx --title "Payments design" --author "Platform team"
    export_doc.py export docs/intro.md docs/ops.md --to pdf -o handover.pdf --toc yes
    export_doc.py export docs/design.md --to docx --reference-doc brand.docx --dry-run
    export_doc.py import ~/Downloads/spec.docx -o docs/spec.md --sandbox

export reads GitHub-flavored Markdown by default, so the file looks the same on GitHub and in
the output. It promotes a lone H1 to the document title, picks the best installed PDF engine,
finds images relative to each input, renders Mermaid when mmdc is installed, and reports
problems such as missing images. Word exports use a generated style template with readable
tables in any viewer, wrapped code, and page numbers.

For a third party's file, pass --sandbox. Without it, a Markdown line such as ![x](/etc/hosts)
embeds that local file in the output. The sandbox blocks this, and it also drops the document's
own local images. It does not limit filters or PDF engines, so review what you run on such input.

Output: one summary line on stdout, or JSON with --json. Diagnostics go to stderr.
Exit codes: 0 success; 1 pandoc failed; 2 bad usage; 3 a required tool is missing (the message
gives install commands); 4 converted, but with problems such as missing images. Pass
--allow-warnings to turn 4 into 0. Standard library only; this script never installs anything.
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

SKILL_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ASSETS = os.path.join(SKILL_ROOT, "assets", "pandoc")
DEFAULTS_DIR = os.path.join(ASSETS, "defaults")

# Alerts, which GitHub-flavored docs use for callouts, arrived in pandoc 3.1.10. --embed-resources
# arrived in 2.19, so anything older cannot run these recipes.
MIN_PANDOC = (2, 19)
GOOD_PANDOC = (3, 1, 10)
# Best-first. Typst is one binary with no TeX. WeasyPrint styles with CSS. pdflatex is last-ditch
# because it fails on Unicode such as arrows, so it is used only when named with --pdf-engine.
PDF_ENGINES = [("typst", "pdf-typst"), ("weasyprint", "pdf-weasyprint"), ("xelatex", "pdf-xelatex"), ("lualatex", "pdf-xelatex"), ("tectonic", "pdf-xelatex")]
PDF_ENGINE_CHOICES = ["auto", "typst", "weasyprint", "xelatex", "lualatex", "tectonic", "pdflatex", "prince", "pagedjs-cli"]
OUTPUT_SUFFIX = {"docx": ".docx", "pdf": ".pdf", "html": ".html"}
# A table of contents is noise on short pages, so "auto" adds one only past both thresholds.
TOC_MIN_WORDS = 1200
TOC_MIN_HEADINGS = 5
# Round-trip check: a Word or HTML file holding much less text than its source lost content.
MIN_TEXT_RATIO = 0.85
BLOCKING_LOG_TYPES = {"CouldNotFetchResource", "CouldNotConvertImage", "IgnoredIOError", "ResourcePathDirNotFound"}
REPORTED_LOG_TYPES = BLOCKING_LOG_TYPES | {"MissingCharacter", "CouldNotHighlight", "NoTitleElement", "Extracting"}
# Page geometry in twentieths of a point, the unit Word uses.
PAPER_TWIPS = {"a4": (11906, 16838), "letter": (12240, 15840)}
MARGIN_TWIPS = {"top": 1304, "bottom": 1304, "left": 1304, "right": 1304, "header": 680, "footer": 567}


def install_help(tools):
    platform = sys.platform
    lines = []
    if "pandoc" in tools:
        lines.append({
            "darwin": "  pandoc: brew install pandoc",
            "win32": "  pandoc: winget install --source winget --exact --id JohnMacFarlane.Pandoc",
        }.get(platform, "  pandoc: download from https://github.com/jgm/pandoc/releases/latest (distro packages lag behind), or: conda install -c conda-forge pandoc"))
    if "pdf" in tools:
        lines.append({
            "darwin": "  PDF engine: brew install typst",
            "win32": "  PDF engine: winget install --id Typst.Typst",
        }.get(platform, "  PDF engine: download Typst from https://github.com/typst/typst/releases/latest, or: conda install -c conda-forge typst"))
    if "mmdc" in tools:
        lines.append("  Mermaid (optional): npm install -g @mermaid-js/mermaid-cli")
    return "\n".join(lines)


def fail(code, message):
    print("error: " + message, file=sys.stderr)
    return code


def pandoc_path():
    return os.environ.get("PANDOC") or shutil.which("pandoc")


def pandoc_version(path):
    try:
        out = subprocess.run([path, "--version"], capture_output=True, text=True, timeout=30).stdout
    except (OSError, subprocess.TimeoutExpired):
        return None
    m = re.search(r"pandoc(?:\.exe)?\s+(\d+(?:\.\d+)*)", out)
    return tuple(int(x) for x in m.group(1).split(".")) if m else None


def version_text(version):
    return ".".join(str(x) for x in version) if version else "unknown"


def require_pandoc():
    """Return (path, version, error_code). Prints why when pandoc cannot be used."""
    path = pandoc_path()
    if not path:
        return None, None, fail(3, "pandoc is not installed. Ask the user before installing it:\n" + install_help({"pandoc"}))
    version = pandoc_version(path)
    if version is None:
        return None, None, fail(3, "could not read the version of %s" % path)
    if version < MIN_PANDOC:
        return None, None, fail(3, "pandoc %s is too old; need %s or later:\n%s" % (version_text(version), version_text(MIN_PANDOC), install_help({"pandoc"})))
    if version < GOOD_PANDOC:
        print("warning: pandoc %s renders GitHub alerts as plain quotes; %s or later is better" % (version_text(version), version_text(GOOD_PANDOC)), file=sys.stderr)
    return path, version, 0


def defaults_for_engine(engine):
    return next((d for e, d in PDF_ENGINES if e == engine), "pdf-xelatex" if engine == "pdflatex" else "pdf-weasyprint")


def installed_pdf_engines():
    return [engine for engine, _ in PDF_ENGINES if shutil.which(engine)]


def choose_pdf_engine(requested):
    """Return the engine to use, or None if none is installed. A dry run never needs one."""
    if requested != "auto":
        return requested if shutil.which(requested) else None
    found = installed_pdf_engines()
    return found[0] if found else None


# ---------------------------------------------------------------- doctor

def cmd_doctor(args):
    path = pandoc_path()
    version = pandoc_version(path) if path else None
    engines = {e: shutil.which(e) for e, _ in PDF_ENGINES}
    mmdc = os.environ.get("MERMAID_BIN") or shutil.which("mmdc")
    chosen = next((e for e, _ in PDF_ENGINES if engines[e]), None)
    missing = set()
    if not path or (version and version < MIN_PANDOC):
        missing.add("pandoc")
    if not chosen:
        missing.add("pdf")
    if not mmdc:
        missing.add("mmdc")
    report = {
        "pandoc": {"path": path, "version": version_text(version) if version else None, "ok": bool(version and version >= MIN_PANDOC)},
        "pdf_engines": engines, "pdf_engine_chosen": chosen, "mermaid": mmdc,
        "can_export": {"docx": "pandoc" not in missing, "html": "pandoc" not in missing, "pdf": "pandoc" not in missing and bool(chosen)},
        "install": install_help(missing),
    }
    if args.json:
        print(json.dumps(report, indent=2))
    else:
        print("pandoc:  %s" % ("%s (%s)" % (version_text(version), path) if path else "not installed"))
        print("PDF:     %s" % (chosen or "no engine installed"))
        print("Mermaid: %s" % (mmdc or "mmdc not installed; diagrams stay as code blocks"))
        if missing:
            print("To enable the missing parts (ask the user before installing):")
            print(install_help(missing))
    return 0


# ---------------------------------------------------------------- reference doc

def _replace_child(block, tag, fragment, before):
    """Set one child element of a style block, keeping the order the OOXML schema requires."""
    m = re.search(r"<w:%s\b[^>]*?(?:/>|>.*?</w:%s>)" % (tag, tag), block, flags=re.S)
    if m:
        return block[:m.start()] + fragment + block[m.end():]
    for later in before:
        m = re.search(r"<w:%s\b" % later, block)
        if m:
            return block[:m.start()] + fragment + block[m.start():]
    return block + fragment


def patch_style(xml, style_id, ppr=None, rpr=None, extra=None):
    m = re.search(r'(<w:style\b[^>]*w:styleId="%s"[^>]*>)(.*?)(</w:style>)' % re.escape(style_id), xml, flags=re.S)
    if not m:
        return xml
    inner = m.group(2)
    if ppr is not None:
        inner = _replace_child(inner, "pPr", "<w:pPr>%s</w:pPr>" % ppr, ("rPr", "tblPr", "trPr", "tcPr", "tblStylePr"))
    if rpr is not None:
        inner = _replace_child(inner, "rPr", "<w:rPr>%s</w:rPr>" % rpr, ("tblPr", "trPr", "tcPr", "tblStylePr"))
    if extra:
        inner = extra(inner)
    return xml[:m.start(2)] + inner + xml[m.end(2):]


def add_style(xml, block):
    return xml.replace("</w:styles>", block + "</w:styles>")


def _fonts(name):
    return '<w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s" />' % (name, name, name)


def style_reference(xml, font, heading_font, mono, accent, muted="595959", rule="BFBFBF", tint="F2F2F2", base_half_points=21):
    """Restyle pandoc's default styles.xml. Fragments follow OOXML element order."""
    xml = re.sub(r"<w:rPrDefault>.*?</w:rPrDefault>",
                 '<w:rPrDefault><w:rPr>%s<w:sz w:val="%d" /><w:szCs w:val="%d" /><w:lang w:val="en-US" w:eastAsia="zh-CN" w:bidi="ar-SA" /></w:rPr></w:rPrDefault>' % (_fonts(font), base_half_points, base_half_points),
                 xml, flags=re.S)
    xml = re.sub(r"<w:pPrDefault>.*?</w:pPrDefault>", '<w:pPrDefault><w:pPr><w:spacing w:after="120" w:line="264" w:lineRule="auto" /></w:pPr></w:pPrDefault>', xml, flags=re.S)
    for sid in ("BodyText", "FirstParagraph"):
        xml = patch_style(xml, sid, ppr='<w:spacing w:before="0" w:after="120" />')
    xml = patch_style(xml, "Compact", ppr='<w:spacing w:before="20" w:after="20" />')
    xml = patch_style(xml, "Title",
                      ppr='<w:keepNext /><w:keepLines /><w:spacing w:before="0" w:after="60" /><w:contextualSpacing />',
                      rpr='%s<w:b /><w:bCs /><w:color w:val="%s" /><w:sz w:val="48" /><w:szCs w:val="48" />' % (_fonts(heading_font), accent))
    xml = patch_style(xml, "Subtitle", ppr='<w:keepNext /><w:keepLines /><w:spacing w:before="0" w:after="120" />',
                      rpr='%s<w:color w:val="%s" /><w:sz w:val="28" /><w:szCs w:val="28" />' % (_fonts(heading_font), muted))
    for sid in ("Author", "Date"):
        xml = patch_style(xml, sid, ppr='<w:keepNext /><w:keepLines /><w:spacing w:before="0" w:after="40" />', rpr='<w:b w:val="0" /><w:bCs w:val="0" /><w:color w:val="%s" /><w:sz w:val="%d" /><w:szCs w:val="%d" />' % (muted, base_half_points, base_half_points))
    for sid, size, before, after, bold in (("Heading1", 32, 360, 120, True), ("Heading2", 26, 280, 80, True), ("Heading3", 23, 220, 60, True), ("Heading4", 21, 180, 40, True)):
        level = int(sid[-1]) - 1
        weight = "<w:b /><w:bCs />" if bold else ""
        xml = patch_style(xml, sid,
                          ppr='<w:keepNext /><w:keepLines /><w:spacing w:before="%d" w:after="%d" /><w:outlineLvl w:val="%d" />' % (before, after, level),
                          rpr='%s%s<w:color w:val="%s" /><w:sz w:val="%d" /><w:szCs w:val="%d" />' % (_fonts(heading_font), weight, accent, size, size))
    xml = patch_style(xml, "TOCHeading", rpr='%s<w:b /><w:bCs /><w:color w:val="%s" /><w:sz w:val="32" /><w:szCs w:val="32" />' % (_fonts(heading_font), accent))
    xml = patch_style(xml, "BlockText", ppr='<w:pBdr><w:left w:val="single" w:sz="12" w:space="8" w:color="%s" /></w:pBdr><w:spacing w:before="80" w:after="80" /><w:ind w:left="240" w:right="240" />' % rule,
                      rpr='<w:color w:val="%s" />' % muted)
    for sid in ("Caption", "TableCaption", "ImageCaption"):
        xml = patch_style(xml, sid, rpr='<w:i /><w:iCs /><w:color w:val="%s" /><w:sz w:val="19" /><w:szCs w:val="19" />' % muted)
    xml = patch_style(xml, "VerbatimChar", rpr='%s<w:sz w:val="20" /><w:szCs w:val="20" />' % _fonts(mono))
    xml = patch_style(xml, "Hyperlink", rpr='<w:color w:val="%s" /><w:u w:val="single" />' % accent)

    borders = "".join('<w:%s w:val="single" w:sz="4" w:space="0" w:color="%s" />' % (side, rule) for side in ("top", "left", "bottom", "right", "insideH", "insideV"))

    def table_style(inner):
        inner = _replace_child(inner, "tblPr",
                               '<w:tblPr><w:tblInd w:w="0" w:type="dxa" /><w:tblBorders>%s</w:tblBorders><w:tblCellMar><w:top w:w="50" w:type="dxa" /><w:left w:w="100" w:type="dxa" /><w:bottom w:w="50" w:type="dxa" /><w:right w:w="100" w:type="dxa" /></w:tblCellMar></w:tblPr>' % borders,
                               ("trPr", "tcPr", "tblStylePr"))
        first_row = '<w:tblStylePr w:type="firstRow"><w:rPr><w:b /><w:bCs /></w:rPr><w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="%s" /><w:vAlign w:val="bottom" /></w:tcPr></w:tblStylePr>' % tint
        return re.sub(r'<w:tblStylePr w:type="firstRow">.*?</w:tblStylePr>', first_row, inner, flags=re.S)

    xml = patch_style(xml, "Table", extra=table_style)
    # Pandoc adds a Source Code style only when the reference doc lacks one. Its version turns word
    # wrap off, so long lines run off the page. Defining the style here keeps wrapping on.
    if 'w:styleId="SourceCode"' not in xml:
        xml = add_style(xml, '<w:style w:type="paragraph" w:customStyle="1" w:styleId="SourceCode"><w:name w:val="Source Code" /><w:basedOn w:val="Normal" /><w:link w:val="VerbatimChar" /><w:pPr><w:shd w:val="clear" w:color="auto" w:fill="%s" /><w:spacing w:before="60" w:after="120" w:line="240" w:lineRule="auto" /></w:pPr><w:rPr>%s<w:sz w:val="19" /><w:szCs w:val="19" /></w:rPr></w:style>' % (tint, _fonts(mono)))
    callout_border = '<w:left w:val="single" w:sz="18" w:space="8" w:color="%s" />' % accent
    xml = add_style(xml, '<w:style w:type="paragraph" w:customStyle="1" w:styleId="Callout"><w:name w:val="Callout" /><w:basedOn w:val="Normal" /><w:qFormat /><w:pPr><w:pBdr>%s</w:pBdr><w:shd w:val="clear" w:color="auto" w:fill="%s" /><w:spacing w:before="0" w:after="120" /><w:ind w:left="200" /></w:pPr></w:style>' % (callout_border, tint))
    xml = add_style(xml, '<w:style w:type="paragraph" w:customStyle="1" w:styleId="CalloutTitle"><w:name w:val="Callout Title" /><w:basedOn w:val="Callout" /><w:next w:val="Callout" /><w:qFormat /><w:pPr><w:keepNext /><w:pBdr>%s</w:pBdr><w:shd w:val="clear" w:color="auto" w:fill="%s" /><w:spacing w:before="120" w:after="0" /><w:ind w:left="200" /></w:pPr><w:rPr><w:b /><w:bCs /><w:color w:val="%s" /></w:rPr></w:style>' % (callout_border, tint, accent))
    if 'w:styleId="Footer"' not in xml:
        xml = add_style(xml, '<w:style w:type="paragraph" w:styleId="Footer"><w:name w:val="footer" /><w:basedOn w:val="Normal" /><w:pPr><w:spacing w:after="0" /><w:jc w:val="right" /></w:pPr><w:rPr><w:color w:val="%s" /><w:sz w:val="18" /><w:szCs w:val="18" /></w:rPr></w:style>' % muted)
    return xml


FOOTER_XML = (
    '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
    '<w:p><w:pPr><w:pStyle w:val="Footer" /></w:pPr><w:r><w:fldChar w:fldCharType="begin" /></w:r><w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r>'
    '<w:r><w:fldChar w:fldCharType="separate" /></w:r><w:r><w:t>1</w:t></w:r><w:r><w:fldChar w:fldCharType="end" /></w:r></w:p></w:ftr>'
)


def style_section(document_xml, paper):
    """Set page size and margins, and reference the footer. Pandoc copies these from the reference doc."""
    width, height = PAPER_TWIPS[paper]
    m = MARGIN_TWIPS
    pg = ('<w:pgSz w:w="%d" w:h="%d" /><w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="%d" w:footer="%d" w:gutter="0" />'
          % (width, height, m["top"], m["right"], m["bottom"], m["left"], m["header"], m["footer"]))
    footer_ref = '<w:footerReference w:type="default" r:id="rIdDocExportFooter" />'
    match = re.search(r"<w:sectPr\b[^>]*>(.*?)</w:sectPr>", document_xml, flags=re.S)
    if match:
        inner = re.sub(r"<w:pgSz\b[^>]*/>|<w:pgMar\b[^>]*/>", "", match.group(1))
        return document_xml[:match.start(1)] + footer_ref + inner + pg + document_xml[match.end(1):]
    return document_xml.replace("</w:body>", "<w:sectPr>%s%s</w:sectPr></w:body>" % (footer_ref, pg))


def build_reference(base_docx, output, font="Calibri", heading_font=None, mono="Consolas", accent="1F4E79", paper="a4"):
    """Write a restyled copy of pandoc's reference.docx. Standard library only, so no Word needed."""
    tmp = output + ".tmp"
    with zipfile.ZipFile(base_docx) as src, zipfile.ZipFile(tmp, "w", zipfile.ZIP_DEFLATED) as dst:
        for item in src.infolist():
            data = src.read(item.filename)
            if item.filename == "word/styles.xml":
                data = style_reference(data.decode("utf-8"), font, heading_font or font, mono, accent).encode("utf-8")
            elif item.filename == "word/document.xml":
                text = data.decode("utf-8")
                if "xmlns:r=" not in text.split(">", 2)[1]:
                    text = text.replace("<w:document ", '<w:document xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ', 1)
                data = style_section(text, paper).encode("utf-8")
            elif item.filename == "word/_rels/document.xml.rels":
                data = data.decode("utf-8").replace("</Relationships>", '<Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Id="rIdDocExportFooter" Target="footer1.xml" /></Relationships>').encode("utf-8")
            elif item.filename == "[Content_Types].xml":
                data = data.decode("utf-8").replace("</Types>", '<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml" /></Types>').encode("utf-8")
            dst.writestr(item, data)
        dst.writestr("word/footer1.xml", FOOTER_XML)
    os.replace(tmp, output)


# Keeps a column of short values from collapsing, and one long cell from taking the whole page.
COLUMN_MIN_CHARS = 8
COLUMN_MAX_CHARS = 60
RULE_COLOR = "BFBFBF"
HEADER_FILL = "F2F2F2"


def _cell_text_length(cell):
    return sum(len(t) for t in re.findall(r"<w:t\b[^>]*>([^<]*)</w:t>", cell))


def _bold_runs(cell):
    """Bold every run in a header cell, keeping rPr's required child order."""
    cell = re.sub(r"<w:r>(?=<w:t\b)", "<w:r><w:rPr><w:b /><w:bCs /></w:rPr>", cell)
    cell = re.sub(r"<w:r><w:rPr>(<w:rStyle\b[^>]*/>)?(?!<w:b\b)", lambda m: "<w:r><w:rPr>%s<w:b /><w:bCs />" % (m.group(1) or ""), cell)
    return cell


def _format_table(table, text_width):
    """Put borders, a header fill, and content-sized widths directly on one table."""
    if table.count("<w:tbl>") > 1 or "<w:gridSpan" in table or "<w:vMerge" in table:
        return table
    columns = len(re.findall(r"<w:gridCol\b", table))
    rows = re.findall(r"<w:tr\b.*?</w:tr>", table, flags=re.S)
    if not columns or not rows:
        return table
    longest = [COLUMN_MIN_CHARS] * columns
    for row in rows:
        for i, cell in enumerate(re.findall(r"<w:tc>.*?</w:tc>", row, flags=re.S)[:columns]):
            longest[i] = max(longest[i], min(_cell_text_length(cell), COLUMN_MAX_CHARS))
    total = sum(longest)
    widths = [text_width * n // total for n in longest]

    def format_row(row):
        header = "<w:tblHeader" in row
        index = [0]

        def format_cell(m):
            cell = m.group(0)
            width = widths[min(index[0], columns - 1)]
            index[0] += 1
            props = '<w:tcW w:w="%d" w:type="dxa" />' % width
            if header:
                props += '<w:shd w:val="clear" w:color="auto" w:fill="%s" />' % HEADER_FILL
                cell = _bold_runs(cell)
            if re.search(r"<w:tcPr\s*/>", cell):
                return re.sub(r"<w:tcPr\s*/>", "<w:tcPr>%s</w:tcPr>" % props, cell, count=1)
            if "<w:tcPr>" in cell:
                return cell.replace("<w:tcPr>", "<w:tcPr>" + props, 1)
            return cell.replace("<w:tc>", "<w:tc><w:tcPr>%s</w:tcPr>" % props, 1)

        row = re.sub(r"<w:tc>.*?</w:tc>", format_cell, row, flags=re.S)
        # A row split across a page is hard to read, so keep each row whole.
        if "<w:trPr>" in row:
            return row.replace("<w:trPr>", "<w:trPr><w:cantSplit />", 1)
        return re.sub(r"<w:tr\b[^>]*>", lambda m: m.group(0) + "<w:trPr><w:cantSplit /></w:trPr>", row, count=1)

    table = re.sub(r"<w:tr\b.*?</w:tr>", lambda m: format_row(m.group(0)), table, flags=re.S)
    grid = "".join('<w:gridCol w:w="%d" />' % w for w in widths)
    table = re.sub(r"<w:tblGrid>.*?</w:tblGrid>", "<w:tblGrid>%s</w:tblGrid>" % grid, table, count=1, flags=re.S)
    borders = "".join('<w:%s w:val="single" w:sz="4" w:space="0" w:color="%s" />' % (s, RULE_COLOR) for s in ("top", "left", "bottom", "right", "insideH", "insideV"))
    margins = '<w:tblCellMar><w:top w:w="50" w:type="dxa" /><w:left w:w="100" w:type="dxa" /><w:bottom w:w="50" w:type="dxa" /><w:right w:w="100" w:type="dxa" /></w:tblCellMar>'
    table = re.sub(r"<w:tblW\b[^>]*/>", '<w:tblW w:w="%d" w:type="dxa" />' % sum(widths), table, count=1)
    return table.replace("<w:tblLook", "<w:tblBorders>%s</w:tblBorders>%s<w:tblLook" % (borders, margins), 1)


def polish_docx(path):
    """Write table formatting into the document.

    Word applies the Table style, but Pages, Google Docs, and macOS preview ignore table styles,
    so a recipient on one of those would otherwise get a borderless grid of equal columns.
    Skipped when the user supplies a reference doc, since their style should win.
    """
    if not zipfile.is_zipfile(path):
        return
    tmp = path + ".polish"
    with zipfile.ZipFile(path) as src, zipfile.ZipFile(tmp, "w", zipfile.ZIP_DEFLATED) as dst:
        for item in src.infolist():
            data = src.read(item.filename)
            if item.filename == "word/document.xml":
                xml = data.decode("utf-8")
                size = re.search(r'<w:pgSz\b[^>]*w:w="(\d+)"', xml)
                left = re.search(r'<w:pgMar\b[^>]*w:left="(\d+)"', xml)
                right = re.search(r'<w:pgMar\b[^>]*w:right="(\d+)"', xml)
                text_width = PAPER_TWIPS["a4"][0] - 2 * MARGIN_TWIPS["left"]
                if size and left and right:
                    text_width = int(size.group(1)) - int(left.group(1)) - int(right.group(1))
                xml = re.sub(r"<w:tbl>.*?</w:tbl>", lambda m: _format_table(m.group(0), text_width), xml, flags=re.S)
                data = xml.encode("utf-8")
            dst.writestr(item, data)
    os.replace(tmp, path)


def make_reference(pandoc, output, args):
    os.makedirs(os.path.dirname(os.path.abspath(output)), exist_ok=True)
    with tempfile.TemporaryDirectory() as d:
        base = os.path.join(d, "default-reference.docx")
        r = subprocess.run([pandoc, "-o", base, "--print-default-data-file", "reference.docx"], capture_output=True, text=True)
        if r.returncode != 0:
            return fail(1, "pandoc could not print its default reference.docx: %s" % r.stderr.strip())
        build_reference(base, output, font=args.font, heading_font=args.heading_font, mono=args.mono, accent=args.accent.lstrip("#").upper(), paper=args.paper)
    return 0


def cmd_reference(args):
    pandoc, _, code = require_pandoc()
    if code:
        return code
    code = make_reference(pandoc, args.output, args)
    if code == 0:
        print("Wrote %s. Pass it with --reference-doc, or edit its styles in Word first." % args.output)
    return code


# ---------------------------------------------------------------- export

def first_heading_info(path):
    """Return (has_title_metadata, promotable_h1, words, headings) for a Markdown file."""
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            text = fh.read()
    except OSError:
        return False, False, 0, 0
    has_title = False
    body = text
    m = re.match(r"---[ \t]*\n(.*?\n)?(?:---|\.\.\.)[ \t]*(?:\n|$)", text, flags=re.S)
    if m:
        has_title = bool(re.search(r"^title\s*:", m.group(1) or "", flags=re.M))
        body = text[m.end():]
    no_code = re.sub(r"^ {0,3}(`{3,}|~{3,}).*?^ {0,3}\1[`~]*\s*$", "", body, flags=re.S | re.M)
    heads = re.findall(r"^(#{1,6})[ \t]+\S", no_code, flags=re.M)
    first = next((ln for ln in no_code.split("\n") if ln.strip()), "")
    promotable = bool(re.match(r"^#[ \t]+\S", first)) and heads.count("#") == 1
    return has_title, promotable, len(re.findall(r"\w+", no_code)), len(heads)


def default_output(inputs, to):
    return os.path.splitext(inputs[0])[0] + OUTPUT_SUFFIX[to]


def git_epoch(path):
    """Commit time of the input, so repeated exports of the same text give identical Word files."""
    try:
        r = subprocess.run(["git", "-C", os.path.dirname(os.path.abspath(path)) or ".", "log", "-1", "--format=%ct", "--", os.path.basename(path)], capture_output=True, text=True, timeout=15)
    except (OSError, subprocess.TimeoutExpired):
        return None
    return r.stdout.strip() or None


def build_export_command(args, pandoc, engine, defaults_name, reference, log_path, output, extra_css=None):
    """Build the pandoc argv. It runs from the first input's folder, and returns that folder too.

    Pandoc rewrites a link between input files only when the link matches the path given on the
    command line, so inputs are passed relative to the folder they link from.
    """
    run_dir = os.path.dirname(os.path.abspath(args.inputs[0]))
    cmd = [pandoc, "--defaults", os.path.join(DEFAULTS_DIR, defaults_name + ".yaml"), "--from", args.source_format]
    if args.to == "pdf" and engine and engine != {"pdf-typst": "typst", "pdf-weasyprint": "weasyprint", "pdf-xelatex": "xelatex"}[defaults_name]:
        cmd.append("--pdf-engine=" + engine)
    if reference:
        cmd.append("--reference-doc=" + os.path.abspath(reference))
    infos = [first_heading_info(p) for p in args.inputs]
    has_title, promotable, words, headings = infos[0]
    words = sum(i[2] for i in infos)
    headings = sum(i[3] for i in infos)
    if args.title:
        cmd += ["--metadata", "title=" + args.title]
    if len(args.inputs) == 1 and not has_title and promotable:
        # With a shift of -1, a lone leading H1 becomes the title and H2s become top-level sections.
        cmd.append("--shift-heading-level-by=-1")
    for key, value in (("subtitle", args.subtitle), ("date", args.date), ("lang", args.lang), ("doc-export-link-base", args.link_base)):
        if value:
            cmd += ["--metadata", "%s=%s" % (key, value)]
    for name in args.author or []:
        cmd += ["--metadata", "author=" + name]
    if args.mermaid == "off":
        cmd += ["--metadata", "doc-export-mermaid=off"]
    want_toc = args.toc == "yes" or (args.toc == "auto" and words >= TOC_MIN_WORDS and headings >= TOC_MIN_HEADINGS)
    if want_toc:
        cmd += ["--toc", "--toc-depth=%d" % args.toc_depth]
    if args.number_sections:
        cmd.append("--number-sections")
    if args.to == "pdf" and defaults_name == "pdf-typst":
        cmd += ["--variable", "papersize=%s" % ("a4" if args.paper == "a4" else "us-letter")]
        if args.font != DEFAULT_FONT:
            cmd += ["--variable", "mainfont=" + args.font]
    if args.to == "pdf" and defaults_name == "pdf-xelatex":
        cmd += ["--variable", "papersize=%s" % ("a4" if args.paper == "a4" else "letter")]
        if args.font != DEFAULT_FONT:
            cmd += ["--variable", "mainfont=" + args.font]
    if args.to == "html" and args.css:
        cmd += ["--css", os.path.abspath(args.css)]
    if extra_css:
        cmd += ["--css", extra_css]
    if args.to == "html" and args.title:
        cmd += ["--metadata", "pagetitle=" + args.title]
    dirs = []
    for p in args.inputs:
        d = os.path.dirname(os.path.abspath(p))
        if d not in dirs:
            dirs.append(d)
    # Naming a resource path removes the working directory from the search unless it is listed.
    cmd.append("--resource-path=" + os.pathsep.join(dirs + [os.getcwd()]))
    if args.sandbox:
        cmd.append("--sandbox")
    cmd += ["--log=" + log_path, "--output", os.path.abspath(output)]
    cmd += [os.path.relpath(os.path.abspath(p), run_dir) for p in args.inputs]
    return cmd, want_toc, run_dir


def read_log(path):
    try:
        with open(path, encoding="utf-8") as fh:
            entries = json.load(fh)
    except (OSError, ValueError):
        return []
    return [e for e in entries if e.get("type") in REPORTED_LOG_TYPES and e.get("verbosity") in ("WARNING", "ERROR")]


def describe_log_entry(entry):
    """Pandoc's log gives a readable sentence in "pretty"; fall back to the raw fields."""
    return entry.get("pretty") or "%s: %s" % (entry.get("type"), {k: v for k, v in entry.items() if k not in ("type", "verbosity")})


def word_count_of(pandoc, path, from_format=None):
    cmd = [pandoc, path, "--to", "plain", "--wrap=none", "--standalone"]
    if from_format:
        cmd += ["--from", from_format]
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, errors="replace", timeout=120)
    except (OSError, subprocess.TimeoutExpired):
        return None
    return len(re.findall(r"\w+", r.stdout)) if r.returncode == 0 else None


def cmd_export(args):
    missing = [p for p in args.inputs if not os.path.isfile(p)]
    if missing:
        return fail(2, "input not found: %s" % ", ".join(missing))
    for p, label in ((args.reference_doc, "--reference-doc"), (args.css, "--css")):
        if p and not os.path.isfile(p):
            return fail(2, "%s file not found: %s" % (label, p))
    if args.reference_doc and args.to != "docx":
        return fail(2, "--reference-doc only applies to --to docx")

    output = args.output or default_output(args.inputs, args.to)
    if os.path.splitext(output)[1].lower() != OUTPUT_SUFFIX[args.to]:
        return fail(2, "-o %s does not end in %s, which --to %s produces" % (output, OUTPUT_SUFFIX[args.to], args.to))
    pandoc = pandoc_path() or "pandoc"
    if not args.dry_run:
        pandoc, _, code = require_pandoc()
        if code:
            return code

    engine, defaults_name = None, {"docx": "docx", "html": "html"}.get(args.to)
    if args.to == "pdf":
        engine = choose_pdf_engine(args.pdf_engine)
        if not engine and args.dry_run:
            engine = "typst" if args.pdf_engine == "auto" else args.pdf_engine
        if not engine:
            found = installed_pdf_engines()
            if args.pdf_engine != "auto" and found:
                return fail(3, "%s is not installed. Installed: %s. Drop --pdf-engine to use it, or ask the user before installing %s." % (args.pdf_engine, ", ".join(found), args.pdf_engine))
            return fail(3, "No PDF engine found. Word output needs none, so offer --to docx. Otherwise ask the user before installing one:\n%s" % install_help({"pdf"}))
        defaults_name = defaults_for_engine(engine)

    with tempfile.TemporaryDirectory() as work:
        reference = args.reference_doc
        if args.to == "docx" and not reference:
            reference = os.path.join(work, "reference.docx")
            if not args.dry_run:
                code = make_reference(pandoc, reference, args)
                if code:
                    return code
        log_path = os.path.join(work, "pandoc-log.json")
        extra_css = None
        if defaults_name == "pdf-weasyprint" and args.paper != "a4":
            extra_css = os.path.join(work, "paper.css")
            with open(extra_css, "w", encoding="utf-8") as fh:
                fh.write("@page { size: %s; }\n" % args.paper)
        cmd, used_toc, run_dir = build_export_command(args, pandoc, engine, defaults_name, reference, log_path, output, extra_css)
        if args.dry_run:
            print(" ".join(shlex_quote(c) for c in cmd))
            return 0
        os.makedirs(os.path.dirname(os.path.abspath(output)), exist_ok=True)
        env = dict(os.environ)
        if "SOURCE_DATE_EPOCH" not in env:
            epoch = git_epoch(args.inputs[0])
            if epoch:
                env["SOURCE_DATE_EPOCH"] = epoch
        r = subprocess.run(cmd, capture_output=True, text=True, errors="replace", env=env, cwd=run_dir)
        filter_notes = [ln for ln in r.stderr.splitlines() if ln.startswith("[doc_export]")]
        if r.returncode != 0:
            return fail(1, "pandoc exited with %d:\n%s" % (r.returncode, r.stderr.strip()))
        entries = read_log(log_path)
    if args.to == "docx" and not args.reference_doc:
        polish_docx(os.path.abspath(output))

    problems = [describe_log_entry(e) for e in entries]
    blocking = [describe_log_entry(e) for e in entries if e.get("type") in BLOCKING_LOG_TYPES]
    notes = [n.replace("[doc_export] ", "") for n in filter_notes]
    ratio = None
    if args.to in ("docx", "html"):
        out_words = word_count_of(pandoc, output)
        in_words = sum(filter(None, (word_count_of(pandoc, p, args.source_format) for p in args.inputs)))
        if out_words is not None and in_words:
            ratio = round(out_words / in_words, 2)
            if ratio < MIN_TEXT_RATIO:
                problems.append("output has %d%% of the source's words; check for dropped content" % int(ratio * 100))
                blocking.append(problems[-1])
    size = os.path.getsize(output) if os.path.exists(output) else 0
    summary = {"output": output, "bytes": size, "format": args.to, "pdf_engine": engine, "toc": used_toc,
               "text_ratio": ratio, "problems": problems, "notes": notes}
    if args.json:
        print(json.dumps(summary, indent=2))
    else:
        via = " via %s" % engine if engine else ""
        print("Wrote %s (%d KB, %s%s%s)" % (output, size // 1024, args.to, via, ", with contents" if used_toc else ""))
    if not args.json:
        for p in problems:
            print("problem: " + p, file=sys.stderr)
        for n in notes:
            print("note: " + n, file=sys.stderr)
    if blocking and not args.allow_warnings:
        return 4
    return 0


def shlex_quote(s):
    return s if re.match(r"^[\w@%+=:,./-]+$", s) else "'" + s.replace("'", "'\\''") + "'"


# ---------------------------------------------------------------- import

def cmd_import(args):
    if not os.path.isfile(args.input):
        return fail(2, "input not found: %s" % args.input)
    if not args.output.lower().endswith((".md", ".markdown")):
        return fail(2, "-o must end in .md; import writes Markdown")
    pandoc = pandoc_path() or "pandoc"
    if not args.dry_run:
        pandoc, _, code = require_pandoc()
        if code:
            return code
    media = args.media_dir or os.path.splitext(args.output)[0] + "-media"
    cmd = [pandoc, args.input, "--to", "gfm", "--wrap=none", "--markdown-headings=atx", "--extract-media=" + media,
           "--track-changes=" + args.track_changes, "--output", args.output]
    if args.from_format:
        cmd += ["--from", args.from_format]
    if args.sandbox:
        cmd.append("--sandbox")
    if args.dry_run:
        print(" ".join(shlex_quote(c) for c in cmd))
        return 0
    os.makedirs(os.path.dirname(os.path.abspath(args.output)), exist_ok=True)
    r = subprocess.run(cmd, capture_output=True, text=True, errors="replace")
    if r.returncode != 0:
        return fail(1, "pandoc exited with %d:\n%s" % (r.returncode, r.stderr.strip()))
    if r.stderr.strip():
        print(r.stderr.strip(), file=sys.stderr)
    size = os.path.getsize(args.output)
    if args.json:
        print(json.dumps({"output": args.output, "bytes": size, "media_dir": media if os.path.isdir(media) else None}, indent=2))
    else:
        print("Wrote %s (%d KB)%s. Its statements are claims from an old document, so verify them before relying on them." % (args.output, size // 1024, "; images in %s" % media if os.path.isdir(media) else ""))
    return 0


# ---------------------------------------------------------------- arguments

DEFAULT_FONT = "Calibri"


def add_style_options(p):
    p.add_argument("--font", default=DEFAULT_FONT, help="body font (default %(default)s)")
    p.add_argument("--heading-font", help="heading font (default: same as --font)")
    p.add_argument("--mono", default="Consolas", help="code font for Word (default %(default)s)")
    p.add_argument("--accent", default="1F4E79", help="heading and link color as hex (default %(default)s)")
    p.add_argument("--paper", choices=["a4", "letter"], default="a4", help="page size (default %(default)s)")


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="command", required=True)

    d = sub.add_parser("doctor", help="report which tools are installed")
    d.add_argument("--json", action="store_true")
    d.set_defaults(func=cmd_doctor)

    e = sub.add_parser("export", help="convert Markdown to docx, pdf, or html")
    e.add_argument("inputs", nargs="+", help="Markdown files, joined in order")
    e.add_argument("--to", choices=["docx", "pdf", "html"], required=True)
    e.add_argument("-o", "--output", help="output path (default: first input with the new suffix)")
    e.add_argument("--from", dest="source_format", default="gfm", help="input format; use commonmark_x or markdown for fenced divs and attributes (default %(default)s)")
    e.add_argument("--title")
    e.add_argument("--subtitle")
    e.add_argument("--author", action="append", help="repeat for several authors")
    e.add_argument("--date", help="date to print, as written")
    e.add_argument("--lang", help="document language, such as en-GB")
    e.add_argument("--toc", choices=["auto", "yes", "no"], default="auto", help="table of contents; auto adds one to long documents (default %(default)s)")
    e.add_argument("--toc-depth", type=int, default=3)
    e.add_argument("--number-sections", action="store_true")
    e.add_argument("--reference-doc", help="Word template to use in place of the generated one")
    e.add_argument("--css", help="extra stylesheet for html")
    e.add_argument("--pdf-engine", choices=PDF_ENGINE_CHOICES, default="auto", help="default: first installed of typst, weasyprint, xelatex, lualatex, tectonic")
    e.add_argument("--link-base", help="URL prefix for repo-relative links, such as https://github.com/org/repo/blob/main/docs; without it those links become plain text")
    e.add_argument("--mermaid", choices=["auto", "off"], default="auto", help="render mermaid blocks when mmdc is installed (default %(default)s)")
    e.add_argument("--sandbox", action="store_true", help="for untrusted input: stop the document reading local files, including its own images")
    e.add_argument("--allow-warnings", action="store_true", help="exit 0 even when images are missing or text was dropped")
    e.add_argument("--dry-run", action="store_true", help="print the pandoc command without running it")
    e.add_argument("--json", action="store_true", help="print a JSON summary on stdout")
    add_style_options(e)
    e.set_defaults(func=cmd_export)

    i = sub.add_parser("import", help="convert docx, html, rst, and other formats to GitHub-flavored Markdown")
    i.add_argument("input")
    i.add_argument("-o", "--output", required=True, help="Markdown file to write")
    i.add_argument("--from", dest="from_format", help="input format, when the file suffix is not enough")
    i.add_argument("--media-dir", help="where to extract images (default: next to the output)")
    i.add_argument("--track-changes", choices=["accept", "reject", "all"], default="accept", help="Word tracked changes (default %(default)s)")
    i.add_argument("--sandbox", action="store_true", help="for a third party's file: stop it reading local files")
    i.add_argument("--dry-run", action="store_true", help="print the pandoc command without running it")
    i.add_argument("--json", action="store_true")
    i.set_defaults(func=cmd_import)

    r = sub.add_parser("reference-doc", help="write a Word style template to commit and reuse")
    r.add_argument("-o", "--output", default="reference.docx")
    add_style_options(r)
    r.set_defaults(func=cmd_reference)

    args = ap.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
