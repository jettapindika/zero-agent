# Zero SDK + Lifecycle Commands Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `zero` SDK-first and add daemon lifecycle commands: `setup`, `start`, `stop`, `restart`, `status`, and `logs`.

**Architecture:** Add a Go SDK module at `packages/sdk-go` that owns localhost REST transport and typed request/response helpers. Keep the core backend unchanged. Refactor the CLI to call the SDK for API operations, then add small process-management helpers for the local daemon using `~/.zero/zero.pid` and `~/.zero/zero.log`.

**Tech Stack:** Go 1.26.3, Cobra, chi backend, SQLite, net/http, os/exec, syscall, standard library tests.

---

## File Structure

- Create `packages/sdk-go/go.mod`: module metadata and replace path to core only if needed.
- Create `packages/sdk-go/client.go`: SDK client, JSON helpers, typed errors, headers.
- Create `packages/sdk-go/sessions.go`: project/session/message/run methods used by CLI.
- Create `packages/sdk-go/collab.go`: room/join/participants/queue methods used by CLI.
- Create `packages/sdk-go/client_test.go`: httptest coverage for SDK request paths and response handling.
- Modify `go.work`: include `./packages/sdk-go`.
- Modify `apps/cli/go.mod`: depend on `github.com/zero-agent/sdk-go` with local replace.
- Modify `apps/cli/main.go`: replace raw API calls with SDK client methods; add lifecycle commands and daemon helpers.
- Modify `README.md`: terminal-first docs and new lifecycle commands.
- Modify `Makefile`: build/test SDK and CLI.
- Modify `.opencode-context.md`: record SDK-first/lifecycle status after implementation.

---

### Task 1: Add Go SDK module and tests

**Files:**
- Create: `packages/sdk-go/go.mod`
- Create: `packages/sdk-go/client.go`
- Create: `packages/sdk-go/sessions.go`
- Create: `packages/sdk-go/collab.go`
- Create: `packages/sdk-go/client_test.go`
- Modify: `go.work`

- [ ] **Step 1: Add SDK module**

Create `packages/sdk-go/go.mod`:

```go
module github.com/zero-agent/sdk-go

go 1.26.3
```

Update `go.work`:

```go
go 1.26.3

use (
	./apps/cli
	./packages/sdk-go
	./services/core
)
```

- [ ] **Step 2: Write SDK tests first**

Create `packages/sdk-go/client_test.go` with `httptest.Server` cases for:
- `EnsureProject` sends `POST /projects/ensure` and decodes `Project`.
- `ListSessions` sends `GET /sessions?projectId=...`.
- `CreateSession` sends `POST /sessions`.
- `SendMessage` sends `POST /sessions/{id}/messages`.
- `RunSession` sends `POST /sessions/{id}/run`.
- `CreateCollabRoom` sends `X-Zero-Client-ID`.
- non-2xx response returns `APIError` with status code and body.

- [ ] **Step 3: Run tests and confirm failure**

Run: `go test ./packages/sdk-go/...`

Expected: FAIL because SDK implementation files are missing.

- [ ] **Step 4: Implement SDK client**

Implement:

```go
type Client struct {
    BaseURL string
    HTTP    *http.Client
    ClientID string
}

func NewClient(baseURL string, opts Options) *Client
func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any) error
```

Typed structs needed by current CLI:
- `Project`, `Session`, `Message`, `Part`
- `CreateSessionInput`, `SendMessageInput`
- `CreateRoomInput`, `CreateRoomResult`, `JoinRoomResult`
- `Participant`, `QueueItem`
- `APIError`

Methods:
- `Health(ctx)`
- `EnsureProject(ctx, path, name string)`
- `ListSessions(ctx, projectID string)`
- `CreateSession(ctx, input)`
- `GetSessionMessages(ctx, sessionID string)`
- `SendMessage(ctx, sessionID string, input)`
- `RunSession(ctx, sessionID string)`
- `CreateCollabRoom(ctx, input)`
- `JoinCollabRoom(ctx, roomID, token, displayName string)`
- `ListParticipants(ctx, roomID string)`
- `ListPromptQueue(ctx, roomID, sessionID string)`

- [ ] **Step 5: Run SDK tests**

Run: `go test ./packages/sdk-go/...`

Expected: PASS.

---

### Task 2: Refactor CLI to use SDK

**Files:**
- Modify: `apps/cli/go.mod`
- Modify: `apps/cli/main.go`

- [ ] **Step 1: Add CLI dependency**

Update `apps/cli/go.mod`:

```go
require (
	github.com/spf13/cobra v1.8.1
	github.com/zero-agent/core v0.0.0
	github.com/zero-agent/sdk-go v0.0.0
)

replace github.com/zero-agent/core => ../../services/core
replace github.com/zero-agent/sdk-go => ../../packages/sdk-go
```

- [ ] **Step 2: Replace raw HTTP helpers with SDK client factory**

Add:

```go
func newSDKClient(clientID string) *sdk.Client {
    return sdk.NewClient(serverAddr, sdk.Options{ClientID: clientID})
}
```

Remove CLI-local API JSON structs where SDK types cover them.

- [ ] **Step 3: Refactor current commands without behavior changes**

Convert these commands to SDK methods:
- `share`
- `join`
- `participants`
- `queue`
- `sessions`
- `config` health check
- `export`
- `ensureSession`
- `sendAndStream`
- `/new` branch inside `runInteractive`

- [ ] **Step 4: Build CLI**

Run: `go build -o bin/zero ./apps/cli`

Expected: PASS.

- [ ] **Step 5: Smoke CLI help**

Run: `./bin/zero --help`

Expected: help lists existing commands; no runtime panic.

---

### Task 3: Add daemon lifecycle commands

**Files:**
- Modify: `apps/cli/main.go`

- [ ] **Step 1: Add daemon path helpers**

Implement:

```go
func zeroDir() string { return filepath.Join(os.Getenv("HOME"), ".zero") }
func pidPath() string { return filepath.Join(zeroDir(), "zero.pid") }
func logPath() string { return filepath.Join(zeroDir(), "zero.log") }
func ensureZeroDir() error { return os.MkdirAll(zeroDir(), 0o700) }
```

- [ ] **Step 2: Add process helpers**

Implement:
- `readPID() (int, error)`
- `processRunning(pid int) bool` using `os.FindProcess` + signal 0 on Unix.
- `serverHealthy(ctx)` using SDK `Health`.
- `removeStalePID()` if PID does not run.

- [ ] **Step 3: Add `setup` command**

Behavior:
- create `~/.zero` with `0700`.
- create `~/.zero/config.json` if absent with `{}`.
- print paths.
- do not start long-lived server.

- [ ] **Step 4: Add `start` command**

Behavior:
- if PID exists and running, print already running.
- if stale PID, remove it.
- start current binary with hidden `serve-daemon` command.
- redirect stdout/stderr to `~/.zero/zero.log`.
- write child PID to `~/.zero/zero.pid`.
- wait until `/health` passes or timeout after 5s.

- [ ] **Step 5: Add hidden `serve-daemon` command**

Behavior:
- call `server.Start(server.DefaultConfig())`.
- no daemonization inside this command; parent `start` creates process.

- [ ] **Step 6: Add `stop`, `restart`, `status`, `logs` commands**

Behavior:
- `stop`: SIGTERM PID, wait briefly, remove pidfile.
- `restart`: call stop logic then start logic.
- `status`: print running/stopped, PID, health result.
- `logs`: print log file by default; support `-f/--follow` optional only if simple.

- [ ] **Step 7: Build and smoke lifecycle**

Run:
```bash
go build -o bin/zero ./apps/cli
./bin/zero setup
./bin/zero start
./bin/zero status
./bin/zero stop
```

Expected: setup creates files, start reaches healthy, status shows running, stop terminates.

---

### Task 4: Docs and root scripts

**Files:**
- Modify: `README.md`
- Modify: `Makefile`
- Modify: `.opencode-context.md`

- [ ] **Step 1: Update Makefile**

Include SDK tests:

```make
test:
	go test ./packages/sdk-go/...
	go test ./services/core/...
	go test ./apps/cli/...
```

Keep `build` producing `bin/zero` and `bin/zero-server`.

- [ ] **Step 2: Update README terminal-first docs**

Document:
- `make build`
- `zero setup`
- `zero start`
- `zero status`
- `zero`
- `zero -p "..."`
- `zero stop`
- SDK-first architecture: Terminal CLI → Go SDK → HTTP/SSE core.

Remove wording that presents Next.js as primary.

- [ ] **Step 3: Update project context**

Mark current status completed and add:
- Go SDK created at `packages/sdk-go`.
- lifecycle commands added.
- CLI no longer owns raw REST transport for supported commands.

---

### Task 5: Final verification

**Files:**
- All modified files.

- [ ] **Step 1: Format Go**

Run: `gofmt -w apps/cli/main.go packages/sdk-go/*.go`

- [ ] **Step 2: Run tests**

Run:
```bash
go test ./packages/sdk-go/...
go test ./services/core/...
go test ./apps/cli/...
```

Expected: all PASS.

- [ ] **Step 3: Build**

Run: `go build -o bin/zero ./apps/cli`

Expected: PASS.

- [ ] **Step 4: Lifecycle smoke**

Run:
```bash
./bin/zero setup
./bin/zero start
./bin/zero status
./bin/zero stop
```

Expected: all commands exit 0; no orphan server process remains.

---

## Self-Review

- Spec coverage: covers approved approach #2: SDK-first layer, lifecycle commands, terminal-first docs, no web deletion in this step.
- Placeholder scan: no TBD/TODO/fill-in placeholders.
- Type consistency: SDK methods named once and referenced consistently by CLI refactor tasks.
