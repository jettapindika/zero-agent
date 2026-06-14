# Zero

> Desktop-first, local-by-default AI coding agent with a thin scriptable CLI.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-v0.1.0-orange)](#)

Zero is a **bring-your-own-model** AI coding agent. It runs entirely on your
machine, talks to any OpenAI-compatible endpoint (OpenAI, OpenRouter,
LiteLLM, Ollama, vLLM, llama.cpp, LM Studio, …), and stores chat history in a
local SQLite file. The desktop app is the primary interface; the `zero` CLI
gives you a line-mode REPL and one-shot prompts for scripting.

```
Desktop app (Tauri)  ─┐
zero CLI (line REPL) ─┼─►  Go SDK  ─►  local HTTP/SSE core  ─►  your provider
```

---

## Quick start

```bash
git clone <this repo>
cd zero-agent
make build
make install
zero setup                                # creates ~/.config/zero/.env
# edit ~/.config/zero/.env and set ZERO_ROUTER_API_KEY
zero start                                # background daemon
zero .                                    # open desktop in current folder
```

That's it. The **only** thing a fresh user has to set is their model
provider's API key. Everything else is opt-in.

### Two ways to deploy

Zero supports both **single-user** and **multi-user** out of the box. Pick
the one that matches your situation:

#### A. Single-user (default — recommended for personal use)

No `.env` flags beyond the API key. Anonymous, no login screen, all data in
local SQLite at `~/.zero/zero.db`. This is what `zero setup` produces by
default.

- ✅ No Google Cloud project needed.
- ✅ No Supabase project needed.
- ✅ Full desktop UI, all tools, full collaboration over `zero://` invite links.
- ✅ Any OpenAI-compatible model endpoint.

#### B. Multi-user (turn on Google sign-in)

Anyone with a Google account can sign in — the daemon auto-creates a `users`
row on first login. There is **no allowlist**: every authenticated Google
identity gets `role: "user"` and full access. `DEV_EMAILS` only decides who
also gets `role: "dev"` (the Dev tab + `/dev/*` endpoints).

Set in `~/.config/zero/.env`:

```bash
ZERO_AUTH_ENABLED=true
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_CALLBACK_URL=http://127.0.0.1:8910/auth/google/callback
SESSION_SECRET=$(openssl rand -hex 32)
DEV_EMAILS=you@example.com           # optional, comma-separated
```

If you also set `ZERO_SUPABASE_DB_URL`, the `users` and `auth_sessions`
tables live in Supabase Postgres so the **same identity follows users
across multiple Zero hosts**. Without it, each host has its own SQLite
`users` table — sign in once per machine.

> **Want to restrict who can sign in?** There is currently no built-in
> allowlist beyond `DEV_EMAILS` (which gates *role*, not *access*). If you
> need invite-only or domain-restricted sign-in, that's a small patch in
> `services/core/internal/auth/service.go` after `FetchUserInfo` and before
> `UpsertUser`. Contributions welcome.

---

## Bring your own model

Zero never talks to a model directly — it always goes through one
configurable HTTP endpoint that speaks the OpenAI API. Set two env vars in
`~/.config/zero/.env` (or in your shell):

```bash
ZERO_ROUTER_BASE_URL=https://api.openai.com/v1     # any /v1 endpoint
ZERO_ROUTER_API_KEY=sk-...                          # your key
```

The desktop app's **Model Picker** live-fetches `${ZERO_ROUTER_BASE_URL}/models`
and shows whatever your provider returns — pick one and it sticks per session.
Tested combinations:

| Provider                    | `ZERO_ROUTER_BASE_URL`              | Notes                                          |
| --------------------------- | ----------------------------------- | ---------------------------------------------- |
| OpenAI                      | `https://api.openai.com/v1`         | Default. Use a real `sk-...` key.              |
| OpenRouter                  | `https://openrouter.ai/api/v1`      | Single key, hundreds of models.                |
| LiteLLM proxy               | `http://<host>:4000/v1`             | Self-host, route to many backends.             |
| Ollama (local)              | `http://127.0.0.1:11434/v1`         | API key may be any non-empty string.           |
| LM Studio (local)           | `http://127.0.0.1:1234/v1`          | Start the LM Studio server first.              |
| llama.cpp `--server`        | `http://127.0.0.1:8080/v1`          | Compatible OpenAI mode.                        |
| vLLM (self-host)            | `http://<host>:8000/v1`             | `--api-key` to set the key.                    |

If the picker comes up empty, your provider's `/models` endpoint is either
unreachable or returns an empty list. The picker falls back to a curated set
of well-known model ids you can still pick.

Optional: pin a default model id (otherwise Zero picks `gpt-4o-mini`):

```bash
ZERO_DEFAULT_MODEL=anthropic/claude-3-5-sonnet-latest
```

---

## CLI

```bash
zero                 # line REPL against the running daemon
zero .               # open desktop app rooted at the current folder
zero <path>          # open desktop app rooted at <path>
zero -p "prompt"     # one-shot prompt, prints the answer
zero setup           # create ~/.zero and config
zero start           # start background daemon
zero stop            # stop daemon
zero restart         # restart daemon
zero status          # show PID + health
zero logs            # tail ~/.zero/zero.log
zero models          # list models from your provider's /models endpoint
zero auth            # show provider config + where to set keys
zero sessions        # list sessions for current project
zero share           # create a team session invite (prints zero:// URL)
zero join <invite>   # join a team session
```

The line REPL recognizes `/new` (fresh session) and `/quit`. Everything else
— chat history, tool approvals, slash commands, model switching — lives in
the desktop app.

---

## Sharing & joining rooms

`zero share` prints a `zero://join/<roomId>?token=<token>` URL. Share it
however (Slack, email, …) — clicking it opens the desktop app on Windows,
macOS, and Linux with the Share folder modal pre-filled in **Join a room**
mode. Click **Join** and you're in.

The desktop bundle registers the `zero://` URL scheme during installation, and
a single-instance guard prevents duplicate windows when an existing session
is already open.

To produce the Windows installer from a non-Windows host:

```bash
make desktop-build-windows   # requires nsis, cargo-xwin, x86_64-pc-windows-msvc
```

Output: `apps/desktop/src-tauri/target/x86_64-pc-windows-msvc/release/bundle/nsis/`.

---

## Building from source

```bash
make build           # bin/zero + bin/zero-server
make test            # go test ./...
make install         # install to ~/.local/bin
make desktop-dev     # tauri dev (HMR)
make desktop-build   # native installer for the current host
make desktop-icons   # regenerate per-platform icons from src-tauri/icons/source.svg
```

Layout:

```
apps/cli           Go CLI (line REPL + lifecycle commands)
apps/desktop       Tauri desktop app (primary interface)
apps/web           legacy/optional web UI
packages/sdk-go    Go SDK used by CLI and desktop bridge
services/core      Go backend daemon (HTTP + SSE on :8910)
config/            config schema
tools/library      agent learned-context library
```

---

## Multi-user mode (Google sign-in)

Off by default. Turn it on when you want the daemon to gate `/sessions/*`,
`/projects/*`, etc. behind a real identity. Once enabled, **any Google
account** can sign in — the daemon auto-creates a `users` row on first
login. There is no allowlist (see "What if I want invite-only?" below).

1. Create a Google Cloud project at <https://console.cloud.google.com> →
   Enable the People API.
2. *APIs & Services → Credentials* → **Create OAuth 2.0 → Web Application**.
   Set Authorized redirect URI to `http://127.0.0.1:8910/auth/google/callback`.
   (Loopback is exempt from Google's HTTPS requirement.)
3. Copy the credentials into `~/.config/zero/.env`:

   ```bash
   GOOGLE_CLIENT_ID=your-id.apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=...
   GOOGLE_CALLBACK_URL=http://127.0.0.1:8910/auth/google/callback
   SESSION_SECRET=$(openssl rand -hex 32)
   DEV_EMAILS=you@example.com
   ZERO_AUTH_ENABLED=true
   ```

4. `make build && make install-app` and relaunch.

When `ZERO_AUTH_ENABLED=true`:

- Every non-public route returns `401` until the user signs in.
- The desktop window shows a centered "Sign in with Google" card.
- Click → system browser opens → consent → daemon `UpsertUser` (insert or
  update keyed on `google_id`) → `auth_sessions` row created → daemon sets
  an `httpOnly; SameSite=Lax` signed cookie → desktop re-fetches `/auth/me`.
- **Any Google account that completes the OAuth flow gets in.** The daemon
  doesn't check the email against an allowlist — first-time visitors are
  auto-onboarded with `role: "user"`.
- Accounts in `DEV_EMAILS` are upserted with `role: "dev"` and see the
  **Dev** tab in the side panel + can hit `/dev/runtime` and
  `/dev/skills/reload`.

Public routes (always reachable without a cookie):

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

Out of the box, anyone with a Google account can sign in. If you need
domain-restricted (`@yourcompany.com` only) or explicit-allowlist
sign-in, add the check between `FetchUserInfo` and `UpsertUser` in
`services/core/internal/auth/service.go::CompleteFlow`. Roughly:

```go
info, err := FetchUserInfo(ctx, s.httpClient, tok.AccessToken)
if err != nil { return ... }
if !strings.HasSuffix(info.Email, "@yourcompany.com") {
    return "", nil, claim, errors.New("email not in allowlist")
}
role := RoleFor(info.Email, s.devEmails)
upserted, err := s.store.UpsertUser(...)
```

Contributions welcome to make this a config-driven knob (e.g.
`ZERO_AUTH_ALLOWED_DOMAINS=acme.com,example.org`) — see issue tracker.

### Supabase Postgres (multi-host shared identity)

When auth is on, `users` and `auth_sessions` live in local SQLite at
`~/.zero/zero.db` by default. That works fine for a single host with many
users — every user signs in once on that host, the daemon remembers them.

If you want the **same Google identity to be recognized across multiple
Zero hosts** (e.g. a personal laptop + a workstation, or a shared team
deployment), point all hosts at a single Supabase Postgres project:

1. Create a Supabase project, install the CLI, and link this repo:

   ```bash
   supabase login
   cd services/core && supabase init
   supabase link --project-ref <your-project-ref>
   supabase db push
   ```

2. Set the DSN in `~/.config/zero/.env`:

   ```bash
   ZERO_SUPABASE_DB_URL="postgresql://postgres:PASSWORD@db.<your-ref>.supabase.co:5432/postgres"
   ```

   The session pooler (port 5432) works on IPv4-only networks; the direct
   connection requires IPv6.

3. Restart the daemon. You should see
   `level=INFO msg="auth backend = supabase postgres"` in the logs.

Chat data (projects, sessions, messages) **never** syncs to Supabase — it
stays in local SQLite by design.

---

## Contributing

Issues and PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the
short version. Security issues: see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © Zero Agent contributors.
