---
name: sare-new-app
description: "Trigger: new app, from scratch, MVP, scaffold, greenfield. Build the first real vertical slice."
license: Apache-2.0
metadata:
  author: "asierluengo"
  version: "1.0"
---

## Activation Contract

Load when creating a new app, MVP, prototype intended to evolve, or repo scaffold from scratch.

## Hard Rules

- Build the actual usable first screen or workflow, not a marketing placeholder.
- Choose boring, maintainable architecture unless the product requires otherwise.
- Do not add infrastructure before the first vertical slice needs it.
- Establish verification on day one: test, lint, type-check, build, or browser checks.
- Keep secrets out of files and logs.

## Decision Gates

| Need | Action |
|---|---|
| Unknown product slice | Ask one question about the first user workflow. |
| Web app | Prefer existing project conventions; otherwise choose a mainstream stack with fast local verification. |
| Backend/API | Define resource model, contract, persistence boundary, and failure modes first. |
| Full-stack app | Build one vertical slice across UI, API, data, and verification. |
| Prototype only | Mark shortcuts explicitly and avoid permanent architecture. |

## Execution Steps

1. Define the first useful workflow and non-goals.
2. Pick stack and architecture with explicit tradeoffs.
3. Scaffold only what the slice needs.
4. Implement the vertical slice.
5. Add minimal tests or executable verification.
6. Run the app locally when relevant and provide the URL.
7. Document setup, scripts, and next architectural decisions.

## Output Contract

Return stack choice, created files, local run command or URL, verification evidence, and next recommended slice.

## References

None.

