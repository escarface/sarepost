# PostFlow MCP Contract

PostFlow exposes a Streamable HTTP MCP server intended to be understandable by any
LLM client, including Codex, Claude, and ChatGPT-compatible MCP integrations.

This document defines the behavioral contract that clients should assume when they
discover and call tools from `/mcp`.

## Endpoint and Auth

- MCP endpoint: `/mcp`
- Transport: Streamable HTTP JSON-RPC
- Discovery methods such as `initialize`, `ping`, and `tools/list` are open
- Tool calls require one of:
  - `Authorization: Bearer <API_TOKEN>`
  - OAuth bearer token issued by the PostFlow auth server

Related endpoints:

- OAuth authorization server metadata: `/.well-known/oauth-authorization-server`
- Protected resource metadata: `/.well-known/oauth-protected-resource`
- Login UI: `/login`
- Dynamic client registration: `POST /oauth/register`

## Server Intent

PostFlow is a publishing and editorial orchestration MCP. It is optimized for agents
that need to:

- inspect available accounts
- validate social content before creating it
- create drafts or scheduled posts
- manage threads
- upload and attach media
- manage editorial campaigns and content plans
- capture raw content sources and generate editorial angles
- inspect failed publications and recover them safely

The MCP favors explicit operations over implicit inference. A client should not guess
hidden defaults when a lookup or preview tool exists.

## Global Conventions

### Time formats

- Prefer RFC3339 timestamps with explicit timezone offsets
- Some scheduling tools also accept `datetime-local` values, interpreted in the UI
  timezone configured in PostFlow
- Responses normalize timestamps to RFC3339

### IDs

PostFlow uses opaque IDs such as `acc_*`, `pst_*`, `med_*`, `cmp_*`, `plan_*`, and
`dlq_*`. Clients must treat them as opaque strings.

### Draft vs scheduled behavior

- If `scheduled_at` is omitted on create, the post is created as a draft
- `postflow_schedule_post` is the explicit mutation to turn a draft into a scheduled
  post
- `postflow_preview_schedule` should be used before scheduling when a client wants
  non-mutating guardrail feedback

### Single post vs thread

- A single post uses top-level `text` and optional top-level `media_ids`
- A thread uses `segments`
- In `segments`, item `0` is the root post and later items are follow-ups
- When `segments` is provided, clients should treat it as the authoritative content
  payload instead of mixing root semantics across `text` and `segments`

### Media rules

- MCP media uploads require `content_base64`
- Clients should not send local filesystem paths to the MCP
- Upload media first, then reference returned `media_id` values from post or content
  plan tools

### Mutations and idempotency

- Tool names that begin with `list`, `get`, `preview`, `validate`, or `health` are
  read-only and safe to retry
- Create/update/delete/schedule/approve/archive/requeue tools mutate state
- `postflow_create_post` supports `idempotency_key`; clients should use it when retry
  safety matters

### Error handling

- Validation and business rule failures are returned as tool errors with plain,
  actionable messages
- Clients should surface these messages to the user and avoid retrying unchanged
  requests blindly
- When a preview tool exists, prefer preview before mutation to reduce avoidable
  failures

## Recommended Agent Flows

### Publish or schedule a new post

1. Call `postflow_list_accounts`
2. Choose the exact target account by `platform` and `display_name`
3. Optionally call `postflow_upload_media`
4. Call `postflow_validate_post`
5. If creating a draft first, call `postflow_create_post` without `scheduled_at`
6. Optionally call `postflow_preview_schedule`
7. Call `postflow_schedule_post`
8. Confirm with `postflow_list_schedule`

### Edit an existing post

1. Discover the target from `postflow_list_schedule` or `postflow_list_drafts`
2. Call `postflow_edit_post`
3. If the schedule changed, verify with `postflow_list_schedule`

### Manage a thread

1. Validate using `postflow_validate_post` with `segments`
2. Create using `postflow_create_post` with `segments`
3. Edit using `postflow_edit_post` with full replacement `segments`

### Recover a failed publication

1. Inspect `postflow_list_failed`
2. Retry with `postflow_requeue_failed` when the payload is still valid
3. Delete from DLQ with `postflow_delete_failed` only when explicitly intended

## Tool Families

### Health and settings

- `postflow_health`
- `postflow_set_timezone`
- `postflow_set_smtp_notifications`

### Accounts

- `postflow_list_accounts`
- `postflow_create_static_account`
- `postflow_connect_account`
- `postflow_disconnect_account`
- `postflow_set_x_premium`
- `postflow_delete_account`

### Posts and schedule

- `postflow_list_schedule`
- `postflow_list_drafts`
- `postflow_create_post`
- `postflow_validate_post`
- `postflow_preview_schedule`
- `postflow_schedule_post`
- `postflow_edit_post`
- `postflow_cancel_post`
- `postflow_delete_post`
- `postflow_approve_post`

### Media

- `postflow_upload_media`
- `postflow_list_media`
- `postflow_delete_media`

### Campaigns and backlog

- `postflow_create_campaign`
- `postflow_list_campaigns`
- `postflow_update_campaign`
- `postflow_archive_campaign`
- `postflow_add_post_to_campaign`
- `postflow_create_campaign_drafts`
- `postflow_generate_campaign_calendar`
- `postflow_list_editorial_backlog`

### Content plans

- `postflow_preview_content_plan`
- `postflow_create_content_plan`
- `postflow_update_content_plan`
- `postflow_list_content_plans`
- `postflow_get_content_plan`
- `postflow_generate_content_plan`
- `postflow_cancel_content_plan`
- `postflow_retry_content_plan`
- `postflow_regenerate_content_plan`
- `postflow_update_content_plan_variant`
- `postflow_schedule_content_plan`

### Content sources

- `postflow_create_content_source`
- `postflow_list_content_sources`
- `postflow_get_content_source`
- `postflow_update_content_source`
- `postflow_archive_content_source`
- `postflow_generate_content_source_angles`

### Dead-letter queue

- `postflow_list_failed`
- `postflow_requeue_failed`
- `postflow_delete_failed`

## Tool Semantics That Matter for LLMs

### `postflow_list_schedule`

- Default `view` is `publications`
- Use `view=posts` when the client needs raw post rows, thread metadata, or direct
  post IDs for later mutations
- Use RFC3339 `from` and `to` filters whenever the user asks about a concrete time
  window

### `postflow_create_post`

- Requires `account_id`
- Creates a draft when `scheduled_at` is omitted
- Creates a thread when `segments` is provided
- Supports `source_post_id` to reuse an existing post or thread as the source copy
  and media for another account
- Supports editorial metadata such as `campaign_id`, `editorial_status`,
  `requires_approval`, and `tags`

### `postflow_validate_post`

- Use before create when the user requests confidence, safety, or platform-fit checks
- Returns `valid`, normalized payload details, and warnings
- Warnings are informative and should not be silently discarded

### `postflow_edit_post`

- Edits a single editable post by `post_id`
- When `segments` is provided, the thread payload is replaced according to the new
  ordered segment list
- `media_ids: []` explicitly removes media where platform rules allow it

### `postflow_upload_media`

- Requires inline base64 content in `content_base64`
- `original_name` is strongly recommended for MIME detection
- Returned `media_id` must be reused in later post or plan calls

### `postflow_create_content_source`

- Captures raw source material before it becomes final post copy
- Requires `title` and `body`
- Optional `source_url` is stored as a reference only; PostFlow does not fetch remote URLs
- Optional `campaign_id`, `brand_profile_id`, and `tags` help downstream generation

### `postflow_generate_content_source_angles`

- Requires `content_source_id`
- Returns generated editorial angles as text, not final posts
- Use these angles as planning material before creating drafts or content plans

## Interoperability Guidance

LLM clients should rely on:

- tool names as stable capability identifiers
- tool descriptions to decide which action to call
- JSON schema field descriptions to build arguments
- preview and validation tools before destructive or user-visible mutations

LLM clients should not rely on:

- undocumented status transitions
- local file paths being readable by the server
- implicit account selection without first listing accounts
- naive retries of mutating calls without an idempotency strategy
