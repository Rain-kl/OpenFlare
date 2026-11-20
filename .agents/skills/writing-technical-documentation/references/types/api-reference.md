# API reference

Reader: a developer looking up an exact fact. Structure mirrors the thing described, and completeness beats narrative.

Generate it. Hand-written reference drifts first, so produce it from the contract, then add what a schema cannot say: semantics, ordering guarantees, gotchas, and tested examples.

| Source | Generator |
|---|---|
| HTTP API | OpenAPI document rendered by Redoc, Swagger UI, or the docs site |
| GraphQL | The schema, rendered by the docs site |
| gRPC | `.proto` files with `protoc-gen-doc` |
| TypeScript | TypeDoc |
| Python | Sphinx autodoc, pdoc, or mkdocstrings |
| C# | DocFX over XML doc comments |
| Java or Kotlin | Javadoc or Dokka |
| Go | Doc comments served by pkgsite |
| Rust | `cargo doc` |

Keep one source of truth for an OpenAPI document: code annotations or a description file, and if both exist, check their consistency in CI. Keep it in source control, validate it in CI with Spectral or Redocly, and publish it so users can generate clients. Hand-editing a generated file makes that file the source of truth.

Entry contents: purpose in one line, method and path or signature, auth, parameters (name, type, required, default, constraints), request example, response example, status codes or errors with causes and fixes, rate limits, idempotency, pagination, side effects, deprecation.

Write field descriptions as facts ("Unix time in seconds, UTC"), and never "The timestamp". Capture examples from a real run or test.

Done when: each parameter, status code, and error appears in the code or schema, and each example ran.

````markdown
### `<METHOD> <path>`

<One line: what it does.>

Auth: <scheme and scope>.

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `<name>` | <path, query, or body> | <type> | <yes or no> | <meaning, units, constraints, default> |

Request:

```bash
curl -X <METHOD> https://api.example.com<path> \
  -H "Authorization: Bearer $API_TOKEN" \
  -d '<body>'
```

Response `<status>`:

```json
<captured response>
```

| Status | Code | Cause | Fix |
|---|---|---|---|
| <4xx> | `<code>` | <cause> | <fix> |

Notes: <idempotency, rate limit, pagination, side effects>.
````
