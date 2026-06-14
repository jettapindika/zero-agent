# File System Expansion — Phased Plan

**Date:** 2026-06-14
**Author:** OpenCode (CodeBuddy)
**Spec source:** in-thread spec "FILE SYSTEM EXPANSION — Full File & Media Support for Zero Agent"
**Scope of this PR:** Phase 1 only.

---

## Why phased

The original spec is a multi-day change touching:

- Schema (new `attachments` table)
- New `internal/filehandler` package (5 file format handlers)
- New `internal/upload` package (multipart receiver + cleanup worker)
- Provider message refactor (`provider.Message.Content string` → content blocks) for vision
- Output writers (.docx, .xlsx generation)
- Two frontends (Tauri desktop + Next.js web)

Doing it all in one PR breaks the existing test contract for providers, runner, and storage. We split into four phases; this PR ships Phase 1 only.

---

## Phase 1 — Ingestion (this PR)

Goal: a user can upload a file, the agent can read its extracted content via a tool, no provider/UI changes.

### Files to add

```
services/core/internal/filehandler/
├── types.go      LoadedFile, MIME constants, errors
├── mime.go       extension → MIME map, isText/isImage helpers
├── loader.go     LoadFile entry point, text loading + chunking
├── image.go      base64 encode (stored, but unused this phase)
├── pdf.go        ReadPDF via ledongthuc/pdf
├── docx.go       ReadDOCX via fumiama/go-docx (read-only this phase)
├── xlsx.go       ReadXLSX via xuri/excelize/v2
└── filehandler_test.go

services/core/internal/upload/
├── store.go      ~/.zero/uploads/{sessionID}/{attachmentID}.{ext}
├── receiver.go   multipart parse, MIME validation, size cap
└── upload_test.go

services/core/internal/storage/migrations/
└── 004_attachments.sql

services/core/pkg/server/
└── attachment_handlers.go   POST/GET/DELETE /sessions/{id}/files
```

### Files to modify

```
services/core/go.mod                         add 3 deps
services/core/internal/storage/session.go    add Attachment + repo methods
services/core/internal/tool/tool.go          register attach_read tool
services/core/internal/tool/safe_tools.go    add attachReadTool implementation
services/core/pkg/server/session_handlers.go mount file routes
```

### Schema (additive only — no breaking changes)

```sql
-- 004_attachments.sql
CREATE TABLE attachments (
  id           TEXT PRIMARY KEY,
  session_id   TEXT NOT NULL REFERENCES sessions(id),
  orig_name    TEXT NOT NULL,
  mime_type    TEXT NOT NULL,
  size_bytes   INTEGER NOT NULL,
  storage_path TEXT NOT NULL,
  extracted    TEXT,           -- nullable; cached extraction (text from PDF/DOCX/XLSX)
  is_chunked   INTEGER NOT NULL DEFAULT 0,
  chunk_count  INTEGER NOT NULL DEFAULT 0,
  created_at   INTEGER NOT NULL,
  deleted_at   INTEGER
);
CREATE INDEX idx_attachments_session ON attachments(session_id, created_at DESC);
```

No part-table changes. Attachments link to sessions, and the agent fetches by ID via the tool.

### HTTP endpoints

```
POST   /sessions/{id}/files               multipart upload, returns [{id, name, mime, size}]
GET    /sessions/{id}/files                list attachments for session
GET    /sessions/{id}/files/{fileId}      stream file bytes (raw download)
DELETE /sessions/{id}/files/{fileId}      soft-delete (set deleted_at, remove file from disk)
```

Constraints:

- 100 MB total upload cap per request (`http.MaxBytesReader`)
- Allowed MIME prefixes: `text/`, `image/`, `application/pdf`, `application/json`,
  `application/toml`, plus the two openxmlformats constants for .docx/.xlsx.
- Files outside whitelist → 415 Unsupported Media Type.
- Storage: `{userHome}/.zero/uploads/{sessionID}/{attachmentID}{ext}`
- On upload success, server emits `attachment.created` event on the bus and inserts a system message in the session: "User attached `<orig_name>` (id=`<attachmentID>`). Use `attach_read` to view it."

### New tool: `attach_read`

```
Name:        attach_read
Description: Read an uploaded attachment by ID (handles PDF / Word / Excel / images / text)
Args:        { id: string, chunk?: int }   // chunk index for large text files
Permission:  false (the user already approved by uploading)
```

Behavior by MIME:

| MIME prefix         | Returned text                                   |
|---------------------|-------------------------------------------------|
| `text/*`, `application/json`, `application/toml` | full content if ≤ 200 KB; chunk-on-request otherwise |
| `application/pdf`   | per-page text with `--- Page N ---` separators  |
| `...wordprocessingml.document` | extracted paragraph text  |
| `...spreadsheetml.sheet` | per-sheet CSV with `Sheet: NAME` headers |
| `image/*`           | text fallback: `[Image attached: name (W×H? unknown), inspect via vision in Phase 2]`. Base64 is captured but never returned in text mode (would blow the context budget). |

Extracted text is cached on `attachments.extracted` so re-reads are O(1) and chunk indexing is stable.

### Out of scope (Phase 2-4)

- **Phase 2 — Vision**: refactor `provider.Message.Content` to `[]ContentBlock`, update OpenAI/Anthropic/Ollama adapters, return image base64 from `attach_read` for vision-capable models.
- **Phase 3 — Output**: `internal/filehandler/output.go`, `WriteDOCX/WriteXLSX`, output download endpoint, output card UI.
- **Phase 4 — Frontend**: drag-drop component in `apps/desktop/src/`, upload progress, attached-file chips. (`apps/web` is legacy and will not get this feature.)

### Verification (Phase 1 only — maps to spec checklist)

- [1]  Large text files load fully — chunking under 200KB threshold inline, above it sliced into `chunk_count` units. **Yes.**
- [2]  Chunk headers via `attach_read --chunk N`. **Yes.**
- [3]  PNG/JPG/WEBP/GIF base64-encoded and stored. Vision dispatch deferred. **Captured-only.**
- [4]  PDF text extracted page by page. **Yes.**
- [5]  .docx text extracted. **Yes.**
- [6]  .xlsx all sheets → CSV. **Yes.**
- [7-8] Output writers — **Phase 3.**
- [9]  POST `/sessions/{id}/files` accepts multipart up to 100 MB. **Yes.**
- [10] Auto-cleanup of temp uploads — replaced with: cleanup on session delete (more correct than 1h timer; uploads survive long conversations).
- [11-14] Frontend — **Phase 4.**
- [15] `go mod tidy` clean after deps. **Verified before merge.**
- [16] No regression in existing 6 test packages. **Verified before merge.**

### Dependency overrides from the spec

| Spec proposed | We use | Why |
|---|---|---|
| `github.com/ledongthuc/pdf` | same | active May 2025, BSD-3, fits text extraction |
| `github.com/nguyenthenguyen/docx` | **`github.com/fumiama/go-docx`** | spec lib is template-replace only — would silently drop headings; fumiama is MIT and supports paragraph creation (needed in Phase 3) |
| `github.com/xuri/excelize/v2` | same | de-facto standard, BSD-3, June 2026 active, Go 1.26 verified |

### Storage location override

| Spec | We use | Why |
|---|---|---|
| `/tmp/zero-uploads/` with 1h cleanup | `~/.zero/uploads/{sessionID}/` deleted on session delete | spec path is wrong on Windows, wiped at reboot on macOS, Tauri sandbox may block /tmp; mid-conversation vanishing is a UX failure. `~/.zero/` matches `zero.db` and `client.json` already living there. |

### Frontend override

Spec assumes Next.js. Repo's Next.js app (`apps/web`) is documented as legacy in `README.md` and `AGENTS.md`. Phase 4 will target `apps/desktop` (Tauri + React 19) only.
