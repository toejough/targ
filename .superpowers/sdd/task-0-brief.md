### Task 0: Clear the pre-existing lint debt blocking green gates

**Files:**
- Modify: `internal/core/command.go`, `internal/flags/flags.go`, `internal/runner/runner.go` (per the live `targ lint-full` enumeration)

**Interfaces:** none — lint-only fixes (named constants for goconst, the govet and modernize findings as the linters direct). No behavior change; no lint suppressions (fix the code, never add nolint overrides — surface to Joe if any finding resists a clean fix).

- [ ] **Step 1:** `targ lint-full` → record the full finding list (expected: the snapshot locations above; the live run governs the exact list and count — Gate A's probe saw a revive cascade appear and clear during fixing).
- [ ] **Step 2:** Fix each finding minimally. RED analogue: the lint findings ARE the failing checks; GREEN = `targ lint-full` clean. Probe-measured tips: put the flags.go goconst const in its own commented declaration (inserting it above `type FlagMode` displaces its doc comment and trips revive `exported`); run `targ reorder-decls` BEFORE committing (both goconst fixes trigger reorder churn — saves an amend cycle).
- [ ] **Step 3:** Commit — subject: `chore(lint): clear pre-existing lint debt blocking green gates` (62 bytes, measured) + trailer. Then `targ check-full` post-commit → every check green (this baseline makes Tasks 1–2's gates achievable). Fix+amend until green.

---

