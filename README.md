# Targ

Targ is a Go library for building CLIs with minimal configuration, with inspiration from Mage (function-based discovery), go-arg (struct tags), and Cobra (subcommands & completion). Named for build targets—you can "targ it" and move on.

## Features

- **Automatic Discovery**: Define commands as structs with a `Run` method or niladic functions.
- **Struct-based Arguments**: Define flags and arguments using struct tags.
- **Subcommands**: Use struct fields to create subcommands.
- **Build Tool Mode**: Run a folder of commands without writing a `main` function (Mage-style).

## Usage

### 1. Library Mode

Embed `targ` in your own main function.

```go
package main

import (
    "fmt"
    "targ"
)

type Greet struct {
    Name string `targ:"required"`
}

func (g *Greet) Run() {
    fmt.Printf("Hello %s\n", g.Name)
}

func main() {
    targ.Run(&Greet{})
}
```

With a single root, you run flags directly without a command name:

```bash
$ your-binary --name Alice
```

If you register multiple roots, you select a command name first (e.g. `your-binary greet --name Alice`).

Niladic functions can also be commands:

```go
func Clean() { fmt.Println("cleaning") }

func main() {
    targ.Run(Clean)
}
```

### 2. Build Tool Mode (Mage-style)

Create a `command.go` file (name doesn't matter) in a directory. DO NOT define a `main` function.
Add the build tag `//go:build targ` to any file you want scanned.

```go
//go:build targ

package main

import "fmt"

type Build struct {
    Target string `targ:"flag"`
}

func (b *Build) Run() {
    fmt.Printf("Building %s\n", b.Target)
}
```

Install the `targ` tool:

```bash
go install github.com/yourusername/targ/cmd/targ@latest
```

Run commands in that directory:

```bash
$ targ build --target prod
```

Build tool mode rules:

- Discovery is recursive and only includes directories with the `//go:build targ` tag.
- Without `--multipackage`, Targ selects the shallowest tagged directory; if multiple exist at that depth, it errors.
- With `--multipackage`, package names are always inserted as the first subcommand.
- Build tool mode never has a default command.
- `--no-cache` forces rebuilding the build tool binary.
- `--keep` keeps the generated bootstrap file for inspection.

Example layout:

```text
repo/
  mage/
    build.go   //go:build targ (package build)
    deploy.go  //go:build targ (package deploy)
  tools/
    gen/
      gen.go   //go:build targ (package gen)
```

Example usage:

```bash
# Without --multipackage, Targ uses the shallowest tagged dir (repo/mage)
$ targ build
$ targ deploy

# With --multipackage, package name is always the first subcommand
$ targ --multipackage build build
$ targ --multipackage deploy deploy
$ targ --multipackage gen generate
```

Build tool example in this repo:

```bash
$ targ list
$ targ create --title "New Issue" --description "..." --priority Medium
$ targ move 4 --status done
$ targ update 40 --status done --description "..." --details "..."
```

### Subcommands

Define subcommands using fields with the `targ:"subcommand"` tag.

```go
type Math struct {
    // Command: "add"
    Add *AddCmd `targ:"subcommand"`
    // Command: "run" (aliased)
    RunCmd *RunCmd `targ:"subcommand=run"`
}

func (m *Math) Run() {
    // This runs if you type just `math`
    fmt.Println("Math root")
}

type AddCmd struct {
    A int `targ:"positional"`
    B int `targ:"positional"`
}

func (a *AddCmd) Run() {
    fmt.Printf("%d + %d = %d\n", a.A, a.B, a.A+a.B)
}
```

When a root has subcommands, its `Run` method is used as the fallback when no subcommand is provided.

`Run` can be `func()`, `func() error`, `func(context.Context)`, or `func(context.Context) error`. Function commands support the same signatures.

### Command Description

Add documentation comments to your `Run` methods to populate the help text.

```go
// Greet the user.
// This command prints a greeting message.
func (g *Greet) Run() {
    // ...
}
```

This will appear in the help output:

```
$ targ greet --help
Usage: greet [flags] [subcommand]

Greet the user.
This command prints a greeting message.

Flags:
...
```

For function commands, descriptions are only available via generated wrappers.
In build tool mode, `targ` generates wrappers automatically from function comments.
In direct binary mode, you can generate wrappers manually (see `targ gen`) and pass the generated struct to `Run`.

### Command Metadata Overrides

If you need to override the command name or description, implement the following optional methods:

```go
func (c *MyCmd) Name() string { return "CustomName" }
func (c *MyCmd) Description() string { return "Custom description." }
```

`Name` is treated like a struct name (it will be converted to kebab-case).
`Description` replaces any comment-derived description.

### Command Wrapper Generation

Use `targ gen` to generate wrappers for exported niladic functions in the current package.
This writes `generated_targ_<pkg>.go`, which defines a struct per function with `Run`,
`Name`, and (when comments exist) `Description`.

### Tags

- `targ:"required"`: Flag is required.
- `targ:"desc=..."`: Description for help text (for flags).
- `targ:"name=..."`: Custom flag name.
- `targ:"short=..."`: Short flag alias (e.g., `short=n` for `-n`).
- `targ:"enum=a|b|c"`: Allowed values for completion.
- `targ:"subcommand=..."`: Rename subcommand.
- `targ:"env=VAR_NAME"`: Default value from environment variable.
- `targ:"positional"`: Map positional arguments to this field.
- `targ:"default=VALUE"`: Default value (only supported default mechanism).

Defaults only come from `default=...` tags. Passing non-zero values in the struct you give to `targ.Run` will return an error.

## Dependencies

Use `targ.Deps` to run command dependencies exactly once per invocation:

```go
func Build() error {
    return targ.Deps(Test, Lint)
}
```

Use `targ.ParallelDeps` to run independent tasks concurrently while still sharing dependencies:

```go
func CI() error {
    return targ.ParallelDeps(Test, Lint)
}
```

## File Checks

Use `targ.Newer` to check inputs against outputs (or an implicit cache when outputs are empty):

```go
needs, err := targ.Newer([]string{"**/*.go"}, []string{"bin/app"})
if err != nil {
    return err
}
if !needs {
    return nil
}
```

When outputs are empty, `Newer` compares the current matches and modtimes to a cached snapshot stored in the XDG cache directory.

Use `target.Checksum` to skip work when file contents are unchanged:

```go
import "targ/target"

changed, err := target.Checksum([]string{"**/*.go"}, ".targ/cache/build.sum")
if err != nil {
    return err
}
if !changed {
    return nil
}
```

## Watch Mode

Use `targ.Watch` to react to file additions, removals, and modifications:

```go
err := targ.Watch(ctx, []string{"**/*.go"}, targ.WatchOptions{}, func(changes targ.ChangeSet) error {
    return sh.Run("go", "test", "./...")
})
```

## Shell Helpers

Use `targ/sh` for simple shell execution helpers:

```go
import "targ/sh"

if err := sh.Run("go", "test", "./..."); err != nil {
    panic(err)
}
out, err := sh.Output("go", "env", "GOMOD")
```

## Shell Completion

To enable shell completion, generate the script and source it.

```bash
# Bash
source <(your-binary --completion bash)

# Zsh
source <(your-binary --completion zsh)

# Fish
your-binary --completion fish | source
```

The completion supports:

- Commands and subcommands
- Long/short flags
- Enum values declared via `enum=` tags

```bash
go get github.com/yourusername/targ
```
