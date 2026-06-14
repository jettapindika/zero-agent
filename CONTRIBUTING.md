# Contributing to Zero

Thanks for considering a contribution! This is a small project; the bar is
"works, has tests, doesn't break anyone else's setup." There's no formal
maintainer rotation yet — open an issue or PR and someone will look.

## Setup

```bash
git clone <fork>
cd zero-agent
cp .env.example ~/.config/zero/.env  # set ZERO_ROUTER_BASE_URL + key
pnpm install
make build
make test
```

You'll need:

- **Go 1.22+** for the daemon, CLI, and SDK.
- **Node 20+ + pnpm 9** for the desktop and (legacy) web app.
- **Rust stable** for the Tauri shell. `rustup default stable` is enough.
- **An OpenAI-compatible endpoint** (any of the providers listed in the
  README's "Bring your own model" section).

## What to work on

- Bugs labeled `good first issue` or `help wanted`.
- Provider quirks — if your favorite OpenAI-compatible endpoint doesn't work,
  a small repro + fix is gold.
- Docs improvements, especially around BYO-model setup.

Avoid:

- Adding new external services without a clear opt-in / opt-out story.
  Zero is local-by-default; please keep it that way.
- Telemetry, analytics, or any "phone home" code unless explicitly requested
  in an issue first.

## PR checklist

Before submitting:

- [ ] `make test` passes.
- [ ] `pnpm --filter @zero-agent/desktop typecheck` passes.
- [ ] `cargo check --manifest-path apps/desktop/src-tauri/Cargo.toml` passes.
- [ ] Code is formatted (`gofmt`, `prettier`, `cargo fmt` defaults).
- [ ] No personal paths, emails, API keys, or other identifying info in
      committed files.
- [ ] If you touched the desktop UI, screenshot before/after in the PR
      description.

## Commit style

One logical change per commit. Subject in imperative mood ("Add X", not
"Added X"). Body explains *why*, not *what* — the diff already shows what.

## Code of conduct

Be kind. Don't be a jerk. Disagreements about technical decisions are fine;
disagreements about people aren't.
