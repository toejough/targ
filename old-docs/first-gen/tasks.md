# Implementation Tasks

The `internal/help` builder package this file scaffolded (TASK-001 through TASK-043,
TASK-045) shipped, under evolved names, in the current `internal/help` package. Those
44 entries were removed 2026-07-28 as resolved.

One task remains open: **TASK-044** (lint rule preventing direct ANSI escape codes
outside `internal/help`) was never implemented — the invariant holds by convention
only, nothing enforces it. Tracked as GitHub issue #66.
