# AI writing pattern catalog

Patterns that mark text as machine-generated or machine-flavored, with the fix for each. Use it during the prose pass and when cleaning someone's draft. Run `scripts/slop_scan.py` first; it covers the lexical and formatting patterns and some structure. The semantic ones need your judgment. It draws on Wikipedia's "Signs of AI writing" (CC BY-SA 4.0, https://en.wikipedia.org/wiki/Wikipedia:Signs_of_AI_writing), which editors keep current, and on the other sources in `docs/sources-and-licenses.md`.

Contents
0. How to use this catalog
1. Root causes (fix these, not symptoms)
2. Vocabulary: words and their replacements
3. Phrases and openers
4. Sentence-level structures
5. Paragraph and document structures
6. Tone and stance
7. Formatting and encoding
8. Technical-documentation-specific patterns
9. Claude-specific and chat-assistant tells
10. Fabrication patterns
11. Leaked-assistant artifacts
12. Not reliable indicators (do not flag these alone)
13. Model generations and drift
14. Scoring and clusters
15. Rewrite procedure and when not to act

<!-- slop-scan: off -->
## 0. How to use this catalog

- Strength varies. Act on one sighting of leaked assistant text, tool markup, or negative parallelism. Treat vocabulary, transition words, and em dashes as weak alone.
- One instance is a yellow flag. Three or more in different categories in a short span is a cluster, and clusters are the signal.
- Fix the cause: a vague sentence needs a fact. Synonym swaps keep the emptiness. If you cannot find the fact, delete the sentence or ask.
- Do not over-police. Formal register, correct grammar, a transition word, a list, or a heading is not a defect by itself. Humans use all of these. Detection by pattern is probabilistic; the goal is good prose, not evasion.
- Vocabulary lists rotate with model generations. Structural patterns persist. Weight structure over words.

## 1. Root causes

| Root cause | Symptom | Fix |
|---|---|---|
| No specific facts | Abstract nouns, adjectives, "various", "numerous" | Add names, numbers, paths, commands |
| No stance | Hedging, both-sides, "it depends" without conditions | Commit, or state the condition that decides |
| No reader in mind | Generic intros, "whether you're a beginner or expert" | Name the reader and their next action |
| Padding to look thorough | Restatement, summaries, preambles | Delete test |
| Optimizing for sounding authoritative | Significance inflation, grand claims | State the fact and let it carry weight |
| Template-filling | Same shape in every section, rule of three | Shape the section to its content |
| Avoiding risk of being wrong | Vague attribution, "can help", "may" | Verify, then state; or mark unverified |
| Imitating style of marketing or essays | Hype, rhetorical questions, dramatic reveals | Plain declarative sentences |

## 2. Vocabulary

Flag these when they appear in unusual density or in the roles shown. Replace with the concrete thing.

### 2.1 Verbs
delve, delves, delving; leverage(s/d); harness; utilize; showcase; underscore; highlight (as a verb for emphasis); foster; facilitate; empower; unlock; unleash; elevate; embark; navigate (abstract); streamline; revolutionize; transform (metaphoric); spearhead; bolster; garner; cultivate; orchestrate (non-technical); resonate; illuminate; unravel; unpack (abstract); dive into; deep dive; shed light on; pave the way; stand as; serve as (for "is"); boast (for "has"); feature (for "has"); encompass; embody; exemplify; amplify; reimagine; supercharge; turbocharge; spearhead; champion (verb); catalyze.
Fix: use "use", "show", "help", "make", "is", "has", or name the action.

### 2.2 Nouns
tapestry; landscape (abstract); realm; testament; beacon; cornerstone; pillar; bedrock; linchpin; paradigm; synergy; ecosystem (non-technical); journey; odyssey; symphony; mosaic; kaleidoscope; interplay; intersection; nexus; hallmark; embodiment; game-changer; powerhouse; treasure trove; myriad; plethora; multitude; spectrum; roadmap (non-literal); toolkit (non-literal); arsenal; stakeholders (non-specific); best practices; key takeaways; deep dive; North Star; value proposition; touchpoint; learnings; ask (noun).
Fix: name the actual thing.

### 2.3 Adjectives and adverbs
pivotal; crucial; vital; essential (as filler); paramount; robust; seamless; comprehensive; multifaceted; nuanced; vibrant; meticulous; intricate; dynamic; innovative; cutting-edge; state-of-the-art; groundbreaking; transformative; revolutionary; unparalleled; unprecedented; holistic; scalable (unproven); powerful; elegant; sophisticated; versatile; invaluable; indispensable; remarkable; notable; profound; compelling; captivating; bustling; rich (as in rich history); enduring; enigmatic; stunning; breathtaking; lasting; renowned; commendable; instrumental; strategic; critical (as filler); evolving; ever-evolving; fast-paced; best-in-class; world-class; enterprise-grade; production-ready (undefined); battle-tested; mission-critical; future-proof; turnkey; plug-and-play; effortlessly; seamlessly; truly; genuinely; fundamentally; incredibly; remarkably; undeniably; arguably; notably; importantly; crucially; ultimately; essentially; basically; effectively; particularly; significantly (without a number).
Fix: remove, or replace with a measurement, a mechanism, or a comparison.

### 2.4 Era vocabulary

Each model generation overuses different ordinary words, and any one of them is normal. Density is what counts. Wikipedia's lists are rough: GPT-4 era: additionally, boasts, bolstered, crucial, delve, emphasizing, enduring, garner, intricate, interplay, key, landscape, meticulously, pivotal, underscore, tapestry, testament, valuable, vibrant. GPT-4o era: align with, enhance, fostering, highlighting, showcasing. Mid-2026 onward: challenge, clearer, dependable, echoed, foster, leverage, matters, multifaceted, practical, prioritize, quietly, steady, universally, along with more periods and shorter sentences of about 8 to 20 words.

### 2.5 Connectors and transitions overused
Moreover; Furthermore; Additionally; In addition; Consequently; Thus; Hence; Nevertheless; Nonetheless; That said; With that said; Having said that; On the other hand; In essence; In summary; In conclusion; Overall; Ultimately; Indeed; Notably; Importantly; Interestingly; Crucially; Specifically (as opener); Accordingly; Subsequently.
These words alone do not indicate AI text, and style guides accept them. Flag when two or more start consecutive sentences or paragraphs, or when each paragraph opens with one. Fix: delete; or let the order of sentences carry the logic; use "and", "but", "so", "because".

### 2.6 Technical-doc stock words
seamless integration; robust architecture; scalable solution; powerful API; intuitive interface; lightweight; blazing fast; developer-friendly; out of the box; under the hood; production-grade; enterprise-ready; modular and extensible; flexible and configurable; highly performant; cutting-edge technology; best practices; industry standard; end-to-end solution; holistic approach; single pane of glass; one-stop shop; full-stack; next-generation.
Fix: state what it does, in numbers or mechanisms.

## 3. Phrases and openers

Delete or replace each.

- "In today's fast-paced / digital / ever-evolving world / landscape"
- "In the realm of", "In the world of", "In the age of AI"
- "Now more than ever"
- "It's worth noting that", "It's important to note that", "It should be noted", "It is worth mentioning", "Notably,", "Importantly,", "Needless to say", "It goes without saying"
- "Let's dive in", "Let's delve into", "Let's explore", "Let's break it down", "Let's unpack this", "Let's get started" (as filler), "Without further ado"
- "Here's the thing", "Here's why", "Here's what you need to know", "Here's the kicker", "Here's where it gets interesting", "It gets better", "The short answer is", "What's more"
- "Imagine a world where"
- "At its core", "At the heart of", "At the end of the day", "When it comes to", "In terms of", "With regard to", "In order to"
- "plays a crucial / pivotal / vital / key role in", "serves as a testament to", "stands as a reminder", "marks a significant milestone / shift", "is a game-changer", "paves the way for", "sets the stage for", "opens the door to", "takes it to the next level", "a rich tapestry of", "a wide range of", "a variety of", "a plethora of", "a myriad of", "a number of" (without a number)
- "Whether you're a beginner or an expert", "Whether you're X or Y"
- "Look no further", "Say goodbye to", "Say hello to"
- "This is where X comes in", "Enter X"
- "The best part?", "The result?", "The catch?", "The truth is", "The reality is", "Here's the reality"
- "I hope this helps", "Feel free to reach out", "Don't hesitate to", "Happy coding!", "Let me know if you have any questions"
- "Great question!", "Certainly!", "Absolutely!", "Sure!", "Of course!"
- "As an AI language model", "As of my knowledge cutoff"
- "Overall,", "In summary,", "In conclusion,", "To sum up,", "To recap,", "All in all,", "In short,", "The takeaway:", "Key takeaways", "Final thoughts"
- "From X to Y" used as vague scope ("from startups to enterprises", "from beginners to experts")
- "Not only... but also..." as flourish
- "Think of it as", "Think of X like Y" repeated
- "It's like a ... for your ..." metaphor stacking
- "This ensures", "This allows you to", "This enables", "This helps to ensure", "This provides a robust way to"
- "ensuring", "highlighting", "underscoring", "showcasing", "reflecting", "contributing to", "fostering", "emphasizing", "demonstrating" as trailing participial phrases
- "can help you", "can be used to", "is designed to", "aims to", "strives to", "seeks to" as vague hedged verbs (use the real verb when the thing does it)
- "a testament to", "a reminder that", "a nod to"
- "Remember, ..." / "Keep in mind that ..." as filler
- "Contrary to popular belief", "You might think", "You may be wondering", "Some might say" (arguing with no one)
- "Trusted by thousands", "loved by developers", "used by teams at" with no number or source
- "By leveraging X, you can Y"
- "Take your X to the next level"
- "unlock the full potential of"
- "navigate the complexities of"
- "in the ever-changing landscape of"
- "this comprehensive guide", "ultimate guide", "complete guide" (self-labeling)

## 4. Sentence-level structures

### 4.1 Negative parallelism
"It's not X, it's Y." "This isn't just X; it's Y." "Not only X but also Y." "Not X. Not Y. Just Z." "No X, no Y, just Z." "More than just X." "Less about X, more about Y." Recent models favor a quieter form: "Y rather than X" and "X, not Y" as a repeated habit.
Why it is slop: it sets up a straw position to sound insightful. Fix: state Y only. A real contrast the reader needs, such as "use `--force-with-lease`, not `--force`", stays.

### 4.2 Self-posed questions
"The result? Faster builds." "Why does this matter? Because..." "What does this mean for you?" Fix: make the statement.

### 4.3 Rhetorical questions as openers
"Ever wondered how X works?" "Struggling with Y?" "Want to level up your Z?" Fix: delete.

### 4.4 Rule of three
Triplets of adjectives, nouns, or clauses ("fast, reliable, and scalable"; "plan, build, and deploy"), especially repeated across a document. Fix: list what is true; items may be two, four, or one.

### 4.5 Trailing participial padding
"..., ensuring optimal performance." "..., highlighting its importance." "..., making it ideal for X." "..., allowing teams to Y." "..., which underscores Z." Fix: delete or make a separate sentence with a fact.

### 4.6 Elegant variation (synonym cycling)
Same entity called "the service", "the platform", "the solution", "the system", "the tool" in successive sentences. Fix: one term.

### 4.7 False ranges
"From logging to monitoring to alerting" when items are not on a real scale. Fix: list items or choose the right word.

### 4.8 Copula avoidance
"serves as", "stands as", "acts as", "functions as", "represents", "boasts", "features", "offers" for plain is/has. Fix: "is", "has".

### 4.9 Hedging stacks
"It may potentially be possible that...", "can often help to some extent." Fix: pick the claim or state the condition.

### 4.10 Em-dash overuse
Em dashes used for asides, drama, or in place of commas, colons, or periods, especially several per paragraph. Fix: use a period, comma, colon, or parentheses. (Em dashes are not proof of AI; density is the signal.)

### 4.11 Colon reveals
"The solution: X." "Here's the thing: X." Fix: plain sentence.

### 4.12 Dramatic fragments
Short punchy fragments for effect ("Simple. Fast. Secure." "Period." "Full stop." "And that's the point."). Fix: one real sentence.

### 4.13 Anaphora and repeated frames
Consecutive sentences starting with the same word. Groups of sentences with identical shape ("X is Y. X is Z. X is W."). Fix: vary by content, not by trick; merge.

### 4.14 Tricolon of sentences
Three short sentences in a row of the same length used for rhythm. Fix: combine or cut.

### 4.15 Gerund-heavy openings
"Leveraging X, this approach enables Y." "Designed with scalability in mind, ..." Fix: subject-verb-object.

### 4.16 Passive and agentless
"Errors are handled", "the configuration is loaded", "steps are taken to ensure". Fix: name the actor.

### 4.17 Nominalizations
"perform an analysis of" (analyze), "make a determination" (decide), "provide support for" (support), "implementation of" (implement). Fix: verb.

### 4.18 Overuse of "this" without noun
"This allows...", "This ensures..." with a vague referent. Fix: name the thing.

### 4.19 "Whether" constructions
"Whether you are deploying to AWS, GCP, or on-prem, X works." Fix: state the supported platforms.

### 4.20 Empty intensifiers and softeners
very, really, extremely, highly, quite, fairly, rather, somewhat, a bit, kind of, sort of, simply, just, merely, actually, literally, truly. Fix: delete.

### 4.21 Quotative framing
"As the saying goes", "As they say". Fix: delete.

### 4.22 Scare quotes and hype quotes
"magic", "smart", "seamless" in quotes. Fix: delete or define.

### 4.23 Pseudo-profound abstractions
"Clarity is the foundation of trust." "Documentation is the bridge between intent and understanding." Fix: delete.

### 4.24 "It" doing nothing
"It is important that..." "It is recommended that..." "It is possible to..." Fix: direct statement.

### 4.25 Over-qualified lists
"such as X, Y, Z, and more", "including but not limited to". Fix: list all, or say how many.

### 4.26 Mismatched specificity
A paragraph with one real fact buried among vague sentences. Fix: lead with the fact; cut the rest.

### 4.27 False balance
"On one hand... on the other hand..." where the code decides. Fix: state which one the system does.

### 4.28 Time parasites
"Over the years", "In recent years", "Nowadays", "Traditionally", "Historically" with no dates. Fix: date or delete.

### 4.29 One-line closers and staged run-ups
A paragraph ending in a short punchy line ("And that's the point.", "That's the whole trick."), or opening with a setup that delays the claim ("Here's what happened."). Fix: state the claim plainly and stop.

### 4.30 Hyphenated pairs
Stacked compound modifiers for rhythm ("fast-moving, high-stakes, data-driven teams"). Fix: name the teams, or cut.

### 4.31 Heading restated
The first sentence under a heading repeats the heading ("## Configuration" then "Configuration is handled by..."). Fix: start with new information.

### 4.32 Writing about the document
"This section explains", "The following table shows", "as this guide demonstrates". Fix: state the content.

### 4.33 False agency
A thing does what only a person or a named piece of code can do: "the data tells us", "the config decides", "the system wants". Fix: name the code or person that acts. "`retry.py` sleeps 2 seconds, then retries."

### 4.34 Narrator from a distance
The text describes the system as if watching it from far away: "one can see that requests flow through the gateway". Fix: say what happens, to what, in the order it happens, with a path.

### 4.35 Appeal to familiarity
"As most developers know", "you have probably seen". It flatters or hedges and carries no fact. Fix: state the fact, or link the concept.

### 4.36 Announced counts
"Three key components", "five main benefits" said before the list, often forced to fit. Fix: use the number of items that exist, and do not announce it.

### 4.37 Analogy in place of mechanism
"Think of the queue as a post office." The picture replaces what the queue does. Fix: name the mechanism and its limits. Keep an analogy only if the reader needs it and it is true in the details that matter.

### 4.38 Candid asides
"Honestly, this part is tricky", "to be fair, it is not perfect". It sounds open and says nothing. Fix: state the actual limit, its trigger, and its effect.

## 5. Paragraph and document structures

### 5.1 Preamble
Intro paragraph announcing the structure ("This document will cover X, Y, and Z"). Fix: delete; start with content.

### 5.2 Recap
Closing paragraph repeating the document. "In summary." "Key takeaways." "Conclusion." Fix: delete.

### 5.3 Fractal summaries
Summary at the start of each section, subsection, and whole doc. Fix: one summary at the top if any.

### 5.4 Uniform section shape
Every section: heading, one-line intro, three bullets, one-line outro. Fix: shape to content.

### 5.5 Heading inflation
Many headings over very little text. H3s with one sentence. Fix: merge.

### 5.6 Filler sections
Overview, Introduction, Conclusion, Best Practices, Key Considerations, Benefits, Use Cases, Future Work, Challenges and Opportunities, Final Thoughts. They are present because templates have them, with generic content. Fix: delete sections with no project-specific content.

### 5.7 Benefits and challenges pairs
Generic list of benefits followed by generic list of challenges. Fix: write real trade-offs with numbers.

### 5.8 "Future directions" speculation
Generic roadmap with no commitments, owners, or issues. Fix: link to the issue tracker or delete.

### 5.9 Everything-is-a-list
Prose broken into bullets with no parallel structure. Fix: write the reasoning.

### 5.10 Bullet lists of bold-lead items
"**Performance:** ...", "**Security:** ...", "**Scalability:** ...". Fix: use a table if attributes, prose if reasoning.

### 5.11 Nested bullets for no reason
Two-level lists where the second level is a restatement. Fix: flatten.

### 5.12 Symmetry
Every list has the same number of items. Every paragraph has the same length. Fix: let content decide.

### 5.13 Over-signposting
"First,... Second,... Third,... Finally,..." across paragraphs in explanatory prose; "Let's move on to". Fix: use the order itself; use numbered lists only for real sequences.

### 5.14 Callout overuse
Notes and warnings on every section; "Pro tip" and "Remember" blocks. Fix: keep one for real hazards.

### 5.15 Table of contents on short pages
Fix: delete.

### 5.16 Introductory definitions of obvious terms
Defining HTTP or JSON for an engineering audience. Fix: delete or link.

### 5.17 Narrated structure
"As mentioned above", "as we'll see below", "as discussed earlier". Fix: delete or link.

### 5.18 Generic examples
`foo`, `bar`, `MyClass`, `example-app` where a real name exists. Fix: use real names.

### 5.19 Repeated idea in different words
Two or three consecutive sentences saying the same thing. Fix: keep the most specific.

### 5.20 Lack of anything concrete
A whole section with no names, numbers, paths, or commands. Fix: investigate and add, or delete.

### 5.21 Tie-back ending
The last line returns to the opening image or phrase to feel finished. Fix: end on the last fact.

### 5.22 Self-echo
A later paragraph repeats an earlier line almost word for word, as if new. Fix: delete the second.

### 5.23 Premise stacking
Several loose claims are lined up, then a conclusion follows from none of them. Fix: one claim, its evidence, then the consequence.

### 5.24 One point stretched
A single idea fills a section, restated at different levels of formality. Fix: say it once. If there is no second point, merge the section into its neighbor.

### 5.25 Question-word headings
Sections titled "What is X", "Why X", "How X works" in a row, with no content that depends on the question. Fix: headings that name the thing or the task ("Retry behavior", "Deploy to staging").

### 5.26 Despite-its-challenges formula
A paragraph lists problems, then one sentence says the system overcomes them, with no mechanism. Fix: for each problem, name the mitigation, or list it as an open issue.

## 6. Tone and stance

- Promotional: adjectives about the product's quality; "world-class"; superlatives. Fix: facts.
- Cheerleading: exclamation marks, "Great news!", "Exciting".
- Sycophancy toward the reader: "You're absolutely right", "Great choice".
- Condescension: simply, just, easy, obviously, clearly, of course, trivial, straightforward, "all you need to do". Fix: delete; give the step.
- Authority posturing: "experts agree", "studies show", "it is widely believed", "industry best practice" with no source. Fix: name the source or own the claim.
- Vague attribution: "some argue", "many developers", "critics say". Fix: who, where.
- Neutral to the point of emptiness: no stance on a design question that has an answer in the code.
- Moralizing: "It's crucial to prioritize security." Fix: say the specific control required.
- Euphemism: "encountered challenges", "suboptimal performance", "opportunities for improvement", "areas for growth", "unexpected behavior". Fix: say what broke.
- Fake experience: "In my experience", "I've seen", "we found" with no basis. Fix: only if true and sourced.
- Emotional claims: "I'm excited to share", "we are thrilled". Fix: delete.
- Humility theater: "of course, this is just one approach". Fix: state the conditions.
- Inflated stakes: "critical", "vital", "game-changing" for ordinary details.
- Overclaiming universality: "always", "never", "all", "every" without checking. Fix: verify or qualify with the condition.
- Overpromising: "guarantees", "ensures" for what is best effort. Fix: describe the actual guarantee and its limits.

## 7. Formatting and encoding

- Em dashes (U+2014) in density; en dashes used as em dashes. Wikipedia now lists em dash overuse as a historical sign, since newer models use far fewer, so absence proves nothing. This skill still avoids them as a style rule.
- Curly (smart) quotes and apostrophes in Markdown or code-adjacent files.
- Unicode arrows and symbols (U+2192, U+21D2, U+2713, U+2714, U+2022 as inline bullet, U+2026 ellipsis) in prose or source docs.
- Emojis in headings, bullets, or status markers (checkmarks, rockets, sparkles, light bulbs, warning signs).
- Bold sprinkled through paragraphs; bold on every list lead-in.
- Title Case Headings For Everything.
- Headings ending in colons or questions.
- Excessive horizontal rules.
- Inconsistent list markers within a file.
- Tables with one column or used for layout.
- Code blocks without language tags; commands with `$` prompts in copy blocks.
- Long inline-code strings of prose.
- Markdown artifacts in plain text: stray `**`, `##`, backticks inside non-Markdown output.
- Hidden or zero-width characters, non-breaking spaces, trailing whitespace pasted from chat UIs.
- Placeholder brackets left in: `[Insert X]`, `[Your Name]`, `TODO`, `TBD`, `lorem ipsum`.
- Citation markup leaked from tools (`oaicite`, `contentReference`, `oai_citation`, `turn0search0`, `attached_file`, `utm_source=chatgpt.com`, `utm_source=openai`, `referrer=grok.com`, a stray `↩` after a footnote).
- Placeholder dates and URLs left in: `2025-XX-XX`, `INSERT_SOURCE_URL`.
- Broken or fabricated reference markup; footnotes pointing nowhere.
- Overlong lines or decorative ASCII boxes.

Fix: normalize to plain ASCII punctuation, sentence case, minimal formatting.

## 8. Technical-documentation-specific patterns

- README opens with a tagline of adjectives instead of what the software does.
- "Features" list of benefits with no behavior ("Blazing fast performance", "Easy to use").
- "Why X?" section that sells.
- Badges and emoji decoration standing in for information.
- Installation steps that skip prerequisites or versions.
- Quick start that does not produce a visible result.
- Commands or flags that do not exist; examples that cannot run.
- Architecture text that describes layers named in the abstract ("presentation layer, business layer, data layer") with no mapping to directories.
- Diagrams with generic boxes ("Backend", "Database", "API") and no real names.
- Component descriptions that restate the name ("The UserService handles user-related operations").
- "Handles", "manages", "processes", "supports" as the only verbs. Fix: name the operations.
- Docstrings that restate the signature.
- Comments that narrate the next line.
- Config tables without defaults, units, or valid ranges.
- Error docs that say "ensure your configuration is correct".
- Troubleshooting that says "check the logs" with no log location or pattern.
- "Best practices" lists unconnected to the repo.
- Security sections of boilerplate ("we take security seriously") with no controls.
- Performance claims without benchmark data.
- Scalability claims without limits or tests.
- "Future-proof", "extensible", "modular" without extension points named.
- ADRs with no rejected alternatives or no costs.
- Design docs with goals and no non-goals; non-goals that are negations of goals.
- Postmortems that blame people, or name "human error" as root cause, or have action items without owners.
- Runbooks with prose paragraphs instead of commands and expected outputs.
- Onboarding guides with no times, no access list, no first task.
- Changelogs that list commits instead of user-visible changes.
- Migration guides without before and after code or rollback.
- Docs describing planned features as shipped.
- Docs that mention the time of writing ("currently", "new in this release") with no version.
- Documents that never mention a failure case.
- A "Limitations" section that is generic ("may not be suitable for all use cases").
- Vendor-neutral mush where the repo clearly uses one vendor; hide-the-name abstractions.
- Repeating the file tree as a bullet list with one-line restatements of folder names.
- AGENTS.md or CLAUDE.md that is a long essay, repeats the README, or lists rules a linter enforces.

## 9. Claude-specific and chat-assistant tells

These recur in text from recent Claude models and other chat assistants. They are structural and tonal rather than lexical.

- Words: "genuinely", "load-bearing", "honest" (as a flag: "honestly", "to be honest"), "straightforward", "worth", "substantive", "compelling", "nuanced", "rigorous", "thoughtful", "careful", "precisely", "surfaces" (verb), "lands" (verb), "sits" ("sits at the intersection"), "carries weight", "unpacks", "grounded", "robust", "the real X", "the actual issue", "the key insight".
- Bold-lead bullets in nearly every list.
- Arrow shorthand in prose or lists (A -> B) outside code.
- Sentences that begin with a reaction: "That's a great point", "That makes sense", "You're right".
- "Let me" / "I'll" narrations inside documents.
- Closing offers: "Would you like me to...", "Let me know if...".
- Trailing "In short," summaries after each section.
- Em dashes for asides and contrasts at a high rate.
- "Not X, but Y" in the second sentence of a paragraph.
- Over-use of "that said" and "worth noting".
- Parenthetical asides after nearly every claim.
- Lists of exactly three items with parallel wording.
- Section headers phrased as slogans.
- Self-reference: "this document", "this guide", "this section" announcing itself.
- Confident but unsourced specifics.
- Meta-commentary about quality: "I want to be careful here", "to be precise".
- Disclaimers about limits inside deliverables.
Fix: apply sections 3 to 7; delete assistant voice from deliverables.

## 10. Fabrication patterns

Treat as defects, not style.
- Non-existent flags, functions, files, endpoints, packages, versions.
- Plausible but wrong defaults or units.
- Fabricated metrics, benchmarks, percentages, dates.
- Fabricated quotes or citations; real-looking but dead URLs; wrong authors or years.
- Invented people, teams, SLAs, compliance claims.
- Output blocks written from imagination.
- Copy-forward of a claim from an old README that the code contradicts.
- Descriptions of what the code "should" do.
- Fake precision (values with false decimals).
Fix: Phase 2 and Phase 5 of the workflow.

## 11. Leaked-assistant artifacts

Delete on sight: "As an AI", "I cannot browse", "knowledge cutoff", "I don't have access to", "Here is the revised version", "Certainly! Here's", "I hope this helps", "Let me know", "Sure, here", "check this before publishing", prompts or instructions echoed in the text, "[Your Name]", "[Company]", "Note to self", markup like `oaicite`, `contentReference`, `oai_citation`, `turn0search`, `utm_source=chatgpt.com`, `citeturn`, `grok_card`, "Source: [1]" with no list, and tool-call remnants.

## 12. Not reliable indicators

Do not flag these alone, and do not "fix" them just to look human:
- Correct grammar, spelling, and punctuation.
- Formal register or neutral tone in reference docs. "Bland" prose is not evidence either.
- Transition words such as "Additionally" or "Notably" on their own.
- Parallel list structure in a real list.
- Use of "however" or "therefore" where the logic needs it.
- A table, headings, or a numbered list.
- Short paragraphs.
- Consistent terminology (it is a goal).
- Em dashes in a small number, in a quoted source, or in a style where they were already the norm.
- A summary at the top of a long document.
- Passive voice where the actor is irrelevant or unknown.
- Uniformity enforced by a style guide.
- Non-native English that is clear and plain.
Adding errors, slang, or quirks to fake humanity is a failure. Aim for specific, plain, accurate.

## 13. Model generations and drift

- Word-level tells shift as models change. See section 2.4, and do not rely on a fixed list.
- Structure persists across generations: negative parallelism, rule of three, bold-lead bullets, preambles and recaps, significance inflation, trailing participles, uniform section shape.
- A 2025 study found people no better than chance at telling LLM text from human text, and detectors have real error rates. Do not claim a text is AI or human on the basis of a score. Use the catalog to improve text.
- Review this catalog's vocabulary lists every few months against Wikipedia's page and current outputs.

## 14. Scoring and clusters

`slop_scan.py` reports two things. The score counts AI-writing signals per 1,000 words, with a cap per category so one habit cannot dominate, and a floor of 300 words so a short text is not judged on two hits. Style notes (filler words, heading habits, wordy phrases) are listed and counted separately, because people write them too.

| Score | Band | Meaning |
|---|---|---|
| Under 8 | Clean | 90 percent of the 115 human-written READMEs and reference pages used for calibration |
| 8 to 20 | Needs a pass | Fix clusters and high-weight hits |
| Over 20 | Rewrite from evidence | 3 percent of the calibration set; template or unedited AI text is likely |

Treat the score as a prompt for judgment. A cluster is three or more AI-signal categories within about eight lines. Fix clusters first.

## 15. Rewrite procedure

1. Mark every sentence the scanner and your read flagged.
2. For each, ask: what fact should be here? Look it up in the ledger or the code.
3. If a fact exists: rewrite the sentence around it.
4. If no fact exists: delete the sentence.
5. If the reader needs the idea but you lack the fact: ask the user or mark `[unverified: ...]`.
6. After rewrites, check for new tells introduced by the rewrite (often "ensuring", colons, and bold leads).
7. Check the rewrite against the original: every supported claim survives, and no fact was added that you cannot source.
8. Rescan. Read once for flow and sentence-length variation.
9. Report the score change and any facts still missing.

When not to act: leave quotations, names, titles, error messages, and identifiers as they are. If the user supplies a writing sample or the text has a clear human voice, that voice overrides these defaults, including punctuation habits. The goal is good prose. Hiding authorship is not the goal, and adding errors to look human is a failure.
