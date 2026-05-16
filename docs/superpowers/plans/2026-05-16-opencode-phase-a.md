# OpenCode Phase A Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Zero feel like OpenCode at the core interaction level: streaming assistant output, visible tool calls, and permission prompts in the terminal.

**Architecture:** Keep the local HTTP/SSE core. Extend provider streaming to publish deltas as they arrive, expose SDK event subscription, and make the TUI subscribe to events while a run is active. Add a minimal structured tool-call path in the agent loop and surface tool/permission events in the TUI; do not attempt full OpenCode parity beyond Phase A.

**Tech Stack:** Go 1.26.3, chi, SQLite, SSE, Bubble Tea, Lip Gloss, Bubbles, existing `packages/sdk-go`.

---

## File Structure

- Modify `services/core/internal/provider/provider.go`: add provider tool-call event types if missing.
- Modify `services/core/internal/provider/openai.go`: parse streaming text and later tool call chunks safely.
- Modify `services/core/internal/agent/runner.go`: publish `part.delta`, `tool.started`, `tool.completed`, `permission.required`, `permission.resolved`; persist assistant text incrementally enough for TUI.
- Modify `services/core/internal/tool/tool.go`, `safe_tools.go`, `dangerous_tools.go`: expose schemas and structured events consistently.
- Modify `services/core/internal/permission/manager.go`: provide blocking wait/resolve API or deterministic denial path for Phase A.
- Modify `services/core/pkg/server/session_handlers.go`: run sessions asynchronously and return run IDs; keep existing blocking behavior only if necessary for `zero -p`.
- Create `packages/sdk-go/events.go`: typed SSE subscription API.
- Modify `packages/sdk-go/sessions.go`: add async run result/run ID if server changes.
- Modify `apps/cli/tui/model.go`: subscribe to events, render live assistant deltas and tool cards.
- Modify `apps/cli/tui/view.go`: readable tool/permission panels.
- Modify `apps/cli/main.go`: wire event runner into TUI; keep `zero -p` stable.
- Add tests in `services/core/internal/agent/runner_test.go`, `packages/sdk-go/events_test.go`, and `apps/cli/tui/model_test.go`.

---

### Task 1: SDK SSE events

**Files:**
- Create: `packages/sdk-go/events.go`
- Create: `packages/sdk-go/events_test.go`

- [ ] **Step 1: Write failing SSE decoder tests**

Test with `httptest.Server` that emits:

```txt
event: part.delta
data: {"id":"e1","type":"part.delta","sessionId":"s1","payload":{"delta":"hi"},"createdAt":1}

event: tool.started
data: {"id":"e2","type":"tool.started","sessionId":"s1","payload":{"name":"read"},"createdAt":2}
```

Expected API:

```go
events, cancel, err := client.SubscribeSession(ctx, "s1")
event := <-events
if event.Type != "part.delta" { t.Fatal(...) }
cancel()
```

- [ ] **Step 2: Run red test**

Run: `go test ./packages/sdk-go/... -run TestSubscribeSession`

Expected: FAIL because `SubscribeSession` does not exist.

- [ ] **Step 3: Implement `Event` and `SubscribeSession`**

Create `packages/sdk-go/events.go`:

```go
type Event struct {
    ID string `json:"id"`
    Type string `json:"type"`
    ProjectID string `json:"projectId,omitempty"`
    SessionID string `json:"sessionId,omitempty"`
    RoomID string `json:"roomId,omitempty"`
    ActorID string `json:"actorId,omitempty"`
    Payload json.RawMessage `json:"payload"`
    CreatedAt int64 `json:"createdAt"`
}

func (c *Client) SubscribeSession(ctx context.Context, sessionID string) (<-chan Event, context.CancelFunc, error)
```

Implementation:
- GET `/events?sessionId=<sessionID>`
- scan SSE lines
- parse `data:` JSON into `Event`
- close channel on context cancel or scanner error

- [ ] **Step 4: Run green test**

Run: `go test ./packages/sdk-go/... -run TestSubscribeSession`

Expected: PASS.

---

### Task 2: Real streaming TUI path

**Files:**
- Modify: `apps/cli/tui/model.go`
- Modify: `apps/cli/tui/view.go`
- Modify: `apps/cli/tui/model_test.go`
- Modify: `apps/cli/main.go`

- [ ] **Step 1: Write failing TUI event tests**

Add tests for messages:

```go
type assistantDeltaMsg struct { Delta string }
type runDoneMsg struct{}
type runErrMsg struct { Err error }
```

Expected:
- while busy, `assistantDeltaMsg{Delta:"Hel"}` creates/updates active ZERO message
- second delta appends, producing `Hello`
- `runDoneMsg{}` clears busy
- `runErrMsg{Err: errors.New("boom")}` clears busy and shows ERROR

- [ ] **Step 2: Run red test**

Run: `go test ./apps/cli/tui/... -run TestAssistantDelta`

Expected: FAIL because event message types are missing.

- [ ] **Step 3: Implement TUI event messages**

In `model.go`:
- add `assistantDeltaMsg`, `toolStartedMsg`, `toolCompletedMsg`, `permissionRequiredMsg`, `runDoneMsg`, `runErrMsg`
- on `assistantDeltaMsg`, append to current assistant message instead of waiting for final response
- on `runDoneMsg`, clear busy
- on `runErrMsg`, clear busy and append error

- [ ] **Step 4: Add event-backed runner interface**

Keep existing `Runner.SendPrompt` for tests and `zero -p`, but add optional interface:

```go
type EventRunner interface {
    SendPromptEvents(ctx context.Context, sessionID string, prompt string, dispatch func(tea.Msg)) error
}
```

If runner implements it, TUI uses event path; otherwise fallback to current async final-response path.

- [ ] **Step 5: Wire CLI runner to SDK events**

In `main.go`, implement `SendPromptEvents`:
1. call SDK `SendMessage`
2. subscribe to session events
3. call SDK `RunSession`
4. dispatch `assistantDeltaMsg` on `part.delta`
5. dispatch tool/permission messages on matching events
6. dispatch `runDoneMsg` on accepted/completion or when run returns
7. dispatch `runErrMsg` on error

- [ ] **Step 6: Run TUI tests**

Run: `go test ./apps/cli/tui/...`

Expected: PASS.

---

### Task 3: Agent event correctness

**Files:**
- Modify: `services/core/internal/agent/runner.go`
- Modify: `services/core/internal/agent/runner_test.go`

- [ ] **Step 1: Write failing runner event test**

Use fake provider stream returning deltas `Hel`, `lo`.

Assert bus receives:
- `message.created`
- `part.delta` with `Hel`
- `part.delta` with `lo`
- `part.created` with final text `Hello`
- `session.status idle`

- [ ] **Step 2: Run red test**

Run: `go test ./services/core/internal/agent/... -run TestRunnerPublishesDeltas`

Expected: may fail if payload shape does not include enough info for TUI.

- [ ] **Step 3: Normalize event payloads**

Ensure `part.delta` payload is:

```json
{"messageId":"...","delta":"..."}
```

Ensure final `part.created` includes final text.

- [ ] **Step 4: Run green test**

Run: `go test ./services/core/internal/agent/... -run TestRunnerPublishesDeltas`

Expected: PASS.

---

### Task 4: Tool calls visible in TUI

**Files:**
- Modify: `services/core/internal/agent/runner.go`
- Modify: `apps/cli/tui/model.go`
- Modify: `apps/cli/tui/view.go`
- Modify: tests in both packages

- [ ] **Step 1: Write failing TUI tool-card tests**

Send `toolStartedMsg{Name:"read", Args:"{...}"}` and `toolCompletedMsg{Name:"read", Result:"ok"}`.

Assert `View()` contains:
- `TOOL read`
- `running` before completion
- `ok` after completion

- [ ] **Step 2: Implement TUI tool cards**

Add `ToolCall` state:

```go
type ToolCall struct { Name string; Args string; Result string; Status string }
```

Render in chat as compact cards:

```txt
TOOL read  ● running
args: {...}
```

- [ ] **Step 3: Publish tool events in agent loop**

Where tool execution exists, publish:
- `tool.started`
- `tool.completed`
- `tool.failed`

If the current agent loop cannot call tools yet, add event plumbing and tests around the tool executor boundary only; do not fake full tool-calling semantics.

- [ ] **Step 4: Run tests**

Run:
```bash
go test ./apps/cli/tui/...
go test ./services/core/internal/agent/...
```

Expected: PASS.

---

### Task 5: Permission prompts in TUI

**Files:**
- Modify: `services/core/internal/permission/manager.go`
- Modify: `services/core/pkg/server/session_handlers.go`
- Modify: `packages/sdk-go/permissions.go` or create it
- Modify: `apps/cli/tui/model.go`, `view.go`, tests

- [ ] **Step 1: Write failing permission UI test**

Feed `permissionRequiredMsg{ID:"p1", Tool:"bash", Summary:"go test ./..."}`.

Assert view contains:
- `PERMISSION REQUIRED`
- `bash`
- `a allow once`
- `d deny`

- [ ] **Step 2: Implement permission state + key handling**

When permission prompt is active:
- `a` resolves allow once
- `d` resolves deny
- `esc` denies/closes

- [ ] **Step 3: Add SDK resolve method**

Expose:

```go
ResolvePermission(ctx, sessionID, permissionID string, input ResolvePermissionInput) error
```

- [ ] **Step 4: Wire TUI permission resolution**

CLI runner gets SDK method and calls resolve endpoint from TUI key handling.

- [ ] **Step 5: Run permission tests**

Run:
```bash
go test ./packages/sdk-go/...
go test ./apps/cli/tui/...
```

Expected: PASS.

---

### Task 6: Final verification

**Files:** all touched files.

- [ ] **Step 1: Full Go tests**

Run:

```bash
go test ./packages/sdk-go/...
go test ./services/core/...
go test ./apps/cli/...
```

Expected: all PASS.

- [ ] **Step 2: Build**

Run:

```bash
go build -o bin/zero ./apps/cli
```

Expected: PASS.

- [ ] **Step 3: Prompt smoke**

Run:

```bash
./bin/zero restart
./bin/zero -p "Say exactly: ok"
```

Expected: prints `ok`.

- [ ] **Step 4: TUI manual check**

Run in a real terminal:

```bash
./bin/zero
```

Expected:
- user message appears immediately
- assistant deltas appear as they stream
- tool cards show when tools run
- permission dialog appears for dangerous tools
- timeout/error clears running state

---

## Self-Review

- Spec coverage: covers Phase A only: streaming, tool visibility, permission UI. Does not attempt agents/subagents/model auth/custom commands.
- Placeholder scan: no TBD/TODO placeholders.
- Type consistency: event names and message names are consistent across SDK/TUI/server tasks.
