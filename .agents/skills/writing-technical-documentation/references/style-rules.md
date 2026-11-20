# Style rules

Condensed from the Google developer documentation style guide and the Microsoft writing style guide, with additions for this skill. If the project has its own style guide or glossary, it wins.

Contents
1. Voice and tone
2. Sentences
3. Words
4. Headings
5. Lists
6. Procedures
7. Tables
8. Code and commands
9. Links
10. Numbers, dates, units
11. Inclusive language
12. Accessibility
13. Global readers
14. Timeless text
15. Placeholders and example data
16. Error messages and UI text
17. Punctuation and characters
18. Formatting limits
19. Final checklist

## 1. Voice and tone

- Second person ("you") for instructions. "We" only in tutorials or where the authors are the actors in a decision ("We chose Postgres because...").
- Active voice. Name the actor. "The worker retries three times", not "Three retries are performed".
- Present tense for behavior. "The API returns 404", not "will return".
- Plain, direct, neutral. Friendly means clear, not chatty. No jokes in reference text, no exclamation marks, no rhetorical questions.
- Match the project's voice when it has a clear one, and when cleaning a person's draft keep their voice. These rules are defaults.
- State requirements with must, should, may in the RFC 2119 sense when precision matters, and say so once.
- Commit to a claim or mark it unverified. Do not hedge with may, might, can potentially, generally, typically unless you can state the condition.

## 2. Sentences

- One idea per sentence. Aim for under about 25 words on average, with natural variation. Do not cut every sentence to the same length.
- Put the important thing first. Put the condition before the instruction: "If the queue is empty, skip step 4."
- Prefer a short sentence to a bulleted fragment pile when reasoning is involved.
- Avoid stacked noun phrases ("user authentication token validation failure handler").
- Avoid ambiguous pronouns. If "it" has two possible referents, repeat the noun.
- Avoid "there is" and "there are" openers when a real subject exists.
- Do not start with a time wasting clause ("In order to", "It should be noted that"). "To deploy, run..."

## 3. Words

<!-- slop-scan: off -->
- Use the term the code uses. One name per concept, everywhere. Define it on first use if the reader may not know it.
- Prefer short, common words: use, not utilize; start, not commence; show, not demonstrate; help, not facilitate; about, not approximately where precision is not needed.
- No Latin abbreviations (e.g., i.e., etc., via, vs.) in prose; write "for example", "that is", "and so on" (better, list all), "through", "versus".
- Spell out an acronym on first use unless the audience uses it daily (HTTP, CPU, SQL). Do not form plurals with apostrophes.
- Avoid phrasal verbs that confuse translators: "set up" is a verb, "setup" is a noun, "log in" verb, "login" noun.
- Avoid idioms and metaphors ("under the hood", "out of the box", "low-hanging fruit", "boil the ocean", "silver bullet", "deep dive").
- Do not use "please" in instructions. Do not use "simply", "just", "easy", "obviously", "of course", "clearly", "trivial". They shame the reader who struggles.
- Do not use "should" for steps. "Run" or "Click", not "You should run".
- Use "can" for ability and "may" for permission or possibility, sparingly.
- Do not use "and/or"; choose, or write "A, B, or both".

<!-- slop-scan: on -->

## 4. Headings

- Sentence case ("Configure the retry policy"), not Title Case.
- Name the content or the task. Task headings start with a verb (imperative) in how-tos. Concept headings are noun phrases.
- No headings that are only filler: Overview, Introduction, Conclusion, Summary, Final thoughts, Key considerations, Best practices (unless they truly contain that).
- Do not skip levels. One H1 per file. Avoid more than three heading levels.
- No terminal punctuation. No links, code, or emphasis in headings unless it is a code identifier.
- Every section earns its heading by having at least two paragraphs or a distinct task. A heading over a single sentence is a sign of over-structure.

## 5. Lists

- Numbered lists for sequences where order matters. Bulleted lists for sets with no order. Do not number unordered items.
- Parallel structure: every item starts the same way grammatically.
- Introduce a list with a sentence that ends with a colon only when the list completes that sentence.
- Capitalize the first word of each item. End with a period only if items are complete sentences.
- Keep to about seven items or fewer. Longer, regroup or use a table.
- Do not nest more than two levels.
- Do not use a list to hide missing reasoning. If items relate causally, write sentences.
- No bold-lead bullets ("**Speed:** it is fast") as a default pattern. Use a definition table or plain sentences.

## 6. Procedures

- Seven steps or fewer per procedure. Split longer ones into sub-procedures with a verification point between them.
- One action per step. Start with an imperative verb. Complete sentences.
- State the outcome the reader should see when a step can fail silently.
- Tell the reader where they are ("In the project root, run:").
- List prerequisites before step 1: versions, access, tools, prior setup.
- Provide a verification step at the end and a path for failure.
- Optional steps are labeled "Optional:" at the start of the step. A step with alternatives lists them as separate subsections, not as inline choices.
- Order a complex step this way: describe the action, give the command, explain placeholders, explain the command if needed, show output, explain the result.
- Say what to do, and do not write "run the following command". Do not use "above" or "below"; link to the place.
- Give the best method only. Link to a repeated procedure instead of copying it.
- Use a one-sentence form, not a numbered list, for a single-step procedure.

## 7. Tables

- Use for data with two dimensions or repeated attributes (parameters, flags, config keys, error codes).
- Header row always. Columns: the thing, then attributes. Keep cell text short; move long text below the table.
- Do not use tables for layout or for lists with one column.
- Keep columns consistent across similar tables in the same doc set.
- Provide a text alternative for complex tables.

## 8. Code and commands

- Fenced code blocks with a language tag. Inline code for identifiers, filenames, paths, flags, env vars, values, and short commands.
- Code samples must run as written, or be marked as excerpts. Say what version they were tested on when that matters.
- Show the command and then its expected output in a separate block, labeled.
- Do not include shell prompts (`$`) in blocks meant for copying. Use them only when mixing commands and output in one block.
- Use long flags (`--verbose`) in docs, short flags only when universal.
- Break long commands with backslashes at flag boundaries in shell, and state the shell.
- Avoid screenshots of text or terminals. If you use an image, add alt text and keep a text version.
- Use real project names and paths. Do not invent `foo`, `bar`, `myapp` when the real name is available. If a placeholder is needed, see section 15.
- Keep code samples minimal: only what illustrates the point, but complete enough to run.
- Never include secrets, real tokens, real emails, or customer data.

## 9. Links

- Descriptive link text. Never "here", "this link", "read more".
- Use relative links inside a repo. Check that targets exist and anchors resolve.
- Link the first mention of a concept that has its own page, not every mention.
- Prefer stable official docs over blog posts. Do not link to a URL you have not seen.
- Name the external system and version next to a link whose content can change.

## 10. Numbers, dates, units

- Use numerals for measurements, versions, ports, counts in technical contexts. Spell out one through nine in prose otherwise, per project convention.
- Put a space between number and unit (5 MB, 200 ms), except for percent (50%) and degrees if the project does.
- Use binary versus decimal units correctly: MiB vs MB, and say which.
- Dates in ISO 8601 (2026-11-05). Times in 24-hour format with timezone, UTC by default for logs and incidents.
- Give ranges with "to" or an en dash only if the project style allows; ASCII hyphen with spaces is fine in code docs.
- Always give units for timeouts, sizes, and rates. A bare "30" is a defect.
- Show defaults and limits exactly as the code has them. Do not round.
- Do not add precision you do not have. Say "about 200 ms at p50 in the staging test on 2026-10-14" or leave it out.

## 11. Inclusive language

Replace:
- whitelist, blacklist: allowlist, denylist
- master/slave: primary/replica, leader/follower, controller/worker
- sanity check: confidence check, quick check
- dummy value: placeholder, sample value
- grandfathered: legacy status, exempted
- man-hours: person-hours, work hours
- guys: everyone, team, folks
- crazy, insane, lame, blind spot, crippled: unexpected, severe, weak, gap, limited
- he/she for a generic user: they, or rewrite in second person
- kill (a process) is acceptable as a technical term (`kill` command); prefer "stop" or "terminate" in prose
- abort is acceptable as a technical term
Do not rename identifiers that exist in code or third-party APIs. Quote them and note the neutral term once if useful.

## 12. Accessibility

- Alt text for every image that carries meaning; empty alt for decoration. Describe the information, not "diagram".
- Do not rely on color alone. Do not describe locations by appearance only ("the red button on the right").
- Heading order that matches content structure. Lists as real lists. Tables with headers.
- Link text that makes sense out of context.
- Keep diagrams backed by text.

## 13. Global readers

- Short, literal sentences. Avoid idioms, culture-specific examples, and humor.
- Keep consistent terms so translation memory works and so non-native readers do not hunt for synonyms.
- Use "that" and other optional words where they remove ambiguity.
- Avoid contractions in reference docs if the project avoids them; contractions are fine in tutorials and READMEs if consistent.
- Avoid abbreviations that are region-specific. Dates in ISO format. Currency with codes.

## 14. Timeless text

Avoid words that tie text to a moment: currently, now, new, newly, recently, soon, upcoming, latest, modern, today, as of this writing, in the future, in the near term, at the moment, still, no longer (unless in migration or changelog), will be (for planned work). Instead, state the version or date ("Since v2.3, ...", "Planned for v3, see issue #123"), or say what is true. Do not announce future features in documentation, and do not document unshipped features as if they exist. Keep roadmap items out of reference pages.

## 15. Placeholders and example data

- Placeholders are uppercase with underscores, in angle brackets if the project does that, and consistent within the doc: `<PROJECT_ID>`, `API_KEY`. Avoid `MY_` and `YOUR_` prefixes, hyphens, and lowercase. Explain each once, right after the code block: "Replace `PROJECT_ID` with your Google Cloud project ID."
- Example domains: use `example.com`, `example.org` (RFC 2606). Example IPv4: 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24 (RFC 5737). IPv6: 2001:db8::/32 (RFC 3849). Example emails: `user@example.com`. Phone numbers: use reserved fictional ranges for the relevant country.
- AWS example account ID: 123456789012. Do not use real account IDs, ARNs, bucket names, or hostnames from the repo unless the user asks for them.
- Example names: neutral, short, realistic for the domain (`orders`, `invoice-service`), not cute.
- Example secrets: obviously fake and not matching real key formats too closely (`sk-REDACTED`).

## 16. Error messages and UI text

- Quote error messages exactly, in code format, copied from source or a run.
- Explain cause and fix next to each message.
- When writing messages in product code: say what happened, why if known, and what to do. No blame, no "Oops", no exclamation marks.
- UI labels in bold exactly as shown (**Save**). Menu paths with a greater-than sign: **Settings** > **Access**. Do not bold anything else for emphasis.

## 17. Punctuation and characters

- ASCII in source Markdown for punctuation: straight quotes, hyphen-minus, three periods only when quoting. No em dashes by default (use a period, comma, colon, or parentheses). Use en dash only for numeric ranges if the project does, otherwise "to".
- No emojis, no decorative symbols, no unicode arrows (use "to", "then", or ->, only in code or diagrams).
- Oxford (serial) comma.
- One space after a period.
- Use parentheses sparingly. A parenthetical that matters should be a sentence.
- Colons introduce lists or examples; do not use them for dramatic reveals.
- Semicolons are rare. Prefer two sentences.
- Quote marks only for literal strings and defined terms on first use.
- Never use ellipsis for drama.

## 18. Formatting limits

- Bold: UI labels and the first definition of a term at most. Not for emphasis, not in the middle of sentences, not as lead-ins for every bullet.
- Italics: rare, for titles or the first use of a term if the project does that.
- Blockquote callouts (Note, Warning): use rarely. A document where everything is a callout has no callouts. Warning is for data loss, security risk, or cost. Note is for a fact the reader will otherwise miss. Put the callout before the step it concerns.
- Horizontal rules: not between every section.
- Table of contents: only when the page is long enough to need navigation (more than about 4 screens).
- Length: as long as the content requires. Cut to the delete test, not to a word count.

## 19. Final checklist

- Does the first screen say what this is, for whom, and what to do?
- Is every command, path, flag, name, number, and link verified?
- Is each term used consistently?
- Is there one document type, with the right structure?
- Does every sentence carry a fact, decision, instruction, or warning?
- Are there leftover AI patterns (run the scanner)?
- Is anything stated that the evidence does not support?
