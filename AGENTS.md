# AGENTS.md

Repository-level guidance for AI coding agents working in `zero-agent`.

## Multi Brain (MANDATORY)

- Read `.multibrain/session.md` before starting work.
- Use `.multibrain/session.md` as the master index only.
- Open only the `.multibrain/indexes/*.md` bucket files that match the current task.
- Open `.multibrain/context/*.md` only when the selected bucket points to deeper context that matters.
- After meaningful work, update the relevant named bucket and refresh the master index if needed.
- Prefer creating a new bucket (e.g. `auth`, `tui`, `desktop`, `core`, `sdk`, `release`) over overloading `agents` once a topic area becomes recurring.

## Project Quick Reference

Zero is a desktop-first AI coding agent with a thin scriptable CLI. Top-level layout:

```text
apps/cli          Go CLI (entry point: zero / zero-server) — line REPL + one-shot prompts
apps/desktop      Tauri desktop app — primary user interface
apps/web          legacy/optional web UI
packages/sdk-go   Go SDK consumed by the CLI and desktop bridge
services/core     Go backend HTTP/SSE server
config/           config schema
tools/library     agent learned-context library (Go module)
bin/              build outputs (zero, zero-server, library)
```

Common commands:

```bash
make build      # builds bin/zero and bin/zero-server
make test
make run
```

See `README.md` for the full developer flow (setup, start/stop, auth, Supabase).

## Working Style

- Keep changes minimal and scoped.
- Match existing Go style and module boundaries.
- Do not invent unrelated refactors mid-task; capture them as follow-ups in the
  relevant Multi Brain bucket instead.

## PDF Generation (MANDATORY when the user asks for a PDF)

Output must look polished — never a raw text dump. Render every PDF via the
**default toolchain: HTML → headless Chromium via `github.com/go-rod/rod`**
(this is a Go-first repo). Generate clean semantic HTML using the styles
below, then print to PDF. Fallbacks only if `go-rod` cannot be used: Node
puppeteer, Python weasyprint, then `gofpdf` for trivial layouts.

### Typography (use exactly)

- Title (H1): 24–28pt, bold, `#1a1a1a`
- Subtitle: 14–16pt, normal, `#555555`
- H2: 16–18pt, bold, `#1a1a1a`
- H3: 13–14pt, semibold, `#333333`
- Body: 11–12pt, normal, `#2d2d2d`, line-height 1.65
- Caption: 9–10pt, italic, `#888888`
- Inline code: 10–11pt monospace, bg `#f4f4f4`
- Font stack body: `"Inter", "Helvetica Neue", "Arial", sans-serif`
- Font stack mono: `"JetBrains Mono", "Fira Code", "Courier New", monospace`
- Banned: Times New Roman, Comic Sans, sizes <9pt, ALL CAPS body text.

### Page layout

- Page size: A4 (210×297mm) or US Letter, agent picks per locale.
- Margins: top 25mm, bottom 25mm, left 28mm, right 28mm.
- Single column for prose; two columns only for comparison/reference.
- Never break mid-sentence across pages.

### Spacing

- Before H1: 0pt; after H1: 16pt.
- Before H2: 24pt; after H2: 10pt.
- Before H3: 16pt; after H3: 6pt.
- Between paragraphs: 8–10pt.
- Body line-height: 1.65.

### Cover page (only when document > 4 pages)

- Top 40% solid color block or clean white.
- Title centered 28–32pt bold; subtitle centered 14pt muted.
- Bottom-left 10pt date; bottom-right 10pt author/org.
- Thin 1pt accent line separating header from body area.

### Section dividers

Between major sections: 1pt horizontal rule color `#e5e5e5`, 20pt above /
16pt below. No heavy borders — they look amateur.

### Tables

- Header row: bg `#1a1a2e` or `#2563eb` (match accent), white bold 10–11pt,
  padding 8px 12px.
- Body rows: alternating white / `#f8f8f8`, text `#2d2d2d` 10–11pt,
  padding 6px 12px, `border-bottom: 0.5pt #e5e5e5`.
- Numbers right-aligned, text left-aligned. **Bottom borders only** — never
  full grid borders on every cell.

### Code blocks

- Background `#f6f8fa` (or `#1e1e1e` dark), `border: 1pt solid #e1e4e8`,
  `border-left: 3pt solid #2563eb`, monospace 9–10pt, padding 10px 14px.
- Optional muted line numbers (`#999`) and syntax highlighting.

### Callout boxes (use these — never plain paragraphs for these intents)

- Note/Info: left-border 3pt `#2563eb`, bg `#eff6ff`, prefix `NOTE:` or ℹ️.
- Warning: left-border 3pt `#f59e0b`, bg `#fffbeb`, prefix `WARNING:` or ⚠️.
- Important: left-border 3pt `#ef4444`, bg `#fef2f2`, prefix `IMPORTANT:` or 🔴.
- Tip: left-border 3pt `#10b981`, bg `#f0fdf4`, prefix `TIP:` or 💡.

### Header / footer (every page except cover)

- Header: left = document title 9pt muted `#999`; right = section name
  9pt muted; `border-bottom: 0.5pt #e5e5e5`.
- Footer: center = `— N —` page number 9pt muted; left = date or version;
  right = author or org.

### Images & figures

Centered, max 90% of text area, caption below 9pt italic muted, optional
0.5pt `#e5e5e5` border for screenshots, never bleed to margins.

### Lists

- Bullets: `•` or `–`, indent 6mm, 4pt between items, no bullet on
  single-item lists.
- Numbered: `1.` `2.` `3.` (never `1)` or `(1)`), hanging indent of
  number + 5mm gap, sub-list adds 6mm.

### Color palettes — pick exactly ONE per document

- **Professional Dark**: primary `#1a1a2e`, accent `#2563eb`, text `#1a1a1a`,
  muted `#6b7280`, bg `#ffffff`, surface `#f8f9fa`.
- **Clean Minimal**: primary `#111111`, accent `#0ea5e9`, text `#1f2937`,
  muted `#9ca3af`, bg `#ffffff`, surface `#f9fafb`.
- **Warm Document**: primary `#1c1917`, accent `#d97706`, text `#1c1917`,
  muted `#78716c`, bg `#fffbf5`, surface `#f5f0e8`.

### Output checklist (every PDF must satisfy)

1. Title block first: title, subtitle, date, author.
2. Table of contents when document has > 3 sections.
3. H2 = major sections, H3 = subsections.
4. Code only inside styled code blocks (never inline-in-prose for
   multi-line code).
5. Notes/warnings in callout boxes (never plain paragraphs).
6. Page numbers + header on every page (except cover).
7. Paragraphs ≤ 5 sentences; otherwise split.
8. Every content type in its proper container — never raw text dumps.
9. End with a summary/conclusion when content warrants it.
10. No heading orphaned at the bottom of a page.

### Toolchain checklist

When generating, the agent must:
- Confirm `github.com/go-rod/rod` is on `go.mod` (or `go get` it).
- Build the HTML with the styles above as a single self-contained file.
- Use `rod`'s `MustPDF` / `PDF` API with `printToPDF` options matching the
  page-size and margin rules above.
- **Save the PDF inside the active project root** (the directory the user
  is prompting from — available to tools as the `cwd` / project path).
  Default destination: `<projectRoot>/outputs/<timestamp>-<slug>.pdf`. If
  the user explicitly names a path, honor it. Never write to
  `~/.zero/outputs/` or `/tmp/` unless the user asks.
- Create the `outputs/` subdirectory if it does not exist (`mkdir -p`).
- Report the absolute output path back to the user.
