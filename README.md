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
cp .env.example ~/.config/zero/.env       # then edit URL + API key
make build
make install
zero start                                # background daemon
zero .                                    # open desktop in current folder
```

That's it. Anything OpenAI-compatible at `ZERO_ROUTER_BASE_URL` will work.

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

## Optional: Google sign-in (multi-user)

Auth is **off by default** and most installs never need it. Single-user
local installs keep working with no changes. Turn it on when you want the
daemon to gate `/sessions/*`, `/projects/*`, etc. behind a real identity:

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
- Click → system browser opens → consent → daemon sets an
  `httpOnly; SameSite=Lax` session cookie → desktop re-fetches `/auth/me`.
- Accounts in `DEV_EMAILS` get `role: "dev"` and see the **Dev** tab in the
  side panel.
- All other accounts default to `role: "user"`.

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

### Optional: Supabase Postgres for auth tables

By default `users` and `auth_sessions` live in local SQLite at
`~/.zero/zero.db`. If you want the same identity to follow you across
machines, point the daemon at a Supabase Postgres project:

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
