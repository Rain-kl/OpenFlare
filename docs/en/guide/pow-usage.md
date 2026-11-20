# Proof-of-Work (PoW) Protection

Open the target rule in **Rule Management** and switch to the dedicated **PoW** tab to enable proof-of-work challenge verification for that route. Configure computation mode, difficulty level, pass token lifetime, and challenge timeout. Changes sync automatically to edge nodes with the site configuration snapshot.

Parallel mode utilizes up to 8 Web Workers; single-threaded mode runs in 1 Worker. Each increase of 1 in difficulty increases the expected computation work approximately 16-fold. Pass credentials are saved as HttpOnly cookies scoped to the accessed domain, and valid subsequent requests to protected paths extend the pass cookie.

## Trigger Path Modes

| Mode | Behavior |
| --- | --- |
| Verify all requests, exclude allowlist | Validates all requests reaching PoW; paths matching the allowlist bypass verification |
| Verify blocklist paths only | Only validates requests matching blocklist paths; at least one blocklist rule required |

Allowlist rules take precedence. Rules within each list evaluate with OR logic (match any). Bypassing PoW continues execution of subsequent WAF and routing rules.

| Match Type | Example | Behavior |
| --- | --- | --- |
| Exact | `/login` | Matches this exact path only |
| Prefix | `/admin/` | Matches this directory and all subpaths |
| Wildcard | `/admin/*` | `*` matches any number of characters, including `/` and empty strings; matches the entire path |
| Regex | `^/admin(?:/|$)` | Matches `/admin` and subpaths, excluding `/administrator` |

Target paths are normalized URI paths on the edge node, excluding query parameters, case-sensitive. Standard paths must start with `/`. Regular expressions are validated using Go regex syntax when saved and executed using PCRE on edge nodes. Each list supports up to 100 rules, with a maximum of 1,024 bytes per rule.

For example, to protect only administrative consoles, select **Verify blocklist paths only**, add blocklist regex `^/admin(?:/|$)`, and add allowlist exact path `/admin/health`. The home page, static assets, and health check endpoints bypass verification, while admin routes require completing the challenge.

The challenge page requires a JavaScript-capable browser. Webhooks, health checks, and API clients that cannot execute JavaScript must be excluded using path allowlist rules.

Unconfigured IP, CIDR, path, or User-Agent lists evaluate as empty lists, maintaining compatibility with `null` values in configurations. Saved policies sync automatically to edge nodes with the site snapshot.

Path matching and bypass actions align with [Cloudflare Ruleset Engine Operators](https://developers.cloudflare.com/ruleset-engine/rules-language/operators/) and [Action Fields](https://developers.cloudflare.com/ruleset-engine/rules-language/actions/).

## Custom Challenge Pages

Navigate to **Response Pages → PoW Challenge Page** to select and configure visual themes and widget styles:

- Page theme: Classic (default), Anubis.
- Verification widget: Progress bar, status transition. Both widgets execute client-side proof-of-work computation.

Click the preview area or select **Edit** to open the HTML editor with live side-by-side preview. Leaving the HTML field empty uses the selected built-in template. Select **Classic** or **Anubis** from the **Built-in Templates** dropdown and click **Load Template** to load baseline markup into the editor for customizing layouts, text, and styles. **Restore Default** clears custom HTML and reverts to the selected preset theme. The preview iframe simulates computation and success states; real browser traffic completes the cryptographic challenge and redirects to the requested URL upon server-side verification.

HTML templates are capped at 256 KiB and support the following placeholders with HTML escaping applied:

| Placeholder | Content |
| --- | --- |
| `{{widget}}` | Selected verification widget element |
| `{{host}}` | Requested hostname |
| `{{lang}}` | Challenge page language code (`zh-CN` or `en`) |
| `{{title}}`, `{{status}}` | Built-in title and status descriptions |
| `{{protected}}` | Protection notice text |
| `{{why}}`, `{{why_description}}` | Verification rationale and description |
| `{{noscript}}` | Fallback message when JavaScript is disabled |

The edge runtime injects necessary scripts automatically. A minimal template:

```html
<!DOCTYPE html>
<html lang="{{lang}}">
<head><meta charset="utf-8"><title>{{title}}</title></head>
<body>
  <h1>{{host}}</h1>
  {{widget}}
  <noscript>{{noscript}}</noscript>
</body>
</html>
```

Custom components can also be placed directly with explicit appearances:

```html
<openflare-pow appearance="progress"></openflare-pow>
<openflare-pow appearance="turnstile"></openflare-pow>
```

Custom scripts can listen for the `openflare:pow` DOM event. `detail.status` transitions through `loading`, `working`, `verifying`, and `success` (or `error` on failure). `progress` provides estimated progress from 0 to 100, reaching 100 only after server-side cryptographic validation completes:

```html
<p id="verification-status" role="status"></p>
<script>
  window.addEventListener('openflare:pow', (event) => {
    document.getElementById('verification-status').textContent = event.detail.message;
  });
</script>
```

The runtime exposes `window.OpenFlarePoW.state`, `subscribe(callback)`, and `retry()`. Calling `subscribe` invokes the callback immediately with the current state and returns an unsubscribe function. Widget styles are encapsulated inside a Shadow DOM and can be customized using CSS variables `--accent`, `--success`, `--error`, `--surface`, and `--line`.

## Legacy WAF PoW Nodes

Sites without saved standalone PoW policies continue executing existing WAF PoW workflow nodes. The WAF node library no longer offers an entry to create new PoW nodes, but existing nodes remain viewable and editable.

Once a site saves a standalone PoW policy, legacy WAF PoW nodes in that route automatically bypass execution along `next`, preserving the order of other WAF rules. A standalone policy takes precedence over legacy nodes even when disabled. Routes that previously used conditional IP, geo, or User-Agent branching for PoW challenges should configure path matching rules in the standalone policy accordingly.
