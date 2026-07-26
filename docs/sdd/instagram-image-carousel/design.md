# Design: Instagram image carousel

## Decision

Reuse `Post.Media` as the source of carousel membership. An Instagram root post with one media item retains its existing path; one with 2--10 images follows the carousel path. No `post_format` field is introduced because carousel intent is fully determined by the account platform and media count in the accepted v1 scope.

`post_media` gains an integer `position` column. All creation and replacement paths insert the zero-based input index and `GetPost` orders by it.

## Provider flow

1. Resolve a public URL for every image.
2. Create an Instagram child container for each image with `image_url` and `is_carousel_item=true`.
3. Create a parent container with `media_type=CAROUSEL`, `children` as the child IDs in order, and the post caption.
4. Publish the parent container through the existing `media_publish` endpoint.

## Alternatives considered

| Alternative | Decision |
|---|---|
| Infer order from `media.created_at` | Rejected: uploaded time is not editorial order and can collide. |
| Add a universal `post_format` field | Rejected: it is redundant for image-only Instagram carousels and would expand every surface. |
| Support image and video slides immediately | Deferred: video child containers introduce readiness polling and partial-processing behavior. |
