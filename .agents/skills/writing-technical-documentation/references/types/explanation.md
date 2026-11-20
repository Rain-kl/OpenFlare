# Explanation (concept page)

Purpose: build understanding of why and how things fit together. Read it away from the keyboard. Prose is right here, and it can take a position.

Contents: context, the concept, the mechanism, design reasons, alternatives and trade-offs, and history only where it explains the present. Follow the data or control flow with actors and verbs, then add the failure path. Leave steps to how-to guides and exhaustive edge cases to reference.

Use one for behavior that spans several modules, where a comment in one file cannot carry it. Open with who it is for and what they will understand, since a reader may have arrived from search. Link to reference and how-to pages instead of repeating them.

```markdown
# <Concept as a noun phrase>

<Who this is for and what they will understand afterwards, in one or two sentences.>

## How it works

<Follow one request, job, or message. Name real components and files, and state the failure path.>

## Why it works this way

<The constraint that forced the design, and what was rejected.>

## Limits

<What it does not do, with numbers where they exist.>

## Related

- <reference page>
- <how-to page>
```
