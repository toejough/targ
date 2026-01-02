# Implementation Plan for Commander

## Phase 1: Core Logic & Discovery (Done)
- [x] **Define Core Interfaces**: `Command` and `Argument` abstractions (internal).
- [x] **Reflection Engine**: Logic to inspect `func(struct)` signatures.
- [x] **Tag Parsing**: `commander:"required, desc=..."`.

## Phase 2: Argument Parsing & Execution (Done)
- [x] **Flag Parsing**: `flag` package integration.
- [x] **Execution**: Handler to invoke functions.
- [x] **Help Generation**: Basic usage listing.
- [x] **Environment Variables**: `env=...` tag.
- [x] **Positional Arguments**: `positional` tag.
- [x] **Variadic Arguments**: Slice support.

## Phase 3: CLI Structure & Subcommands (Done)
- [x] **Root Command**: `Run(funcs...)`.
- [x] **Subcommands**: Struct methods as subcommands (`math add`).
- [x] **Name Normalization**: CamelCase to kebab-case/spaced (`RemoteAdd` -> `remote add`).

## Phase 4: Polish & Advanced Features (Todo)
- [ ] **Shell Completion**: Bash/Zsh/Fish completion integration.
- [ ] **Short Flags**: `-a` alias support.
- [ ] **Custom Help**: Better help formatting using descriptions.

## Phase 5: Testing & Documentation (Partial)
- [x] **Unit Tests**: Coverage for parsing and execution.
- [x] **Examples**: `examples/simple/main.go`.
- [ ] **Documentation**: Readme.
