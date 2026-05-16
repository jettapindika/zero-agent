# OpenCode-Like TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Zero’s plain scanner prompt with an OpenCode-like interactive terminal UI MVP while keeping `zero -p` unchanged.

**Architecture:** Add Bubble Tea, Lip Gloss, and Bubbles to `apps/cli`. Move interactive UI into `apps/cli/tui` with a small SDK-backed runner interface so the TUI can be tested without network. Keep non-interactive and lifecycle commands in `main.go`.

**Tech Stack:** Go 1.26.3, Cobra, Bubble Tea, Lip Gloss, Bubbles viewport/textarea/spinner/list, existing `packages/sdk-go`.

---

## File Structure

- Create `apps/cli/tui/model.go`: Bubble Tea model, state, messages, key handling.
- Create `apps/cli/tui/view.go`: header/sidebar/chat/input/footer rendering.
- Create `apps/cli/tui/runner.go`: SDK-backed prompt runner interface and adapter.
- Create `apps/cli/tui/model_test.go`: tests for send/new/quit state behavior.
- Modify `apps/cli/main.go`: replace `runInteractive()` body with `tui.Run(...)`; keep `runNonInteractive()` unchanged.
- Modify `apps/cli/go.mod`: add Charm deps.
- Modify `README.md`: document interactive TUI shortcuts.

---

### Task 1: Add testable TUI model

**Files:**
- Create: `apps/cli/tui/model_test.go`
- Create: `apps/cli/tui/model.go`

- [ ] **Step 1: Write failing tests**

Tests must cover:
- initial model has session ID, welcome assistant message, focused textarea
- `ctrl+n` calls `Runner.NewSession` and appends system message
- `enter` with text appends user + assistant messages
- `ctrl+c` quits

- [ ] **Step 2: Run red test**

Run: `go test ./apps/cli/tui/...`

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement minimal model**

Create Bubble Tea model using:
- `textarea.Model` for prompt input
- `viewport.Model` for chat transcript
- `spinner.Model` for status
- simple `Message{Role, Text}` slice
- `Runner` interface with `SendPrompt(ctx, sessionID, prompt)` and `NewSession(ctx)`

- [ ] **Step 4: Run green test**

Run: `go test ./apps/cli/tui/...`

Expected: PASS.

---

### Task 2: Render OpenCode-like layout

**Files:**
- Create: `apps/cli/tui/view.go`
- Modify: `apps/cli/tui/model.go`

- [ ] **Step 1: Add render tests**

Test `View()` contains:
- `Zero`
- `Sessions`
- `Chat`
- `ctrl+p palette`
- current session short ID

- [ ] **Step 2: Run red test**

Run: `go test ./apps/cli/tui/...`

Expected: FAIL until layout is implemented.

- [ ] **Step 3: Implement layout**

Use Lip Gloss boxes:

```txt
┌ Zero │ model │ status ┐
├ Sessions ┬ Chat ┤
│ sidebar  │ msgs │
├ prompt textarea ┤
└ shortcuts footer ┘
```

Responsive rules:
- width < 100: hide sidebar
- width >= 100: show sidebar

- [ ] **Step 4: Run green test**

Run: `go test ./apps/cli/tui/...`

Expected: PASS.

---

### Task 3: Wire SDK runner and CLI entry

**Files:**
- Create: `apps/cli/tui/runner.go`
- Modify: `apps/cli/main.go`

- [ ] **Step 1: Add runner adapter**

Implement adapter that wraps existing helpers:
- initial session from `ensureSession()`
- prompt send via `sendAndStream(sessionID, prompt)` for MVP
- new session via a shared `createTerminalSession()` helper

- [ ] **Step 2: Replace `runInteractive()`**

`runInteractive()` should:
- start embedded server
- ensure session
- call `tui.Run(tui.Config{SessionID, Model, Runner})`

- [ ] **Step 3: Build**

Run: `go build -o bin/zero ./apps/cli`

Expected: PASS.

---

### Task 4: Docs and verification

**Files:**
- Modify: `README.md`
- Modify: `.opencode-context.md`

- [ ] **Step 1: Document shortcuts**

Add:
- `enter`: send
- `ctrl+j`: newline
- `ctrl+n`: new session
- `ctrl+c`: quit
- sidebar responsive behavior

- [ ] **Step 2: Run full checks**

Run:
```bash
go test ./packages/sdk-go/...
go test ./services/core/...
go test ./apps/cli/...
go build -o bin/zero ./apps/cli
```

Expected: all pass.

- [ ] **Step 3: Smoke help/non-interactive**

Run:
```bash
./bin/zero --help
./bin/zero -p "Say exactly: ok"
```

Expected: help prints commands; prompt returns `ok`.

---

## Self-Review

- Spec coverage: OpenCode-like phased MVP, Bubble Tea shell, shortcuts, SDK-backed prompt runner, non-interactive preserved.
- Placeholder scan: no placeholders.
- Type consistency: `Runner`, `Message`, `Config`, and `Run` are named consistently across tasks.
