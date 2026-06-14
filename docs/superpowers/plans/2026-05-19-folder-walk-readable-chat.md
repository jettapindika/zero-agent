# Folder Walk and Readable Chat Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a safe bounded folder-walk capability to Zero agents and improve terminal chat readability.

**Architecture:** Implement `walk` as a safe project-scoped tool beside existing `ls`, `glob`, and `grep`. Wire it into built-in agents and the executor schema, then refactor TUI rendering helpers without changing session/provider flow.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, existing Zero core tool/agent architecture.

---

## File Structure

- Modify: `services/core/internal/tool/safe_tools.go` — add `walkTool` implementation.
- Modify: `services/core/internal/agent/executor.go` — expose `walk` through tool schemas.
- Modify: `services/core/internal/agent/agent.go` — add `walk` to built-in agents and prompt guidance.
- Modify: `services/core/internal/tool/tools_test.go` — add walk tests.
- Modify: `services/core/internal/agent/agent_test.go` — assert built-in agents include `walk`.
- Modify: `services/core/internal/agent/executor_test.go` — assert executor schema includes `walk`.
- Modify: `apps/cli/tui/model.go` — refactor chat rendering helpers and width-aware formatting.
- Modify: `apps/cli/tui/view.go` — add role label styles.
- Modify: `apps/cli/tui/model_test.go` — add readability tests.

### Task 1: Add Failing Core Tool Tests

**Files:**
- Modify: `services/core/internal/tool/tools_test.go`

- [ ] **Step 1: Add walk tests**

Append tests that create a temp project tree, execute `Walk()`, and assert nested output, depth limiting, file limiting, ignored dirs, and path scope errors.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/tool -run Walk -v` from `services/core`.
Expected: FAIL because `Walk` is undefined.

### Task 2: Implement Walk Tool

**Files:**
- Modify: `services/core/internal/tool/safe_tools.go`

- [ ] **Step 1: Add `walkTool`**

Implement `Walk() Tool`, `Name() == "walk"`, description, JSON schema, defaults, ignored dirs, hidden filtering, bounded traversal, tree output, and summary.

- [ ] **Step 2: Run walk tests**

Run: `go test ./internal/tool -run Walk -v` from `services/core`.
Expected: PASS.

### Task 3: Wire Walk Into Agents and Executor

**Files:**
- Modify: `services/core/internal/agent/executor.go`
- Modify: `services/core/internal/agent/agent.go`
- Modify: `services/core/internal/agent/agent_test.go`
- Modify: `services/core/internal/agent/executor_test.go`

- [ ] **Step 1: Add failing tests**

Add assertions that `DefaultAgents()` include `walk` in `build`, `plan`, `explore`, and that `ToolExecutor.ToolSchemas()` exposes `walk`.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/agent -run 'Agent|Executor|ToolSchema' -v` from `services/core`.
Expected: FAIL because `walk` is not wired.

- [ ] **Step 3: Wire implementation**

Add `walk` to `toolNames()` and each built-in agent `AllowedTools`. Update system prompts with concise walk-first guidance.

- [ ] **Step 4: Run agent tests**

Run: `go test ./internal/agent -run 'Agent|Executor|ToolSchema' -v` from `services/core`.
Expected: PASS.

### Task 4: Add Failing TUI Readability Tests

**Files:**
- Modify: `apps/cli/tui/model_test.go`

- [ ] **Step 1: Add tests**

Add tests for readable role labels, prose wrapping, fenced code preservation, blank-line normalization, and tool preview trimming.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./tui -run 'Readable|Wrap|Code|Tool' -v` from `apps/cli`.
Expected: FAIL for new formatting expectations.

### Task 5: Refactor TUI Renderer

**Files:**
- Modify: `apps/cli/tui/model.go`
- Modify: `apps/cli/tui/view.go`

- [ ] **Step 1: Add helper functions**

Replace monolithic message formatting with `renderMessageCard`, `renderToolCard`, `formatMessageText`, and `wrapLine`.

- [ ] **Step 2: Preserve code fences**

Make `formatMessageText` track lines starting with triple backticks and skip wrapping until the closing fence.

- [ ] **Step 3: Trim tool results**

Show full tool name/status, args when present, and result preview capped to a small number of lines.

- [ ] **Step 4: Add role styles**

Add user, assistant, system, error, and tool label styles in `view.go`.

- [ ] **Step 5: Run TUI tests**

Run: `go test ./tui -run 'Readable|Wrap|Code|Tool|Initial|Enter|Running' -v` from `apps/cli`.
Expected: PASS.

### Task 6: Full Verification

**Files:**
- All modified files above.

- [ ] **Step 1: Core tests**

Run: `go test ./...` from `services/core`.
Expected: PASS.

- [ ] **Step 2: CLI tests**

Run: `go test ./...` from `apps/cli`.
Expected: PASS.

- [ ] **Step 3: Build**

Run: `make build` from repo root.
Expected: PASS and binaries under `bin/`.

- [ ] **Step 4: Update context**

Update `.opencode-context.md` with the new `walk` tool and TUI renderer convention if implementation succeeds.
