# targ#25 — coverage leg `-timeout=30s` → `-timeout=10m`; file the two follow-ups

Issue: https://github.com/toejough/targ/issues/25 · Joe's disposition (2026-07-23, verbatim):
"do 1, file 2, and file an issue for 3 with enough context for a new agent to understand the
rationale & pick it up and execute it successfully." — where, from the investigation briefing:
**1** = set the coverage leg's go-test budget to 10m in targ; **2** = engram issue for its e2e
test cost; **3** = targ issue for the structural options (serialize the coverage leg / cap
parallel dep fan-out). The bootstrap-cache observation from the probe was raised in the same
briefing and NOT included in Joe's enumeration — it stays **raised, not filed** (per the
standing raise-don't-file rule for undispositioned work); the close-out report lists it as
awaiting his call.

## Ask trace and evidence chain (all measured this session, 2026-07-23)

- The 30s at dev/targets.go:2227 (`testForFail`) is an inherited speed guard — copied from
  imptest's tooling in 9886c58, re-tuned five times there purely to track suite speed, never
  framed as hang protection. Hang protection survives the change: go test's default 10m panic,
  targ's opt-in global `--timeout`, the unused per-target `.Timeout()` builder.
- Mechanism (measured): check-full fans out all 8 legs with no concurrency cap; go test adds
  10-way package parallelism; engram's internal/cli needs ~15s unloaded and 52–63s under load.
- Probe (isolated worktrees, patch-liveness verified via panic banner, `--no-binary-cache`):
  control fails with the 30s banner; treatment (10m) → coverage leg PASS 3/3, leg wall-clock
  63s/72s/82s, "Coverage OK (min: 80.0%)". Logs: session scratchpad `probe2-control-run1.log`,
  `probe2-treatment-{1,2,3}.log`.
- Value choice: explicit `-timeout=10m` over deleting the flag — behaviorally identical (10m is
  go's default) but the explicit value documents intent; a bare absence reads as an oversight.

## Tasks

- **T1 — the change.** dev/targets.go:2227: `"-timeout=30s"` → `"-timeout=10m"`. One line.
  TDD note (honest): no unit seam exists for exec-arg literals in dev targets (testForFail
  passes literals straight to targ.RunContext); the RED/GREEN for this change is the measured
  probe above (control = RED, treatment = GREEN 3/3). In-repo validation: grep confirms no
  `-timeout=30s` remains in dev/; `go test -tags targ ./dev/` green; `targ check-full` green
  post-commit on targ itself. No Gate B (no refactor phase — a literal swap has no design
  surface); Gate A's code angle covers the change form.
- **T2 — file the engram issue (item 2).** Repo toejough/engram. Content (mechanism as
  verified against the engram tree by Gate A round 1, correcting the investigation report):
  the five internal/cli `*_EndToEnd` tests each `go build` the full engram binary (linking the
  90MB go:embed ONNX model) inside the test body — the shared dominant repeated cost
  (4.7–13.8s per test standalone; package ~15s unloaded, 52–63s under check-full load). Model
  load + ONNX runtime init are lazy (internal/embed/hugot.go — first Embed/Dims call;
  ModelID deliberately short-circuits without init, hugot.go:229-239, and the NewLazyEmbedder
  doc comment at :188-190 stating otherwise is stale — worth a bonus note in the issue)
  and happen only in the subprocesses of TestEngramLearn_Fact/Feedback_EndToEnd (auto-embed)
  and TestEngramQuery_F6F91_EndToEnd (query), which inherit the parent env and hit the warm
  user cache; extraction costs appear only cold (e.g. CI). TestRunCommand_EndToEnd
  (`update --dry-run`) and TestOpenDebugFile_EndToEnd never reach the embedder — their
  XDG_CACHE_HOME pins are hygiene, not model cost. Ask: share ONE built binary across the
  suite (TestMain or sync.Once), keep the warm-cache inheritance for the three model-loading
  tests where each test's purpose permits, and prefer the cheapest real invocation for
  startup-composed capabilities — citing the existing engram lesson to that effect. Frame:
  restores real headroom regardless of any gate budget; the gate-side 30s→10m fix (targ#25)
  lands independently.
- **T3 — file the targ structural issue (item 3).** Repo toejough/targ. Pickup-ready content
  (a fresh agent must be able to execute without this session): the two structural options with
  the measured facts — (a) ordered dep groups (native: split CheckFull's single
  `.Deps(8 targets, DepModeParallel, CollectAllErrors)` call at dev/targets.go:37 into
  coverage-serial-first + parallel-rest; caveat measured from code: runDeps returns on first
  group error, so a failing first group suppresses later groups' collected errors —
  internal/core/target.go:497-516); (b) concurrency cap in runGroupParallelAll
  (internal/core/target.go:905-945 spawns one goroutine per dep, no semaphore; a cap
  benefits every consumer without ordering trades). Include: the mechanism summary, why
  neither is needed today (10m fix suffices, probe-validated), the trigger condition for
  picking it up (a consumer's suite outgrowing even relaxed budgets, or wall-clock pressure
  on check-full), and the `--dep-mode serial` CLI caveat (flattens groups, silently drops
  CollectAllErrors — command.go:1944-1966). Framing note (Gate A docs finding): `--dep-mode`
  is a public flag (listed in `targ --help`) but is absent from every spec flag enumeration
  (specs/implementation.md:40/:121, architecture.md:21, tests.md:34, requirements.md:21) —
  the issue describes it as the public flag it is; the spec-registry staleness is RAISED in
  the close-out for Joe's disposition, not fixed in this cycle.
- **T4 — close #25 + validation + capture.** Commit T1 (conventional message, `AI-Used:
  [claude]` trailer); `targ check-full` post-commit; push (close comment cites the SHA,
  mirroring the #22 (2bc60d2) and #23 (4476ed2) close comments this session); close #25 with
  the AC→evidence chain citing the probe. Gate D over ALL outward prose before anything
  ships: T1's commit message, both issue bodies, the #25 close comment. Raised-not-actioned
  items the close-out report MUST carry for Joe's disposition: (1) engram goes green on
  check-full only after it re-bumps its targ pin; (2) the bootstrap-cache non-invalidation
  observed during the probe (vault note 371) — raised in the briefing, not in Joe's
  enumeration, so not filed; (3) the `--dep-mode` spec-registry staleness (public flag absent
  from all spec enumerations). Lessons audit + closing /learn.

## Doc-surface disposition (non-waivable grep, run 2026-07-23)

Grep: `30s|timeout` over README.md, CLAUDE.md, docs/**, .claude/**, examples/**.

| File | Disposition | Reason |
| --- | --- | --- |
| CLAUDE.md:37 | N/A — keep | the no-flaky-tests rule; the principle motivating this fix, no value cited |
| docs/archive/architecture.md:318, design.md:410/:530 | N/A — archive | examples of the unrelated `--timeout` runtime-override CLI flag (coincidental "30s") |
| docs/specs/{tests,architecture,implementation}.md | N/A | document the `--timeout` runtime override, a different mechanism |
| dev/targets.go | source | the change itself |

No prose doc mentions the coverage leg's go-test `-timeout` value. Gate C expected N/A
(no docs touched); Gate A docs angle verifies this table.

## /please step tracking

1. ✅ Capture (open) — sweep current (session); probe-trap lesson crystallized (note 371)
2. ✅ Orient — /recall glance run; note 335 applied (bootstrap-cache: raise-not-file);
   assessment stated (ask sound; explicit 10m over flag deletion)
3. ✅ Plan — rev 2: Gate A round 1 (ask 1 Imp + 1 Min; code 1 Imp — T2 mechanism corrected to
   the tree-verified account; docs 2 — --dep-mode framing + raised spec staleness; clarity
   ACK) all addressed; ACK round: ask/docs/clarity ACK, code 1 Minor (ModelID does not
   trigger lazy init — self-verified against hugot.go:229-239, fixed above). Gate A closed
4. ☐ Execute (T1; probe = evidence chain, no unit seam)
5. ☐ Document (expected N/A per disposition table; Gate C subject-absent if so)
6. ☐ Complete (T2, T3 filings; close #25; commit; Gate D over all outward prose)
7. ☐ Capture (close) — lessons audit + closing /learn
