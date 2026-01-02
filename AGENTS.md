# Commander Agent Guide

This repository contains the `commander` library, a Go toolkit for building CLIs with the simplicity of Mage, the configuration of `go-arg`, and the power of Cobra.

## Project Context

- **Goal**: Create a library that allows Go developers to build CLIs by defining exported functions and argument structs with tags.
- **Inspiration**: Mage (discovery), go-arg (struct tags), Cobra (subcommands, completion).
- **Core Philosophy**: "Just works" discovery of commands, with an optional explicit registration path for standalone binaries.

## Codebase Structure

- `go.mod`: Go module definition.
- `thoughts.md` & `thoughts2.md`: Design documents and RFC-style exploration of features.
- `issues.md`: High-level tracking of project status.

## Development Patterns

### Command Definition
- **Root Commands**: Struct with a `Run()` method.
- **Subcommands**: Field on a struct with `commander:"subcommand"` tag.
- **Arguments**: Fields on the struct with tags (`commander:"flag"`, `commander:"positional"`).

### Example
```go
type Root struct {
    Sub *SubCmd `commander:"subcommand"`
}
type SubCmd struct {
    Flag string `commander:"name=flag"`
}
func (s *SubCmd) Run() { ... }
```

### Testing
- Use standard Go testing: `go test ./...`.
- Focus on end-to-end tests that verify command discovery and argument parsing.

## Workflow

1. **Read `thoughts2.md`**: This contains the most up-to-date API design.
2. **Implement Iteratively**: Start with basic command discovery and argument parsing.
3. **Verify**: Ensure the reflection logic correctly maps struct tags to flags.

## Useful Commands

- `go test ./...`: Run all tests.
- `go build`: Build the project.
