# Targ

<p align="center">
  <img src="assets/targ.png" alt="Targ logo" width="500">
</p>

Build CLIs and run build targets with minimal configuration. Inspired by Mage, go-arg, and Cobra.

## Quick Reference

**Key files:** `targ.go` (public API), `sh/` (shell execution), `file/` (file utilities)

| Want to... | Do this |
|------------|---------|
| Run build targets | `//go:build targ` files + `targ <command>` |
| Parse CLI flags | Struct with `targ:"..."` tags + `targ.Run(&cmd{})` |
| Run shell commands | `sh.Run("go", "build")` or `sh.RunContext(ctx, ...)` |
| Skip unchanged work | `file.Newer(inputs, outputs)` or `file.Checksum(...)` |
| Watch for changes | `file.Watch(ctx, patterns, opts, callback)` |
| Run deps once | `targ.Deps(A, B, C)` or `targ.ParallelDeps(...)` |

## Installation

```bash
# Build tool (run targets)
go install github.com/toejough/targ/cmd/targ@latest

# Library (embed in your binary)
go get github.com/toejough/targ
```

## Quick Start

**Build tool** (no main function needed):

```go
//go:build targ

package main

import "github.com/toejough/targ/sh"

func Build() error { return sh.Run("go", "build", "./...") }
func Test() error  { return sh.Run("go", "test", "./...") }
func Lint() error  { return sh.Run("golangci-lint", "run") }
```

```bash
targ build
targ test
```

**CLI with flags**:

```go
type Deploy struct {
    Env   string `targ:"positional,required,enum=dev|staging|prod"`
    Force bool   `targ:"flag,short=f,desc=Skip confirmation"`
}

func (d *Deploy) Run() { fmt.Printf("Deploying to %s\n", d.Env) }

func main() { targ.Run(&Deploy{}) }
```

```bash
./deploy prod --force
```

## From Build Targets to Dedicated CLI

Targ makes it easy to start with simple build targets and evolve to a full CLI.

### Stage 1: Simple Functions

Start with plain functions for quick automation:

```go
//go:build targ

package main

import "github.com/toejough/targ/sh"

func Build() error { return sh.Run("go", "build", "-o", "myapp", "./...") }
func Test() error  { return sh.Run("go", "test", "./...") }
```

```bash
targ build  # Run with targ
```

### Stage 2: Add Flags

Need options? Convert to a struct:

```go
//go:build targ

package main

type Build struct {
    Output  string `targ:"flag,short=o,default=myapp,desc=Output binary name"`
    Verbose bool   `targ:"flag,short=v,desc=Verbose output"`
}

func (b *Build) Run() error {
    args := []string{"build", "-o", b.Output}
    if b.Verbose {
        args = append(args, "-v")
    }
    return sh.Run("go", append(args, "./...")...)
}
```

```bash
targ build --output=myapp --verbose
```

### Stage 3: Dedicated Binary

Ready to ship? Remove the build tag and add main:

```go
package main

import "github.com/toejough/targ"

type Build struct {
    Output  string `targ:"flag,short=o,default=myapp,desc=Output binary name"`
    Verbose bool   `targ:"flag,short=v,desc=Verbose output"`
}

func (b *Build) Run() error { /* same as before */ }

type Test struct {
    Cover bool `targ:"flag,desc=Enable coverage"`
}

func (t *Test) Run() error { /* ... */ }

func main() {
    targ.Run(&Build{}, &Test{})
}
```

```bash
go build -o mytool .
./mytool build --verbose
./mytool test --cover
```

The same struct definitions work in both modes. Your targets become your CLI.

## Tags

Configure fields with `targ:"..."` struct tags:

| Tag            | Description                                 |
| -------------- | ------------------------------------------- |
| `required`     | Field must be provided                      |
| `positional`   | Map positional args to this field           |
| `flag`         | Explicit flag (default for non-positional)  |
| `name=X`       | Custom flag/positional name                 |
| `short=X`      | Short flag alias (e.g., `short=f` for `-f`) |
| `desc=...`     | Description for help text                   |
| `enum=a\|b\|c` | Allowed values (enables completion)         |
| `default=X`    | Default value                               |
| `env=VAR`      | Default from environment variable           |
| `global`       | Flag available to all subcommands           |
| `subcommand`   | Field is a subcommand                       |
| `subcommand=X` | Subcommand with custom name                 |

Combine with commas: `targ:"positional,required,enum=dev|prod"`

## Library Mode

Embed targ in your own binary:

```go
package main

import (
    "fmt"
    "github.com/toejough/targ"
)

type Greet struct {
    Name string `targ:"positional,required"`
}

func (g *Greet) Run() {
    fmt.Printf("Hello %s\n", g.Name)
}

func main() {
    targ.Run(&Greet{})
}
```

With a single root command, flags are used directly:

```bash
./greet Alice
```

With multiple roots, select a command first:

```go
func main() {
    targ.Run(&Greet{}, &Farewell{})
}
```

```bash
./cli greet Alice
./cli farewell Bob
```

Niladic functions also work as commands:

```go
func Clean() { fmt.Println("cleaning") }

func main() {
    targ.Run(Clean)
}
```

## Build Tool Mode

Create command files. Do use the build tag: `//go:build targ`. Don't supply a main function.

```go
//go:build targ

package main

import "github.com/toejough/targ/sh"

// Build compiles the project.
func Build() error {
    return sh.Run("go", "build", "./...")
}

// Test runs all tests.
func Test() error {
    return sh.Run("go", "test", "./...")
}

// CI runs the full pipeline.
func CI() error {
    return targ.Deps(Build, Test, Lint)
}
```

Run from that directory:

```bash
targ build
targ test
targ ci
```

### Commands with Flags

Struct commands work the same way in build tool mode:

```go
//go:build targ

package main

type Build struct {
    Target  string `targ:"positional,default=./..."`
    Verbose bool   `targ:"flag,short=v"`
}

func (b *Build) Run() error {
    args := []string{"build"}
    if b.Verbose {
        args = append(args, "-v")
    }
    args = append(args, b.Target)
    return sh.Run("go", args...)
}
```

### Multi-Directory Layout

Discovery is recursive. Commands are namespaced by earliest unique path:

```text
repo/
  tools/
    issues/
      targets.go  //go:build targ
    deploy/
      systemA/
        commands.go  //go:build targ
      systemB/
        commands.go  //go:build targ
        otherCommands.go //go:build targ
```

```bash
# running from repo/
targ issues list
targ deploy systemA prod
targ deploy systemB commands prod
```

If only one tagged file exists, commands appear at the root (no prefix).

```text
repo/
  tools/
    issues/
      targets.go  //go:build targ
```

```bash
# running from repo/
targ list
```

## Subcommands

Define subcommands with struct fields:

```go
type Math struct {
    Add *AddCmd `targ:"subcommand"`
    Mul *MulCmd `targ:"subcommand=multiply"`
}

func (m *Math) Run() {
    fmt.Println("Usage: math <add|multiply>")
}

type AddCmd struct {
    A, B int `targ:"positional"`
}

func (a *AddCmd) Run() {
    fmt.Printf("%d\n", a.A+a.B)
}
```

```bash
./math add 2 3      # 5
./math multiply 2 3 # 6
```

## Command Signatures

`Run` methods and function commands support these signatures:

- `func()`
- `func() error`
- `func(context.Context)`
- `func(context.Context) error`

## Command Descriptions

Document commands with comments:

```go
// Deploy pushes code to the specified environment.
// Requires valid AWS credentials.
func (d *Deploy) Run() error { ... }
```

Or implement `Description()`:

```go
func (d *Deploy) Description() string {
    return "Deploy to " + d.defaultEnv()
}
```

## Command Names

Command names are derived from struct or function names, converted to kebab-case:

| Definition               | Command     |
| ------------------------ | ----------- |
| `type BuildAll struct{}` | `build-all` |
| `func RunTests()`        | `run-tests` |

Override with `Name()`:

```go
func (c *MyCmd) Name() string { return "custom-name" }
```

## Dependencies

Run dependencies exactly once per invocation:

```go
func Build() error {
    return targ.Deps(Generate, Compile)
}

func Test() error {
    return targ.Deps(Build)
}

func Lint() error {
    return targ.Deps(Build)
}
```

```bash
targ test lint  # runs Generate, Compile, Build, Test, then Lint, all only once
```

Run independent tasks concurrently:

```go
func CI() error {
    return targ.ParallelDeps(Test, Lint)  // runs Test and Lint concurrently
}
```

For watch mode, reset the cache between iterations:

```go
func Watch(ctx context.Context) error {
    return file.Watch(ctx, []string{"**/*.go"}, file.WatchOptions{}, func(_ file.ChangeSet) error {
        targ.ResetDeps()  // allow targets to run again
        return Check()
    })
}
```

## Shell Helpers

Use `targ/sh` for command execution:

```go
import "github.com/toejough/targ/sh"

// Run a command, inherit stdout/stderr
err := sh.Run("go", "build", "./...")

// Run with verbose output (prints command before running)
err := sh.RunV("go", "test", "./...")

// Capture output
out, err := sh.Output("go", "env", "GOMOD")
```

### Context-Aware Execution

For cancellable commands (e.g., in watch mode), use the context variants. When cancelled, the entire process tree is killed:

```go
// Cancellable command
err := sh.RunContext(ctx, "go", "test", "./...")

// Cancellable with verbose output
err := sh.RunContextV(ctx, "golangci-lint", "run")

// Cancellable with output capture
out, err := sh.OutputContext(ctx, "go", "list", "./...")
```

## File Checks

Skip work when files haven't changed:

```go
import "github.com/toejough/targ/file"

// Compare input modtimes against outputs
needs, err := file.Newer([]string{"**/*.go"}, []string{"bin/app"})
if !needs {
    return nil
}
```

When outputs are empty, `Newer` uses a cached snapshot.

Content-based checking:

```go
import "github.com/toejough/targ/file"

changed, err := file.Checksum([]string{"**/*.go"}, ".targ/cache/build.sum")
if !changed {
    return nil
}
```

## Watch Mode

React to file changes:

```go
import "github.com/toejough/targ/file"

func Watch(ctx context.Context) error {
    return file.Watch(ctx, []string{"**/*.go"}, file.WatchOptions{}, func(_ file.ChangeSet) error {
        targ.ResetDeps()  // reset so targets run again
        return sh.RunContext(ctx, "go", "test", "./...")
    })
}
```

For interruptible watch (cancel running command on new changes):

```go
func Watch(ctx context.Context) error {
    var cancel context.CancelFunc
    return file.Watch(ctx, []string{"**/*.go"}, file.WatchOptions{}, func(_ file.ChangeSet) error {
        if cancel != nil {
            cancel()  // stop previous run
        }
        runCtx, cancel = context.WithCancel(ctx)
        targ.ResetDeps()
        return sh.RunContext(runCtx, "go", "test", "./...")
    })
}
```

## Shell Completion

Generate and source completion scripts:

```bash
# Bash
source <(your-binary --completion bash)

# Zsh
source <(your-binary --completion zsh)

# Fish
your-binary --completion fish | source
```

Supports commands, subcommands, flags, and enum values.

## Example Help Output

```
$ ./deploy --help
Deploy pushes code to the specified environment.

Usage: deploy <env> [flags]

Arguments:
  env    Environment to deploy to (required, one of: dev, staging, prod)

Flags:
  -f, --force    Skip confirmation
  -h, --help     Show this help
```

## Dynamic Overrides

Override command or field metadata at runtime.

### Command Metadata

```go
func (c *MyCmd) Name() string        { return "custom-name" }
func (c *MyCmd) Description() string { return "Dynamic description" }
```

### Tag Options

Override any tag option dynamically:

```go
type Deploy struct {
    Env string `targ:"positional,enum=dev|prod"`
}

func (d *Deploy) TagOptions(field string, opts targ.TagOptions) (targ.TagOptions, error) {
    if field == "Env" {
        // Load allowed environments from config
        opts.Enum = strings.Join(loadEnvs(), "|")
    }
    return opts, nil
}
```

Useful for:

- Loading enum values from config/database
- Conditional required fields
- Environment-specific defaults

## Patterns

### Conditional Build

```go
func Build() error {
    needs, _ := file.Newer([]string{"**/*.go"}, []string{"bin/app"})
    if !needs {
        fmt.Println("up to date")
        return nil
    }
    return sh.Run("go", "build", "-o", "bin/app", "./...")
}
```

### CI Pipeline

```go
func CI() error {
    if err := targ.Deps(Generate); err != nil {
        return err
    }
    // Run independent checks in parallel
    if err := targ.ParallelDeps(Build, Lint); err != nil {
        return err
    }
    return targ.Deps(Test)
}
```

### Testing Commands

```go
func TestDeploy(t *testing.T) {
    result, err := targ.Execute([]string{"app", "deploy", "prod", "--force"}, &Deploy{})
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(result.Output, "Deploying to prod") {
        t.Errorf("unexpected output: %s", result.Output)
    }
}
```

### Variadic Positional Args

```go
type Cat struct {
    Files []string `targ:"positional,required,desc=Files to concatenate"`
}

func (c *Cat) Run() error {
    for _, f := range c.Files {
        // process each file
    }
    return nil
}
```

```bash
./cat file1.txt file2.txt file3.txt
```

## When to Use Targ

| Need | Tool |
|------|------|
| Build targets + CLI parsing | **Targ** |
| Simple build targets only | Targ or Mage |
| Complex CLI with plugins/middleware | Cobra |
| Just struct-to-flags mapping | go-arg |

**Targ's sweet spot**: You want build automation that can evolve into a full CLI, or you want CLI parsing with minimal boilerplate.

## Build Tool Flags

| Flag         | Description                                  |
| ------------ | -------------------------------------------- |
| `--no-cache` | Force rebuild of the build tool binary       |
| `--keep`     | Keep generated bootstrap file for inspection |

## Cache Management

Targ caches compiled build tool binaries in `~/.cache/targ/` (or `$XDG_CACHE_HOME/targ/`). Each project gets a subdirectory based on a hash of its path. The cache is invalidated when source files or `go.mod`/`go.sum` change.

To force a fresh build:

```bash
targ --no-cache <command>
```

To completely clear the cache:

```bash
rm -rf ~/.cache/targ/
```
