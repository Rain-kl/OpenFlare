# Uptime Kuma Sync Design

OpenFlare synchronizes active reverse-proxy sites to an external Uptime Kuma instance from a background control-plane job (`internal/apps/openflare/uptimekuma/`) over Socket.IO. Using a dedicated `OpenFlare` tag boundary, the sync routine performs incremental add, edit, and delete diffs without resetting historical SLA metrics or touching unrelated monitors managed manually in Kuma.

---

## 1. Architecture and Execution Flow

Synchronization runs strictly on the Server control plane without adding load or credential exposure on edge nodes:

```text
OpenFlare Server (1-minute Cron ticker evaluating UptimeKumaSyncInterval)
  └─ Mutex lock skips overlapping runs
       └─ Socket.IO connect & authenticate with Uptime Kuma
            ├─ Ensure dedicated "OpenFlare" tag exists (default color #4f46e5)
            ├─ Wait for full monitorList payload before computing diffs
            └─ Reconcile tagged monitors only (add / editMonitor / deleteMonitor)
```

* **Full-list barrier**: Diff calculation runs only after Uptime Kuma delivers the complete `monitorList` Socket.IO event, preventing premature deletions during partial loads.

---

## 2. Tag Isolation and Differential State Machine

1. **Tag isolation**: After fetching `monitorList`, only monitors tagged with `OpenFlare` enter reconciliation. Any manually created monitor without the `OpenFlare` tag stays invisible to both edits and cleanups.
2. **Concrete URL selection**: For each monitored proxy route, the sync routine selects a concrete bound hostname (`https://` or `http://` according to `enable_https`). Routes bound solely to wildcard domains (e.g., `*.example.com`) are skipped safely without deleting existing monitors.
3. **Differential comparison**: Matching monitors update via `editMonitor` only when at least one of five configured attributes differs from OpenFlare options:
   * Target `Url`
   * Probe interval `Interval` (default `60s`)
   * `MaxRetries`
   * `RetryInterval` (default `60s`)
   * `Timeout` (default `48s`)
4. **Cleanup**: Sites removed or disabled from monitoring scope have their `OpenFlare`-tagged monitor removed via `deleteMonitor`.
