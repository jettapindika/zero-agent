# Tauri Desktop Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a ready-to-use Tauri desktop app that manages the Zero Go server sidecar and exposes a desktop chat/status UI.

**Architecture:** Add `apps/desktop` as a Vite React + Tauri 2 app. Rust commands manage server/provider status and sidecar lifecycle; the frontend calls the existing Zero HTTP/SSE API at `127.0.0.1:8910`.

**Tech Stack:** Tauri 2, Rust, Vite, React 19, TypeScript, existing Go `zero-server`.

---

### Task 1: Desktop Package Scaffold

**Files:**
- Modify: `pnpm-workspace.yaml`
- Modify: `package.json`
- Create: `apps/desktop/package.json`
- Create: `apps/desktop/tsconfig.json`
- Create: `apps/desktop/vite.config.ts`
- Create: `apps/desktop/index.html`

- [ ] Add `apps/desktop` to the pnpm workspace.
- [ ] Add root scripts for desktop dev/build.
- [ ] Add a Vite React package with TypeScript.

### Task 2: Tauri Rust Shell

**Files:**
- Create: `apps/desktop/src-tauri/Cargo.toml`
- Create: `apps/desktop/src-tauri/build.rs`
- Create: `apps/desktop/src-tauri/tauri.conf.json`
- Create: `apps/desktop/src-tauri/capabilities/default.json`
- Create: `apps/desktop/src-tauri/src/main.rs`

- [ ] Configure Tauri 2 window, sidecar permissions, and shell plugin.
- [ ] Add `server_status`, `start_server`, `stop_server`, and `provider_status` commands.
- [ ] Use HTTP health checks for server/provider diagnostics.

### Task 3: Desktop Frontend

**Files:**
- Create: `apps/desktop/src/main.tsx`
- Create: `apps/desktop/src/App.tsx`
- Create: `apps/desktop/src/styles.css`
- Create: `apps/desktop/src/tauri.ts`
- Create: `apps/desktop/src/zero-api.ts`

- [ ] Show server and provider status cards.
- [ ] Add start/stop/refresh controls.
- [ ] Add basic session creation, prompt send, and message rendering using the existing Zero API.

### Task 4: Build Wiring

**Files:**
- Modify: `Makefile`
- Modify: `apps/desktop/package.json`

- [ ] Ensure `make build` creates `bin/zero-server` before desktop packaging.
- [ ] Add a desktop sidecar preparation script that copies `bin/zero-server` into Tauri's expected external binary location for the current platform.

### Task 5: Verification

**Commands:**
- [ ] Run `make test` and expect all Go tests to pass.
- [ ] Run `make build` and expect `bin/zero` plus `bin/zero-server`.
- [ ] Run `pnpm install` if dependencies are missing.
- [ ] Run `pnpm --filter @zero-agent/desktop typecheck`.
- [ ] Run `pnpm --filter @zero-agent/desktop build`.
- [ ] Run `pnpm --filter @zero-agent/desktop tauri:build` if Tauri platform prerequisites are installed.
