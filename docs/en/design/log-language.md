# Runtime Language and Internationalization

## Operational Output and Interface Copy

Server, Edge node, background task, installer/uninstaller script, and browser PoW debugging log strings are fixed in English. Switching interface language does not alter console logs or OpenTelemetry exception details, ensuring consistent cross-language log querying and troubleshooting. Task parameters, filenames, user-configured strings, and third-party error messages retain their original values, which may include non-English text.

The frontend uses `next-intl` supporting `en`, `zh-CN`, and `ru`, with `en` as the default locale. User language preference is resolved in order from the `NEXT_LOCALE` cookie, the browser's `Accept-Language`, and the default language; Russian regional variants (such as `ru-RU`) normalize to `ru`. Text copy is maintained under `frontend/messages/fragments` and assembled into root message bundles via `node scripts/merge-i18n-fragments.mjs`. Manual edits to the generated `en.json`, `zh-CN.json`, and `ru.json` files are prohibited. Technical samples and HTML snippets read raw message strings to prevent curly braces in code from being parsed as ICU arguments.

The language selector displays native language names directly from each language's own resource bundle: `English`, `中文`, and `Русский`, without translating language names based on the active UI locale. Dates, numbers, and calendar components format according to the active locale; switching languages does not alter user timezones. Russian pluralization employs ICU `one`, `few`, `many`, and `other` categories.

Static exports pre-render English pages. A standalone bootstrap script in `<head>` inspects the language preference before initial body render; if the preference differs from the pre-rendered language, body rendering is briefly suspended until the Provider mounts the appropriate locale messages, preventing unlocalized text flashes upon reload. The script has no dependency on the application bundle and falls back to the browser language if cookies are corrupt. If script initialization errors or stalls beyond 10 seconds, a localized recovery prompt with a reload button displays; once recovered, prompts and listeners clear to prevent blank pages. Formatting locales synchronize before Provider context updates, ensuring initial hydration of existing data applies correct date and numeric formatting.

Client-side utilities executing outside React component trees (such as network exception handlers and standalone form helpers) invoke `next-intl` messages via `frontend/i18n/runtime.ts`. Messages resolve locale dynamically at call time rather than freezing upon module import. API clients send `Accept-Language` corresponding to the active UI locale.

## Server-Side Presentation Boundaries

`pkg/i18n/locales` maintains English, Chinese, and Russian versions of built-in server messages, with English serving as the baseline in runtime source code. Presentation APIs evaluate the `NEXT_LOCALE` cookie first, followed by `Accept-Language`, falling back to English when unsupported.

- The error handling middleware logs the raw English exception and OpenTelemetry span status before translating `error_msg` in the HTTP response body. Control prefixes used in login workflows (`smtp_invalid:` and `need_email_code:`) remain unlocalized.
- Task types, parameter descriptions, execution records, and built-in notification metadata translate copies within request scopes, avoiding mutation of shared registries or persisted rows.
- Default scheduled task names support translation; administrator-customized names, notification recipients, and saved message templates retain original input text.
- Notification recipient routing recognizes only technical identifiers (`system`, `0`) and the legacy alias (`系统`). Display translations (`System`, `Система`) do not expand reserved targets or divert notifications intended for ordinary users.
- New task execution records persist English system messages. Legacy Chinese text renders as English at read time using registered message keys and format templates, without database rewrites.

Format templates match only registered system messages, preserving captured parameters and timestamps verbatim. Unrecognized messages return unchanged. This compatibility mechanism applies strictly to system-owned display fields; it must never be used to translate arbitrary user data or evaluate business logic based on translated strings. Task retries and storage validation classify issues by error types.

## Persistence Compatibility

The six legacy upload category names serve simultaneously as database counter keys and API identifiers and cannot be renamed as simple UI labels. Retaining these keys, frontend charts, legends, and top-category summaries apply localized labels from message bundles without clearing or reconstructing historical statistics.

Historical SQL migrations, test fixtures, Chinese project documentation, and user-persisted records do not constitute runtime strings requiring replacement. Offline pages and notification templates customized and saved by administrators are treated as user data and do not rewrite on locale changes.

System configuration descriptions and built-in email template names and descriptions may originate from historical seeds. The management console displays `builtin-metadata` translations only when configuration keys or template keys are registered and their text matches known language defaults; unrecognized keys, custom descriptions, and non-built-in templates preserve original text. Edit forms retain original content; saving unrelated fields does not write UI translations back to the database. Email subjects and bodies render saved content. Built-in origin error pages and offline pages preview in the active locale during template selection, while saved custom HTML remains untouched.

System status diagnostic durations and size strings retain English text and standard technical units, converted to localized equivalents at display boundaries; raw strings remain in use for memory computations. Database counts format via the current `next-intl` context. Calendar pickers use DayPicker locale extensions, translating navigation controls, dropdowns, and accessibility labels.

## Verification and Expansion

Running `node scripts/audit-non-english.mjs` at the repository root generates a full manifest of non-ASCII text, outputting by default to `.codex-artifacts/non-english-audit/` (ignored by Git). `occurrences.csv` records file paths, line numbers, modules, categories, and raw strings; JSON output includes character offsets, Unicode code points, character sets, and skipped files; `review-candidates.csv` lists files requiring manual review. The command evaluates all tracked and untracked files without filtering by business directory names, and recognizes Unicode escape sequences.

Manifests include punctuation, emojis, comments, documentation, locale files, and migrations; hits do not automatically imply required changes. Dependencies, build outputs, and private environment files are excluded; binary files are listed separately without OCR inspection or extraction. Generated Swagger artifacts and Chinese documentation configs retain Chinese text, while legacy upload category keys preserve compatibility. Character sets do not equate directly to languages; for example, Han characters may represent Chinese or Japanese.

`pnpm check:i18n` validates keys, tags, and `$` variable consistency across all three language packs, and audits Go, TypeScript/TSX, and installation scripts for unlocalized Hanzi, Kana, Hangul, or Cyrillic characters; comments, locale assets, test cases, the six upload persistence keys, and legacy notification aliases have explicit boundaries. This check integrates into both `make code-check` and frontend CI. Russian language tests verify named arguments, rich-text rendering, ICU formatting, plural boundaries, reproducible rate-limit syntax, and HTML template placeholders; backend tests verify format verbs, template variables, and enumeration values. Technical identifiers (such as `10r/s`, `30m`, `true`, `and`) do not translate.

When adding a language, provide frontend message fragments, locale configuration, Provider message mappings, and the corresponding backend language directory under `pkg/i18n/locales`, verifying placeholder consistency. New system messages continue using English as the canonical source text, with translations added to corresponding locale resources. New protocols must use stable identifiers and parameters rather than localized strings as persistence keys.
