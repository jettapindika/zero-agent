# Tauri Desktop Shell Design

## Goal
- Build a ready-to-use Tauri desktop app for Zero that launches or detects the local `zero-server`, shows server/provider health, and provides a desktop chat surface backed by the existing HTTP/SSE API.

## Chosen Approach
- Use option B: Integrated Desktop Shell.
- Add a new `apps/desktop` package rather than converting the existing Next.js `apps/web` package.
- Keep the Go CLI/TUI and Go server unchanged as the product backbone.
- Bundle `zero-server` as a Tauri sidecar for packaged builds, while allowing dev mode to use `bin/zero-server` or an already-running server.

## Architecture
- Frontend: Vite + React + TypeScript in `apps/desktop`.
- Native shell: Tauri 2 Rust app in `apps/desktop/src-tauri`.
- Backend: existing Go `zero-server` on `127.0.0.1:8910`.
- Provider: existing OpenAI-compatible router at `ZERO_ROUTER_BASE_URL`, defaulting to `http://127.0.0.1:20128/v1`.
- Desktop commands expose `server_status`, `start_server`, `stop_server`, and `provider_status` to the React UI.

## User Experience
- The desktop window opens to a status header and chat layout.
- Server status is explicit: running, stopped, or error.
- Provider status is explicit: connected or offline with a readable error such as connection refused.
- User can start or stop the bundled sidecar from the desktop UI.
- Chat/session APIs continue to use the same local HTTP/SSE server.

## Scope
- In scope: Tauri scaffold, sidecar commands, health/status UI, basic session creation, prompt sending, message rendering.
- Out of scope: Rust rewrite of server, mobile targets, auto-updater, code signing, provider setup wizard, replacing CLI/TUI.

## Verification
- `make test`
- `make build`
- `pnpm --filter @zero-agent/desktop typecheck`
- `pnpm --filter @zero-agent/desktop build`
- `pnpm --filter @zero-agent/desktop tauri:build` when platform prerequisites and sidecar naming are available.
