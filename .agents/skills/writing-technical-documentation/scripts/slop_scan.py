#!/usr/bin/env python3
"""Scan prose for AI writing patterns, style problems, and structure statistics.

Usage:
    slop_scan.py FILE_OR_DIR [FILE_OR_DIR ...] [options]
    cat draft.md | slop_scan.py -

Examples:
    slop_scan.py README.md
    slop_scan.py docs --quiet
    slop_scan.py docs/guide.md --allow ecosystem,scalable --max-score 10
    slop_scan.py docs --format github --max-score 15

Score counts AI-writing signals only: weighted hits per 1,000 words, with a floor of 300
words so a short text is not judged on two hits, and a cap per category so one habit cannot
dominate. Style notes (filler words, heading habits, wordy phrases) are listed and counted
separately because people write them too. Bands: under 8 clean; 8 to 20 needs a pass; above
20 rewrite from evidence. Treat the score as a prompt for judgment and never as proof of authorship.

The scanner finds symptoms. Fix the cause: replace a vague sentence with a specific fact
from the code, or delete it. Swapping a synonym keeps the emptiness.

To quote a pattern on purpose, wrap the passage:
    <!-- slop-scan: off --> ... <!-- slop-scan: on -->

Exit codes: 0 finished; 1 a file scored above --max-score; 2 bad usage or unreadable file.
Standard library only.
"""
import argparse
import json
import os
import re
import statistics
import sys

import doclib

EM = "—"
EN = "–"
# Keeps a short text from scoring off the chart on a couple of hits.
SCORE_WORD_FLOOR = 300
# One habit (say, bold-lead bullets everywhere) counts once at full strength. Breadth across
# categories is the signal, per the Wikipedia "Signs of AI writing" guidance on clusters.
CATEGORY_WEIGHT_CAP = 8
# Set with tests/calibrate_slop_scan.py on 115 human-written READMEs and reference pages: 90 percent
# scored under BAND_CLEAN and 3 percent reached BAND_PASS. See docs/research.md in the source repo.
BAND_CLEAN = 8
BAND_PASS = 20
STYLE_CATEGORIES = {
    "filler-word", "wordy-phrase", "em-dash", "en-dash", "title-case-heading", "heading-punctuation",
    "heading-level", "heading-inflation", "long-sentence", "timeless-text", "repeated-phrase", "repeated-opener",
}
DEFAULT_FINDING_LIMIT = 60
LONG_SENTENCE_WORDS = 40
# Directory scans print details for the worst few files; --quiet prints only the table.
DETAIL_FILES_IN_DIR_SCAN = 3

# Weights: 1 = weak signal, 2 = moderate, 3 = strong.
WORDS_HIGH = [
    "delve", "delves", "delving", "tapestry", "testament", "beacon", "cornerstone", "multifaceted",
    "myriad", "plethora", "realm", "interplay", "unparalleled", "groundbreaking", "game-changer",
    "game changer", "game-changing", "paradigm", "synergy", "synergies", "seamless", "seamlessly",
    "meticulous", "meticulously", "intricate", "intricacies", "bustling", "unlock", "unlocks", "unleash",
    "unleashes", "supercharge", "supercharges", "revolutionize", "revolutionizes", "revolutionary",
    "cutting-edge", "state-of-the-art", "best-in-class", "world-class", "battle-tested", "future-proof",
    "blazing fast", "blazingly fast", "enterprise-grade", "mission-critical", "turnkey", "holistic",
    "transformative", "indelible", "linchpin", "nexus",
]
WORDS_MED = [
    "leverage", "leverages", "leveraged", "leveraging", "harness", "harnesses", "harnessing", "utilize",
    "utilizes", "utilized", "utilizing", "showcase", "showcases", "showcasing", "underscoring", "foster", "fosters", "fostering", "facilitate", "facilitates", "facilitating", "empower",
    "empowers", "empowering", "elevate", "elevates", "elevating", "embark", "embarks", "streamline",
    "streamlines", "streamlining", "bolster", "bolsters", "bolstered", "garner", "garners", "pivotal",
    "crucial", "vital", "paramount", "robust", "comprehensive", "nuanced", "vibrant", "innovative", "journey",
    "spearhead", "resonate", "resonates", "cultivate", "cultivates", "indispensable", "invaluable",
    "noteworthy", "commendable", "profound", "compelling", "sophisticated", "elegant", "versatile",
    "developer-friendly", "load-bearing", "best practices", "deep dive", "key takeaways", "unpack",
    "unpacking", "reimagine", "reimagined", "ever-evolving", "fast-paced",
]
# These have real technical senses, so a single use is a weak signal at most.
WORDS_AMBIGUOUS = [
    "dynamic", "lightweight", "end-to-end", "scalable", "ecosystem", "orchestrate", "orchestrates",
    "landscape", "stakeholders", "powerful", "production-ready", "evolving", "intuitive", "out of the box",
    "under the hood", "genuinely", "honestly", "substantive", "rigorous", "thoughtful",
]
WORDS_FILLER = [
    "truly", "really", "very", "extremely", "incredibly", "remarkably", "undeniably", "arguably",
    "literally", "basically", "essentially", "fundamentally", "quite", "simply", "just", "easily", "easy",
    "obviously", "clearly", "of course", "straightforward", "trivial", "trivially", "significantly",
]
# Common words that later model generations overuse (Wikipedia, "Signs of AI writing"). Any one
# is normal, so only the density across a document counts.
ERA_WORDS = [
    "align with", "aligns with", "enhance", "enhances", "enhancing", "quietly", "steady", "dependable",
    "echoed", "prioritize", "prioritizes", "universally", "valuable", "enduring", "key",
]
ERA_DENSITY_PER_1000 = 12
ERA_MIN_WORDS = 200
# Transition words alone do not indicate AI text, so only heavy paragraph-opening use counts.
CONNECTORS = (
    "moreover|furthermore|additionally|in addition|consequently|thus|hence|therefore|however|nevertheless|"
    "nonetheless|overall|ultimately|notably|importantly|crucially|indeed|accordingly|subsequently|that said"
)

PHRASES = [
    (3, r"\bin today'?s (?:fast[- ]paced|digital|modern|ever[- ]changing|rapidly evolving)\b", "time-parasite opener; delete"),
    (3, r"\bin the (?:realm|world|age|era) of\b", "filler opener; name the subject"),
    (3, r"\bnow more than ever\b", "time parasite; delete"),
    (3, r"\b(?:it'?s|it is) (?:worth|important|crucial|essential) (?:noting|to note|mentioning|to mention|to remember)\b", "filler; state the fact"),
    (3, r"\bit (?:should|must) be noted\b", "filler; state the fact"),
    (2, r"\bneedless to say\b|\bit goes without saying\b", "filler; delete"),
    (3, r"\blet'?s (?:dive|delve|explore|unpack|break (?:it|this) down|get started)\b", "assistant filler; delete"),
    (2, r"\bwithout further ado\b", "filler; delete"),
    (2, r"\bhere'?s (?:the thing|the kicker|why|what you need to know|the catch|where it gets)\b|\bit gets better\b|\bwhat'?s more\b|\bthe short (?:answer|version) is\b", "staged run-up; state the point"),
    (3, r"\bimagine a world where\b", "dramatic setup; delete"),
    (2, r"\bat (?:its|the) core\b|\bat the heart of\b|\bat the end of the day\b|\bat the intersection of\b", "filler; name the mechanism"),
    (3, r"\bplays? an? (?:crucial|pivotal|vital|key|important|significant|essential) role\b", "significance inflation; give the fact"),
    (3, r"\b(?:serves?|stands?) as an? (?:testament|reminder|beacon|cornerstone)\b|\ba testament to\b", "significance inflation"),
    (3, r"\bmarks? an? (?:significant|pivotal|important|major) (?:milestone|moment|shift|step)\b|\bkey turning point\b", "significance inflation"),
    (2, r"\bpaves? the way (?:for|to)\b|\bsets? the stage for\b|\bopens? the door to\b", "vague causation; name the effect"),
    (2, r"\btakes? (?:it|\w+) to the next level\b|\bunlock(?:s|ing)? the (?:full )?potential\b", "hype; delete"),
    (2, r"\ba (?:wide|broad|vast) (?:range|array|variety|spectrum) of\b|\ba (?:variety|multitude|host) of\b", "vague quantity; list or count them"),
    (3, r"\bwhether you(?:'re| are) an? \w+(?: or| and)? (?:an? )?\w+", "audience mush; name the reader"),
    (2, r"\bnavigate the (?:complexities|challenges|landscape)\b", "stock phrase; delete"),
    (3, r"\bi hope this helps\b|\bfeel free to (?:reach out|ask)\b|\bdon'?t hesitate to\b|\blet me know\b|\bwould you like me to\b|\bi can also\b|\bas requested\b", "leaked assistant text; delete"),
    (3, r"\bas an ai\b|\bas a language model\b|\bknowledge cutoff\b|\bas of my last\b|\bbased on the (?:provided|information you)\b", "leaked assistant text; delete"),
    (3, r"\bhere(?:'s| is) (?:a|an|the|your) (?:\w+ )?(?:draft|revised|rewritten|updated|polished) \w+|\b(?:check|verify|confirm)\b[^.\n]{0,40}\bbefore (?:publishing|sending)\b", "leaked assistant text; delete"),
    (2, r"\b(?:great|good|excellent) question\b|\byou'?re absolutely right\b", "sycophancy; delete"),
    (2, r"^\s*(?:certainly|absolutely|sure|of course)[!,.]", "assistant opener; delete"),
    (2, r"\b(?:in summary|in conclusion|to sum up|to recap|all in all|in short|overall),", "recap opener; usually delete"),
    (2, r"\bkey takeaways?\b|\bfinal thoughts\b", "recap section; delete"),
    (2, r"\b(?:this|the following) (?:comprehensive|complete|ultimate|detailed) guide\b", "self-labeling; delete"),
    (2, r"\bthis (?:document|guide|section|article|page) (?:will|aims to|provides?|covers?|explores?|outlines?|describes?)\b", "preamble; start with content"),
    (1, r"\bas (?:mentioned|discussed|noted) (?:above|earlier|previously|before)\b|\bas we'?ll see (?:below|later)\b", "narrated structure; link or delete"),
    (2, r"\bexperts? (?:say|agree|believe|argue)\b|\bstudies (?:show|suggest|have shown)\b|\bresearch (?:shows|suggests)\b|\bindustry (?:best practices?|standards?|experts?|reports?)\b|\bit is (?:widely|generally|commonly) (?:believed|accepted|known)\b|\bobservers have\b", "vague attribution; cite or own it"),
    (2, r"\bsome (?:argue|believe|say|critics)\b|\bmany (?:developers|engineers|people|teams) (?:believe|think|say|find)\b|\bcritics (?:say|argue)\b", "vague attribution"),
    (2, r"\bin my experience\b|\bi'?ve (?:seen|found|noticed)\b|\bwe'?re (?:excited|thrilled|proud)\b|\bi'?m (?:excited|thrilled)\b", "invented experience or emotion"),
    (2, r"\bthink of (?:it|this|\w+) (?:as|like)\b", "metaphor stacking; state the mechanism"),
    (2, r"\bthe (?:key|real|actual|true) (?:insight|issue|problem|challenge|question) (?:is|here)\b", "dramatic framing; state it"),
    (2, r"\bcontrary to popular belief\b|\byou might (?:think|assume|expect)\b|\byou may be wondering\b|\bsome might say\b", "arguing with no one; state what is true"),
    (2, r"\b(?:\w+) is (?:the )?(?:foundation|bridge|backbone|lifeblood|currency) (?:of|to|between)\b", "pseudo-profound saying; delete"),
    (2, r"\band that'?s (?:the point|what matters|the whole point)\b|\bfull stop\.|(?<=[.!?] )Period\.", "dramatic closer; delete"),
    (2, r"\b(?:trusted|loved|used) by (?:thousands|millions|teams|developers|companies)\b", "promotional claim; give the number and source, or cut"),
    (1, r"\bit (?:is|'s) important (?:that|to)\b", "empty subject; use a direct statement"),
    (2, r"\bunderscores? (?:the|its|how|that|why)\b", "significance inflation; give the fact"),
    (2, r"\bfeel free to\b", "filler; use an imperative"),
    (2, r"\bthe (?:best|good|great) part\?|\bthe (?:result|catch|answer|upshot|kicker|takeaway)\?", "self-answered question; state it"),
    (2, r"\b(?:can|may|might) (?:potentially|possibly) \w+", "hedge stack; state the condition"),
    (2, r"\b(?:is|are) designed to\b|\baims? to\b|\bstrives? to\b|\bseeks? to\b", "vague purpose verb; use the real verb"),
    (1, r"\bcan help (?:you|to|ensure|improve)\b|\bhelps? to (?:ensure|improve|reduce)\b|\bhelps? ensure\b", "vague benefit; give the mechanism"),
    (2, r"\bthis (?:ensures|provides|helps)\b", "vague 'this' subject; name the thing"),
    (2, r"\bincluding but not limited to\b|\band much more\b", "open-ended list; enumerate or count"),
    (2, r"\b(?:easy|simple|straightforward) to (?:use|set up|install|configure|integrate)\b", "unproven adjective; show the steps and the time"),
    (2, r"\bi want to be (?:careful|precise|clear)\b|\bto be (?:honest|frank)\b", "assistant meta-commentary; delete"),
]
# Run on the unmasked text, because these often hide inside URLs and code.
ARTIFACTS = (
    r"oaicite|contentReference|oai_citation|turn\d+search\d+|utm_source=(?:chatgpt|openai)|citeturn|grok_card|"
    r"referrer=grok|attached_file|↩"
)
PLACEHOLDERS = (
    r"\[(?i:insert|your|company|name|date|link|todo|tbd)\b[^\]\n]*\](?!\()|(?i:\blorem ipsum\b)|\b(?:TBD|FIXME|TODO)\b|"
    r"\b\d{4}-XX-XX\b|\bINSERT_[A-Z_]+\b"
)

NEG_PARALLEL = [
    re.compile(r"\b(?:it|this|that|these|they)(?:'s| is| isn't| is not| are not|'re not| aren't)(?: just| only| merely| simply)? (?:a |an |the )?[^.;:\n]{1,60}[,;] (?:it|this|that|they)(?:'s| is| are|'re)\b", re.I),
    re.compile(r"\bnot (?:just|only|merely|simply) (?:a |an |the )?[^.;:\n]{1,60}[,;] (?:but|it|this|they)\b", re.I),
    re.compile(r"\bnot only\b.{1,80}\bbut also\b", re.I),
    re.compile(r"\b(?:isn't|is not|aren't|are not|wasn't|doesn't just|don't just)\s+(?:just|only|merely|simply)\b", re.I),
    re.compile(r"\bmore than just\b|\bless about \w+(?: \w+)?,? more about\b", re.I),
    re.compile(r"\bno \w+(?: \w+)?,? no \w+(?: \w+)?,? just\b", re.I),
    re.compile(r"\bisn'?t (?:about|just about) [^.;:\n]{1,60}[.;,] it'?s about\b", re.I),
]
TRAILING_PARTICIPLE = re.compile(
    r",\s+(?:ensuring|highlighting|underscoring|showcasing|reflecting|symbolizing|contributing to|fostering|"
    r"emphasizing|demonstrating|empowering|helping|paving|aligning with|reinforcing|cementing|solidifying|"
    r"signaling|enhancing|encompassing|cultivating|making it|making them)\b[^.\n]{0,100}[.\n]", re.I)
COPULA_AVOID = re.compile(r"\b(?:serves|stands|acts|functions|operates) as\b|\bboasts?\b|\brepresents? a\b|\bfeatures? (?:a|an|the) \w+", re.I)
TIME_WORDS = re.compile(
    r"\b(?:currently|presently|at the moment|as of (?:this writing|now|today)|in the (?:near )?future|"
    r"upcoming|soon|newly|recently|nowadays|these days)\b", re.I)
HEDGES = re.compile(r"\b(?:may|might|could|perhaps|possibly|potentially|generally|typically|usually|often|likely|somewhat|arguably)\b", re.I)
RATHER_THAN = re.compile(r"\brather than\b", re.I)
WORDY = [
    (r"\bwhen it comes to\b|\bwith regard to\b|\bin order to\b", "wordy; shorten"),
    (r"\band more\.\s|\betc\.", "open-ended list; enumerate or count"),
    (r"\bit (?:is|'s) (?:recommended|advisable|possible) (?:that|to)\b", "empty subject; use a direct statement"),
    (r"\bthis (?:allows|enables)\b", "vague 'this' subject; name the thing"),
    (r"\bplease note\b|\bkeep in mind that\b|\bremember,? (?:that|to)\b", "filler; state the fact"),
]

TITLE_CASE_WORDS_MIN = 3
SMALL_WORDS = {"a", "an", "the", "and", "or", "but", "for", "nor", "on", "at", "to", "from", "by", "of", "in", "with", "as", "vs"}
BOLD_LEAD = re.compile(r"^[ \t]*(?:[-*+]|\d+[.)])[ \t]+\*\*[^*\n]{1,60}\*\*", re.M)
EMOJI = re.compile("[\U0001F300-\U0001FAFF\U00002600-\U000027BF\U0001F000-\U0001F2FF⭐✅❌]")
SMART_QUOTES = re.compile("[‘’“”]")
ARROWS = re.compile("[→⇒←↔⟶]")
ZERO_WIDTH = re.compile("[​‌‍⁠﻿]")
NBSP = re.compile(" ")
FILLER_HEADINGS = {
    "overview", "introduction", "conclusion", "summary", "final thoughts", "key takeaways", "key considerations",
    "best practices", "benefits", "use cases", "future work", "future directions", "future outlook",
    "getting started with confidence", "why it matters", "the bottom line", "wrapping up", "key features",
}
PAIR_HEADING_WORDS = {
    "challenges", "opportunities", "benefits", "limitations", "advantages", "disadvantages", "legacy", "impact",
    "trends", "future", "outlook", "considerations", "recommendations", "features", "recognition", "awards",
}
UNVERIFIED_MARK = re.compile(r"\[(?:unverified|to confirm):[^\]\n]*\]", re.I)


def mask_regions(raw):
    """Return (text with front matter and off-regions blanked, scan text with code also blanked)."""
    _, text = doclib.strip_front_matter(raw)

    def blank(m):
        return re.sub(r"[^\n]", " ", m.group(0))
    text = re.sub(r"<!--\s*slop-scan:\s*off\s*-->.*?(?:<!--\s*slop-scan:\s*on\s*-->|\Z)", blank, text, flags=re.S)
    return text


def mask_for_prose(text, mask_code=True):
    def blank(m):
        return re.sub(r"[^\n]", " ", m.group(0))
    scan_text = text
    if mask_code:
        scan_text = doclib.split_code(scan_text)[0]
        scan_text = doclib.mask_inline_code(scan_text)
    scan_text = re.sub(r"<!--.*?-->", blank, scan_text, flags=re.S)
    # Link targets are not prose; artifact patterns inside URLs are checked on the raw text.
    scan_text = re.sub(r"(?<=\])\([^)\n]*\)", blank, scan_text)
    scan_text = re.sub(r"https?://\S+", blank, scan_text)
    return scan_text


def line_of(text, pos):
    return text.count("\n", 0, pos) + 1


def snippet(text, start, end, width=70):
    s = max(0, start - 20)
    e = min(len(text), end + width - 20)
    return re.sub(r"\s+", " ", text[s:e]).strip()


def split_sentences(text):
    plain = re.sub(r"^\s*(?:#+|[-*+]|\d+[.)]|>|\|)\s*", "", text, flags=re.M)
    parts = re.split(r"(?<=[.!?])\s+(?=[A-Z\"'(])|\n\s*\n", plain)
    return [p.strip() for p in parts if p.strip()]


def word_count(text):
    return len(re.findall(r"[A-Za-z0-9][A-Za-z0-9'_-]*", text))


def scan(raw, mask=True, allow=()):
    """Scan one text. Returns score, words, hits, by_category, clusters, and stats."""
    text = mask_regions(raw)
    scan_text = mask_for_prose(text, mask_code=mask)
    allowed = {w.lower() for w in allow}
    hits = []

    def add(cat, weight, start, end, hint, source=None):
        src = scan_text if source is None else source
        hits.append({"category": cat, "weight": weight, "line": line_of(src, start),
                     "match": src[start:end].strip()[:80], "context": snippet(src, start, end), "fix": hint})

    def add_doc(cat, weight, match, context, hint):
        hits.append({"category": cat, "weight": weight, "line": 0, "match": match, "context": context, "fix": hint})

    for words, weight, cat in ((WORDS_HIGH, 3, "vocabulary-high"), (WORDS_MED, 2, "vocabulary"),
                               (WORDS_AMBIGUOUS, 1, "vocabulary-weak"), (WORDS_FILLER, 1, "filler-word")):
        usable = [w for w in words if w not in allowed]
        pattern = re.compile(r"\b(?:" + "|".join(re.escape(w) for w in sorted(usable, key=len, reverse=True)) + r")\b", re.I)
        for m in pattern.finditer(scan_text):
            add(cat, weight, m.start(), m.end(), "replace with the concrete fact, or delete")

    for weight, pat, hint in PHRASES:
        for m in re.finditer(pat, scan_text, flags=re.I | re.M):
            add("phrase", weight, m.start(), m.end(), hint)
    for pat, hint in WORDY:
        for m in re.finditer(pat, scan_text, flags=re.I):
            add("wordy-phrase", 1, m.start(), m.end(), hint)
    for m in re.finditer(ARTIFACTS, text):
        add("leaked-artifact", 3, m.start(), m.end(), "tool output leaked into the text; delete", source=text)
    # Link targets stay visible here, so "[Name](url)" is not mistaken for a "[Name]" placeholder.
    code_masked = doclib.mask_inline_code(doclib.split_code(text)[0]) if mask else text
    for m in re.finditer(PLACEHOLDERS, code_masked):
        add("placeholder", 3, m.start(), m.end(), "fill with a verified fact, mark [To confirm: ...], or delete", source=code_masked)

    taken = []
    for rx in NEG_PARALLEL:
        for m in rx.finditer(scan_text):
            # Several patterns can match one sentence; count it once.
            if not any(m.start() < e and s < m.end() for s, e in taken):
                taken.append((m.start(), m.end()))
                add("negative-parallelism", 3, m.start(), m.end(), "state the true claim only")
    for m in TRAILING_PARTICIPLE.finditer(scan_text):
        add("trailing-participle", 2, m.start(), m.end(), "delete, or make a separate sentence with a fact")
    for m in COPULA_AVOID.finditer(scan_text):
        add("copula-avoidance", 1, m.start(), m.end(), "use 'is' or 'has'")
    for m in TIME_WORDS.finditer(scan_text):
        add("timeless-text", 1, m.start(), m.end(), "use a version or date, or state what is true")

    em_count = scan_text.count(EM)
    for m in re.finditer(EM, scan_text):
        add("em-dash", 1, m.start(), m.end(), "use a period, comma, colon, or parentheses")
    for m in re.finditer(EN, scan_text):
        if not re.search(r"\d\s*" + EN + r"\s*\d", scan_text[max(0, m.start() - 3):m.end() + 3]):
            add("en-dash", 1, m.start(), m.end(), "use a hyphen or restructure")
    for m in BOLD_LEAD.finditer(scan_text):
        add("bold-lead-bullet", 2, m.start(), m.end(), "use a table or plain sentences")
    for m in EMOJI.finditer(scan_text):
        add("emoji", 2, m.start(), m.end(), "delete")
    for m in SMART_QUOTES.finditer(scan_text):
        add("smart-quote", 1, m.start(), m.end(), "use straight quotes")
    for m in ARROWS.finditer(scan_text):
        add("unicode-arrow", 1, m.start(), m.end(), "use words")
    for m in ZERO_WIDTH.finditer(scan_text):
        add("hidden-char", 3, m.start(), m.end(), "remove zero-width character")
    for m in NBSP.finditer(scan_text):
        add("hidden-char", 1, m.start(), m.end(), "replace non-breaking space")

    headings = _scan_headings(scan_text, add, add_doc)
    stats = _scan_structure(text, scan_text, headings, em_count, add_doc, add)

    by_cat = {}
    for h in hits:
        c = by_cat.setdefault(h["category"], {"count": 0, "weight": 0})
        c["count"] += 1
        c["weight"] += h["weight"]
    ai_hits = [h for h in hits if h["category"] not in STYLE_CATEGORIES]
    clusters = _find_clusters(ai_hits)
    words = stats["words"]
    effective_words = max(words, SCORE_WORD_FLOOR)
    capped = sum(min(v["weight"], CATEGORY_WEIGHT_CAP) for k, v in by_cat.items() if k not in STYLE_CATEGORIES)
    score = round((capped + 3 * len(clusters)) / effective_words * 1000, 1) if words else 0.0
    stats["ai_signal_hits"] = len(ai_hits)
    stats["style_notes"] = len(hits) - len(ai_hits)
    marks = [{"line": line_of(text, m.start()), "text": m.group(0)} for m in UNVERIFIED_MARK.finditer(text)]
    return {"score": score, "words": words, "hits": hits, "by_category": by_cat, "clusters": clusters,
            "stats": stats, "unverified_markers": marks}


def _scan_headings(scan_text, add, add_doc):
    found = []
    for m in re.finditer(r"^(#{1,6})[ \t]+(.+?)[ \t]*$", scan_text, flags=re.M):
        level, title = len(m.group(1)), m.group(2).strip().rstrip("#").strip()
        found.append((level, title, m.start()))
        plain = re.sub(r"[`*_]", "", title)
        words = re.findall(r"[A-Za-z][A-Za-z'-]*", plain)
        low = plain.lower().rstrip(":?.").strip()
        if low in FILLER_HEADINGS:
            add("filler-heading", 2, m.start(), m.end(), "delete the section, or name its content")
        pair = re.match(r"^(\w+)(?:,? and | & )(\w+)$", low)
        if pair and pair.group(1) in PAIR_HEADING_WORDS and pair.group(2) in PAIR_HEADING_WORDS:
            add("filler-heading", 2, m.start(), m.end(), "generic paired heading; name what the section says")
        if len(words) >= TITLE_CASE_WORDS_MIN + 1:
            rest = words[1:]
            capped = [w for w in rest if w[0].isupper() and w.lower() not in SMALL_WORDS]
            if len(capped) >= 2 and len(capped) >= 0.6 * len([w for w in rest if w.lower() not in SMALL_WORDS]):
                add("title-case-heading", 1, m.start(), m.end(), "use sentence case")
        if title.endswith((":", "?")):
            add("heading-punctuation", 1, m.start(), m.end(), "remove trailing punctuation")
        if EMOJI.search(title):
            add("emoji", 1, m.start(), m.end(), "remove the emoji from the heading")

    h1s = sum(1 for lv, _, _ in found if lv == 1)
    if h1s > 1:
        add_doc("heading-level", 1, "%d level-1 headings" % h1s, "a document should have one H1", "keep one H1")
    for (a, _, _), (b, title, pos) in zip(found, found[1:]):
        if b > a + 1:
            add("heading-level", 1, pos, pos + len(title) + b, "heading level skips from H%d to H%d" % (a, b))

    thin = 0
    for i, (level, title, pos) in enumerate(found):
        end = found[i + 1][2] if i + 1 < len(found) else len(scan_text)
        section = scan_text[pos:end]
        body = section.split("\n", 1)[1] if "\n" in section else ""
        if level >= 3 and 0 < word_count(body) < 12:
            thin += 1
        first = next((ln.strip() for ln in body.split("\n") if ln.strip()), "")
        stem = re.sub(r"^(?:the|a|an|to|how to)\s+", "", re.sub(r"[`*_]", "", title).lower())
        lead = re.sub(r"^(?:the|a|an|to)\s+", "", first.lower())
        if len(stem.split()) >= 2 and lead.startswith(stem) and level >= 2:
            add("heading-restated", 1, pos, pos + len(title) + level, "the first sentence repeats the heading; add new information")
    if thin >= 3:
        add_doc("heading-inflation", 1, "%d thin sections" % thin, "headings over fewer than 12 words", "merge into the parent section unless this is a reference page")
    return found


def _scan_structure(text, scan_text, headings, em_count, add_doc, add):
    prose = re.sub(r"^\s*\|.*$", " ", scan_text, flags=re.M)
    prose = re.sub(r"^#{1,6}[ \t].*$", " ", prose, flags=re.M)
    sentences = split_sentences(prose)
    lengths = [word_count(s) for s in sentences if word_count(s) > 0]
    # Parallel list items are correct, so opener and fragment checks use paragraphs only.
    paragraph_text = re.sub(r"^[ \t]*(?:[-*+]|\d+[.)])[ \t].*$", " ", prose, flags=re.M)
    body_sentences = [s for para in re.split(r"\n\s*\n", paragraph_text) for s in split_sentences(para)]
    stats = {}
    words = word_count(prose)
    stats["words"] = words
    if lengths:
        stats["sentences"] = len(lengths)
        stats["mean_sentence_words"] = round(statistics.mean(lengths), 1)
        stats["stdev_sentence_words"] = round(statistics.pstdev(lengths), 1)

    long_found = 0
    for s in sentences:
        if word_count(s) > LONG_SENTENCE_WORDS and "\n" not in s.strip() and long_found < 5:
            pos = scan_text.find(s[:40])
            if pos >= 0:
                long_found += 1
                add("long-sentence", 1, pos, pos + 60, "split it; one idea per sentence")

    openers = [" ".join(re.findall(r"[A-Za-z']+", s.lower())[:2]) for s in body_sentences]
    run = 1
    for i in range(1, len(openers)):
        if openers[i] and openers[i] == openers[i - 1]:
            run += 1
            if run == 3:
                add_doc("repeated-opener", 1, openers[i], "three consecutive sentences start with '%s'" % openers[i], "merge or restructure")
        else:
            run = 1

    short_run = 0
    for s in body_sentences:
        short_run = short_run + 1 if word_count(s) <= 3 and s.rstrip().endswith((".", "!")) else 0
        if short_run == 3:
            add_doc("dramatic-fragments", 2, s[:30], "three very short sentences in a row", "write one real sentence")
            break

    conn = re.findall(r"(?:^|\n)\s*(?:%s)\b" % CONNECTORS, scan_text, flags=re.I)
    paragraphs = [p for p in re.split(r"\n\s*\n", prose) if word_count(p) > 8]
    if len(paragraphs) >= 4 and len(conn) / max(1, len(paragraphs)) > 0.4:
        add_doc("connector-density", 2, "%d/%d paragraphs" % (len(conn), len(paragraphs)), "many paragraphs open with a transition word", "delete the transitions")

    triples = re.findall(r"\b\w+(?:\s\w+)?,\s\w+(?:\s\w+)?,\s(?:and|or)\s\w+(?:\s\w+)?\b", prose)
    sent_total = max(1, len(sentences))
    if len(triples) >= 4 and len(triples) / sent_total > 0.12:
        add_doc("rule-of-three", 2, "%d triplets in %d sentences" % (len(triples), sent_total), "frequent three-item lists", "list the items that exist")

    if words and em_count / max(1, words) * 1000 > 5:
        add_doc("em-dash-density", 2, "%d em dashes" % em_count, "%.1f per 1,000 words" % (em_count / words * 1000), "replace with other punctuation")

    if len(lengths) >= 12 and stats.get("stdev_sentence_words", 99) < 3.5:
        add_doc("uniform-rhythm", 1, "stdev %.1f" % stats["stdev_sentence_words"], "sentence lengths are unusually uniform", "let content set the length")

    if words >= ERA_MIN_WORDS:
        era = len(re.findall(r"\b(?:%s)\b" % "|".join(re.escape(w) for w in ERA_WORDS), prose, flags=re.I))
        if era / words * 1000 > ERA_DENSITY_PER_1000:
            add_doc("era-vocabulary", 2, "%d words" % era, "dense use of words that recent models overuse", "replace with concrete facts")
        hedges = len(HEDGES.findall(prose))
        stats["hedges_per_1000"] = round(hedges / words * 1000, 1)
        if hedges / words * 1000 > 15:
            add_doc("hedge-density", 2, "%d hedges" % hedges, "%.1f per 1,000 words" % stats["hedges_per_1000"], "commit to the claim, or state the condition")
        rather = len(RATHER_THAN.findall(prose))
        if rather / words * 1000 > 5:
            add_doc("rather-than-density", 1, "%d uses" % rather, "'Y rather than X' framing, common in recent model output", "state Y only")

    concrete = (len(re.findall(r"(?<!`)`[^`\n]+`(?!`)", text)) + len(re.findall(r"\b\d[\d.,]*\b", prose))
                + len(re.findall(r"[\w./-]+\.(?:py|js|ts|go|rs|java|cs|yml|yaml|json|toml|md|sh|tf)\b", prose)))
    stats["concrete_items"] = concrete
    stats["concrete_per_100_words"] = round(concrete / max(1, words) * 100, 1)
    if words >= 150 and stats["concrete_per_100_words"] < 1.0:
        add_doc("low-specificity", 3, "%.1f concrete items per 100 words" % stats["concrete_per_100_words"], "few names, numbers, paths, or commands", "add evidence-backed specifics or cut")

    lines = [ln for ln in scan_text.split("\n") if ln.strip()]
    bullets = sum(1 for ln in lines if re.match(r"\s*(?:[-*+]|\d+[.)])\s", ln))
    if len(lines) >= 20:
        stats["bullet_line_ratio"] = round(bullets / len(lines), 2)
        if bullets / len(lines) > 0.6:
            add_doc("over-listing", 1, "%d%% list lines" % int(100 * bullets / len(lines)), "most lines are list items", "write reasoning as prose where items relate")

    counts = []
    h2 = [h for h in headings if h[0] == 2]
    for i, (_, _, pos) in enumerate(h2):
        end = h2[i + 1][2] if i + 1 < len(h2) else len(scan_text)
        counts.append(len(re.findall(r"^\s*(?:[-*+]|\d+[.)])\s", scan_text[pos:end], flags=re.M)))
    if len(counts) >= 4:
        top = max(set(counts), key=counts.count)
        if top >= 3 and counts.count(top) / len(counts) >= 0.7:
            add_doc("uniform-sections", 2, "%d of %d sections have %d list items" % (counts.count(top), len(counts), top), "sections share one template shape", "shape each section to its content")

    bold = len(re.findall(r"\*\*[^*\n]+\*\*", prose))
    if words and bold / max(1, words) * 1000 > 15:
        add_doc("bold-density", 1, "%d bold spans" % bold, "bold used heavily", "reserve bold for UI labels")

    toks = re.findall(r"[a-z']+", prose.lower())
    grams = {}
    for i in range(len(toks) - 3):
        g = " ".join(toks[i:i + 4])
        grams[g] = grams.get(g, 0) + 1
    repeats = [(g, c) for g, c in grams.items() if c >= 4 and len(set(g.split())) >= 3]
    for g, c in sorted(repeats, key=lambda x: -x[1])[:3]:
        add_doc("repeated-phrase", 1, g, "phrase appears %d times" % c, "merge, or keep the most specific instance")
    return stats


def _find_clusters(hits):
    """Find runs of three or more categories within about eight lines (roughly 150 words)."""
    positions = sorted((h["line"], h["category"]) for h in hits if h["line"])
    clusters = []
    i = 0
    while i < len(positions):
        j = i
        cats = set()
        while j < len(positions) and positions[j][0] - positions[i][0] <= 8:
            cats.add(positions[j][1])
            j += 1
        if len(cats) >= 3:
            clusters.append({"start_line": positions[i][0], "end_line": positions[j - 1][0], "categories": sorted(cats)})
            i = j
        else:
            i += 1
    return clusters


def band_of(score):
    return "clean" if score < BAND_CLEAN else ("needs a pass" if score < BAND_PASS else "rewrite from evidence")


def render(name, res, min_weight, quiet, limit):
    out = ["== %s ==" % name]
    s0 = res["stats"]
    out.append("words: %d   score: %s per 1,000 words   AI signals: %d   style notes: %d   clusters: %d" % (
        res["words"], res["score"], s0.get("ai_signal_hits", 0), s0.get("style_notes", 0), len(res["clusters"])))
    out.append("band: %s" % band_of(res["score"]))
    s = res["stats"]
    out.append("sentence length mean/stdev: %s/%s   concrete items per 100 words: %s" % (s.get("mean_sentence_words", "-"), s.get("stdev_sentence_words", "-"), s.get("concrete_per_100_words", "-")))
    if res["by_category"]:
        out += ["", "By category:"]
        for cat, v in sorted(res["by_category"].items(), key=lambda x: -x[1]["weight"]):
            out.append("  %-24s count %-3d weight %d" % (cat, v["count"], v["weight"]))
    if res["clusters"]:
        out += ["", "Clusters (3+ categories close together; fix first):"]
        for c in res["clusters"]:
            out.append("  lines %d-%d: %s" % (c["start_line"], c["end_line"], ", ".join(c["categories"])))
    if res["unverified_markers"]:
        out += ["", "Unverified markers to resolve before publishing:"]
        for m in res["unverified_markers"]:
            out.append("  L%d %s" % (m["line"], m["text"]))
    if not quiet and res["hits"]:
        shown = [h for h in res["hits"] if h["weight"] >= min_weight]
        hidden = 0
        if limit and len(shown) > limit:
            hidden = len(shown) - limit
            shown = sorted(shown, key=lambda h: (-h["weight"], h["line"]))[:limit]
        shown.sort(key=lambda h: (h["line"], -h["weight"]))
        out += ["", "Findings:"]
        for h in shown:
            loc = "L%d" % h["line"] if h["line"] else "doc"
            out.append("  [%s] w%d %-22s '%s'" % (loc, h["weight"], h["category"], h["match"]))
            out.append("        fix: %s" % h["fix"])
        if hidden:
            out.append("  ... %d lower-weight findings hidden; rerun with --limit 0" % hidden)
    return "\n".join(out)


def render_github(name, res, min_weight):
    lines = []
    for h in res["hits"]:
        if h["weight"] < min_weight:
            continue
        level = "warning" if h["weight"] >= 2 else "notice"
        msg = "%s: '%s'. %s" % (h["category"], h["match"], h["fix"])
        msg = msg.replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")
        where = "file=%s" % name + (",line=%d" % h["line"] if h["line"] else "")
        lines.append("::%s %s,title=slop-scan::%s" % (level, where, msg))
    return "\n".join(lines)


def expand(paths):
    """Expand directories to prose files. Returns (files, missing, any_directory_given)."""
    files, missing, any_dir = [], [], False
    for p in paths:
        if p == "-" or os.path.isfile(p):
            files.append(p)
        elif os.path.isdir(p):
            any_dir = True
            listed, _, _ = doclib.list_repo_files(p)
            files.extend(os.path.join(p, f) for f in listed if f.lower().endswith((".md", ".mdx", ".markdown", ".rst", ".adoc")))
        else:
            missing.append(p)
    return files, missing, any_dir


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("files", nargs="+", help="files or directories to scan, or - for stdin")
    ap.add_argument("--json", action="store_true", help="emit JSON (same as --format json)")
    ap.add_argument("--format", choices=["text", "json", "github"], default="text", help="github prints CI annotations")
    ap.add_argument("--max-score", type=float, default=None, help="exit 1 if any score exceeds this")
    ap.add_argument("--min-weight", type=int, default=1, help="hide findings below this weight, 1 to 3 (default %(default)s)")
    ap.add_argument("--allow", default="", help="comma-separated words that are real terms in this project")
    ap.add_argument("--limit", type=int, default=DEFAULT_FINDING_LIMIT, help="max findings per file; 0 for all (default %(default)s)")
    ap.add_argument("--no-code-mask", action="store_true", help="scan code blocks too")
    ap.add_argument("--quiet", action="store_true", help="summary only")
    args = ap.parse_args(argv)
    fmt = "json" if args.json else args.format
    allow = [w.strip() for w in args.allow.split(",") if w.strip()]

    files, missing, any_dir = expand(args.files)
    if missing or not files:
        print("error: nothing to scan; not found: %s" % (", ".join(missing) or ", ".join(args.files)), file=sys.stderr)
        return 2
    results = {}
    for f in files:
        raw = sys.stdin.read() if f == "-" else doclib.read_text(f)
        results["<stdin>" if f == "-" else f] = scan(raw, mask=not args.no_code_mask, allow=allow)
    worst = max(r["score"] for r in results.values())

    if fmt == "json":
        print(json.dumps(results, indent=2, ensure_ascii=True))
    elif fmt == "github":
        for name, res in results.items():
            text = render_github(name, res, args.min_weight)
            if text:
                print(text)
    else:
        ranked = sorted(results.items(), key=lambda kv: -kv[1]["score"])
        if any_dir or len(results) > 5:
            print("Score  Band                   Words  File")
            for name, res in ranked:
                print("%-6s %-22s %-6d %s" % (res["score"], band_of(res["score"]), res["words"], name))
            print()
            if args.quiet:
                ranked = []
            else:
                ranked = ranked[:DETAIL_FILES_IN_DIR_SCAN]
                print("Details for the %d highest scores; pass a file directly for others.\n" % len(ranked))
        for name, res in ranked:
            print(render(name, res, args.min_weight, args.quiet, args.limit))
            print()
    if args.max_score is not None and worst > args.max_score:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
