# Pages Static Hosting Usage

You will learn: how to deploy pre-built static sites via local upload, Remote URL, or public GitHub Release assets; configure SPA Fallback and API reverse proxy; and safely check for updates, auto-publish, and roll back.

---

## Core Mechanics and Page Structure

OpenFlare Pages is inspired by Cloudflare Pages' Direct Upload and deployment history interaction, but currently handles **pre-built artifacts** rather than building from repository source. The project detail is organized as "current production deployment → deployment source → deployment history": source configuration can change, while created deployments stay immutable.

```text
Local upload ─> unified validation / upload.Ingest ─> new candidate ─> admin explicit activation ─┐
Remote URL ── Server restricted download ─────────────┐                                            │
GitHub Release asset ─ Server resolves ───────────────┴─> create/load deployment ────────────────┤
                                                       └─> source sync atomic activation ────────┘
                                                                                                   |
                                                                                                   v
                                                                        Agent pulls per-project latest
                                                                                                   |
                                                                                                   v
                                                                        OpenResty local static serving
```

External URLs, GitHub metadata, and auto-checks are handled only by the Server. The Agent only pulls the currently active deployment package from the control plane; it does not receive external source credentials, nor does it run `git clone`, dependency installation, or build commands.

## Step 1: Create a Project

1. Log in to the admin panel, go to **「Pages」**, click **「Create Project」**.
2. Fill in the project name and a unique Slug.
3. Configure the content entry:
   * **Entry file name**: default `index.html`.
   * **Static asset root path (RootDir)**: fill in the relative path when artifacts are in a subdirectory like `dist/`; leave empty when artifacts are at the archive root.
4. Set SPA Fallback and API proxy as needed. RootDir and entry file are project-level configs applied uniformly to all sources.

## Step 2: Choose a Deployment Source

### 1. Manual Upload

Without a persistent source configured, the project stays in manual mode. Click **「Upload Deployment Package」** to select a pre-built archive; a successful upload creates a candidate deployment, which you then explicitly activate from the deployment history. Re-uploading does not modify existing deployments.

Supported formats: `zip`, `tar.gz` / `tgz`, `tar.xz` / `txz`, `tar.bz2` / `tbz2`, `tar`, and `7z`.

### 2. Remote URL

In the deployment source card select **Remote URL**, fill in the HTTP(S) address and choose a network policy:

* **public**: default policy; rejects loopback, private network, link-local addresses, DNS rebinding, self-signed TLS, and redirects to non-public targets.
* **trusted_internal**: only for explicitly trusted intranet or self-signed services; requires a second risk confirmation before saving.

After saving, the address is only displayed masked. You don't need to re-enter it when editing other configs; only submit a new URL when choosing to change the address. Remote sources only offer **「Sync and Publish」**: the Server downloads, validates, and atomically activates each time — no "check for updates", scheduled checks, or auto-updates.

### 3. GitHub Release

GitHub sources only support public `github.com` repositories. Fill in:

* A repository address in `https://github.com/{owner}/{repo}` format;
* **Latest Release** or a **fixed Tag**;
* An exact, case-sensitive Release Asset filename, default `dist.zip`.

Both options support manual **「Check for Updates」** and **「Sync and Publish」**. Differences:

* **latest**: supports a check interval of 5–1440 minutes, default 1440 minutes (24 hours); auto-update is off by default. When enabled, the scanner asynchronously syncs and publishes only when a new revision is found.
* **tag**: only supports manual admin checks and sync; does not participate in the scheduled scanner.

"Check for updates" only resolves the Release/asset and advances the version cursor without downloading the deployment package; "Sync and publish" downloads, validates, creates or reuses a deployment, and activates it. If the asset under the same Release is replaced, the source enters **「Needs Confirmation」** — you must confirm the exact revision shown before publishing, to avoid silent overwrites.

GitHub Release sources only import pre-built artifacts; they do not build from repository source.

### 4. Switch or Delete a Source

You can switch between Manual, Remote, and GitHub Release. Modifying or deleting a source does not delete the current production deployment or historical deployments; switching back to manual mode lets you continue uploading and explicitly activating.

## Deployment Package Security Limits

Deployment packages must satisfy these constraints:

* Archive size is controlled by the system config `pages_max_package_size_mb`, default 100 MiB, configurable 1–2048 MiB.
* Expanded single-file and total size limits are "package size limit × 4", with a floor of 100 MiB; at most 1,000 regular files.
* The control plane streams regular file bodies, checking declared size against actual bytes, and validates the project entry file.
* Absolute paths, `..` path traversal, symlinks, hard links, and special files in archives are all rejected.

The Agent also verifies SHA-256, real response byte limits, and post-extraction file count and total size on download; failures do not switch the existing `current`.

## Step 3: Configure Advanced Routing Rules

### 1. SPA Fallback

When using front-end routing like React Router or Vue Router, enable **「SPA Fallback」** and set the entry path (usually `/index.html`). When a visitor accesses a physical path that doesn't exist, OpenResty falls back to the entry file for the front-end router to handle.

### 2. API Reverse Proxy

Pages can forward a specified prefix to a backend API under the same domain:

* **APIProxyPath**: match prefix, e.g. `/api`.
* **APIProxyPass**: backend address, e.g. `http://10.0.0.5:8080`.
* **APIProxyRewrite**: optional path rewrite rule.

Requests matching the API prefix go through the reverse proxy; other requests continue to be served by the static site.

## Step 4: Bind a Route and First Publish

1. Create or edit a proxy rule.
2. Set the origin type to **Pages** and select the Pages **project**.
3. Preview the config, then publish and activate.

The route binds to a stable project ID, not a specific deployment. The first publish gives the Agent the project anchor; afterwards, local uploads, source syncs, auto-updates, or manual rollbacks only change the project's active deployment — the Agent converges via the latest hash reconciliation without needing to republish the main config.

## Operations, Status, and Rollback

* The source card shows the last check/sync time, found vs. applied revision, next check time, and security errors. While a check or sync task runs, the page polls the task status; when latest is idle, it refreshes at low frequency only near the check time.
* A failed auto-update does not replace the old active deployment; a single source failure does not block the scanner from processing other projects.
* Activating another deployment in the history is a manual rollback. The system fences in-flight source tasks and disables that source's auto-update to avoid the next latest round overwriting your manual choice; re-activating the current version is a no-op.
* The Agent downloads to a temp file, verifies SHA-256, extracts safely, then atomically switches `current`. Any failure keeps the old content; with multi-project reconciliation, a single project failure does not affect others.

> [!TIP]
> For the source state machine, auto scanner, upload compensation, immutable deployments, and Agent atomic switching, see [Pages Static Hosting Design](../design/pages-design.md).
