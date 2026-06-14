# Security Policy

## Reporting a vulnerability

If you find a security issue in Zero, **please don't open a public GitHub
issue**. Instead, open a GitHub Security Advisory on this repository, or
email the maintainers privately if a private contact is listed in the repo
metadata.

We aim to acknowledge reports within 7 days and ship a fix within 30 days
for high-severity issues.

## Threat model in scope

Zero is a desktop-first, local-by-default tool. The following are in scope:

- The Go daemon (`services/core`) listening on `127.0.0.1:8910` — request
  smuggling, auth bypass, SSRF via tool calls, path traversal in the file
  tools, and any way an attacker could escalate from "loaded a malicious
  project" to "exfiltrate `~/.zero/zero.db`".
- The desktop app's IPC surface (Tauri commands) — anything callable from
  the WebView that shouldn't be.
- The collaboration deep-link flow (`zero://join/...`) — token replay, room
  hijacking, host impersonation.
- The Supabase auth path when `ZERO_SUPABASE_DB_URL` is configured.

## Out of scope

- Issues that require a malicious local user with shell access.
- Issues in upstream dependencies (Tauri, Go stdlib, ImageMagick, etc.) —
  please report those upstream.
- Provider-side issues (OpenAI, Ollama, etc.) — please report those to the
  provider.
- Anything stemming from a user-supplied custom system prompt or tool
  config that the agent then executes. Zero will execute what you ask it
  to; that's the point.

## Best practices for users

- Treat `~/.config/zero/.env` like any other secret — `chmod 600`.
- Don't share invite URLs (`zero://join/...?token=...`) in public channels.
  The token is a bearer credential.
- When `ZERO_AUTH_ENABLED=true`, set `SESSION_SECRET` to 32+ bytes of
  cryptographic random (`openssl rand -hex 32`).
