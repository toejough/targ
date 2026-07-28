# old-docs

Legacy documentation, migrating into `openspec/specs/`. **Authoritative only where
verified correct against the code** — anything here may be stale; check the source
before relying on it.

- `traced/` — a four-layer traced record (use cases → requirements/design →
  architecture → tests → implementation). The most recently maintained of the
  two sets. (Its `state.toml`, dead process state for the traced workflow being
  retired by the OpenSpec migration, was deleted 2026-07-28.)
- `first-gen/` — the original architecture, design, and requirements docs, plus
  a research note on `go.mod` tree traversal and a one-line pointer to the sole
  still-open scaffolding task. Older; predates much of the code it describes.
  Its issue backlog (`ISSUE-001`–`ISSUE-021`) was resolved 2026-07-28: 5 had
  shipped, and the other 16 were filed as GitHub issues #50–#65; `TASK-044`
  became #66. The `issues.md` file was deleted.
- `session-learning-prompt.md`, `state.toml` (dead workflow state from an
  abandoned 2026-02-01 run), `updates.jsonl` — process artifacts from earlier
  workflow runs.

## How this directory shrinks

When a claim here is found incorrect, it is **not** edited in place. The incorrect
content is deleted from this directory and its corrected form is written as an
OpenSpec spec under the bounded context it belongs to. A file that empties is
deleted; when this directory empties, it is deleted along with every reference to
it. When what remains here falls below 20% of total spec content, the remainder is
converted in one push.

`README.md`'s Specifications section is the canonical statement of this standard;
the summary above is a convenience copy.

Unrelated to `openspec/changes/archive/`, which is OpenSpec's own store of
completed changes.
