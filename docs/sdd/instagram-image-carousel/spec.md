# Spec: Instagram image carousel

## Intent

Allow a scheduled or immediate Instagram publication to contain an ordered carousel of images, while preserving the existing single-image and single-video (Reel) behavior.

## Scope

### In

- Publish an Instagram carousel when a root post has 2 to 10 supported images.
- Preserve the media order supplied when a post is created or edited.
- Reject Instagram carousels with video or unsupported media in this first version.
- Validate and publish the carousel through the existing HTTP API, MCP, CLI, Web UI, and worker flows.

### Out

- Generating a Reel video from images.
- Mixed image/video Instagram carousels.
- Video-only carousels.
- New user-facing post-format controls; the Instagram provider derives carousel mode from the ordered media list.

## Requirements

### R1: Ordered post media

The system MUST persist and return each post's media in the caller-provided order.

#### Scenario: Create an ordered post

Given a post created with media IDs `[first, second, third]`
When the post is read for preview or publishing
Then its media order is `[first, second, third]`.

### R2: Instagram carousel validation

The system MUST accept an Instagram root post with 2 to 10 JPEG or PNG images, and reject multi-media posts containing non-images.

#### Scenario: Validate an image carousel

Given a connected Instagram account and two PNG images
When the draft is validated
Then validation succeeds.

#### Scenario: Reject a mixed carousel

Given a connected Instagram account, one PNG image, and one MP4 video
When the draft is validated
Then validation fails with an actionable image-only carousel error.

### R3: Instagram carousel publication

The system MUST publish an accepted image carousel by creating one child media container per image, creating a parent carousel container with those children in order, and publishing the parent container.

#### Scenario: Publish an ordered carousel

Given an Instagram post with two public PNG image URLs
When the worker publishes the post
Then Meta receives two child containers marked as carousel items, followed by one carousel container containing their IDs in order, followed by one publish request for that parent.

### R4: Backwards compatibility

The system MUST retain the current behavior for an Instagram post with exactly one image or exactly one video.

## Acceptance Criteria

- [ ] R1: Creation and edit tests prove media order is preserved.
- [ ] R2: Provider and API validation accept 2--10 images and reject invalid image carousel inputs.
- [ ] R3: Provider HTTP test proves correct child, parent, and publish requests.
- [ ] R4: Existing Instagram image and Reel tests remain green.

## Verification Strategy

- Unit: focused `internal/postflow` and `internal/db` tests.
- Integration: API validation test for a multi-image Instagram post.
- E2E/manual: not required; the HTTP adapter is covered with an `httptest` Meta API server.
- Build/type-check: `go test ./...`.

## Risks And Tradeoffs

| Risk | Mitigation |
|---|---|
| Meta can fail after some child containers are created | Treat the provider operation as failed; existing retry/DLQ behavior remains the recovery mechanism. |
| Creation order differs from upload time | Store an explicit position on `post_media`; do not infer it from media timestamps. |
| Supporting video carousel items increases container readiness and retry complexity | Restrict v1 to images only. |
