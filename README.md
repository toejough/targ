# Targ

<p align="center">
  <img src="targ.png" alt="Targ logo" width="300">
</p>

Build CLIs with minimal configuration. Inspired by Mage, go-arg, and Cobra.

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
go install github.com/toejough/targ/cmd/targ@latest
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

Discovery is recursive. Commands are namespaced by path:

```text
repo/
  tools/
    issues/
      issues.go  //go:build targ
    deploy/
      deploy.go  //go:build targ
```

```bash
targ issues list
targ deploy prod
```

If only one tagged file exists, commands appear at the root (no prefix).

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

| Definition | Command |
|------------|---------|
| `type BuildAll struct{}` | `build-all` |
| `func RunTests()` | `run-tests` |

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
    return targ.Deps(Build) // Generate won't run twice
}
```

Run independent tasks concurrently:

```go
func CI() error {
    return targ.ParallelDeps(Test, Lint, Build)
}
```

## Shell Helpers

Use `targ/sh` for command execution:

```go
import "github.com/toejough/targ/sh"

// Run a command, inherit stdout/stderr
err := sh.Run("go", "build", "./...")

// Capture output
out, err := sh.Output("go", "env", "GOMOD")

// Check if command exists
if sh.Which("docker") != "" { ... }
```

## File Checks

Skip work when files haven't changed:

```go
// Compare input modtimes against outputs
needs, err := targ.Newer([]string{"**/*.go"}, []string{"bin/app"})
if !needs {
    return nil
}
```

When outputs are empty, `Newer` uses a cached snapshot.

Content-based checking:

```go
import "github.com/toejough/targ/target"

changed, err := target.Checksum([]string{"**/*.go"}, ".targ/cache/build.sum")
if !changed {
    return nil
}
```

## Watch Mode

React to file changes:

```go
err := targ.Watch(ctx, []string{"**/*.go"}, targ.WatchOptions{}, func(changes targ.ChangeSet) error {
    return sh.Run("go", "test", "./...")
})
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

## Build Tool Flags

| Flag         | Description                                  |
| ------------ | -------------------------------------------- |
| `--no-cache` | Force rebuild of the build tool binary       |
| `--keep`     | Keep generated bootstrap file for inspection |

## Wrapper Generation

Generate struct wrappers for function commands to enable descriptions in library mode:

```bash
targ gen
```

This creates `generated_targ_<pkg>.go` with struct wrappers that include `Name()` and `Description()` methods from function comments.

In build tool mode, this happens automatically.

## Installation

```bash
go get github.com/toejough/targ
```
