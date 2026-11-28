# Pages Static Hosting

Serve pre-built static asset bundles directly from edge nodes with SPA fallback, same-origin API proxying, and atomic rollbacks (source compilation is not performed).

## 1. Create a Project

1. Open **Pages**, click **Create Project**, and enter a name and unique slug.
2. Configure **Entry file** (default `index.html`) and **RootDir** (e.g., `dist/`, or blank if assets sit at the archive root).
3. Optionally enable SPA Fallback or API Reverse Proxy, then save.

## 2. Choose a Deployment Source

You can switch sources at any time without removing active or historical deployments:

| Source | Behavior & Restrictions |
| :--- | :--- |
| Manual Upload | Upload `zip`, `tar.gz`/`tgz`, `tar.xz`/`txz`, `tar.bz2`/`tbz2`, `tar`, or `7z` archives as immutable candidate versions, then activate from history |
| Remote URL | Enter an HTTP(S) URL and select a network policy: `public` (blocks private/loopback/DNS rebinding/self-signed TLS) or `trusted_internal` (requires risk confirmation); supports manual **Sync and Publish** only |
| GitHub Release | Public `github.com` repositories only; specify repo URL, exact asset filename (default `dist.zip`), and mode: **Latest Release** (`5–1440` minute polling, auto-update off by default) or **Fixed Tag** (manual only); replaced tag assets enter **Needs Confirmation** |

> Package uploads are bounded by `pages_max_package_size_mb` (default `100 MiB`, range `1–2048 MiB`), `4x` extracted size (minimum `100 MiB`), and `1,000` regular files; symlinks, hard links, and path traversal entries are rejected.

## 3. Configure SPA Fallback & API Proxy

Under project creation or **Settings** (click **Save** to apply):

- **SPA Fallback**: Serves the configured fallback entry (typically `/index.html`) for unmatched static paths.
- **API Reverse Proxy**: Set **APIProxyPath** (e.g., `/api`), **APIProxyPass** (e.g., `http://10.0.0.5:8080`), and optional **APIProxyRewrite** to forward API calls under the same domain.

## 4. Bind Route & Roll Back

1. Create or edit a route under **Proxy Routes**, set origin mode to **Pages**, select the project, and save.
2. To roll back, activate any earlier version in **Deployment History**; manual activation automatically disables source auto-update so scheduled checks do not overwrite it.

See [Pages Static Hosting Design](../design/pages-design.md) for deployment state transitions and atomic switching internals.
