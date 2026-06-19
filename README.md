# Sarepost

A lightweight, self-hosted social publishing service with a Web UI, HTTP API, MCP endpoint, and CLI. Schedule, validate, and publish posts across X (Twitter), LinkedIn, Facebook, and Instagram — from a single place.

Built in Go with SQLite. LLM-first: every capability is consistently exposed through the API, MCP, and CLI surfaces.

---

## Features

- **In-app AI generation** — generate post copy and images from the Web UI with configurable LLM/image providers (Anthropic, OpenAI) and reusable brand profiles
- **Multi-platform publishing** — X, LinkedIn (profiles + company pages), Facebook, and Instagram
- **Thread/thread support** — create root posts with follow-up replies or comments
- **Media management** — upload images and video, attach to posts
- **Scheduling** — set posts to publish at a future date and time
- **Draft workflow** — create posts as drafts, validate, then schedule
- **Editorial campaigns** — group posts under campaign briefs with audience, tone, CTA, tags, and archive state
- **Editorial approval** — mark posts as `needs_review`, require approval, and block scheduling until approved
- **Editorial backlog** — search pending content by campaign, platform, editorial status, tag, and date range
- **Dead-letter queue** — inspect and requeue failed publications
- **OAuth account connection** — connect social accounts via standard OAuth flows
- **MCP endpoint** — expose publishing tools to LLMs (Claude, Codex, ChatGPT)
- **CLI** — manage posts, accounts, media, and settings from the terminal
- **SMTP failure notifications** — get email alerts when a post fails to publish
- **Encrypted secrets** — passwords and tokens stored with AES-GCM encryption
- **Single-binary** — one executable with embedded SQLite, zero external dependencies

---

## Architecture

Sarepost is a **modular monolith** with a clean application layer:

```
cmd/              → entrypoints (server, CLI)
internal/api      → HTTP + MCP adapters
internal/cli      → CLI adapter
internal/worker   → background publishing runtime
internal/application → business use cases (posts, media, DLQ, notifications)
internal/db       → SQLite persistence
internal/postflow → provider SDK clients (X, LinkedIn, Meta)
internal/genai    → text/image generation clients (Anthropic, OpenAI, mock)
internal/secure   → encryption
internal/domain   → entities and enums
```

See [`docs/architecture.md`](docs/architecture.md) for layer rules and design decisions.

---

## AI generation

The **Generate** tab in the Web UI generates post copy and images with AI, then hands
the result off to the new-post composer (text prefilled, generated image attached).

- Configure providers in **Settings → AI generation**: pick a provider and model for
  text (Anthropic, OpenAI) and image (OpenAI) and paste an API key. Keys are encrypted
  at rest with `POSTFLOW_MASTER_KEY`.
- Define **brand profiles** (system prompt, tone, and an uploaded image-style
  reference) that are applied automatically to generation, alongside the target
  platform's rules. The reference image guides generated images (image-to-image).
- With `POSTFLOW_DRIVER=mock` (the default), generation uses built-in mock providers and
  makes no network calls — useful for local development and tests. Set
  `POSTFLOW_DRIVER=live` to call real provider APIs.

Generation is a **Web-UI-only** feature: MCP and CLI clients already generate content
through their own LLM, so no generation tools are exposed on those surfaces.

---

## Editorial campaigns

Campaigns turn PostFlow into an editorial planning layer instead of only a scheduler.
Create a campaign with its brief fields (`objective`, `audience`, `tone`, `cta`,
`restrictions`, `tags`, `timezone`), then create or attach posts with `campaign_id`.

Posts keep their technical publication status (`draft`, `scheduled`, `published`, etc.)
separate from editorial status (`idea`, `drafting`, `needs_review`, `approved`,
`scheduled`). If `requires_approval=true`, PostFlow rejects scheduling until the post
is approved. Posts linked to archived campaigns are also blocked from create/schedule
flows.

Scheduling includes editorial guardrails: PostFlow rejects account-level calendar
conflicts in the nearby time window and recent duplicate text for the same account.
These checks run before drafts are scheduled or editable drafts are converted into
scheduled posts.

The Web UI includes **Campaigns** and **Backlog** tabs. API, MCP, and CLI expose the
same campaign, approval, and backlog capabilities for LLM-first workflows.

---

## Quick Start

**Requirements:** Go 1.26+

```bash
git clone https://github.com/escarface/sarepost.git
cd sarepost
cp .env.example .env
```

Generate secrets:

```bash
# 32-byte base64 master key (required)
openssl rand -base64 32

# API token (recommended for CLI/MCP clients)
openssl rand -hex 32

# Owner password hash for the web UI login
go run ./scripts/hash-password.go 'your-password-here'
```

Edit `.env`:

```dotenv
PORT=8080
POSTFLOW_MASTER_KEY=<base64-from-openssl>
API_TOKEN=<hex-token>
PUBLIC_BASE_URL=http://localhost:8080
OWNER_EMAIL=owner@example.com
OWNER_PASSWORD_HASH='$2a$10$...'
POSTFLOW_DRIVER=mock
```

> **Note:** `POSTFLOW_DRIVER=mock` simulates publishing without hitting real APIs. Set to `live` when ready to publish.

Start the server:

```bash
go run ./cmd/postflow-server
```

Open:
- **Web UI:** `http://localhost:8080`
- **MCP endpoint:** `http://localhost:8080/mcp`

---

## Environment Variables

### Core

| Variable | Required | Description |
|---|---|---|
| `POSTFLOW_MASTER_KEY` | Yes | 32-byte base64 key for encrypting secrets |
| `API_TOKEN` | Recommended | Bearer token for API, CLI, and MCP auth |
| `OWNER_EMAIL` | For web UI | Email for single-user local login |
| `OWNER_PASSWORD_HASH` | For web UI | Bcrypt hash of the owner password |
| `PUBLIC_BASE_URL` | For OAuth + media | Public URL of the deployment |
| `PORT` | No (default: `8080`) | HTTP port |
| `DATABASE_PATH` | No (default: `postflow.db`) | SQLite database file path |
| `DATA_DIR` | No (default: `data`) | Uploaded media storage directory |

### Social network credentials

| Network | Variables | Source |
|---|---|---|
| X (Twitter) | `X_CLIENT_ID`, `X_CLIENT_SECRET` | [X Developer Portal](https://developer.x.com) |
| LinkedIn | `LINKEDIN_CLIENT_ID`, `LINKEDIN_CLIENT_SECRET` | [LinkedIn Developer](https://developer.linkedin.com) |
| Facebook / Instagram | `META_APP_ID`, `META_APP_SECRET` | [Meta for Developers](https://developers.facebook.com) |

### Runtime

| Variable | Default | Description |
|---|---|---|
| `POSTFLOW_DRIVER` | `mock` | `mock` for local dev, `live` for real publishing |
| `POSTFLOW_BASE_URL` | `http://localhost:8080` | Server URL (CLI only) |
| `POSTFLOW_API_TOKEN` | — | Token for CLI auth (CLI only) |

---

## MCP Setup (for LLMs)

Sarepost exposes publishing tools to AI agents via the Model Context Protocol (MCP).

**Endpoint:** `http://localhost:8080/mcp`

Auth header: `Authorization: Bearer <API_TOKEN>`

### Codex

```bash
codex mcp add postflow --url http://localhost:8080/mcp
```

`~/.codex/config.toml`:

```toml
[mcp_servers.postflow]
url = "http://localhost:8080/mcp"
bearer_token_env_var = "POSTFLOW_API_TOKEN"
```

### Claude Code

```bash
claude mcp add -t http postflow http://localhost:8080/mcp --header "Authorization: Bearer <API_TOKEN>"
```

### ChatGPT / OAuth clients

- Authorization metadata: `http://localhost:8080/.well-known/oauth-authorization-server`
- Protected resource metadata: `http://localhost:8080/.well-known/oauth-protected-resource`
- Login: `http://localhost:8080/login`
- Dynamic client registration: `POST /oauth/register`

Discovery requests (`initialize`, `tools/list`, `ping`) are open. Tool calls require OAuth bearer auth or `API_TOKEN`.

### Available MCP Tools

| Tool | Description |
|---|---|
| `postflow_health` | Server health check |
| `postflow_list_schedule` | List scheduled posts |
| `postflow_list_drafts` | List draft posts |
| `postflow_list_accounts` | List connected accounts |
| `postflow_create_static_account` | Create account without OAuth |
| `postflow_connect_account` | Initiate OAuth connection |
| `postflow_disconnect_account` | Disconnect an account |
| `postflow_set_x_premium` | Toggle X Premium status |
| `postflow_delete_account` | Delete an account |
| `postflow_list_failed` | List failed posts (DLQ) |
| `postflow_create_post` | Create a new post |
| `postflow_cancel_post` | Cancel a scheduled post |
| `postflow_schedule_post` | Schedule a draft post |
| `postflow_preview_schedule` | Preview schedule guardrails without mutating a draft |
| `postflow_edit_post` | Edit post content or schedule |
| `postflow_delete_post` | Delete a post |
| `postflow_approve_post` | Approve editorial content before scheduling |
| `postflow_validate_post` | Validate post before saving |
| `postflow_create_campaign` | Create an editorial campaign |
| `postflow_list_campaigns` | List editorial campaigns |
| `postflow_update_campaign` | Update an editorial campaign |
| `postflow_archive_campaign` | Archive an editorial campaign |
| `postflow_add_post_to_campaign` | Attach an existing post to a campaign |
| `postflow_create_campaign_drafts` | Generate draft variants from a campaign brief |
| `postflow_generate_campaign_calendar` | Generate multi-day campaign draft plans with `planned_at` slots |
| `postflow_list_editorial_backlog` | List content pending editorial action |
| `postflow_upload_media` | Upload image or video |
| `postflow_list_media` | List uploaded media |
| `postflow_delete_media` | Delete media |
| `postflow_requeue_failed` | Requeue a failed post |
| `postflow_delete_failed` | Delete from DLQ |
| `postflow_set_timezone` | Set publishing timezone |
| `postflow_set_smtp_notifications` | Configure SMTP alerts |

All tools support thread/multi-segment posts via the `segments` field, where segment `1` is the root post and segments `2..N` are follow-ups.

---

## CLI (`postflow`)

### Install

```bash
# Homebrew (recommended)
brew tap escarface/tap
brew install escarface/tap/postflow

# Or run from source
go run ./cmd/postflow --help
```

### Configure

```bash
export POSTFLOW_BASE_URL="http://localhost:8080"
export POSTFLOW_API_TOKEN="<API_TOKEN>"
```

### Common Commands

```bash
# Health check
postflow health

# Schedule
postflow schedule list --from 2026-03-01T00:00:00Z --to 2026-03-31T23:59:59Z
postflow schedule list --view posts --from 2026-03-01T00:00:00Z --to 2026-03-31T23:59:59Z

# Drafts
postflow drafts list --limit 20

# Validate without saving
postflow posts validate --account-id acc_xxx --text "Check this content"
postflow posts validate --account-id acc_xxx --segments-json '[{"text":"root"},{"text":"reply"}]'

# Create
postflow posts create --account-id acc_xxx --text "Hello world" --scheduled-at 2026-03-01T10:00:00Z
postflow posts create --account-id acc_xxx --segments-json '[{"text":"root"},{"text":"reply","media_ids":["med_x"]}]' --scheduled-at 2026-03-01T10:00:00Z
postflow posts create --account-id acc_xxx --campaign-id cmp_xxx --editorial-status needs_review --requires-approval --tags launch,linkedin --text "Review this"

# Schedule a draft
postflow posts preview-schedule --id pst_xxx --scheduled-at 2026-03-01T10:00:00Z
postflow posts schedule --id pst_xxx --scheduled-at 2026-03-01T10:00:00Z

# Editorial approval
postflow posts approve --id pst_xxx

# Campaigns and backlog
postflow campaigns create --name "Q3 launch" --objective "Drive qualified demand" --audience "SaaS founders" --tone "direct" --brand-profile "Sare Digital" --visual-style "technical-minimal" --image-prompt "Warm white process blueprint, restrained golden nodes, readable Spanish headline" --image-size 1080x1350 --cta "Book a demo" --tags launch,q3
postflow campaigns list --status active
postflow campaigns update --id cmp_xxx --status paused --name "Q3 launch revised"
postflow campaigns archive --id cmp_xxx
postflow campaigns add-post --campaign-id cmp_xxx --post-id pst_xxx --editorial-status needs_review --requires-approval
postflow campaigns create-drafts --id cmp_xxx --account-id acc_xxx --idea "Turn the launch proof point into a founder-facing LinkedIn post" --variants-per-post 3
postflow campaigns create-drafts --id cmp_xxx --account-id acc_xxx --brand-profile "Sare Digital" --idea "Override the campaign default brand for this batch"
postflow campaigns create-drafts --id cmp_xxx --account-id acc_xxx --brand-profile "Sare Digital" --generate-images --image-prompt "Minimal technical visual with process map, warm white background, charcoal lines, restrained golden nodes" --idea "Create an Instagram-ready campaign draft with matching creative"
postflow campaigns generate-calendar --id cmp_xxx --account-id acc_xxx --from 2026-07-06T09:00:00+02:00 --days 7 --slots 09:00,17:00 --idea "One week of launch education posts"
postflow campaigns generate-calendar --id cmp_xxx --account-id acc_xxx --from 2026-07-06T09:00:00+02:00 --days 7 --slots 09:00,17:00 --generate-images --image-prompt "Premium Sare Digital visual system, readable Spanish headline, process blueprint layout" --idea "One week of launch education posts with attached creatives"
postflow campaigns backlog --campaign-id cmp_xxx --editorial-status needs_review

# Edit
postflow posts edit --id pst_xxx --text "Updated copy" --intent schedule --scheduled-at 2026-03-01T10:30:00Z
postflow posts edit --id pst_xxx --segments-json '[{"text":"root updated"},{"text":"reply updated"}]'

# Cancel / delete
postflow posts cancel --id pst_xxx
postflow posts delete --id pst_xxx

# Accounts
postflow accounts list
postflow accounts list --kind x
postflow accounts list --kind linkedin

# DLQ
postflow dlq list --limit 50
postflow dlq requeue --id dlq_xxx

# Media
postflow media upload --file ./image.jpg --kind image --tags launch,product
postflow media list --limit 20 --tag launch

# Settings
postflow settings set-timezone --timezone Europe/Madrid
postflow settings set-smtp --host smtp.sendgrid.net --port 587 --username apikey --password "$SMTP_PASSWORD" --from sarepost@example.com --to owner@example.com
```

> `--text` and `--segments-json` are mutually exclusive. Use `--json` for machine-readable output.

---

## Deployment

### GHCR Image

Prebuilt Docker images are available:

```
ghcr.io/escarface/sarepost:latest
ghcr.io/escarface/sarepost:vX.Y.Z
```

### Docker Compose

```yaml
services:
  postflow:
    image: ghcr.io/escarface/sarepost:latest
    ports:
      - "8080:8080"
    environment:
      POSTFLOW_MASTER_KEY: <base64-key>
      API_TOKEN: <hex-token>
      PUBLIC_BASE_URL: https://your-domain.com
      OWNER_EMAIL: owner@example.com
      OWNER_PASSWORD_HASH: '$2a$10$...'
      POSTFLOW_DRIVER: live
      DATABASE_PATH: /srv/data/postflow.db
    volumes:
      - postflow_data:/srv/data

volumes:
  postflow_data:
```

### Coolify

See [`docs/coolify-deploy.md`](docs/coolify-deploy.md) for a full production deployment guide.

---

## Development

```bash
# Clone
git clone https://github.com/escarface/sarepost.git
cd sarepost

# Copy env
cp .env.example .env

# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Start dev server (hot reload with air)
air
```

### Code quality

```bash
go fmt ./...
go test ./...
go test -race ./...
./scripts/check-coverage.sh
golangci-lint run ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

---

## Troubleshooting

| Problem | Check |
|---|---|
| `401 unauthorized` | Verify `API_TOKEN` and `Authorization: Bearer` header |
| OAuth callback errors | Ensure `PUBLIC_BASE_URL` matches your public domain; check app settings in the provider developer portal |
| Instagram media upload fails | `PUBLIC_BASE_URL` must be publicly reachable; use JPEG/PNG for images, MP4/MOV for video; max 512 MiB |
| CLI auth errors | Confirm `POSTFLOW_API_TOKEN` matches server `API_TOKEN` |
| LinkedIn company page OAuth | Use `account_kind=organization` on the OAuth start URL to request company page scopes |

---

## Docs

- [Architecture](docs/architecture.md)
- [API spec](docs/specs/openapi.yaml)
- [Deployment guide](docs/coolify-deploy.md)
- [Release guide](docs/RELEASING.md)
- [ADR: Modular monolith](docs/adr/0001-modular-monolith-application-layer.md)
