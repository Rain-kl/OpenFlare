# AutoUpdate 6-Hour Interval and Exponential Backoff Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Throttle agent/edge auto-update checks to once every 6 hours, and apply exponential backoff on failure (doubling up to a maximum of 3 hours) to prevent exhausting GitHub API rate limits.

**Architecture:** Add check interval and exponential backoff state tracking to `edgeupdater.Service` protected by a mutex, with clock injection (`NowFunc`) for deterministic testing. Expose `CanCheck(options UpdateOptions) bool` and integrate it into both `CheckAndUpdate` and `edgeheartbeat.TryAutoUpdate` so unthrottled checks and noisy logs are eliminated during periodic heartbeat loops.

**Tech Stack:** Go (1.25), standard library (`sync`, `time`, `net/http`, `log/slog`).

## Global Constraints
- Normal update check interval: 6 hours (`6 * time.Hour`).
- Initial backoff on failure: 1 minute (`1 * time.Minute`), doubling on each consecutive failure.
- Maximum retry backoff: 3 hours (`3 * time.Hour`).
- Success resets failure count and backoff, and sets next check to `now + 6h`.
- Force check (`UpdateOptions.Force = true`) must bypass backoff and interval immediately.
- TDD required: write failing tests first, watch them fail, implement minimal code, verify pass.

---

### Task 1: Add Interval and Exponential Backoff to `edgeupdater.Service`

**Files:**
- Modify: `internal/apps/edge/updater/service.go`
- Test: `internal/apps/edge/updater/service_test.go`

**Interfaces:**
- Consumes: `UpdateOptions{Channel, TagName, Force}`
- Produces: `(s *Service) CanCheck(options UpdateOptions) bool`, `(s *Service) CheckAndUpdate(...) error` with throttling and exponential backoff

- [x] **Step 1: Write the failing tests in `service_test.go`**

Add tests covering:
- `TestCheckAndUpdateIntervalAndBackoff`:
  1. Initial state allows check.
  2. Successful check sets next check time 6 hours later. Calls before 6 hours return false on `CanCheck` and are skipped in `CheckAndUpdate`.
  3. After 6 hours, check is allowed again.
  4. Failed check (e.g. mock server returns 403) schedules next check 1 minute later.
  5. Second failed check schedules next check 2 minutes later.
  6. Subsequent failures double up to 3 hours (capped at 3 hours).
  7. Force check (`Force: true`) always succeeds in running regardless of cooldown/backoff.
  8. Success after failure resets backoff to initial and schedules next check in 6 hours.

- [x] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/apps/edge/updater -run TestCheckAndUpdateIntervalAndBackoff`
Expected: FAIL (fields or methods missing or behavior not implemented).

- [x] **Step 3: Write minimal implementation in `service.go`**

1. Define constants:
```go
const (
	DefaultCheckInterval  = 6 * time.Hour
	DefaultInitialBackoff = 1 * time.Minute
	DefaultMaxBackoff     = 3 * time.Hour
)
```

2. Extend `Config`:
```go
type Config struct {
	LocalVersion   string
	AssetPrefix    string
	LogLabel       string
	CheckInterval  time.Duration
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	NowFunc        func() time.Time
}
```

3. Extend `Service` struct with:
```go
type Service struct {
	httpClient          *http.Client
	lastCheckKey        string
	localVersion        string
	assetPrefix         string
	logLabel            string
	checkInterval       time.Duration
	initialBackoff      time.Duration
	maxBackoff          time.Duration
	nowFunc             func() time.Time
	mu                  sync.Mutex
	nextCheckTime       time.Time
	currentBackoff      time.Duration
	consecutiveFailures int
}
```

4. Implement `CanCheck(options UpdateOptions) bool`:
```go
func (s *Service) CanCheck(options UpdateOptions) bool {
	if options.Force {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nextCheckTime.IsZero() {
		return true
	}
	now := s.now()
	return !now.Before(s.nextCheckTime)
}
```

5. In `CheckAndUpdate`:
- Check `CanCheck(options)`. If false, return `nil`.
- On `getRelease` error: calculate next backoff (initial backoff on 1st failure, double previous backoff capped at maxBackoff on subsequent), increment `consecutiveFailures`, set `nextCheckTime = now.Add(currentBackoff)`, return error.
- On `getRelease` success: reset `consecutiveFailures = 0`, reset `currentBackoff = initialBackoff`, set `nextCheckTime = now.Add(checkInterval)`.

- [x] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/apps/edge/updater/...`
Expected: PASS.

---

### Task 2: Integrate `CanCheck` in `edgeheartbeat.TryAutoUpdate`

**Files:**
- Modify: `internal/apps/edge/heartbeat/autoupdate.go`
- Test: `internal/apps/edge/heartbeat/autoupdate_test.go`

**Interfaces:**
- Consumes: `(s *Service) CanCheck(options UpdateOptions) bool`
- Produces: `TryAutoUpdate(ctx context.Context, updater *edgeupdater.Service, settings *AutoUpdateSettings, logLabel string)`

- [x] **Step 1: Write the failing test in `autoupdate_test.go`**

Test that `TryAutoUpdate` checks `updater.CanCheck(opts)` and skips both log output and `CheckAndUpdate` call when throttled.

- [x] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/apps/edge/heartbeat/...`
Expected: FAIL.

- [x] **Step 3: Update `autoupdate.go`**

Update `TryAutoUpdate` to evaluate `updater.CanCheck(opts)` before logging `checking for updates` and executing `CheckAndUpdate`.

- [x] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/apps/edge/heartbeat/...`
Expected: PASS.

---

### Task 3: Full Verification of Edge and Agent Packages

**Files:**
- Test: All tests in `internal/apps/edge/...` and `internal/apps/agent/...`

- [x] **Step 1: Run comprehensive tests**

Run:
```bash
go test -v ./internal/apps/edge/... ./internal/apps/agent/...
```
Expected: PASS with zero failures or race conditions.
