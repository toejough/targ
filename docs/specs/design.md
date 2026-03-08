# L2: Design (Adoption)

Induced bottom-up from L3 ARCH items. Interaction model and UX specification.

## DES-1: Target Definition UX

Users define targets by passing a Go function or shell command string to `Targ()`. Struct parameters are automatically parsed into CLI arguments via `targ:"..."` tags. Tag keys: `flag`, `positional`, `short`, `default`, `desc`, `env`, `required`, `enum`. Field names auto-convert to kebab-case. Shell command targets use `$placeholder` substitution for flag values.

- Traces to: (deferred — L1 not yet created)
- L3 items: ARCH-1, ARCH-6, ARCH-11

## DES-2: Execution Model

Targets run via `Main()` (CLI entry), `Execute()` (programmatic), or `ExecuteRegistered()` (init-based). Dependencies are declared via `.Deps()` with serial (default) or parallel modes. Parallel execution prefixes output with target names. Lifecycle hooks (OnStart/OnStop) run at execution boundaries. Runtime overrides (--cache, --watch, --times) modify behavior without code changes.

- Traces to: (deferred)
- L3 items: ARCH-2, ARCH-9, ARCH-10

## DES-3: Help and Completion UX

`--help` produces styled terminal output with sections in fixed order: usage line, description, subcommands, global flags, command flags, positionals, examples. ANSI codes are paired and examples are code-only (no styling). Shell completion integrates with fish/bash/zsh. Binary mode strips targ-specific flags from help.

- Traces to: (deferred)
- L3 items: ARCH-5, ARCH-7

## DES-4: Organization Model

Targets are organized via `Group()` for nested namespacing. Groups are addressed by dot-separated paths. `Register()` from `init()` adds targets to the global registry. Remote packages provide targets via blank import. `DeregisterFrom()` removes a package's targets before re-registering replacements. Conflicts surface all issues at once with fix suggestions.

- Traces to: (deferred)
- L3 items: ARCH-3, ARCH-4, ARCH-13

## DES-5: Runner Workflow

The `targ` CLI discovers target files, compiles a temporary binary, and invokes it with the user's arguments. `--create` scaffolds a new target file with correct Go code. `--sync` imports remote targets. Code generation uses AST manipulation for correctness.

- Traces to: (deferred)
- L3 items: ARCH-12, ARCH-14
