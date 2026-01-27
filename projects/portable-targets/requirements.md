# Portable Targets - Product Specification

## Problem Statement

Developers using targ across multiple Go projects copy-paste build/test/lint targets between projects and manually adapt them. Improvements don't propagate, local tweaks blend with generally useful changes, and keeping projects in sync is "manual version control without git." Targ has remote sync, but targets are too project-specific (hardcoded paths, thresholds, allowed dependencies) to be portable.

### Who is affected

Go developers using targ across multiple projects. The immediate user is the targ author across personal projects, but the solution should be generic enough for any targ user who wants to share workflows across repos or pull from community-maintained target collections.

### Impact

Without solving this:
- Bug fixes and improvements to build workflows must be manually propagated to every project
- Developers spend time on mental diffs ("what did I change here for this project vs. what's generally useful?")
- Remote sync - a key targ feature - is impractical for real-world use
- New projects require significant setup time adapting copied targets
- The README can't credibly say "try out targ's sync" because it doesn't work well for non-trivial targets

## Current State

### User Journey

1. Start new Go project
2. Copy `dev/targets.go` and `dev/golangci-*.toml` from an existing project (e.g., targ itself)
3. Manually edit to replace project-specific bits:
   - Package paths in test/coverage targets
   - Allowed dependencies in depguard lint config
   - Coverage thresholds
   - Entry point / skip path lists
4. Over time, improve targets in whichever project you're actively working on
5. Periodically (when taking a break from the actual project), try to sync improvements back to other projects
6. Struggle to identify which changes were project-specific vs. generally useful
7. Reconcile similar-but-not-identical changes made independently in different projects

### Pain Points

- **No propagation**: Fixing a bug in one project's coverage check doesn't help other projects
- **Mental diff burden**: Must manually separate local customization from general improvements
- **Drift**: Projects diverge silently over time
- **Config files too**: Not just target code - golangci configs have project-specific and generic parts interleaved
- **No tooling**: The process is entirely manual, like version control without git

### Constraints

- Targ's current sync mechanism exists but may need changes to support the full vision
- Targets are Go code with a `//go:build targ` tag, living in `dev/` directories
- Config files (TOML) are separate from target code and have different portability characteristics

## Desired Future State

### Success Criteria

1. A developer can sync targets from multiple remote sources into a single project
2. Synced targets work out of the box for projects following Go conventions, or with env var overrides for non-standard projects
3. Re-syncing pulls upstream improvements without losing local customizations
4. It is always clear where a target came from and what is overriding what
5. Config files (golangci TOML, etc.) can be shared alongside targets
6. A developer can opt-in to a few targets from a large collection, OR opt-out of a few from a large collection - whichever is less work for their case
7. The override model is simple with few, clear locations (avoid the SPF13 vim problem of too many injection/override points that become impossible to track)

### User Stories

- As a developer starting a new Go project, I want to sync a standard set of lint/test/coverage targets so that I get working CI workflows immediately
- As a developer maintaining multiple projects, I want to re-sync upstream target improvements so that bug fixes propagate without manual effort
- As a developer with project-specific needs, I want to override specific settings (thresholds, paths, allowed deps) so that generic targets work in my context
- As a developer debugging a build issue, I want to see exactly where each target and override comes from so that I can trace unexpected behavior
- As a developer choosing targets, I want to pull from multiple sources (linting from one, testing from another) so that I can compose the best toolkit
- As a developer with 28/30 useful targets from a source, I want to exclude just the 2 I don't need rather than listing all 28
- As a developer with 1/30 useful targets from a source, I want to include just the 1 rather than excluding 29

### Acceptance Criteria

- Targets synced from a remote source work without modification in a project following Go conventions (`internal/`, `cmd/`, standard test layout)
- Non-conventional projects can override specific values via a clear, single mechanism (env vars, config file, or similar)
- Re-sync detects upstream changes and merges them, preserving local overrides
- Conflicts between upstream changes and local overrides are surfaced clearly (error, not silent override)
- When two remote sources provide targets with the same name, targ errors rather than silently picking one
- The user can see a manifest of what came from where (source attribution)

## Edge Cases

### Error Scenarios

- **Two remotes provide same target name**: Error with clear message naming both sources. User must resolve (exclude from one, rename, etc.)
- **Synced target references env var that isn't set**: Error with message naming the variable, what it configures, and the convention it defaults to (if any)
- **Remote source disappears or changes URL**: Sync fails with clear message. Local copies continue to work (they're already in the repo)
- **Local override references a target that no longer exists upstream**: Error or warning on next sync

### Boundary Conditions

- Project with zero local overrides (pure convention): Everything works with no config
- Project with every target overridden: Effectively local targets, sync is a no-op (no value, but shouldn't break)
- Syncing from a source with 0 targets: No-op, possibly a warning
- Syncing from a source with 100+ targets: Must be performant, opt-out model must scale
- Re-syncing when upstream made no changes: No-op
- Re-syncing when local made no changes: Clean update

### Invariants

- Local targets always take precedence over synced targets with the same name
- Synced target files are always present in the repo (not fetched at runtime)
- The user can always see the full list of active targets and their sources
- Transparency: no magic. Every decision targ makes about target resolution is traceable

## Solution Guidance

### Approaches to Consider

- **Convention + env vars**: Targets use env vars for project-specific values, with sensible defaults based on Go conventions. `.env` or similar provides local values. This fits with targ's existing env var support for arguments.
- **Lean on git for merge/update**: Synced targets are regular files. Re-sync can be a git-aware operation that shows diffs and lets the user review. The hard problem (merge upstream changes with local tweaks) is what git already solves.
- **Manifest file**: A file tracking what was synced from where, at what version. Enables "what changed upstream since I last synced?" without targ reinventing version tracking.
- **Opt-in/opt-out model**: Config specifies either `include: [target1, target2]` or `exclude: [target3]` per remote source. Small list wins.
- **Config files as templates**: Golangci configs could have a generic base with project-specific sections injected via env vars or includes (TOML doesn't have includes natively, but targ could preprocess).

### Approaches to Avoid

- **Too many override locations**: The SPF13 vim config problem - multiple files and functions that override each other in non-obvious order. Keep it to one or two clear places.
- **Runtime fetching**: Targets must be local files, not fetched from remote at build time. Network dependency in the build is fragile.
- **Silent conflict resolution**: Never silently pick one target over another. Always error on ambiguity.
- **Reinventing git**: Don't build a full VCS merge system. Lean on git's existing diff/merge where possible.
- **Heavyweight configuration**: If the config to manage synced targets becomes more complex than just writing the targets yourself, the feature has failed.

### Constraints

- Must be written in Go
- Transparency is non-negotiable: user must always understand what's happening, where config comes from, and how to debug/override
- Targ's sync mechanism can change if needed to support this
- Must work for the targ author's personal projects now AND be generic enough for the README

### References

- SPF13's vim config (remote defaults + local overrides - good concept, too many injection points in practice)
- Package managers (npm, go modules) for the "multiple sources, version tracking, conflict detection" model
- Git submodules / subtrees (embedding external code in a repo with update tracking)
- ESLint `extends` (layered config inheritance from shared packages)

## Open Questions

1. **What is the right abstraction for "version" of a synced target?** Git commit hash? Semantic version? Content hash? This affects how "what changed upstream?" works.
2. **Should config files (golangci TOML) be synced as whole files or composed from fragments?** Whole files are simpler but less flexible. Fragments allow mixing generic + project-specific but add complexity.
3. **Where does the sync manifest live?** In `dev/`? In a dedicated config file? In `go.mod`-style lockfile?
4. **How does this interact with targ's existing `targ sync` command?** Is this an evolution of that command, or a new mechanism?
5. **Should there be a "starter" repo that targ maintains?** A canonical `github.com/toejough/go-targets` with best-practice Go targets that new projects can sync from?
6. **How do env var defaults work?** Convention-based defaults (e.g., `COVERAGE_THRESHOLD` defaults to 80) or explicit defaults in the target code?
7. **Is "partial sync" (opt-in/opt-out) per-target or per-file?** A single file might define multiple targets. Granularity matters.
