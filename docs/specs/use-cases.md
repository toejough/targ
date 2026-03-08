# L1: Use Cases

Items induced from: L2 requirements and design items (bottom-up)

## UC-1: Define and Run Build Targets

**Actor:** Developer
**Starting state:** A Go project with build automation needs
**End state:** Developer runs `targ <command>` and targets execute with the specified configuration
**Key interactions:** Define targets via functions or shell strings, configure with builder methods (deps, cache, watch, timeout, retry), run from CLI
**Constraints:** Configuration conflicts between compile-time config and CLI flags produce errors rather than silent precedence (No Surprises principle)

**Traces to:** REQ-1, REQ-2, REQ-3, REQ-4, REQ-9, REQ-10, DES-2, DES-3, DES-5

## UC-2: Build a Standalone CLI Application

**Actor:** Developer shipping a CLI tool
**Starting state:** Build targets exist in `//go:build targ` files
**End state:** A standalone binary with the same targets, invocable without the `targ` build tool
**Key interactions:** Remove build tag, switch `Register()` → `Main()`, compile as regular Go binary

**Traces to:** REQ-1, REQ-2, REQ-5, REQ-6, DES-1, DES-2

## UC-3: Discover Help, Completion, and Source Info

**Actor:** Developer using targ-built CLI
**Starting state:** A targ application exists (build tool or standalone binary)
**End state:** Developer understands available commands, flags, and usage
**Key interactions:** `--help` shows formatted help with flags/positionals/subcommands/examples/source, `--completion` generates shell completion scripts (bash/zsh/fish), source file:line shown in help

**Traces to:** REQ-11, REQ-12, REQ-14, REQ-15

## UC-4: Scaffold, Sync, and Convert Targets

**Actor:** Developer managing target definitions
**Starting state:** A Go project (possibly without targ files)
**End state:** Targets are created, imported from remote modules, or converted between function/string forms
**Key interactions:** `--create NAME [CMD]` scaffolds targets, `--sync PACKAGE` imports remote targets, `--to-func`/`--to-string` convert target types

**Traces to:** REQ-13, DES-4

## UC-5: Run Targets in Parallel with Organized Output

**Actor:** Developer running CI or multi-target workflows
**Starting state:** Multiple targets need to execute concurrently
**End state:** Targets run in parallel with clear, prefixed output and proper error reporting
**Key interactions:** `.Deps(a, b, targ.DepModeParallel)` or `--parallel` flag, each target's output prefixed with its name, result summary shows per-target pass/fail/cancelled/errored status

**Traces to:** REQ-7, REQ-8, DES-5

## UC-6: Discover Targets Across the Directory Tree

**Actor:** Developer running targ from any working directory
**Starting state:** Target files exist in ancestor directories (e.g., `~/dev/targs.go`) and/or the current subtree
**End state:** Targets from both ancestor directories and the current subtree are discovered, compiled, and available to run
**Key interactions:** From any CWD, targ walks down (full subtree, unchanged) and up (linear ancestor path, checking each ancestor directory and its `dev/` subtree). Each ancestor with targets is built as its own module group (existing multi-module path). No sibling discovery. No root boundary. Conflicts handled identically to today (`ConflictError` with source locations).
**Constraints:** Upward discovery is automatic (no opt-in). Only the linear ancestor path is searched — no sibling directories. Each unmoduled ancestor directory becomes its own isolated build unit. Ancestor targets appear in help with existing source attribution.

**Traces to:** REQ-13, REQ-16, REQ-17, DES-6
