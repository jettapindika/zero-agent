# Zero

Terminal-first AI coding agent.

## Architecture

```txt
Terminal CLI/TUI → Go SDK → local HTTP/SSE core → providers/tools/storage
```

Zero runs locally. The terminal command is the primary interface; browser UI is legacy/optional and not required for normal use.

## Build

```bash
make build
```

This creates:

```txt
bin/zero
bin/zero-server
```

Install the command so you can run `zero` without `./bin/`:

```bash
make install
zero
```

By default this installs to `~/.local/bin`. If your shell cannot find `zero`, add `~/.local/bin` to `PATH`.

## First run

```bash
./bin/zero setup
./bin/zero start
./bin/zero status
./bin/zero
```

Stop the background server:

```bash
./bin/zero stop
```

## Commands

```bash
zero                 # open OpenCode-like terminal UI
zero -p "prompt"     # run one prompt
zero setup           # create ~/.zero and config
zero start           # start background server
zero stop            # stop background server
zero restart         # restart background server
zero status          # show PID + health
zero logs            # print ~/.zero/zero.log
zero sessions        # list sessions for current project
zero share           # create team session invite
zero join <invite>   # join team session
```

## TUI shortcuts

```txt
enter    send prompt
ctrl+j   send prompt (for terminals that map Enter to Ctrl+J)
ctrl+n   create new session
ctrl+p   command palette hint/reserved
ctrl+c   quit
```

The UI uses a responsive OpenCode-like layout: sidebar + chat on wide terminals, chat-focused mode on narrow terminals.

## TUI slash commands

```txt
/new                         create a new session
/clear, /reset               clear visible chat
/status, /info               show session/model/agent status
/history                     show message count for this session
/model [provider/model]      show or set model
/models                      show current model help
/agent [build|plan|explore]  show or set agent
/plan, /ask, /code           switch to plan/explore/build mode
/compact, /summarize         request context compaction
/editor, /edit               open $EDITOR for prompt drafting
/shortcuts, /keys            show keyboard shortcuts
/help                        show slash commands
/quit, /exit, /q             quit
```

## Provider

Default provider is 9router-compatible OpenAI API:

```txt
ZERO_ROUTER_BASE_URL=http://127.0.0.1:20128/v1
ZERO_ROUTER_API_KEY=sk_9router
```

Start 9router before sending prompts.

## Development

```bash
make test
make build
make run
```

## Structure

```txt
apps/cli          Go terminal command
apps/cli/tui      Bubble Tea terminal UI
packages/sdk-go   Go SDK used by CLI
services/core     Go backend server
apps/web          legacy/optional web UI
config/           config schema
```
# zero-agent

## Optional: enable Google sign-in on the desktop app

Auth is **off by default**. Existing single-user installs keep working with no
changes. To turn it on:

1. **Create a Google Cloud project** at <https://console.cloud.google.com>.
2. **Enable the People API** (Google+ API is deprecated; the OIDC userinfo
   endpoint Zero uses works without People API too, but enabling it is the
   recommended path).
3. **Create OAuth 2.0 credentials → Web application** in the
   *APIs & Services → Credentials* page.
4. Set the **Authorized redirect URI** to:
   ```
   http://127.0.0.1:8910/auth/google/callback
   ```
   Loopback (`127.0.0.1`) is exempt from Google's HTTPS requirement, so plain
   HTTP works for local desktop dev. The `Secure` cookie flag activates
   automatically when the daemon is exposed off-loopback.
5. Copy the client id + secret into `~/.config/zero/.env` (or your shell):
   ```bash
   GOOGLE_CLIENT_ID=your-id.apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=...
   GOOGLE_CALLBACK_URL=http://127.0.0.1:8910/auth/google/callback
   SESSION_SECRET=$(openssl rand -hex 32)
   DEV_EMAILS=you@example.com
   ZERO_AUTH_ENABLED=true
   ```
6. Rebuild and relaunch:
   ```bash
   make build
   make desktop-build
   open apps/desktop/src-tauri/target/release/bundle/macos/Zero.app
   ```

When `ZERO_AUTH_ENABLED=true`:

- Every non-public route returns `401` until the user signs in.
- The desktop window shows a centered "Sign in with Google" card.
- Clicking the button opens the system browser. After consent Google redirects
  to the daemon, which sets an `httpOnly; SameSite=Lax` session cookie and
  emits an `auth.signed_in` event on the SSE bus. The desktop window picks
  this up and re-fetches `/auth/me`.
- Accounts in `DEV_EMAILS` get `role: "dev"`, see a **DEV MODE** tab in the
  side panel, and can hit `/dev/runtime` + `/dev/skills/reload`.
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

Sign-out: click the avatar in the topbar → **Sign out**, or call
`POST /auth/logout` directly.

### Optional: store accounts in Supabase Postgres

By default the `users` and `auth_sessions` tables live in local SQLite
(`~/.zero/zero.db`). To centralize accounts in Supabase so the same
`jettabackupacc1@gmail.com` identity follows you across machines:

1. **Sign in to the Supabase CLI** (one-time, from your shell):
   ```bash
   supabase login
   ```
2. **Link this repo to your Supabase project**:
   ```bash
   cd services/core
   supabase init      # creates services/core/supabase/config.toml
   supabase link --project-ref nhhglsucankdrmedrxpm
   ```
   Migrations live at `services/core/supabase/migrations/00001_auth.sql`
   and are committed to the repo.
3. **Push the schema**:
   ```bash
   supabase db push
   ```
   Or apply directly with `psql "$ZERO_SUPABASE_DB_URL" -f services/core/supabase/migrations/00001_auth.sql`.
4. **Set the DSN** in your env (Supabase shows it under
   *Project Settings → Database → Connection string → URI*):
   ```bash
   export ZERO_SUPABASE_DB_URL="postgresql://postgres:PASSWORD@db.nhhglsucankdrmedrxpm.supabase.co:5432/postgres"
   ```
5. **Relaunch the daemon**. On startup you'll see:
   ```
   level=INFO msg="auth backend = supabase postgres" host=db.nhhglsucankdrmedrxpm.supabase.co:5432
   ```
   Without `ZERO_SUPABASE_DB_URL`, the log shows
   `auth backend = local sqlite`.

**Security**:
- The DSN contains your DB password. Keep it in `~/.config/zero/.env` or
  your shell rc, never in the desktop bundle. The Tauri app never sees it.
- Row-Level Security is enabled on both tables; the daemon bypasses RLS
  by connecting as the postgres role. PostgREST / anon-key access is
  denied by default.
- Chat data is intentionally not synced to Supabase. Use S2 in the auth
  plan if you want full sync (multi-day refactor, not in this commit).
