# Folder Walk and Readable Chat Design

## Goal
- Make Zero better at understanding a project folder before acting.
- Make terminal chat replies easier to scan while preserving the current Bubble Tea TUI.

## Context
- Zero is terminal-first: CLI/TUI → Go SDK → local core → providers/tools/storage.
- Built-in agents live in `services/core/internal/agent/agent.go`.
- Safe tools live in `services/core/internal/tool/safe_tools.go` and are scoped by `scopedPath()` in `path.go`.
- TUI rendering lives in `apps/cli/tui/model.go` and styling in `apps/cli/tui/view.go`.

## Design
- Add a safe `walk` tool for bounded recursive folder maps.
- Add `walk` to built-in agent tool lists so `build`, `plan`, and `explore` can inspect folder structure before deeper reads.
- Update built-in agent prompts to prefer `walk` for repo/module overview, then targeted `grep` and `read`.
- Refactor TUI chat rendering into small helpers for message cards, tool cards, text normalization, and wrapping.
- Keep implementation local to existing Go packages; no new runtime dependencies.

## Walk Tool
- File: `services/core/internal/tool/safe_tools.go`.
- Function: `Walk() Tool` returning `walkTool{}`.
- Args:
  - `path`: string, default `"."`.
  - `maxDepth`: integer, default `3`.
  - `maxFiles`: integer, default `200`.
  - `includeHidden`: boolean, default `false`.
- Behavior:
  - Resolve path through `scopedPath()`.
  - Traverse using `filepath.WalkDir`.
  - Skip `.git`, `node_modules`, `dist`, `build`, `coverage`, `.next`, `tmp`, `vendor` unless directly requested and inside scope.
  - Hide dotfiles/hidden dirs unless `includeHidden` is true.
  - Stop after `maxFiles` entries.
  - Output a readable tree with `/` suffix for directories and a summary line.

## Agent Integration
- Add `walk` to `ToolExecutor.toolNames()` in `services/core/internal/agent/executor.go`.
- Add `walk` to `AllowedTools` for `build`, `plan`, and `explore` in `services/core/internal/agent/agent.go`.
- Prompt update:
  - `explore`: start with `walk` when asked about a folder or broad codebase area.
  - `plan`: use `walk` for module maps before planning changes.
  - `build`: inspect relevant folder structure before multi-file edits.

## Chat Readability
- File: `apps/cli/tui/model.go`.
- Replace the single `renderMessages()` body with helper functions:
  - `renderMessageCard(msg Message, width int) string`
  - `renderToolCard(tool ToolCall, width int) string`
  - `formatMessageText(text string, width int) string`
  - `wrapLine(line string, width int) string`
- Output style:
  - Headers: `You`, `Zero`, `Zero ●`, `System`, `Error`.
  - Indent message body by two spaces.
  - Preserve fenced code blocks.
  - Normalize excessive blank lines.
  - Wrap prose to viewport width while leaving code lines unchanged.
  - Tool cards show args and a trimmed result preview.
- File: `apps/cli/tui/view.go`.
  - Add role styles for user, assistant, system, error, and tool labels.
  - Keep existing responsive layout.

## Tests
- `services/core/internal/tool/tools_test.go`:
  - `walk` lists nested files.
  - honors `maxDepth`.
  - honors `maxFiles`.
  - skips ignored directories.
  - rejects paths escaping project root through existing scope checks.
- `services/core/internal/agent/agent_test.go`:
  - built-in agents include `walk`.
- `services/core/internal/agent/executor_test.go`:
  - executor exposes `walk` schema.
- `apps/cli/tui/model_test.go`:
  - readable labels remain visible.
  - assistant text wraps.
  - fenced code is preserved.
  - tool result preview is readable.

## Non-Goals
- No background autonomous subagent scheduler.
- No new web UI work.
- No unsafe traversal outside project root.
- No markdown renderer dependency.

## Verification
- Run `go test ./...` from `services/core`.
- Run `go test ./...` from `apps/cli`.
- Run `make build` from repo root.
