# Repo Reorg Plan (Draft)

Goal: align code layout with the feature taxonomy while keeping a clear split between user-facing API and internal implementation.

## Principles
- Keep user-facing API under `targ/` (public and documented).
- Move non-user-facing code under `internal/` (implementation details).
- Keep tooling under `dev/` and docs under `docs/`.
- Preserve current behavior; no functional changes during reorg unless required.

## Proposed Top-Level Layout
| Directory | Purpose |
| --- | --- |
| `cmd/` | Installable CLI(s), including `targ` |
| `targ/` | Public library API expected for direct-binary use |
| `internal/` | Implementation details for discovery, build-tool mode, parsing, caching |
| `docs/` | Design notes, taxonomy, architecture docs |
| `dev/` | Project tooling (issue tracker, scripts, build helpers) |

## Proposed File Structure (Draft)
```
cmd/
  targ/
    main.go
    main_test.go
docs/
  design.md
  feature_taxonomy.md
  plan.md
  thoughts.md
  thoughts2.md
  thoughts4.md
dev/
  issuefile/
    issuefile.go
    issuefile_test.go
  issues/
    issues.go
    issues_test.go
internal/
  args/
    parse_value.go
  buildtool/
    discover.go
    discover_test.go
    generate.go
    generate_test.go
    select_test.go
  completion/
    format.go
  exec/
    run_env.go
    run_env_mock_test.go
  fs/
    match.go
    match_test.go
  help/
    command_meta.go
  target/
    checksum.go
    checksum_test.go
    newer.go
    newer_test.go
    watch.go
    watch_test.go
targ/
  completion.go
  deps.go
  deps_test.go
  doc.go
  target.go
  targs.go
  targs_test.go
```

## Taxonomy Alignment (Public vs Internal)
| Taxonomy Area | Public API (`targ/`) | Internal (`internal/`) |
| --- | --- | --- |
| Command Model | Run helpers, command structs, naming | reflection discovery, command graph |
| Arguments & Parsing | tag types, defaults, env, enums | parsing engine, validation |
| Help & Completion | help generation entrypoints, completion API | formatters, template rendering |
| Build-Tool Mode | CLI flags and config structs | module bootstrap, caching, discover/generate |
| Build Helpers | public helpers (newer/watch/checksum) | filesystem/glob internals |

## Likely Moves (Initial Pass)
| Current Path | Proposed Path | Rationale |
| --- | --- | --- |
| `completion.go` | `targ/completion.go` | Public helper for direct-binary completion |
| `buildtool/*` | `internal/buildtool/*` | Implementation detail of build-tool mode |
| `internal/issuefile/*` | `dev/issuefile/*` | Dev tooling |
| `tools/issues/*` | `dev/issues/*` | Dev tooling |
| `feature_taxonomy.md` | `docs/feature_taxonomy.md` | Documentation |
| `thoughts*.md` | `docs/` | Design docs |
| `design.md` | `docs/` | Design docs |
| `plan.md` | `docs/plan.md` | Planning doc |

## Open Questions
- Which items in `targ/` are truly public vs internal? (e.g., help formatting helpers)
- Should build helpers (newer/watch/checksum) live in `targ/build/` or `targ/targets/`?
- Does `cmd/targ` import anything currently considered internal?

## Next Steps
1. Confirm public API surface intended for direct-binary users.
2. Decide final taxonomy buckets under `targ/` (e.g., `targ/build`, `targ/args`, `targ/help`).
3. Execute the moves in small, testable slices; update imports and docs.
4. Run tests and update taxonomy doc to match final layout.
