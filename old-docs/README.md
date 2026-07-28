# old-docs

Legacy documentation, migrating into `openspec/specs/`. **Authoritative only where
verified correct against the code** — anything here may be stale; check the source
before relying on it.

- `traced/` — a four-layer traced record (use cases → requirements/design →
  architecture → tests → implementation; `state.toml` holds the item rosters and
  layer topology). The most recently maintained of the two sets.
- `first-gen/` — the original architecture, design, requirements, task and issue
  docs, plus a research note on `go.mod` tree traversal. Older; predates much of
  the code it describes.
- `session-learning-prompt.md`, `state.toml`, `updates.jsonl` — process artifacts
  from earlier workflow runs.

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
