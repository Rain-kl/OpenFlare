# Documentation styles and standards

Read this when the developer names a style, a standard, or a document type, or asks which one fits. Each entry gives the reader, when it fits, the usual sections, and which files in `references/types/` to build from.

Contents
1. Rules for using a style
2. Chooser
3. Whole-system documents and frameworks
4. Requirements, design, and decisions
5. Reference
6. Guides for people doing a task
7. Operations and delivery
8. Assurance: security, test, incidents
9. AI systems and agent files

## 1. Rules for using a style

- The developer's own table of contents or template wins over every entry here. Follow it as given: same headings, same order, same numbering.
- A section list here is a starting outline. Drop a section with no project-specific content. If the developer's template or a formal review needs every heading kept, write one line under it saying why it does not apply.
- Entries for published standards describe their structure in this skill's own words. They are not the standard. Write "structured after <standard>". Never write "conforms to" or "compliant with" unless the developer has the standard text and asks for that check.
- A style that is not listed: ask for a sample or the template. If there is none, name the closest entry here and propose its outline. Never invent the section list of a named standard.
- Mixed requests are normal, such as "arc42 with an API chapter". Start from the named style, then insert the extra sections where a reader would look for them.
- Each section still follows the evidence rules. A style changes the headings. It does not change what counts as a fact.

## 2. Chooser

| The developer says or needs | Start from |
|---|---|
| "Document this project", one deliverable, no type named | Overall technical document |
| A docs site or a set of pages | Diataxis |
| A formal architecture document, review, or audit | arc42, or Architecture views |
| A design that an institution or contract asks for by name | IEEE 1016 SDD, HLD, LLD, or MIL-STD-498 style |
| What the system must do, before or apart from design | SRS |
| Why it was built this way | Design doc, ADR |
| Facts to look up | API, SDK, CLI, configuration, data dictionary |
| How to do a task | README, tutorial, how-to, user guide, deployment guide |
| Keeping it running | Runbook, operations guide, infrastructure |
| Passing it to another team | Handover document |
| Proof it was tested or is safe | Test documentation, threat model |

## 3. Whole-system documents and frameworks

### Overall technical document
- For: the people who take the system over: maintainers, reviewers, another team's engineers.
- Use when: the request names no type, or one document with a table of contents is wanted. This is the default.
- Sections: overview and scope, architecture, components, data, interfaces, configuration, setup and deployment, operations, security, testing, known gaps, glossary. Variants by project kind are in `references/outlines.md`.
- Build from: `types/architecture.md` for the spine, then `types/api-reference.md`, `types/infrastructure.md`, `types/runbook.md`, `types/onboarding.md`, `types/ai-systems.md`, `types/glossary.md` for the chapters that apply.

### Diataxis
- For: users of a product or library with different needs at different times.
- Use when: the output is a docs site or several pages, and readers both learn and look things up.
- Sections: four kinds of page kept apart: tutorials, how-to guides, reference, explanation. One page, one kind.
- Build from: `types/tutorial.md`, `types/how-to.md`, `types/explanation.md`, and the reference types in section 5.

### arc42
- For: architects, developers, and reviewers who want one architecture document with a known shape.
- Use when: the developer asks for arc42, or a system is going through a formal architecture review.
- Sections: twelve areas, in this order: goals and stakeholders; constraints; context and scope; solution strategy; building blocks; runtime behavior; deployment; cross-cutting concepts; architecture decisions; quality requirements; risks and technical debt; glossary. Take the exact headings from the developer's arc42 template or arc42.org. arc42 itself says to leave out what a system does not need.
- Build from: `types/architecture.md` for context, building blocks, runtime, and deployment; `types/adr.md` for decisions; `types/glossary.md`.

### C4 model documentation
- For: anyone who needs the map of a system at a chosen zoom level.
- Use when: the developer asks for C4, or the document is mainly diagrams with supporting text.
- Sections: system context, containers, then components only where a reader needs them. Optional: deployment diagram, dynamic diagram for one flow, system landscape for several systems. Each diagram gets a title, a key, and a paragraph of what it shows.
- Build from: `types/architecture.md` and `references/diagrams.md`.

### Architecture views (4+1, Views and Beyond, ISO/IEC/IEEE 42010)
- For: stakeholders with different concerns: developers, operators, security, management.
- Use when: the developer asks for "views" or "viewpoints", or an architecture description must show who cares about what.
- Sections: stakeholders and their concerns; one section per view (for 4+1: logical, process, development, physical, tied together by a few scenarios); how the views relate; rationale.
- Build from: `types/architecture.md` once per view, `types/adr.md` for rationale.

### High-level design (HLD)
- For: architects, leads, and reviewers before or after the build.
- Use when: the developer asks for an HLD, or for the "big picture" design as its own document.
- Sections: scope and requirements summary, architecture diagram and component list, data flow, data stores, external interfaces and integrations, technology choices with reasons, security, deployment topology, non-functional targets, assumptions and risks.
- Build from: `types/architecture.md`, `types/design-doc.md` for choices and alternatives.

### Low-level design (LLD)
- For: the developers who implement or maintain one component.
- Use when: the developer asks for an LLD or a "detailed design" of a module.
- Sections: per component: responsibility, classes or modules and their relationships, function and method contracts, data structures and schema, algorithms and state, error handling, configuration, test notes. Sequence or class diagrams where they carry the logic.
- Build from: `types/design-doc.md`, `types/api-reference.md`, `types/docstrings.md`. Take every signature from the code.

### IEEE 1016 software design description (SDD)
- For: design reviewers, maintainers, testers, in settings that name this standard.
- Use when: a course, contract, or quality system asks for an "SDD" or "IEEE 1016".
- Sections: identification (date, authors, scope, references); design stakeholders and their concerns; design views, each written from a stated viewpoint; design rationale. Common viewpoints: context, composition, logical, dependency, information, interface, structure, interaction, state dynamics, algorithm, resource. Choose the viewpoints that answer the stated concerns. The 2009 edition is no longer maintained by IEEE, so ask which edition or house template applies.
- Build from: `types/architecture.md` for context and composition views, `types/design-doc.md` for rationale, `types/api-reference.md` for the interface view.

### MIL-STD-498 style document set
- For: contract and defense-style deliveries that name documents by acronym.
- Use when: the developer asks for an SDD, IDD (interface design), DBDD (database design), SVD (version description), SUM (user manual), STP or STR (test plan or report) by those names.
- Sections: each document has its own fixed outline in the standard's data item descriptions. Get the outline from the developer's contract template. The set is meant to be tailored: pick only the documents and paragraphs the project needs.
- Build from: SDD: `types/architecture.md` and `types/design-doc.md`. IDD: Interface control document below. DBDD: Data dictionary below. SVD: `types/changelog.md`. SUM: User guide below. STP and STR: Test documentation below.

### Docs as code
- For: teams that want docs maintained like source.
- Use when: the developer asks how to store, review, or publish docs, more than what to write.
- Sections: this is a workflow: plain-text markup, version control, review in pull requests, automated checks, publishing from CI. It works with any style here.
- Build from: `references/docs-ci.md`, `references/audit-and-maintain.md`.

## 4. Requirements, design, and decisions

### Software requirements specification (SRS)
- For: customers, testers, and developers agreeing on what the software must do.
- Use when: the developer asks for an SRS, a requirements document, or "IEEE 830" (replaced by ISO/IEC/IEEE 29148).
- Sections: introduction (purpose, scope, product overview, users, limits, definitions); references; requirements grouped as functions, external interfaces, performance, usability, data, design constraints, and quality attributes; how each requirement is verified; assumptions and dependencies. Each requirement gets an ID and one testable statement.
- Build from: no type file. Use this outline. When documenting existing code, state requirements only where a ticket, test, contract, or the developer supplies them. Code shows what the system does, not what it was required to do, so mark reverse-engineered items "observed behavior".

### Design doc and RFC
- For: reviewers deciding whether to build something a certain way.
- Use when: a change is proposed or was recently made and the reasoning matters.
- Sections: context, goals and non-goals, proposed design, alternatives considered, trade-offs, risks, rollout.
- Build from: `types/design-doc.md`.

### Architecture decision record (ADR)
- For: future maintainers asking "why is it like this?".
- Use when: one decision needs a durable record. Nygard, MADR, and Y-statement formats all fit.
- Sections: title, status, context, decision, consequences. MADR adds options considered and their pros and cons.
- Build from: `types/adr.md`.

## 5. Reference

### API reference
- For: developers calling the API.
- Use when: there are HTTP, RPC, or event interfaces. Prefer generating from OpenAPI or AsyncAPI where a contract exists.
- Sections: authentication, base URL and versioning, per-operation request and response with examples, errors, limits, pagination.
- Build from: `types/api-reference.md`.

### SDK or client library guide
- For: developers integrating through a library.
- Use when: the project ships a package others import.
- Sections: install and supported versions, authentication and client setup, a quickstart that makes one real call, task guides for common jobs, error handling and retries, generated reference, upgrade notes.
- Build from: `types/readme.md` for install and quickstart, `types/how-to.md`, `types/api-reference.md`, `types/migration.md`.

### CLI reference
- For: people running the tool.
- Sections: synopsis, examples first, commands and flags with defaults, configuration precedence, exit codes.
- Build from: `types/cli-reference.md`.

### Configuration reference
- For: operators and developers changing behavior without changing code.
- Use when: the project has more than a handful of settings or environment variables.
- Sections: one table per source (env, file, flags): name, type, default, valid range or values, effect, whether a restart is needed. Precedence between sources. Never values of secrets.
- Build from: `types/cli-reference.md` and `types/infrastructure.md`. Take names and defaults from the code that reads them.

### Data dictionary and data model
- For: analysts, developers, and anyone querying or migrating the data.
- Use when: the system owns tables, collections, events, or files that others read.
- Sections: per store or dataset: purpose and owner. Per field: name as stored, plain-language meaning, type, required or nullable, allowed values or range, units and format, default, source or derivation, sensitivity. Then keys and relationships, an entity relationship diagram, retention.
- Build from: no type file. Use this outline with `references/diagrams.md` (entity relationship). Take fields from migrations, schema files, or ORM models, never from sample rows alone.

### Interface control document (ICD)
- For: two teams or systems that meet at an interface.
- Use when: an integration needs an agreed written contract beyond an API reference.
- Sections: the two parties and scope, transport and protocol, message or file formats, field definitions, sequencing and timing, error handling, security, versioning and change process.
- Build from: `types/api-reference.md`, `references/diagrams.md` (sequence).

### Glossary
- Build from: `types/glossary.md`.

### Docstrings and code comments
- Build from: `types/docstrings.md`.

## 6. Guides for people doing a task

### README
- For: someone who just found the repo.
- Build from: `types/readme.md`.

### Tutorial, how-to, explanation
- Build from: `types/tutorial.md`, `types/how-to.md`, `types/explanation.md`.

### User guide or user manual
- For: end users or administrators, who may not read code.
- Use when: the developer asks for a "user manual", "user guide", or "admin guide".
- Sections: what the product does and who it is for, getting access and first use, tasks grouped by goal with steps and expected results, settings, messages and errors with fixes, where to get help.
- Build from: `types/how-to.md` per task, `types/tutorial.md` for first use, `types/troubleshooting.md`. Take UI labels and messages from the source strings.

### Installation and deployment guide
- For: whoever installs or ships the system.
- Use when: setup has more steps than a README should hold, or differs per environment.
- Sections: prerequisites with versions, access and accounts needed, configuration, step-by-step install or deploy with the expected result of each step, verification, rollback, upgrade, uninstall.
- Build from: `types/infrastructure.md`, `types/how-to.md`, `types/runbook.md` for rollback.

### Onboarding guide
- Build from: `types/onboarding.md`.

### Troubleshooting guide and FAQ
- Build from: `types/troubleshooting.md`.

### Migration or upgrade guide
- Build from: `types/migration.md`.

## 7. Operations and delivery

### Runbook and playbook
- For: the on-call engineer.
- Build from: `types/runbook.md`.

### Operations guide and infrastructure documentation
- For: the team that runs the system day to day.
- Sections: environments, topology, how a change ships, scaling, backups and restore, monitoring and alerts, routine tasks, cost drivers.
- Build from: `types/infrastructure.md`, with a link or section per runbook.

### Handover or transition document
- For: the team that receives the system: operations, a client, or a new maintainer.
- Use when: ownership changes hands at the end of a project or phase.
- Sections: scope of what is handed over and what is not; system summary with a link to the technical document; repositories, environments, and accounts as an inventory (names and owners, never credentials); how to build, deploy, and roll back; operating tasks and schedules; known issues, open work, and technical debt; third-party services and licenses; contacts and support terms as supplied by the developer; acceptance or sign-off list if the developer wants one.
- Build from: `types/architecture.md` (short form), `types/infrastructure.md`, `types/runbook.md`, `types/onboarding.md`. Run the audit in `references/audit-and-maintain.md` section 1 first, so stale docs are not handed over. Owners, contacts, and support terms come only from the developer.

### Changelog, release notes, version description
- For: users deciding whether and how to upgrade.
- Build from: `types/changelog.md`. A version description also lists the delivered files and the build they came from.

### CONTRIBUTING and SECURITY files
- Build from: `types/contributing-security.md`.

## 8. Assurance: security, test, incidents

### Threat model and security overview
- For: security reviewers and the engineers who fix what is found.
- Use when: the developer asks for a threat model, a security section, or a security review document.
- Sections: scope and assets; a data flow diagram with external entities, processes, data stores, flows, and trust boundaries; assumptions; threats per element (STRIDE is the usual checklist); existing controls and planned mitigations with owners; how the model was validated; open risks.
- Build from: no type file. Use this outline with `references/diagrams.md`. Record only controls you can point to in code or config. A threat list is analysis, so label it as such and have the developer confirm ratings.

### Test documentation
- For: testers, release managers, and auditors.
- Use when: the developer asks for a test plan, test report, or "test documentation". ISO/IEC/IEEE 29119-3 is the usual standard.
- Sections: test plan: scope, items and features under test, approach and levels, environments and data, entry and exit criteria, risks, roles, schedule. Test cases or procedures: ID, precondition, steps, expected result, trace to a requirement. Test report: what ran, on which build and environment, results, open defects, deviations from plan, assessment against exit criteria.
- Build from: no type file. Use this outline. Report only results you ran or were given: quote the command, the date, and the counts. Never write a pass rate from reading test code.

### Postmortem
- Build from: `types/postmortem.md`.

## 9. AI systems and agent files

### Model card, system card, dataset documentation
- For: people deciding whether a model or dataset is fit for their use.
- Sections: intended use and out-of-scope use, training and evaluation data, metrics with numbers and dates, limits and known failure cases, how to reproduce.
- Build from: `types/ai-systems.md`.

### LLM application and agent flow documentation
- Build from: `types/ai-systems.md`, with `references/diagrams.md` for the flow.

### AGENTS.md, CLAUDE.md, llms.txt
- Build from: `types/agent-instructions.md`.
