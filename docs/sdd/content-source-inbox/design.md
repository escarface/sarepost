# Design: Content Source Inbox

## Architecture

Add a small `internal/application/contentsources` package for validation, CRUD orchestration, and angle generation. Persistence lives in `internal/db` with an additive SQLite migration. The domain model lives in `internal/domain` beside other editorial aggregates.

Adapters remain thin:

- HTTP parses JSON and maps errors.
- MCP exposes the same operations for agent workflows.
- CLI forwards commands to HTTP.

Angle generation delegates to `internal/application/generation.Service` through a minimal interface so the source package does not depend on provider details. The prompt includes the source title, body, tags, optional reference URL, optional caller instructions, and asks for concise editorial angles, not final posts.

## Data Model

`content_sources`

- `id TEXT PRIMARY KEY`
- `title TEXT NOT NULL`
- `body TEXT NOT NULL`
- `source_url TEXT NOT NULL DEFAULT ''`
- `campaign_id TEXT NOT NULL DEFAULT ''`
- `brand_profile_id TEXT NOT NULL DEFAULT ''`
- `tags TEXT NOT NULL DEFAULT '[]'`
- `status TEXT NOT NULL`
- `created_at TEXT NOT NULL`
- `updated_at TEXT NOT NULL`

Status values: `new`, `processed`, `archived`.

## Tradeoffs

### Store raw body instead of fetching URLs

Fetching URLs would be attractive but introduces SSRF, content extraction, timeouts, and legal/content-quality ambiguity. Storing user-provided body text plus an optional URL gives value immediately and keeps the trust boundary simple.

### Return generated angles as text

Structured angle objects would be useful, but providers vary and parsing adds fragility. Returning text keeps the MVP reliable. A later iteration can add persisted `content_source_angles` once product usage proves the shape.

### Include all API/MCP/CLI surfaces in MVP

This is more work than a Web-only MVP, but it matches PostFlow's LLM-first architecture. Generation itself has a documented Web-only exception; source capture is an automation primitive and should preserve parity.
