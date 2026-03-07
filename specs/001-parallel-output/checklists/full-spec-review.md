# Full Spec Review Checklist: Parallel Output

**Purpose**: Thorough requirements quality validation across all functional requirements, success criteria, user stories, edge cases, and assumptions
**Created**: 2026-02-19
**Feature**: [spec.md](../spec.md)
**Depth**: Thorough (formal gate)
**Audience**: Reviewer (pre-implementation)

## Requirement Completeness

- [x] CHK001 - Are requirements defined for the overall process exit code when parallel execution contains mixed outcomes? FR-003 classifies per-target results but the spec does not define what exit code the process returns. [Gap] **Resolved: Added FR-015 — non-zero exit on any non-pass, determined by first error.**
- [ ] CHK002 - Are requirements specified for stderr vs stdout distinction in prefixed output? FR-008 says "capture and prefix shell command stdout and stderr" but does not state whether they are merged into a single stream or remain distinguishable. [Gap, Spec §FR-008]
- [ ] CHK003 - Are requirements defined for nested parallel groups (a parallel target whose dependencies are also parallel)? The spec addresses "parallel group" but does not address nesting behavior. [Gap]
- [ ] CHK004 - Are thread-safety requirements explicitly stated for `Print`/`Printf` when called concurrently from multiple target goroutines? FR-011 implies concurrent use but does not state a concurrency guarantee. [Gap, Spec §FR-011]
- [ ] CHK005 - Are requirements defined for what happens when a target's context is already cancelled before execution begins (e.g., fail-fast triggered during target startup)? [Gap, Spec §FR-005]
- [x] CHK006 - Are requirements defined for the ordering between individual stop messages (FR-013) and the summary line (FR-004)? The spec does not specify whether stop announcements precede or follow the summary. [Gap, Spec §FR-004/FR-013] **Resolved: Added FR-016 — stop messages before summary, stop messages prefixed, summary unprefixed.**
- [ ] CHK007 - Are requirements specified for duration precision and rounding in lifecycle stop messages? User Story 5 shows "1.2s" but no formatting requirement is stated. [Gap, Spec §FR-013]
- [ ] CHK008 - Are requirements defined for targets that panic rather than returning an error? FR-003 classifies based on error return but does not address panics. [Gap, Spec §FR-003]
- [ ] CHK009 - Is a requirement present for the Printer's channel buffer sizing strategy? The edge case mentions "small output buffer" and "writes may block" but no sizing requirement is stated. [Gap, Edge Cases]
- [ ] CHK010 - Are requirements defined for how `targ.Print`/`targ.Printf` handle the case where they are called outside any target execution context (no ExecInfo, not serial mode)? [Gap, Spec §FR-011]

## Requirement Clarity

- [ ] CHK011 - Is "line" precisely defined for line atomicity (FR-002)? Is a "line" a newline-terminated byte sequence, a Unicode-aware text line, or something else? [Clarity, Spec §FR-002]
- [ ] CHK012 - Is the prefix format in FR-001 unambiguous given FR-014? FR-001 specifies `[target-name]` followed by "a space", but FR-014 requires right-padding. Does the padding replace the single space, or is it in addition to it? [Clarity, Spec §FR-001/FR-014]
- [ ] CHK013 - Is "first target in a parallel group fails" (FR-005) defined for the case where two targets fail at nearly the same instant? How is "first" determined — by which goroutine's error is received first? [Clarity, Spec §FR-005]
- [ ] CHK014 - Is "reasonable time" in User Story 3 scenario 1 quantified? SC-003 gives "within 1 second" but the acceptance scenario uses the vague "reasonable time". [Ambiguity, US3 §Scenario 1]
- [ ] CHK015 - Is "customizable hooks" (FR-013) specified with a hook signature, invocation context, and error handling contract? The spec says hooks are customizable but does not define the callback interface. [Clarity, Spec §FR-013]
- [ ] CHK016 - Is the padding character for right-padding (FR-014) specified? The requirement says "right-padded" but does not state whether padding uses spaces, tabs, or another character. [Clarity, Spec §FR-014]
- [ ] CHK017 - Is "partial (unterminated) output line" (FR-009) clearly defined? Does it mean bytes without a trailing `\n`, or does it include other line terminators (`\r\n`, `\r`)? [Clarity, Spec §FR-009]
- [ ] CHK018 - Does "identical output" in SC-004 account for the addition of `targ.Print`/`targ.Printf` functions? Existing targets using `fmt.Print` directly would be unchanged, but does SC-004 also require that `targ.Print` in serial mode produces byte-identical output to `fmt.Print`? [Clarity, Spec §SC-004]

## Requirement Consistency

- [x] CHK019 - Are FR-005 (fail-fast on first failure) and FR-007 (timeout is ERRORED, not FAIL) consistent? If a timeout occurs, does it also trigger fail-fast cancellation of siblings, or does only FAIL status trigger cancellation? [Consistency, Spec §FR-005/FR-007] **Resolved: Updated FR-005 — any non-pass outcome (FAIL, ERRORED) triggers fail-fast.**
- [ ] CHK020 - Is the summary line format consistent between FR-004 and User Story 2? FR-004 shows `PASS:2 FAIL:1` while US2 scenario 2 shows `PASS:1 FAIL:1 CANCELLED:1`. Both show non-zero only, but is the ordering (PASS, FAIL, CANCELLED, ERRORED) specified? [Consistency, Spec §FR-004/US2]
- [ ] CHK021 - Are lifecycle announcement requirements (FR-013, User Story 5) consistent with the prefix format (FR-001)? US5 shows `[build] starting...` and `[build] PASS (1.2s)` — are these lifecycle messages also subject to right-padding per FR-014? [Consistency, Spec §FR-013/FR-014]
- [ ] CHK022 - Is the edge case "parallel execution has only one target" consistent with the prefix format requirements? FR-014 says "right-padded ... when target names have different lengths" — does padding still apply with a single target (pad to its own length, effectively no padding)? [Consistency, Edge Cases/FR-014]

## Acceptance Criteria Quality

- [ ] CHK023 - Is SC-005 ("identify the source of any output line at a glance") objectively measurable? Unlike SC-001 (100%, zero interleaved) or SC-003 (1 second), "at a glance" is subjective and cannot be automated. [Measurability, Spec §SC-005]
- [ ] CHK024 - Does SC-003 ("within 1 second of the first failure") define the measurement points precisely? Is it wall-clock time from when the failing target returns an error to when the cancelled target's context is cancelled, or to when the cancelled target's goroutine exits? [Measurability, Spec §SC-003]
- [ ] CHK025 - Are acceptance scenarios for User Story 2 complete? All three scenarios involve non-empty results. Is there an acceptance scenario for zero targets (empty parallel group)? [Coverage, US2]
- [ ] CHK026 - Are acceptance scenarios for User Story 6 complete? They cover serial and parallel modes but not the transition case: a target that switches between modes across invocations. [Coverage, US6]

## Scenario Coverage

- [ ] CHK027 - Are requirements defined for the "all targets fail" scenario? User Story 2 covers "all succeed" and "mixed outcomes" but not "all fail". How does the summary line look when every target fails? [Coverage, Gap]
- [ ] CHK028 - Are requirements defined for a serial target that has parallel dependencies? The spec covers top-level parallel and dep-level parallel but does not address a serial execution flow that triggers a parallel dependency group. [Coverage, Gap]
- [ ] CHK029 - Are requirements specified for what output appears when a target is cancelled before producing any output? Does the lifecycle stop message still appear for a target that never started? [Coverage, Spec §FR-013/FR-005]
- [ ] CHK030 - Are requirements defined for multiple parallel groups executed sequentially within a single invocation? If a target has two dep groups (both parallel), does each group get its own summary line, or is there one summary at the end? [Coverage, Gap]

## Edge Case Coverage

- [ ] CHK031 - Are requirements defined for binary (non-text) output from shell commands in parallel mode? FR-008 specifies capture and prefix but does not address non-UTF-8 byte sequences. [Edge Case, Gap]
- [ ] CHK032 - Are requirements defined for output that coincidentally contains bracket patterns (e.g., a target printing `[build] ...`)? Could this be confused with prefixed output from another target? [Edge Case, Gap]
- [ ] CHK033 - Are requirements defined for empty target names? FR-001 specifies `[target-name]` prefix format but does not state that target names must be non-empty. [Edge Case, Gap]
- [ ] CHK034 - Are requirements defined for a very large number of parallel targets (e.g., 100+)? Are there scalability constraints on the number of concurrent goroutines, channel buffer size, or prefix padding width? [Edge Case, Gap]
- [ ] CHK035 - Is the edge case "target writes a very long line without a newline" bounded? Is there a maximum line buffer size, or can a single partial line consume unbounded memory? [Edge Case, Spec §Edge Cases]

## Non-Functional Requirements

- [ ] CHK036 - Are memory constraints specified for the output buffering system? The spec mentions "writes may block until buffer space is available" but does not define maximum buffer sizes or memory limits. [Non-Functional, Gap]
- [ ] CHK037 - Are latency requirements defined for output delivery? The assumptions state "slight delays in line appearance" are acceptable, but is there a maximum acceptable delay for output to appear after a target produces it? [Non-Functional, Gap]
- [ ] CHK038 - Are requirements defined for output encoding? The spec assumes text output but does not specify UTF-8 or other encoding requirements. [Non-Functional, Gap]

## Dependencies & Assumptions

- [ ] CHK039 - Is the assumption "only one parallel execution scope is active at a time" documented in the spec? The plan and design docs state this, but the spec's Assumptions section does not address it. [Assumption, Gap]
- [ ] CHK040 - Is the assumption that `context.Canceled` reliably indicates sibling-triggered cancellation validated? If a user's target function cancels its own context, this classification would be incorrect. [Assumption, Spec §FR-006]
- [ ] CHK041 - Is the scope boundary for "run all regardless" mode clearly stated in the spec? The assumptions say it's out of scope, but FR-005 says "MUST cancel" with no opt-out. Is this intentionally non-configurable? [Assumption, Spec §Assumptions]

## Notes

- Check items off as completed: `[x]`
- Add resolution notes inline when closing items
- Items marked [Gap] indicate requirements that may need to be added to spec.md
- Items marked [Clarity] or [Ambiguity] indicate requirements that need rewording
- Items marked [Consistency] indicate potential conflicts between requirements
