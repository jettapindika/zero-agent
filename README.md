<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="apps/desktop/src-tauri/icons/128x128.png">
  <img src="apps/desktop/src-tauri/icons/128x128.png" width="96" alt="Zero">
</picture>

# `Zero`

### A local-first, bring-your-own-model AI coding agent

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-v0.1.0-orange)](#)
[![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)](#)

**Desktop app + thin CLI. Any OpenAI-compatible model. Local SQLite. No cloud lock-in.**

```
┌────────────────────┐     ┌────────────────┐     ┌──────────────────────┐
│  Zero.app (Tauri)  │     │  zero CLI      │     │  zero:// deep-link   │
│  line REPL         │     │  one-shot mode │     │  invite URLs         │
└─────────┬──────────┘     └───────┬────────┘     └──────────┬───────────┘
          │                        │                          │
          ▼                        ▼                          ▼
          ┌─────────────────────────────────────────────────────┐
          │            Go SDK (packages/sdk-go)                 │
          └─────────────────────────┬───────────────────────────┘
                                    ▼
          ┌─────────────────────────────────────────────────────┐
          │   zero-server  ·  HTTP + SSE daemon on :8910        │
          │   auth · sessions · collab rooms · tool approvals   │
          │   attachments · agent runner · file tools · chat    │
          └─────────────────────────┬───────────────────────────┘
                                    ▼
          ┌─────────────────────────────────────────────────────┐
          │         your OpenAI-compatible provider             │
          │   OpenAI · OpenRouter · LiteLLM · Ollama · vLLM ·   │
          │   llama.cpp · LM Studio · 9router · any /v1 endpoint│
          └─────────────────────────────────────────────────────┘
```

[Install](#installation) · [Quick start](#quick-start) · [CLI reference](#cli-reference) · [Bring your own model](#bring-your-own-model) · [Collaboration](#collaboration--rooms) · [Multi-user](#multi-user-mode-google-sign-in)

</div>

---

## ✨ What you get

| Area | What Zero does |
|------|----------------|
| **Desktop UI** | Tauri 2 app with chat, model picker, agent picker, tool-approval cards, files sidebar, task panel, activity feed, dev tab |
| **CLI** | Line REPL + one-shot `-p` mode + 17 lifecycle commands (`setup`, `start`, `share`, `join`, …) |
| **Model support** | Any OpenAI-compatible `/v1` endpoint — model list live-fetched from your provider |
| **Agent tools** | `read`, `write`, `edit`, `bash`, `grep`, `glob`, `ls`, `fetch`, `attach_read` — dangerous ones (`bash`, `write`, `edit`, `fetch`) need in-chat approval |
| **Collaboration** | `zero://join/...` deep links, multi-user chat, prompt-review queue, role-based permissions |
| **Attachments** | Drag-drop files onto the composer — PDFs, images, DOCX, XLSX, text all auto-read |
| **Storage** | Local SQLite at `~/.zero/zero.db` — your data never leaves your machine |
| **Auth** | Off by default. Toggle Google sign-in for multi-user — no allowlist, anyone with a Google account can sign in |
| **Cross-platform** | macOS, Windows (NSIS), Linux (deb + AppImage) bundles, `zero://` URL scheme on all three |

---

## Installation

### Prerequisites (build-from-source)

| Tool | Where to get it | Used by |
|------|-----------------|---------|
| **Go ≥ 1.22** | [go.dev](https://go.dev/dl/) | daemon, CLI, SDK |
| **Node ≥ 20 + pnpm ≥ 9** | [nodejs.org](https://nodejs.org/) / `npm i -g pnpm` | desktop frontend |
| **Rust stable** | [rustup](https://rustup.rs/) | Tauri shell |
| **System webkit** | bundled on macOS; `libwebkit2gtk-4.1-dev` on Ubuntu; WebView2 on Windows | Tauri runtime |
| *(Windows only)* **NSIS** | `brew install nsis` / `apt install nsis` | cross-compile `.exe` installer |
| *(Windows only)* **cargo-xwin** | `cargo install --locked cargo-xwin` | cross-compile Rust → MSVC |
| *(Windows only)* Rust target | `rustup target add x86_64-pc-windows-msvc` | Windows build |
| *(optional)* **ImageMagick** | `brew install imagemagick` / `apt install imagemagick` | regenerate icons |

> 💡 **Not building from source?** Skip to the [pre-built installers](#pre-built-installers) section below.

---

### 🍎 macOS

```bash
# 1. Clone
git clone <this-repo>
cd zero-agent

# 2. Build Go binaries + desktop bundle
make build              # produces bin/zero + bin/zero-server + bin/library
make desktop-build      # produces Zero.app + Zero_0.1.0_aarch64.dmg

# 3. Install (CLI → ~/.local/bin, app → ~/Applications)
make install
make install-app

# 4. Configure + launch
zero setup                                    # writes ~/.config/zero/.env
# edit ~/.config/zero/.env, set ZERO_ROUTER_API_KEY
zero start                                    # background daemon on :8910
zero .                                        # open desktop in current folder
```

**Bundle outputs:**
- `apps/desktop/src-tauri/target/release/bundle/macos/Zero.app`
- `apps/desktop/src-tauri/target/release/bundle/dmg/Zero_0.1.0_aarch64.dmg`

**Deep-link registration:** the `zero://` URL scheme is registered the first time `Zero.app` launches — no manual setup needed.

---

### 🪟 Windows

Two paths: **native build** (on a Windows box) or **cross-compile** (from macOS/Linux).

#### Native build (on Windows)

```powershell
git clone <this-repo>
cd zero-agent
make build                        # bin/zero + bin/zero-server
make desktop-build                # produces Zero_0.1.0_x64-setup.exe
# run the .exe installer — it registers the zero:// URL scheme automatically
zero setup                        # writes %USERPROFILE%\.config\zero\.env
# edit the .env, set ZERO_ROUTER_API_KEY
zero start
zero .
```

#### Cross-compile from macOS / Linux

```bash
# Install cross-tooling (one-time):
brew install nsis llvm            # or: apt install nsis llvm
cargo install --locked cargo-xwin
rustup target add x86_64-pc-windows-msvc

# Then from the repo root:
make build
make desktop-build-windows
```

**Output:** `apps/desktop/src-tauri/target/x86_64-pc-windows-msvc/release/bundle/nsis/Zero_0.1.0_x64-setup.exe`

**Deep-link registration:** the NSIS installer writes the `zero://` scheme into `HKCU\Software\Classes\zero`. On dev builds, `register_all()` also fires at startup so the scheme works without installing.

> 🔒 **First-run on Windows:** SmartScreen may warn because the `.exe` is unsigned. Click **More info → Run anyway**. See [Tauri's signing docs](https://tauri.app/distribute/sign/windows/) for production code-signing.

---

### 🐧 Linux

```bash
# Ubuntu / Debian prerequisites:
sudo apt install libwebkit2gtk-4.1-dev build-essential curl wget \
                 libssl-dev libgtk-3-dev libayatana-appindicator3-dev \
                 librsvg2-dev

git clone <this-repo>
cd zero-agent

make build
make desktop-build                  # produces .deb + .AppImage

# Install CLI to ~/.local/bin, then pick ONE bundle:
make install
sudo dpkg -i apps/desktop/src-tauri/target/release/bundle/deb/zero-agent_0.1.0_amd64.deb
# or run the AppImage:
chmod +x apps/desktop/src-tauri/target/release/bundle/appimage/zero-agent_0.1.0_amd64.AppImage
./apps/desktop/src-tauri/target/release/bundle/appimage/zero-agent_0.1.0_amd64.AppImage

zero setup
# edit ~/.config/zero/.env, set ZERO_ROUTER_API_KEY
zero start
zero .
```

**Bundle outputs:**
- `apps/desktop/src-tauri/target/release/bundle/deb/zero-agent_0.1.0_amd64.deb`
- `apps/desktop/src-tauri/target/release/bundle/appimage/zero-agent_0.1.0_amd64.AppImage`

**Deep-link registration:** the `.deb` / `.rpm` postinst writes a `.desktop` file with `MimeType=x-scheme-handler/zero`. AppImages need an AppImage launcher to integrate the scheme — otherwise call `register_all()` at runtime (the daemon does this on Linux automatically).

---

## Quick start

```bash
zero setup              # creates ~/.zero + ~/.config/zero/.env
# edit the .env: set ZERO_ROUTER_BASE_URL + ZERO_ROUTER_API_KEY
zero start              # daemon on :8910
zero .                  # open desktop app rooted at current folder
zero -p "hi"            # or a one-shot prompt in the terminal
```

That's it. The **only** env var you must set is `ZERO_ROUTER_API_KEY`.
Everything else has sensible defaults.

### Two deployment modes

| Mode | How | Who it's for |
|------|-----|--------------|
| **A. Single-user** (default) | Leave `ZERO_AUTH_ENABLED` unset | Personal use. No login screen, all data in local SQLite. |
| **B. Multi-user** | Set `ZERO_AUTH_ENABLED=true` + Google OAuth + `SESSION_SECRET` | Teams. Anyone with a Google account can sign in. |

See [Multi-user mode](#multi-user-mode-google-sign-in) for details.

---

## Bring your own model

Zero never talks to a model directly — it always goes through one configurable HTTP endpoint that speaks the OpenAI API. Set two env vars in `~/.config/zero/.env` (or in your shell):

```bash
ZERO_ROUTER_BASE_URL=https://api.openai.com/v1
ZERO_ROUTER_API_KEY=sk-...
```

The desktop **Model Picker** live-fetches `${ZERO_ROUTER_BASE_URL}/models` and shows whatever your provider returns — pick one and it sticks per session.

### Tested providers

| Provider | `ZERO_ROUTER_BASE_URL` | Notes |
|----------|------------------------|-------|
| **OpenAI** | `https://api.openai.com/v1` | Default. Use a real `sk-...` key. |
| **OpenRouter** | `https://openrouter.ai/api/v1` | Single key, hundreds of models. |
| **LiteLLM proxy** | `http://<host>:4000/v1` | Self-host, route to many backends. |
| **Ollama (local)** | `http://127.0.0.1:11434/v1` | API key may be any non-empty string. |
| **LM Studio (local)** | `http://127.0.0.1:1234/v1` | Start the LM Studio server first. |
| **llama.cpp `--server`** | `http://127.0.0.1:8080/v1` | Compatible OpenAI mode. |
| **vLLM (self-host)** | `http://<host>:8000/v1` | `--api-key` to set the key. |
| **9router (local proxy)** | `http://127.0.0.1:20128/v1` | `sk_9router` key; models like `cb/claude-opus-4.7-1m`. |

If the picker comes up empty, the `/models` endpoint is either unreachable or returns an empty list. The picker falls back to a curated set of well-known model ids (`gpt-4o`, `gpt-4o-mini`, `claude-3-5-sonnet-latest`, `llama3.1`, …) you can still pick.

### Pin a default model

```bash
ZERO_DEFAULT_MODEL=anthropic/claude-3-5-sonnet-latest
```

Without this, Zero falls back to `gpt-4o-mini`.

---

## CLI reference

```
zero [path]                 # no args → line REPL; with path → open desktop there
zero -p "prompt"            # one-shot prompt, prints answer
zero -c                     # continue the last session
zero -c --fork              # fork session when continuing
zero -m provider/model      # override default model
zero --agent <name>         # run with a specific agent (build, plan, …)
zero -s <session-id>        # continue a specific session
```

### Lifecycle commands

| Command | Description |
|---------|-------------|
| `zero setup` | Initialize `~/.zero/` + `~/.config/zero/.env` (starter template) |
| `zero start` | Start background daemon on `:8910` |
| `zero stop` | Stop daemon |
| `zero restart` | Restart daemon |
| `zero status` | Show PID + daemon health |
| `zero logs` | Tail `~/.zero/zero.log` |
| `zero server` | Start server in foreground (for debugging) |

### Session & agent commands

| Command | Description |
|---------|-------------|
| `zero sessions` | List sessions for current project |
| `zero export <session-id>` | Export session to Markdown |
| `zero models` | List models from your provider's `/models` endpoint |
| `zero agents` | List configured agents (build, plan, plus any custom) |
| `zero config` | Show current `~/.zero/config.json` |
| `zero auth` | Show provider config + where to set keys |

### Collaboration commands

| Command | Description |
|---------|-------------|
| `zero share` | Create a team session invite (prints `zero://` URL) |
| `zero join <invite>` | Join a team session (URL, or `--room` + `--token` flags) |
| `zero participants --room <id>` | List participants in a room |
| `zero queue --room <id> --session <id>` | List queued prompts awaiting review |

### Misc

| Command | Description |
|---------|-------------|
| `zero install-opencode` | Port `~/.config/opencode/` into `~/.config/zero/` (non-destructive) |
| `zero completion` | Generate shell completion script (bash/zsh/fish/powershell) |

### The line REPL

When invoked with no args, `zero` opens a simple line-based REPL. Type a prompt and press Enter; Ctrl+D exits. Special inputs:

| Input | Action |
|-------|--------|
| `/new` | Start a fresh session |
| `/quit`, `/exit` | Leave the REPL |

Everything else — chat history, tool approvals, slash commands, model switching — lives in the desktop app.

---

## Agent tools & permissions

The agent has eight built-in tools. **Safe tools run automatically; dangerous ones require your approval via an in-chat PermissionCard.**

| Tool | Auto-allowed? | What it does |
|------|:-------------:|--------------|
| `read` | ✅ | Read a file |
| `ls` | ✅ | List a directory |
| `glob` | ✅ | Find files by pattern |
| `grep` | ✅ | Content search via regex |
| `bash` | ❌ | Run a shell command (30s timeout, approval required) |
| `write` | ❌ | Create or overwrite a file |
| `edit` | ❌ | Targeted string replacement in a file |
| `fetch` | ❌ | HTTP fetch a URL |
| `attach_read` | ✅ | Read a previously-uploaded attachment (sandboxed to `~/.zero/uploads/`) |

Permissions are configured per-project in `~/.zero/config.json` under `tools`.

## Agent presets

Two agents ship out of the box:

| Agent | Default model | Purpose |
|-------|---------------|---------|
| `build` | `anthropic/claude-sonnet-4-5` | General coding — read + write + run |
| `plan` | `anthropic/claude-haiku-4-5` | Planning & reasoning, read-only |

Add your own (e.g. `explorer` for fast codebase reads) by editing `~/.zero/config.json`:

```json
{
  "agents": {
    "build":    { "model": "9router/cb/claude-opus-4.7-1m", "maxSteps": 100 },
    "plan":     { "model": "9router/cb/claude-haiku-4.5",   "maxSteps": 50 },
    "explorer": { "model": "9router/cb/claude-sonnet-4.6",  "maxSteps": 50 }
  }
}
```

Pick an agent with `zero --agent explorer` or from the desktop UI's agent picker.

---

## Collaboration & rooms

`zero share` prints a `zero://join/<roomId>?token=<token>` URL. Share it however (Slack, email, …) — clicking it opens the desktop app on **Windows, macOS, and Linux** with the Share folder modal pre-filled in **Join a room** mode. Click **Join** and you're in.

### Host-side flow

1. In the desktop, click **Share folder** → pick **Create a room**.
2. Walk through the 3-step wizard: folder scope → token mode → guest permissions.
3. You get an invite URL + a 12-character code (`XXXX-XXXX-XXXX`) in the CollabBar.
4. Share it. Guests join; the bar shows guest count live via SSE.

### Guest-side flow

1. Click the `zero://join/...` link the host sent you.
2. The desktop opens (or focuses if already running) in **Join a room** mode, room ID + token pre-filled.
3. Click **Join room** → Step 4 shows connection status → success card with your role.
4. The chat sidebar opens — you can now talk with the host and other guests.

### Room features

- **Role-based permissions** — `host`, `maintainer`, `prompter`, `viewer`.
- **Prompt-review queue** — when the host enables `requireApproval`, guest prompts land in a queue. Host can approve / reject / edit / cancel.
- **Live chat** — multi-user chat alongside the agent's work.
- **File sync** — guests see the host's folder contents in real-time (when `syncFiles` is on).
- **Host approval gate** — dangerous tool uses can be configured to require host sign-off.

### Single-instance guard

The desktop runs a single-instance guard via `tauri-plugin-single-instance`. On Windows + Linux, clicking a second `zero://` link doesn't spawn a new window — it focuses the existing one and delivers the URL.

---

## Attachments

Drag-drop files onto the composer. Supported kinds: `IMG`, `VID`, `AUD`, `PDF`, `DOC`, `XLS`, `PPT`, `TXT`, `FILE`. Files upload to `~/.zero/uploads/` and the agent auto-reads them via `attach_read` before responding.

- **Upload** happens in the background with a live status indicator (● uploading → ✓ done → ✗ error).
- **Send button** blocks while any chip is errored or still uploading.
- **System message** announces the attachment to the agent, which uses `attach_read` (sandboxed to `~/.zero/uploads/`) rather than the project-scoped `read` tool.

---

## The Library (agent skills)

Zero ships with a standalone knowledge library at `tools/library/`. It's a **per-machine learned-context store** that captures mistakes, insights, patterns, and fixes so future sessions don't repeat errors.

> ⚠️ **Honest status:** the library CLI + storage are complete and shipped (`make library` → `bin/library`). The runtime wiring that would inject library entries into the agent's system prompt is **not yet connected** — contributions welcome.

```bash
make library                                # bin/library
bin/library init                            # create library/ layout
bin/library add --type mistake --title "..." --body "..."
bin/library list --tag react --limit 5
bin/library show <id>
bin/library use <id> --helped               # mark as useful
bin/library inject                          # print markdown block for system prompt
bin/library auto-archive                    # archive low-scoring entries
bin/library reindex                         # rebuild index.json + LIBRARY.md
```

Entry types: `mistake` (highest priority, +0.20 score boost), `insight`, `pattern`, `convention`, `fix`, `user-correction`.

Storage lives in `library/` — contents are gitignored (they contain per-machine private context), only the directory layout + this README are versioned.

---

## Multi-user mode (Google sign-in)

Off by default. Turn it on when you want the daemon to gate `/sessions/*`, `/projects/*`, etc. behind a real identity. **Any Google account** can sign in — the daemon auto-creates a `users` row on first login. There is no allowlist.

### Setup

1. Create a Google Cloud project at <https://console.cloud.google.com> → Enable the People API.
2. *APIs & Services → Credentials* → **Create OAuth 2.0 → Web Application**.
   Set Authorized redirect URI to `http://127.0.0.1:8910/auth/google/callback`.
   (Loopback is exempt from Google's HTTPS requirement.)
3. Copy the credentials into `~/.config/zero/.env`:

   ```bash
   ZERO_AUTH_ENABLED=true
   GOOGLE_CLIENT_ID=your-id.apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=...
   GOOGLE_CALLBACK_URL=http://127.0.0.1:8910/auth/google/callback
   SESSION_SECRET=$(openssl rand -hex 32)
   DEV_EMAILS=you@example.com           # optional, comma-separated
   ```

4. Restart the daemon: `zero restart`.

### What changes when `ZERO_AUTH_ENABLED=true`

- Every non-public route returns `401` until the user signs in.
- The desktop window shows a centered **Sign in with Google** card.
- Click → system browser opens → consent → daemon `UpsertUser` (insert or update keyed on `google_id`) → `auth_sessions` row created → daemon sets an `httpOnly; SameSite=Lax` signed cookie → desktop re-fetches `/auth/me`.
- **Any Google account that completes the OAuth flow gets in.** The daemon doesn't check the email against an allowlist — first-time visitors are auto-onboarded with `role: "user"`.
- Accounts in `DEV_EMAILS` are upserted with `role: "dev"` and see the **Dev** tab in the side panel + can hit `/dev/runtime` and `/dev/skills/reload`.

### Public routes (always reachable without a cookie)

```
GET  /health
GET  /openapi.json
GET  /events
GET  /auth/google/start
GET  /auth/google/callback
GET  /auth/me
POST /auth/logout
```

### What if I want invite-only?

Out of the box, anyone with a Google account can sign in. If you need domain-restricted (`@yourcompany.com` only) or explicit-allowlist sign-in, add the check between `FetchUserInfo` and `UpsertUser` in `services/core/internal/auth/service.go::CompleteFlow`. Roughly:

```go
info, err := FetchUserInfo(ctx, s.httpClient, tok.AccessToken)
if err != nil { return ... }
if !strings.HasSuffix(info.Email, "@yourcompany.com") {
    return "", nil, claim, errors.New("email not in allowlist")
}
role := RoleFor(info.Email, s.devEmails)
upserted, err := s.store.UpsertUser(...)
```

Contributions welcome to make this a config-driven knob (e.g. `ZERO_AUTH_ALLOWED_DOMAINS=acme.com,example.org`) — see issue tracker.

### Shared community auth database

**By default, all Zero installs share a community Supabase database.** When you enable `ZERO_AUTH_ENABLED=true`, the daemon connects to a shared Postgres instance where `users` and `auth_sessions` live. This means:

- Your Google identity is recognized across all your machines (laptop, workstation, etc.)
- You don't need to set up your own Supabase project
- The DSN in `.env.example` uses a **restricted role** that can only read/write auth tables — no superuser access, safe to distribute

The community DSN is already configured in `.env.example`:

```bash
ZERO_SUPABASE_DB_URL=postgresql://zero_auth_user:zero_community_auth_2026@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres
```

Just enable auth and restart the daemon — you're done.

#### Want isolation? Bring your own Supabase

If you prefer your own private auth database (e.g., for a team deployment or to avoid sharing with the community), create a new Supabase project:

```bash
supabase login
cd services/core && supabase init
supabase link --project-ref <your-project-ref>
supabase db push

# In ~/.config/zero/.env:
ZERO_SUPABASE_DB_URL="postgresql://postgres:PASSWORD@db.<your-ref>.supabase.co:5432/postgres"
```

Restart the daemon. You should see `level=INFO msg="auth backend = supabase postgres"` in the logs.

> Chat data (projects, sessions, messages, parts) **never** syncs to Supabase — it stays in local SQLite by design.

---

## Building from source (all Makefile targets)

| Target | What it does |
|--------|--------------|
| `make build` | Build `bin/zero`, `bin/zero-server` |
| `make library` | Build `bin/library` (agent knowledge store CLI) |
| `make library-test` | Run library test suite |
| `make test` | `go test ./packages/sdk-go/... ./services/core/... ./apps/cli/... ./tools/library/...` |
| `make install` | Install `bin/zero` + `bin/zero-server` to `~/.local/bin` |
| `make desktop-dev` | Tauri dev server (HMR for frontend) |
| `make desktop-build` | Native installer for current host (dmg on macOS, exe on Windows, deb/AppImage on Linux) |
| `make desktop-build-windows` | Cross-compile Windows `.exe` from macOS/Linux (needs `nsis` + `cargo-xwin`) |
| `make desktop-icons` | Regenerate per-platform icons from `src-tauri/icons/source.svg` |
| `make install-app` | Copy `Zero.app` to `~/Applications` (macOS only) |
| `make install-all` | `make install` + `make install-app` |
| `make uninstall` | Remove CLI binaries + `Zero.app` |
| `make clean` | Remove `bin/`, `.next/`, `node_modules/.cache` |
| `make install-opencode-config` | Port `~/.config/opencode/*` into `~/.config/zero/*` |

### Layout

```
apps/cli           Go CLI (line REPL + lifecycle commands)
apps/desktop       Tauri desktop app (primary interface)
apps/web           legacy/optional web UI
packages/sdk-go    Go SDK used by CLI and desktop bridge
services/core      Go backend daemon (HTTP + SSE on :8910)
config/            config schema
tools/library      agent learned-context library (standalone Go module)
bin/               build outputs (zero, zero-server, library)
```

---

## Configuration reference

### Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `ZERO_ROUTER_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible provider URL |
| `ZERO_ROUTER_API_KEY` | *(required)* | Provider API key |
| `ZERO_DEFAULT_MODEL` | `gpt-4o-mini` | Default model for new sessions |
| `ZERO_AUTH_ENABLED` | `false` | Enable Google sign-in |
| `GOOGLE_CLIENT_ID` | | OAuth client id (when auth enabled) |
| `GOOGLE_CLIENT_SECRET` | | OAuth client secret (when auth enabled) |
| `GOOGLE_CALLBACK_URL` | `http://127.0.0.1:8910/auth/google/callback` | OAuth redirect URI |
| `SESSION_SECRET` | *(required when auth enabled)* | 32+ byte HMAC key for cookie signing |
| `DEV_EMAILS` | | Comma-separated list of emails with `role: "dev"` |
| `ZERO_SUPABASE_DB_URL` | `postgresql://zero_auth_user:...@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres` | Shared community auth database (restricted role, safe to distribute). Override with your own Supabase DSN for isolation. |

### User config (`~/.zero/config.json`)

```json
{
  "model": "anthropic/claude-sonnet-4-5",
  "theme": "tokyonight",
  "agents": {
    "build":    { "model": "anthropic/claude-sonnet-4-5", "maxSteps": 100 },
    "plan":     { "model": "anthropic/claude-haiku-4-5",  "maxSteps": 50 }
  },
  "tools": {
    "bash":  { "permission": "ask",    "timeout": 30000 },
    "write": { "permission": "ask" },
    "edit":  { "permission": "ask" },
    "read":  { "permission": "always" },
    "grep":  { "permission": "always" },
    "glob":  { "permission": "always" },
    "ls":    { "permission": "always" },
    "fetch": { "permission": "ask" }
  },
  "ui": { "sidebar": true, "detailPanel": true }
}
```

The daemon reads this in priority order: `~/.zero/config.json` → project-local `zero.json` → project-local `.zero/config.json` (last wins).

---

## Daemon HTTP API

The daemon exposes HTTP + SSE on `127.0.0.1:8910`. Key routes:

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/health` | Liveness check |
| `GET` | `/events` | Global SSE event bus |
| `GET` | `/openapi.json` | OpenAPI spec |
| `GET` | `/providers/models` | Live-fetched model list |
| `GET` | `/identity` | Current client identity (`clientId`, `displayName`) |
| `POST` | `/projects/ensure` | Upsert project by path |
| `POST` | `/collab/rooms` | Create collab room (returns invite token) |
| `POST` | `/collab/rooms/{id}/join` | Join collab room with token + displayName |
| `GET` | `/collab/rooms/{id}/events` | Per-room SSE bus |
| `POST` | `/collab/rooms/{id}/queue` | Submit prompt (gated by role) |
| `POST` | `/collab/rooms/{id}/queue/{item}/approve` | Approve a queued prompt |
| `POST` | `/sessions/{id}/files` | Upload attachments |
| `GET` | `/auth/google/start` | Start OAuth flow |
| `GET` | `/auth/google/callback` | Complete OAuth flow |
| `GET` | `/auth/me` | Current authenticated user |
| `POST` | `/auth/logout` | Sign out |

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `zero status` → stopped | Daemon crashed on startup | `zero logs` — usually missing env var or DB lock. |
| Model picker empty | Provider `/models` unreachable | Check `ZERO_ROUTER_BASE_URL` + `ZERO_ROUTER_API_KEY`. Picker falls back to curated list. |
| `zero share` fails with 401 | Session cookie expired (auth on) | Sign in again via desktop card. |
| `zero://join/...` link doesn't open desktop | Scheme not registered | Re-run installer; on Linux AppImage you need an AppImage launcher. |
| Daemon says `auth backend = local sqlite` | `ZERO_SUPABASE_DB_URL` unset | Expected unless you configured Supabase. |
| SmartScreen / Gatekeeper warning on first launch | Bundle is unsigned | More info → Run anyway. Production deployments should code-sign. |
| `extractTasks(messages, activity.live.text)` TS error | Stale LSP state, not a real error | `make clean && pnpm typecheck` confirms — tsc passes. |

---

## Contributing

Issues and PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the short version. Security issues: see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © Zero Agent contributors.

---

<div align="center">

**Built with [Tauri](https://tauri.app/) · [Go](https://go.dev/) · [chi](https://github.com/go-chi/chi) · [React](https://react.dev/) · [lucide](https://lucide.dev/)**

*Makes mistakes only once — the library remembers them for you.*

</div>
