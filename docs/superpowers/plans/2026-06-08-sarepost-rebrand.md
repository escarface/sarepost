# Sarepost Rebrand — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate project identity from `github.com/antoniolg/postflow` to `github.com/saredigital/sarepost` — module path, Docker/GHCR images, Homebrew tap, docs URLs, logos, and branding strings.

**Architecture:** Mechanical find-and-replace changes across ~120 Go files, Docker/CI configs, docs, and asset files. No logic changes. Backward compatible for all public surfaces (env vars, MCP tool names, binaries, internal identifiers remain untouched).

**Tech Stack:** Go 1.26.3, SQLite, Docker, GitHub Actions, Homebrew

---

### Task 1: Change Go module path and all imports

**Files:**
- Modify: `go.mod:1`
- Modify: all `*.go` files importing `github.com/antoniolg/postflow/...` (~120 files)

- [ ] **Step 1: Update go.mod module path**

Edit `go.mod` line 1 from:
```
module github.com/antoniolg/postflow
```
to:
```
module github.com/saredigital/sarepost
```

- [ ] **Step 2: Replace all Go import paths**

Run:
```bash
find . -name "*.go" -exec sed -i '' 's|github.com/antoniolg/postflow|github.com/saredigital/sarepost|g' {} +
```

- [ ] **Step 3: Update Dockerfile ldflags**

In `Dockerfile` line 19, change:
```
-X github.com/antoniolg/postflow/cmd/postflow-server.Version=${APP_VERSION}
```
to:
```
-X github.com/saredigital/sarepost/cmd/postflow-server.Version=${APP_VERSION}
```

(Note: Mass sed in step 2 already updates this if it appears in a `.go` file. But Dockerfile is not a `.go` file, so do it explicitly.)

- [ ] **Step 4: Update release-cli-homebrew.yml ldflags**

In `.github/workflows/release-cli-homebrew.yml` line 80, change:
```
-X github.com/antoniolg/postflow/internal/cli.Version=${TAG}
```
to:
```
-X github.com/saredigital/sarepost/internal/cli.Version=${TAG}
```

- [ ] **Step 5: Run go mod tidy**

```bash
go mod tidy
```

- [ ] **Step 6: Verify build compiles**

```bash
go build ./...
```

Expected: No errors. If any import was missed, the compiler will report it.

- [ ] **Step 7: Run tests**

```bash
go test ./...
```

Expected: All tests pass.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "refactor: rename go module to github.com/saredigital/sarepost"
```

---

### Task 2: Update Docker and GHCR image references

**Files:**
- Modify: `docker-compose.yml:6`
- Modify: `.github/workflows/release-image.yml:13`
- Modify: `.github/workflows/release-cli-homebrew.yml:20`

- [ ] **Step 1: Update docker-compose.yml image**

In `docker-compose.yml` line 6, change:
```
    image: antoniolg/postflow:latest
```
to:
```
    image: ghcr.io/saredigital/sarepost:latest
```

- [ ] **Step 2: Update release-image.yml IMAGE_NAME**

In `.github/workflows/release-image.yml` line 13, change:
```
  IMAGE_NAME: ghcr.io/antoniolg/postflow
```
to:
```
  IMAGE_NAME: ghcr.io/saredigital/sarepost
```

- [ ] **Step 3: Update release-cli-homebrew.yml GH_REPO**

In `.github/workflows/release-cli-homebrew.yml` line 20, change:
```
  GH_REPO: antoniolg/postflow
```
to:
```
  GH_REPO: saredigital/sarepost
```

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml .github/workflows/release-image.yml .github/workflows/release-cli-homebrew.yml
git commit -m "refactor: update docker and CI image references to saredigital/sarepost"
```

---

### Task 3: Update Homebrew tap references

**Files:**
- Modify: `.github/workflows/release-cli-homebrew.yml:19`
- Modify: `README.md:206-207`
- Modify: `docs/RELEASING.md:79`

- [ ] **Step 1: Update release-cli-homebrew.yml tap repo**

In `.github/workflows/release-cli-homebrew.yml` line 19, change:
```
  HOMEBREW_TAP_REPO: antoniolg/homebrew-tap
```
to:
```
  HOMEBREW_TAP_REPO: saredigital/homebrew-tap
```

- [ ] **Step 2: Update README.md brew install commands**

In `README.md` lines 206-207, change:
```
brew tap antoniolg/tap
brew install antoniolg/tap/postflow
```
to:
```
brew tap saredigital/tap
brew install saredigital/tap/postflow
```

- [ ] **Step 3: Update RELEASING.md tap reference**

In `docs/RELEASING.md` line 79, change:
```
- `Release CLI Homebrew` publishes a formula to `antoniolg/homebrew-tap`
```
to:
```
- `Release CLI Homebrew` publishes a formula to `saredigital/homebrew-tap`
```

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release-cli-homebrew.yml README.md docs/RELEASING.md
git commit -m "refactor: update homebrew tap to saredigital/homebrew-tap"
```

---

### Task 4: Update docs URLs (README, coolify-deploy, RELEASING)

**Files:**
- Modify: `README.md:22-23,257,263`
- Modify: `docs/coolify-deploy.md:8,18,26`
- Modify: `docs/RELEASING.md:80,84-87`

- [ ] **Step 1: Update README.md clone URL**

In `README.md` lines 22-23, change:
```
git clone https://github.com/antoniolg/postflow.git
cd postflow
```
to:
```
git clone https://github.com/saredigital/sarepost.git
cd sarepost
```

- [ ] **Step 2: Update README.md GHCR URLs**

In `README.md` line 257, change:
```
ghcr.io/antoniolg/postflow:latest
```
to:
```
ghcr.io/saredigital/sarepost:latest
```

In `README.md` line 263, change:
```
ghcr.io/antoniolg/postflow:vX.Y.Z
```
to:
```
ghcr.io/saredigital/sarepost:vX.Y.Z
```

- [ ] **Step 3: Update coolify-deploy.md GHCR URLs**

In `docs/coolify-deploy.md` line 8, change:
```
2. Image: `ghcr.io/antoniolg/postflow:latest` (or pin a tag like `ghcr.io/antoniolg/postflow:vX.Y.Z`).
```
to:
```
2. Image: `ghcr.io/saredigital/sarepost:latest` (or pin a tag like `ghcr.io/saredigital/sarepost:vX.Y.Z`).
```

In `docs/coolify-deploy.md` line 18, change:
```
1. In Coolify, create a new service from Git repository `antoniolg/postflow`.
```
to:
```
1. In Coolify, create a new service from Git repository `saredigital/sarepost`.
```

In `docs/coolify-deploy.md` line 26, change:
```
- GitHub Actions publishes Docker images to `ghcr.io/antoniolg/postflow`.
```
to:
```
- GitHub Actions publishes Docker images to `ghcr.io/saredigital/sarepost`.
```

- [ ] **Step 4: Update RELEASING.md GHCR URLs**

In `docs/RELEASING.md` line 80, change:
```
- `Release Docker Image` publishes `ghcr.io/antoniolg/postflow:<tag>`
- `Release Docker Image` also refreshes `ghcr.io/antoniolg/postflow:latest`
```
to:
```
- `Release Docker Image` publishes `ghcr.io/saredigital/sarepost:<tag>`
- `Release Docker Image` also refreshes `ghcr.io/saredigital/sarepost:latest`
```

- [ ] **Step 5: Commit**

```bash
git add README.md docs/coolify-deploy.md docs/RELEASING.md
git commit -m "docs: update URLs to saredigital/sarepost"
```

---

### Task 5: Rename logo files and update template references

**Files:**
- Rename: `internal/api/assets/icons/postflow-logo-header-transparent-64.png` → `sarepost-logo-header-transparent-64.png`
- Rename: `internal/api/assets/icons/postflow-logo-4096.png` → `sarepost-logo-4096.png`
- Modify: `internal/api/templates/login.html:174`
- Modify: `internal/api/templates/schedule.html:3974`

- [ ] **Step 1: Rename logo files**

```bash
mv internal/api/assets/icons/postflow-logo-header-transparent-64.png internal/api/assets/icons/sarepost-logo-header-transparent-64.png
mv internal/api/assets/icons/postflow-logo-4096.png internal/api/assets/icons/sarepost-logo-4096.png
```

- [ ] **Step 2: Update login.html logo reference**

In `internal/api/templates/login.html` line 174, change:
```
      <img class="logo-mark" src="/assets/icons/postflow-logo-header-transparent-64.png" alt="" aria-hidden="true" />
```
to:
```
      <img class="logo-mark" src="/assets/icons/sarepost-logo-header-transparent-64.png" alt="" aria-hidden="true" />
```

- [ ] **Step 3: Update schedule.html logo reference**

In `internal/api/templates/schedule.html` line 3974, change:
```
        <img class="logo-mark" src="/assets/icons/postflow-logo-header-transparent-64.png" alt="" aria-hidden="true" />
```
to:
```
        <img class="logo-mark" src="/assets/icons/sarepost-logo-header-transparent-64.png" alt="" aria-hidden="true" />
```

- [ ] **Step 4: Commit**

```bash
git add internal/api/assets/icons/sarepost-logo-header-transparent-64.png internal/api/assets/icons/sarepost-logo-4096.png internal/api/templates/login.html internal/api/templates/schedule.html
git add internal/api/assets/icons/postflow-logo-header-transparent-64.png internal/api/assets/icons/postflow-logo-4096.png
git commit -m "refactor: rename logos to sarepost and update template references"
```

---

### Task 6: Update branding strings (MCP name, server log, CLI description, skill)

**Files:**
- Modify: `internal/api/mcp.go:25`
- Modify: `cmd/postflow-server/main.go:117`
- Modify: `internal/cli/run.go:601`
- Rename: `skills/postflow-cli/SKILL.md` → `skills/sarepost-cli/SKILL.md`

- [ ] **Step 1: Update MCP server name**

In `internal/api/mcp.go` line 25, change:
```
		Name:    "postflow-mcp",
```
to:
```
		Name:    "sarepost-mcp",
```

- [ ] **Step 2: Update server startup log message**

In `cmd/postflow-server/main.go` line 117, change:
```
	slog.Info("postflow listening", "addr", ":"+cfg.Port, "log_level", cfg.LogLevel)
```
to:
```
	slog.Info("sarepost listening", "addr", ":"+cfg.Port, "log_level", cfg.LogLevel)
```

- [ ] **Step 3: Update CLI description**

In `internal/cli/run.go` line 601, change:
```
	fmt.Fprintln(w, `postflow - CLI for SareDigital HTTP API
```
to:
```
	fmt.Fprintln(w, `postflow - CLI for Sarepost HTTP API
```

- [ ] **Step 4: Rename skill directory**

```bash
mv skills/postflow-cli skills/sarepost-cli
```

- [ ] **Step 5: Update skill SKILL.md content**

In `skills/sarepost-cli/SKILL.md`, make these changes:

Line 2: Change `name: postflow-cli` to `name: sarepost-cli`

Line 3: Change `description: Use the postflow CLI to manage scheduled posts, validate/create posts, and operate DLQ entries against the PostFlow HTTP API. Use when the user asks to inspect schedule, create posts, or requeue failed publications quickly from terminal.` to `description: Use the postflow CLI to manage scheduled posts, validate/create posts, and operate DLQ entries against the Sarepost HTTP API. Use when the user asks to inspect schedule, create posts, or requeue failed publications quickly from terminal.`

Line 9: Change `# PostFlow CLI` to `# Sarepost CLI`

Line 11: Change `Use this skill to operate PostFlow from terminal via \`postflow\` (HTTP API, no MCP required).` to `Use this skill to operate Sarepost from terminal via \`postflow\` (HTTP API, no MCP required).`

Line 13: Change `For Antonio's workflows, this is the canonical/default path for social publishing. Prefer the CLI over direct API calls unless you are debugging a CLI failure.` to `Prefer the CLI over direct API calls unless you are debugging a CLI failure.`

Line 107: Change `- PostFlow preserves classic Markdown emphasis in post text. When the copy needs emphasis, keep \`**bold**\` and \`*italic*\` in the payload instead of stripping them.` to `- Sarepost preserves classic Markdown emphasis in post text. When the copy needs emphasis, keep \`**bold**\` and \`*italic*\` in the payload instead of stripping them.`

- [ ] **Step 6: Verify build still passes**

```bash
go build ./...
go test ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/api/mcp.go cmd/postflow-server/main.go internal/cli/run.go skills/sarepost-cli/
git add skills/postflow-cli/
git commit -m "refactor: update branding strings to sarepost"
```

---

### Task 7: Full gate verification

**No file changes — verification only.**

- [ ] **Step 1: Run full test suite**

```bash
go test ./...
```

Expected: All tests pass.

- [ ] **Step 2: Run race detector**

```bash
go test -race ./...
```

Expected: All tests pass, no race conditions.

- [ ] **Step 3: Run coverage check**

```bash
./scripts/check-coverage.sh
```

Expected: Coverage thresholds met.

- [ ] **Step 4: Run linter**

```bash
golangci-lint run ./...
```

Expected: No new lint issues.

- [ ] **Step 5: Run vulnerability check**

```bash
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Expected: No vulnerabilities.

- [ ] **Step 6: Verify go.mod is tidy**

```bash
go mod tidy
git diff --exit-code go.mod go.sum
```

Expected: No changes to go.mod or go.sum (already tidy).

- [ ] **Step 7: Verify no remaining old references**

```bash
rg "antoniolg/postflow" --type-not binary
```

Expected: No output (all references updated).

> Note: The git remote URL in `.git/config` will still point to the old repo. This is intentional — the remote should be updated when the new repo is created. Run: `git remote set-url origin https://github.com/saredigital/sarepost.git`
