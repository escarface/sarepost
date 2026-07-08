# Spec: Content Source Inbox

## Intent

PostFlow already generates posts and durable content plans, but users still need to manually turn raw material into prompts. The Content Source Inbox gives users a first-class place to capture source material such as notes, pasted article excerpts, call takeaways, and reference URLs, then generate reusable editorial angles from that material before creating drafts or plans.

## Scope

### In

- Persist content sources with title, body, optional reference URL, optional campaign, optional brand profile, tags, and lifecycle status.
- List, retrieve, update, and archive content sources through the shared application layer.
- Generate editorial angles from one content source using the existing text generation service.
- Expose the MVP through HTTP API, MCP, and CLI to preserve automation parity.
- Document the feature in the OpenAPI and MCP specs.

### Out

- Automatic URL fetching, crawling, or article extraction.
- Creating posts directly from selected angles.
- Web UI inbox management beyond API-backed primitives.
- Multi-source clustering, deduplication, or semantic search.
- Per-source generated angle persistence beyond returning the generation result.

## Requirements

### R1: Capture Content Sources

The system MUST allow a caller to create a content source with a non-empty title and body, optional metadata, tags, and a default `new` status.

#### Scenario: Create a valid source

Given an authenticated API, MCP, or CLI caller
When the caller submits a title and body
Then the system persists a content source with status `new` and returns its ID and metadata.

#### Scenario: Reject incomplete source

Given an authenticated caller
When the caller submits an empty title or body
Then the system rejects the request with a validation error and does not persist a source.

### R2: Manage Content Sources

The system MUST let callers list, retrieve, update, and archive content sources without deleting historical data.

#### Scenario: Archive source

Given an existing content source
When the caller archives it
Then the source status becomes `archived` and it is excluded from default list results unless archived sources are requested.

### R3: Generate Editorial Angles

The system MUST generate a configurable number of editorial angles from a stored content source using the existing generation provider configuration and optional brand profile.

#### Scenario: Generate angles

Given a stored source and configured text generation provider
When the caller requests editorial angles
Then the response includes concise angle suggestions grounded in the source body and includes provider metadata.

### R4: Preserve Surface Parity

The system MUST expose source capture, management, and angle generation through HTTP, MCP, and CLI with consistent field names.

#### Scenario: Surface parity

Given the source inbox capability exists
When capability parity tests inspect HTTP, MCP, and CLI surfaces
Then each surface exposes equivalent content source operations.

## Acceptance Criteria

- [ ] R1 create validates title and body and persists valid sources.
- [ ] R2 list/get/update/archive behavior is covered by application and DB tests.
- [ ] R3 angle generation uses existing `generation.Service` and returns provider/model metadata.
- [ ] R4 HTTP, MCP, CLI, and specs expose matching content source operations.
- [ ] Existing post generation and content plan behavior remain unchanged.

## Verification Strategy

- Unit: application service validation and prompt construction tests.
- Integration: SQLite store migration and CRUD tests.
- Surface: HTTP handler tests, MCP tool tests, CLI tests, parity checks where practical.
- Build/type-check: `go test ./...`.

## Risks And Tradeoffs

| Risk | Mitigation |
|---|---|
| URL fetching adds security and reliability risk | MVP stores `source_url` only and does not fetch remote content. |
| Feature duplicates content plans | Sources stay upstream of plans and focus on raw input capture plus angle generation. |
| Surface parity increases implementation size | Keep operations narrow and reuse application service contracts. |
| Generated angles may be hard to parse structurally | Return provider text as an angles payload in MVP; structured angle persistence can follow later. |
