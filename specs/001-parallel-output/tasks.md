# Tasks: Parallel Output

**Input**: Design documents from `/specs/001-parallel-output/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/api.md

**Tests**: TDD is mandatory per project conventions. Each implementation task follows the red/green/refactor cycle: write failing test, implement minimally, refactor. Test commands use `go test -tags sqlite_fts5`.

**Organization**: Tasks grouped by user story. US1 and US6 are merged (Print/Printf IS the mechanism for readable output).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to
- Include exact file paths in descriptions
- Each task is a complete TDD cycle (test first, then implement)

## Path Conventions

Go single-module project:
- Source: `internal/core/`, `internal/sh/`
- Public API: `targ.go` (root)
- Tests: `*_test.go` files colocated with source (blackbox: `package core_test`)

---

## Phase 1: Setup

**Purpose**: No setup required — existing Go project with established structure and test infrastructure.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core building blocks that MUST be complete before ANY user story can be implemented. These are standalone types with no external dependencies.

- [X] T001 [P] Create ExecInfo context metadata type with WithExecInfo/GetExecInfo in internal/core/exec_info.go (+ internal/core/exec_info_test.go)
- [X] T002 [P] Create Printer goroutine with NewPrinter/Send/Close in internal/core/printer.go (+ internal/core/printer_test.go)

**Checkpoint**: ExecInfo and Printer are independently testable. Both pass unit tests before proceeding.

---

## Phase 3: User Story 1 + User Story 6 — Readable Parallel Output + Context-Aware Print (Priority: P1) MVP

**Goal**: Every line of output from a parallel target is prefixed with `[target-name]` and no lines interleave. Print functions auto-detect parallel vs. serial mode. Prefixes are right-padded for alignment.

**Independent Test**: Run two targets in parallel that each produce output via `targ.Print`. Verify every line is prefixed with the correct target name, columns align, and no interleaving occurs. Run the same targets in serial mode and verify no prefix appears.

**Covers**: FR-001, FR-002, FR-010, FR-011, FR-012, FR-014, SC-001, SC-004, SC-005

### Implementation

- [X] T003 [US1] Create Print/Printf with parallel-aware context detection in internal/core/print.go (+ internal/core/print_test.go). Serial mode writes directly; parallel mode prefixes each line and sends to Printer.
- [X] T004 [US1] Add FormatPrefix with right-padding (compute max name length, pad with spaces) in internal/core/print.go. Exported as FormatPrefix for use by executors and PrefixWriter.
- [X] T005 [US1] Wire ExecInfo injection and Printer lifecycle into dep-level parallel executor (runGroupParallel) in internal/core/target.go. Create Printer with env's output writer, inject ExecInfo per target goroutine, close Printer after all goroutines complete.
- [X] T006 [US1] Wire ExecInfo injection and Printer lifecycle into top-level parallel executors (executeDefaultParallel, executeMultiRootParallel) in internal/core/run_env.go. Same pattern as T005.
- [X] T007 [US1] Re-export Print and Printf from targ.go as public API.
- [X] T008 [US1] Integration test for prefixed parallel output in internal/core/parallel_output_test.go. Test dep-level and top-level parallel execution produce prefixed, aligned, non-interleaved output. Test serial mode is unchanged.

**Checkpoint**: Parallel dep-level and top-level execution produce prefixed output. Serial mode is unchanged. US1 and US6 acceptance scenarios pass.

---

## Phase 4: User Story 2 — Per-Target Result Tracking (Priority: P1)

**Goal**: After all parallel targets complete, a summary line shows non-zero counts per status category (PASS, FAIL, CANCELLED, ERRORED). Individual target stop messages appear before the summary.

**Independent Test**: Run targets with known outcomes (all pass, one fails) and verify the summary line shows correct counts with only non-zero categories.

**Covers**: FR-003, FR-004, FR-015, FR-016, SC-002

### Implementation

- [X] T009 [P] [US2] Create Result type (Pass/Fail/Cancelled/Errored), TargetResult struct, ClassifyResult function, and FormatSummary in internal/core/result.go (+ internal/core/result_test.go). ClassifyResult uses error type + isFirstFailure flag.
- [X] T010 [US2] Wire result collection into parallel executors in internal/core/target.go (runGroupParallel) and internal/core/run_env.go (executeDefaultParallel, executeMultiRootParallel). Collect TargetResult per goroutine, classify after completion, print summary after Printer.Close(). Return first error for non-zero exit code (FR-015).
- [X] T011 [US2] Re-export Result, Pass, Fail, Cancelled, Errored from targ.go.
- [X] T012 [US2] Integration test for result tracking and summary line in internal/core/parallel_output_test.go. Test all-pass shows PASS:N. Test mixed outcomes show correct counts. Test summary appears after stop messages (FR-016).

**Checkpoint**: Parallel execution produces accurate summary lines. Exit code is non-zero on failure.

---

## Phase 5: User Story 3 — Fail-Fast with Cancellation (Priority: P2)

**Goal**: When any target terminates with a non-pass outcome, remaining targets are cancelled promptly. Cancelled targets are classified as CANCELLED (not FAIL). Timed-out targets are classified as ERRORED.

**Independent Test**: Run a fast-failing target alongside a slow target. Verify the slow target is cancelled within 1 second and reported as CANCELLED. Run a target with a short timeout and verify it reports ERRORED.

**Covers**: FR-005, FR-006, FR-007, SC-003

### Implementation

- [X] T013 [US3] Enhance parallel executor wiring to correctly classify ERRORED (context.DeadlineExceeded) and CANCELLED (context.Canceled when not first failure) in internal/core/target.go and internal/core/run_env.go. Ensure any non-pass outcome triggers cancel (not just FAIL).
- [X] T014 [US3] Integration test for fail-fast with status classification in internal/core/parallel_output_test.go. Test fast-fail cancels slow sibling (CANCELLED). Test timeout produces ERRORED. Test first-failure target is FAIL not CANCELLED.

**Checkpoint**: Fail-fast works for FAIL and ERRORED. CANCELLED/ERRORED distinction is correct in summary.

---

## Phase 6: User Story 4 — Shell Command Output Capture (Priority: P2)

**Goal**: Shell command stdout and stderr are captured and prefixed with the target name in parallel mode, just like programmatic output. Partial lines are flushed on completion.

**Independent Test**: Run a shell-command target (`echo hello`) in parallel and verify output lines are prefixed. Run a multi-line shell command and verify each line is individually prefixed.

**Covers**: FR-008, FR-009, SC-006

### Implementation

- [X] T015 [P] [US4] Create PrefixWriter io.Writer adapter with line buffering and Flush in internal/core/prefix_writer.go (+ internal/core/prefix_writer_test.go). Buffers partial lines, sends complete prefixed lines to Printer. Flush emits remaining partial content with trailing newline.
- [X] T016 [US4] Wire PrefixWriter into shell command execution in internal/core/target.go (runShellCommand) or internal/core/command.go (executeShellCommand). In parallel mode, create PrefixWriter as cmd.Stdout and cmd.Stderr. Flush after command completes. Export DefaultCleanup from internal/sh/sh.go if needed for ShellEnv construction.
- [X] T017 [US4] Integration test for shell command prefixed output in internal/core/parallel_output_test.go. Test single-line shell output is prefixed. Test multi-line shell output has each line prefixed. Test partial line (no trailing newline) is flushed with prefix.

**Checkpoint**: Shell-command targets and programmatic targets produce identically formatted prefixed output (SC-006).

---

## Phase 7: User Story 5 — Lifecycle Announcements (Priority: P3)

**Goal**: Each target announces start and stop (with result and duration) in parallel mode. Defaults are provided; custom hooks override them.

**Independent Test**: Run a target in parallel and verify `[name] starting...` and `[name] PASS (Xs)` messages appear. Configure custom hooks and verify they fire instead of defaults.

**Covers**: FR-013

### Implementation

- [X] T018 [US5] Add onStart/onStop fields, OnStart/OnStop builder methods, and GetOnStart/GetOnStop getters to Target struct in internal/core/target.go (+ test in internal/core/target_test.go).
- [X] T019 [US5] Wire OnStart/OnStop hook calls into parallel executors in internal/core/target.go (runGroupParallel) and internal/core/run_env.go. Fire onStart before target execution, onStop after with Result and duration. Use defaults when hooks are nil.
- [X] T020 [US5] Re-export OnStart, OnStop builder methods from targ.go (already accessible via *Target, but ensure Result type is available for hook signatures).
- [X] T021 [US5] Integration test for lifecycle messages in internal/core/parallel_output_test.go. Test default start/stop messages appear. Test custom hooks override defaults. Test duration is included in stop message.

**Checkpoint**: All lifecycle messages appear in correct order (start → output → stop → summary). Custom hooks work.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final validation across all user stories

- [X] T022 Run full regression test suite: `go test -tags sqlite_fts5 ./...`
- [X] T023 Validate backward compatibility: run existing tests with serial-mode targets and verify zero output changes (SC-004)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — start immediately. T001 and T002 run in parallel.
- **US1+US6 (Phase 3)**: Depends on Phase 2 (ExecInfo + Printer). BLOCKS US2, US3, US5 (they need wiring infrastructure).
- **US2 (Phase 4)**: Depends on Phase 3 (parallel executors wired). T009 (Result type) can start in parallel with Phase 3 since it's a separate file.
- **US3 (Phase 5)**: Depends on Phase 4 (Result classification in executors).
- **US4 (Phase 6)**: Depends on Phase 2 (Printer). T015 (PrefixWriter) can start in parallel with Phase 3. T016 depends on Phase 3 (executors wired).
- **US5 (Phase 7)**: Depends on Phase 4 (Result type for OnStop signature).
- **Polish (Phase 8)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1+US6 (P1)**: First story. Establishes the parallel output infrastructure.
- **US2 (P1)**: Depends on US1 (parallel executors must exist). Can start T009 early.
- **US3 (P2)**: Depends on US2 (classification logic). Enhances existing behavior.
- **US4 (P2)**: Independent of US2/US3. Can start T015 early. T016 depends on US1 wiring.
- **US5 (P3)**: Depends on US2 (Result type). Independent of US3/US4.

### Within Each User Story

- Unit-level components (TDD cycle) before wiring
- Wiring before integration tests
- Re-exports alongside or after wiring
- Integration test validates the complete story

### Parallel Opportunities

```text
Phase 2:  T001 ──┐
          T002 ──┤ (parallel: different files)
                 │
Phase 3:  T003 ──┤ (sequential within phase)
          T004 ──┤
          T005 ──┤
          T006 ──┤
          T007 ──┤
          T008 ──┘
                 │
          T009 ──┤ (can start during Phase 3 — separate file)
                 │
Phase 4:  T010 ──┤
          T011 ──┤
          T012 ──┘
                 │
Phase 5:  T013 ──┤
          T014 ──┘
                 │
          T015 ──┤ (can start during Phase 3 — separate file)
                 │
Phase 6:  T016 ──┤
          T017 ──┘
                 │
Phase 7:  T018 ──┤
          T019 ──┤
          T020 ──┤
          T021 ──┘
                 │
Phase 8:  T022 ──┤
          T023 ──┘
```

---

## Parallel Example: Foundational + Early Starts

```bash
# Launch foundational components in parallel:
T001: "Create ExecInfo in internal/core/exec_info.go"
T002: "Create Printer in internal/core/printer.go"

# After foundational, these can start in parallel (different files):
T003: "Create Print/Printf in internal/core/print.go"      # US1 - needs T001, T002
T009: "Create Result type in internal/core/result.go"        # US2 - no deps on T001/T002
T015: "Create PrefixWriter in internal/core/prefix_writer.go" # US4 - needs T002 only
```

---

## Implementation Strategy

### MVP First (US1 + US6 Only)

1. Complete Phase 2: Foundational (T001, T002)
2. Complete Phase 3: US1+US6 (T003–T008)
3. **STOP and VALIDATE**: Parallel output is prefixed, aligned, atomic. Serial mode unchanged.
4. This is a usable increment — developers can identify output sources.

### Incremental Delivery

1. Foundational → US1+US6 → **MVP: Readable parallel output**
2. Add US2 → **Result tracking with summary line**
3. Add US3 → **Fail-fast with correct CANCELLED/ERRORED classification**
4. Add US4 → **Shell commands get prefixed output too**
5. Add US5 → **Lifecycle announcements with hooks**
6. Each story adds value without breaking previous stories.

### Parallel Agent Strategy

With multiple agents:

1. All agents complete Foundational (Phase 2) together
2. Once Foundational is done:
   - Agent A: US1+US6 (Phase 3) — critical path
   - Agent B: T009 (Result type) + T015 (PrefixWriter) — early starts
3. After Phase 3:
   - Agent A: US2 wiring (Phase 4)
   - Agent B: US4 shell wiring (Phase 6, needs Phase 3)
4. After Phase 4:
   - Agent A: US3 (Phase 5)
   - Agent B: US5 (Phase 7)

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [Story] label maps task to specific user story for traceability
- Each task is a complete TDD cycle: write failing test → implement → verify pass
- Test command: `go test -tags sqlite_fts5 ./internal/core/ -run TestName -v`
- Full suite: `go test -tags sqlite_fts5 ./...`
- Blackbox tests in `package core_test`
- Commit after each task or logical group
- Detailed implementation code is in `docs/plans/2026-02-19-parallel-output.md`
