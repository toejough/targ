# Feature Specification: Parallel Output

**Feature Branch**: `001-parallel-output`
**Created**: 2026-02-19
**Status**: Draft
**Input**: User description: "Add prefixed, line-atomic output for parallel target execution with per-target result tracking and summary line"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Readable Parallel Output (Priority: P1)

A developer runs multiple build targets in parallel. Currently, output from all targets interleaves unpredictably, making it impossible to tell which target produced which line. With this feature, each line of output is prefixed with the target name (e.g., `[build] compiling...`), so the developer can instantly identify the source of every message.

**Why this priority**: Without readable output, parallel execution is essentially unusable for debugging or monitoring — the core value proposition of parallel mode is undermined.

**Independent Test**: Can be fully tested by running two targets in parallel that each produce output and verifying every line is prefixed with the correct target name.

**Acceptance Scenarios**:

1. **Given** two targets configured to run in parallel, **When** both targets produce output simultaneously, **Then** every output line is prefixed with `[target-name]` and no line contains interleaved content from multiple targets.
2. **Given** a target named "build" running in parallel mode, **When** it prints "compiling...", **Then** the output line reads `[build] compiling...`.
3. **Given** targets with names of different lengths running in parallel, **When** output is displayed, **Then** prefixes are right-padded so output columns align visually.

---

### User Story 2 - Per-Target Result Tracking (Priority: P1)

After all parallel targets finish, the developer sees a summary line showing how many targets passed, failed, were cancelled, or errored. Each target's final status is also announced individually so the developer knows exactly which target had which outcome.

**Why this priority**: Equally critical to prefixed output — without result tracking, the developer must manually inspect all output to determine if parallel execution succeeded, which defeats the purpose of automation.

**Independent Test**: Can be fully tested by running targets with known outcomes (success, failure) and verifying the summary line shows correct counts.

**Acceptance Scenarios**:

1. **Given** three parallel targets that all succeed, **When** execution completes, **Then** a summary line shows `PASS:3`.
2. **Given** three parallel targets where one fails, **When** execution completes, **Then** the summary line shows counts for each status category (e.g., `PASS:1 FAIL:1 CANCELLED:1`), and only non-zero categories appear.
3. **Given** a target that fails, **When** other targets are cancelled as a result, **Then** the cancelled targets are classified as "CANCELLED" (not "FAIL") in the summary.

---

### User Story 3 - Fail-Fast with Cancellation (Priority: P2)

When one target in a parallel group fails, the system cancels remaining targets promptly rather than waiting for them all to finish. The developer sees which target failed first and which targets were cancelled as a consequence.

**Why this priority**: Important for fast feedback loops, but the system is usable without it (targets would just all run to completion).

**Independent Test**: Can be fully tested by running a fast-failing target alongside a slow target and verifying the slow target is cancelled and reported as such.

**Acceptance Scenarios**:

1. **Given** a parallel group with a fast-failing target and a slow target, **When** the fast target fails, **Then** the slow target is cancelled within a reasonable time.
2. **Given** a target that was cancelled due to another target's failure, **When** results are displayed, **Then** the cancelled target shows status "CANCELLED" and the failing target shows "FAIL".
3. **Given** a target that exceeds a deadline, **When** results are displayed, **Then** that target shows status "ERRORED" (distinct from a normal failure).

---

### User Story 4 - Shell Command Output Capture (Priority: P2)

When a parallel target runs a shell command (e.g., `echo hello`), the command's stdout and stderr are captured and prefixed with the target name, just like programmatic output.

**Why this priority**: Shell commands are a common target type; without this, shell-based targets would produce unprefixed output, breaking the consistency of parallel mode.

**Independent Test**: Can be fully tested by running a shell-command target in parallel and verifying its output lines are prefixed.

**Acceptance Scenarios**:

1. **Given** a shell-command target named "echo-test" running in parallel, **When** the shell command outputs "hello-from-shell", **Then** the output reads `[echo-test] hello-from-shell`.
2. **Given** a shell command that produces multi-line output, **When** running in parallel, **Then** each line is individually prefixed.
3. **Given** a shell command that writes partial lines (no trailing newline), **When** the target completes, **Then** any remaining partial output is flushed with the prefix appended.

---

### User Story 5 - Lifecycle Announcements (Priority: P3)

Each target announces when it starts and when it stops (with its result and duration). These announcements use the same prefixed format and are customizable via hooks.

**Why this priority**: Nice-to-have for observability — the core feature works without lifecycle announcements, but they improve the developer experience for long-running targets.

**Independent Test**: Can be fully tested by running a target and verifying start/stop messages appear with correct prefix and result.

**Acceptance Scenarios**:

1. **Given** a target named "build" starting in parallel mode, **When** no custom hooks are configured, **Then** a default start message like `[build] starting...` appears.
2. **Given** a target completing with status PASS in 1.2 seconds, **When** no custom hooks are configured, **Then** a default stop message like `[build] PASS (1.2s)` appears.
3. **Given** a target with custom start/stop hooks, **When** the target runs in parallel, **Then** the custom hooks fire instead of the defaults.

---

### User Story 6 - Context-Aware Print Functions (Priority: P2)

Target code uses dedicated print functions that automatically detect whether the target is running in parallel or serial mode. In parallel mode, output is prefixed and serialized. In serial mode, output goes directly to the console as before.

**Why this priority**: Essential for backward compatibility — existing targets that use print functions must continue working in serial mode without changes.

**Independent Test**: Can be fully tested by calling the print function in both serial and parallel contexts and verifying the output format differs appropriately.

**Acceptance Scenarios**:

1. **Given** a target calling the print function in serial mode, **When** it prints "hello world", **Then** the output is `hello world` (no prefix, no change from current behavior).
2. **Given** a target calling the print function in parallel mode with name "build", **When** it prints "hello world", **Then** the output is `[build] hello world`.
3. **Given** a target calling the formatted print function in parallel mode, **When** it prints with format arguments, **Then** the output is correctly formatted and prefixed.

---

### Edge Cases

- What happens when a target produces no output at all? The summary line should still include it in the count.
- What happens when many targets write simultaneously to a small output buffer? Output must never be lost; writes may block until buffer space is available.
- What happens when a target writes a very long line without a newline? The line is buffered until a newline arrives or the target completes (at which point the partial line is flushed).
- What happens when parallel execution has only one target? It should still use prefixed output format for consistency.
- What happens when target names contain special characters? Prefixes should wrap the name as-is in brackets.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST prefix every line of output from a parallel target with `[target-name]` followed by a space.
- **FR-002**: System MUST guarantee line atomicity — no output line may contain content from multiple targets.
- **FR-003**: System MUST classify each target's outcome as one of: pass, fail, cancelled, or errored.
- **FR-004**: System MUST print a summary line after all parallel targets complete showing non-zero counts per status category (e.g., `PASS:2 FAIL:1`).
- **FR-005**: System MUST cancel remaining targets when any target in a parallel group terminates with a non-pass outcome (fail-fast behavior). This includes FAIL, ERRORED (timeout), and any other error.
- **FR-006**: System MUST distinguish between a target that failed on its own (FAIL) and a target that was cancelled due to another's failure (CANCELLED).
- **FR-007**: System MUST distinguish between a target that timed out (ERRORED) and one that failed for other reasons (FAIL).
- **FR-008**: System MUST capture and prefix shell command stdout and stderr when running in parallel mode.
- **FR-009**: System MUST flush any partial (unterminated) output line when a target completes.
- **FR-010**: System MUST preserve output ordering within a single target — lines from one target appear in the order they were produced.
- **FR-011**: System MUST provide print functions that automatically detect parallel vs. serial mode and format output accordingly.
- **FR-012**: System MUST NOT change output behavior for targets running in serial mode — backward compatibility is required.
- **FR-013**: System MUST announce target lifecycle events (start, stop with result and duration) in parallel mode, with customizable hooks.
- **FR-014**: System MUST right-pad target name prefixes within a parallel group so output columns align when target names have different lengths.
- **FR-015**: System MUST return a non-zero exit code when any target in a parallel group has a non-pass outcome. The exit code is determined by the first error encountered.
- **FR-016**: System MUST print individual target stop messages (lifecycle announcements) before the aggregate summary line. Stop messages use the prefixed format; the summary line is unprefixed.

### Key Entities

- **Target**: A unit of work that produces output and returns a result. Has a name, optional lifecycle hooks, and runs either in serial or parallel mode.
- **Target Result**: The outcome of executing a target — includes the target name, status (pass/fail/cancelled/errored), duration, and any error information.
- **Summary**: An aggregated view of all target results in a parallel group, showing counts per status category.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of output lines from parallel targets are prefixed with the correct target name — zero interleaved lines.
- **SC-002**: Every parallel execution produces a summary line that accurately reflects the count of each outcome category.
- **SC-003**: Cancelled targets appear in the summary within 1 second of the first failure (fail-fast responsiveness).
- **SC-004**: Existing serial-mode targets produce identical output before and after this feature is added (zero regressions).
- **SC-005**: Developers can identify the source of any output line at a glance without scrolling or cross-referencing.
- **SC-006**: Shell-command targets and programmatic targets produce identically formatted prefixed output in parallel mode.

## Assumptions

- Parallel groups are defined at build-configuration time; this feature does not add a new way to specify parallelism, it enhances the output of existing parallel execution.
- The fail-fast cancellation model (cancel all on first failure) is the only cancellation strategy needed. A "run all regardless" mode is out of scope.
- Prefix alignment (right-padding) uses the longest target name within a single parallel group, not across different groups.
- Default lifecycle messages ("starting...", "PASS (1.2s)") are sufficient for most users; hooks are an extension point for advanced customization.
- Output buffering may cause slight delays in line appearance, but lines are never lost. This tradeoff is acceptable for line atomicity.
