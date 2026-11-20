# Diagrams

A diagram is optional. Ask at intake which case applies, then follow it.

Contents
1. The developer already has diagrams
2. New diagrams are needed
3. No diagrams
4. Choose the diagram
5. Syntax that breaks

## 1. The developer already has diagrams

Their diagrams stay theirs. Embed them and check them. Do not redraw them, and do not convert a diagram to another notation unless asked.

- Embed with a relative link to the file (PNG, SVG, draw.io export, Mermaid or PlantUML source). Keep the source file next to the export if one exists, and name both.
- Add a caption naming what the diagram shows, and a one-sentence alt text stating the main facts, so the document stands without the image.
- Check the diagram against the code: every box against a real service, module, or store; every arrow against a real call or data flow. Report each mismatch with its evidence. Do not edit the diagram.
- Say the date or version the diagram reflects, if it carries one. If it carries none, say that it is undated.
- If a diagram is a picture only (screenshot, whiteboard photo), describe it in text, and say the contents could not be checked mechanically.
- In Word or PDF output, check that the image is present and readable at page width.

## 2. New diagrams are needed

Draw a diagram when relationships are the point: components and data flow, a sequence across services, a state machine, a deployment topology. Do not draw a diagram of a list. Write diagrams as code so they diff, review, and update with the repo. Default to Mermaid, because GitHub and GitLab render it inline. Use PlantUML, D2, or Structurizr only if the repo already does, or the developer asks.

Ask which diagrams the developer wants. If they say "you decide", start with a system context and a container view, and add a sequence for the one flow that crosses services. Stop there unless a reader needs more.

Rules for each new diagram are in the sections below.

## 3. No diagrams

Write none. Describe structure in a table of components (name, responsibility, technology, location) and numbered flows with file references. Do not leave a heading for a diagram that is not there.

## 4. Choose the diagram

| Question the reader has | Diagram | Mermaid type |
|---|---|---|
| What is this system and what does it talk to? | C4 system context | `flowchart LR` |
| What runs, and how do the parts communicate? | C4 container | `flowchart LR` with `subgraph` |
| What happens, in order, for this request? | Sequence | `sequenceDiagram` |
| What states can this entity be in? | State | `stateDiagram-v2` |
| How is the data related? | Entity relationship | `erDiagram` |
| Where does it run? | Deployment | `flowchart TB` with `subgraph` per environment |

The C4 model's authors say context and container diagrams are enough for most teams. Add component diagrams only where a reader needs them, and skip code-level diagrams, since the code and an IDE show that. Mermaid's native C4 syntax is marked experimental, so use a flowchart and follow C4's notation.

### Rules for new diagrams

- Evidence first. Take boxes from config, IaC, or imports, and arrows from real calls. A diagram is a set of claims, so an arrow without a caller is a fabrication.
- One question per diagram. Split it before you shrink the text.
- Give it a title naming its type and scope, and a key if shape or color carries meaning.
- Use real names. State each element's type and technology ("Orders API, FastAPI") and a short responsibility.
- Make each arrow one-way, with a label saying what flows and over what protocol. "Uses" says nothing.
- Do not rely on color alone. Check it works in greyscale.
- Date or version a diagram that can drift, and check it against the code when the doc changes.

### Accessibility

Every diagram gets `accTitle` and `accDescr`, placed right after the declaration line. Mermaid writes them into the SVG, and this skill's pandoc filter uses them as alt text in Word and PDF. Put the same facts in prose next to the diagram when a reader must act on them.

````markdown
```mermaid
flowchart LR
    accTitle: Orders service, container diagram
    accDescr: The API writes orders to Postgres and publishes to SQS. The worker consumes from SQS and calls the payment provider.
    client["Web client"] -->|"POST /orders, HTTPS"| api["Orders API, FastAPI"]
    api -->|"INSERT order, SQL"| db[("Postgres")]
    api -->|"order.created, SQS"| queue[["orders-queue"]]
    queue -->|"poll"| worker["Worker, Python"]
    worker -->|"charge, HTTPS"| psp["Payment provider"]
```
````

Sequence diagrams carry failure paths well, which is what readers usually lack:

````markdown
```mermaid
sequenceDiagram
    accTitle: Order creation with payment failure
    accDescr: The worker retries a failed charge three times, then marks the order failed.
    participant W as Worker
    participant P as Payment provider
    participant D as Postgres
    W->>P: POST /charges
    alt charge succeeds
        P-->>W: 201
        W->>D: UPDATE status = paid
    else timeout, up to 3 tries
        P-->>W: 504
        W->>D: UPDATE status = failed
    end
```
````

## 5. Syntax that breaks

- Quote labels that contain parentheses, brackets, or punctuation: `api["Orders API (v2)"]`.
- The word `end` in lowercase breaks flowcharts and sequence diagrams. Quote it or capitalize it.
- Unknown words and misspellings break a diagram, while bad parameters fail silently.
- Comments use `%%` on their own line. Avoid `{}` inside them, since they look like directives.
- `accTitle` is one line. A multi-line description uses `accDescr { ... }` with no colon.

If `mmdc` is installed, render to catch errors: `mmdc --input diagram.mmd --output /tmp/out.png`. Otherwise, say that the diagram was not rendered.
