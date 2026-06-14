# Agent Library

Per-machine knowledge store for the AI coding agent. Captures mistakes,
insights, patterns, conventions, and fixes so future sessions don't repeat
errors.

**Storage is gitignored** (entries contain private project context). Only
this README and the directory layout are versioned.

## Layout

```
library/
  entries/        active entries  (one .json per entry)
  archive/        archived entries (low-scoring, kept for restoration)
  indexes/
    index.json    counts + ordered entry IDs
    tags.json     tag -> [ids]
  LIBRARY.md      auto-generated human summary (regenerated on every write)
```

## CLI

The binary lives at `tools/library/cmd/library`. Build it with:

```bash
go build -o bin/library ./tools/library/cmd/library
```

Commands (all accept `--root <dir>`; default `./library`):

| Command | What it does |
|---|---|
| `library init` | Create directory layout |
| `library add --type <t> --title "..." [--body ... --tags a,b --files p1,p2 --confidence 0.8]` | Capture a new entry |
| `library add --json` (stdin) | Capture from JSON on stdin |
| `library capture-correction ...` | Same as `add` but tags source as `user-correction` |
| `library list [--tag X --type T --all --json --limit N]` | List entries by score |
| `library show <id>` | Print one entry as JSON |
| `library use <id> [--helped]` | Increment usage stats (hit if `--helped`) |
| `library delete <id>` | Archive (entries are never hard-deleted) |
| `library restore <id>` | Move entry back from archive |
| `library stats` | Counts by type |
| `library inject` | Print markdown block for system-prompt injection |
| `library auto-archive [--min-use N --threshold 0.30]` | Archive low-scoring entries |
| `library reindex` | Rebuild index.json, tags.json, LIBRARY.md |

## Entry types

- `mistake` — things to never do (highest injection priority, +0.20 score boost)
- `convention` — project rules / versions / style (+0.15 boost)
- `fix` — recurring fixes (+0.05 boost)
- `pattern` — reusable approaches
- `insight` — observations about this codebase

## Ranking

`Score = 0.4·confidence + 0.6·(hit/use)` blended with type boost.
New entries (use=0) fall back to `confidence + boost`.

## Injection budget

`library inject` returns at most **8 entries**. Mistakes are placed first,
then everything else by score. Archived entries are excluded.

Use this in your agent shell init:

```bash
# Append learned context to the agent system prompt at session start
library inject >> .agent-context.md
```

## Capturing on user correction

When the user corrects the agent, the agent should run:

```bash
library capture-correction \
  --type mistake \
  --title "<one-line lesson>" \
  --body "<what went wrong and the fix>" \
  --tags go,auth \
  --files services/core/auth.go \
  --confidence 0.85
```

Body can also be piped via `--stdin`, or the whole entry via `--json`.

## Constraints

- Library writes are async-friendly (small, single-file writes; never block long).
- Mistakes and conventions always rank highest.
- Entries are archived, not deleted — always restorable via `library restore <id>`.
- Reindex runs on every Put/Archive/Restore, regenerating LIBRARY.md.
