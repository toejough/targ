# Targs

Targs is a Go library for building CLIs with minimal configuration, combining the best of Mage (function-based discovery), go-arg (struct tags), and Cobra (subcommands).

## Features

- **Automatic Discovery**: Define commands as structs with a `Run` method or niladic functions.
- **Struct-based Arguments**: Define flags and arguments using struct tags.
- **Subcommands**: Use struct fields to create subcommands.
- **Build Tool Mode**: Run a folder of commands without writing a `main` function (Mage-style).

## Usage

### 1. Library Mode

Embed `targs` in your own main function.

```go
package main

import (
    "fmt"
    "targs"
)

type Greet struct {
    Name string `targs:"required"`
}

func (g *Greet) Run() {
    fmt.Printf("Hello %s\n", g.Name)
}

func main() {
    targs.Run(&Greet{})
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
    targs.Run(Clean)
}
```

### 2. Build Tool Mode (Mage-style)

Create a `command.go` file (name doesn't matter) in a directory. DO NOT define a `main` function.
Add the build tag `//go:build targs` to any file you want scanned.

```go
//go:build targs

package main

import "fmt"

type Build struct {
    Target string `targs:"flag"`
}

func (b *Build) Run() {
    fmt.Printf("Building %s\n", b.Target)
}
```

Install the `targs` tool:
```bash
go install github.com/yourusername/targs/cmd/targs@latest
```

Run commands in that directory:
```bash
$ targs build -target prod
```

Build tool mode rules:
- Discovery is recursive and only includes directories with the `//go:build targs` tag.
- Without `--multipackage`, Targs selects the shallowest tagged directory; if multiple exist at that depth, it errors.
- With `--multipackage`, package names are always inserted as the first subcommand.
- Build tool mode never has a default command.
- `--no-cache` forces rebuilding the build tool binary.

Example layout:

```text
repo/
  mage/
    build.go   //go:build targs (package build)
    deploy.go  //go:build targs (package deploy)
  tools/
    gen/
      gen.go   //go:build targs (package gen)
```

Example usage:

```bash
# Without --multipackage, Targs uses the shallowest tagged dir (repo/mage)
$ targs build
$ targs deploy

# With --multipackage, package name is always the first subcommand
$ targs --multipackage build build
$ targs --multipackage deploy deploy
$ targs --multipackage gen generate
```

Build tool example in this repo:

```bash
$ targs list
$ targs create --title "New Issue" --description "..." --priority Medium
$ targs move 4 --status done
$ targs update 40 --status done --description "..." --details "..."
```

### Subcommands

Define subcommands using fields with the `targs:"subcommand"` tag.

```go
type Math struct {
    // Command: "add"
    Add *AddCmd `targs:"subcommand"`
    // Command: "run" (aliased)
    RunCmd *RunCmd `targs:"subcommand=run"`
}

func (m *Math) Run() {
    // This runs if you type just `math`
    fmt.Println("Math root")
}

type AddCmd struct {
    A int `targs:"positional"`
    B int `targs:"positional"`
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
$ targs greet --help
Usage: greet [flags] [subcommand]

Greet the user.
This command prints a greeting message.

Flags:
...
```

For function commands, descriptions are only available via generated wrappers.
In build tool mode, `targs` generates wrappers automatically from function comments.
In direct binary mode, you can generate wrappers manually (see `targs gen`) and pass the generated struct to `Run`.

### Command Metadata Overrides

If you need to override the command name or description, implement the following optional methods:

```go
func (c *MyCmd) Name() string { return "CustomName" }
func (c *MyCmd) Description() string { return "Custom description." }
```

`Name` is treated like a struct name (it will be converted to kebab-case).
`Description` replaces any comment-derived description.

### Command Wrapper Generation

Use `targs gen` to generate wrappers for exported niladic functions in the current package.
This writes `generated_targs_<pkg>.go`, which defines a struct per function with `Run`,
`Name`, and (when comments exist) `Description`.

### Tags

- `targs:"required"`: Flag is required.
- `targs:"desc=..."`: Description for help text (for flags).
- `targs:"name=..."`: Custom flag name.
- `targs:"short=..."`: Short flag alias (e.g., `short=n` for `-n`).
- `targs:"enum=a|b|c"`: Allowed values for completion.
- `targs:"subcommand=..."`: Rename subcommand.
- `targs:"env=VAR_NAME"`: Default value from environment variable.
- `targs:"positional"`: Map positional arguments to this field.
- `targs:"default=VALUE"`: Default value (only supported default mechanism).

Defaults only come from `default=...` tags. Passing non-zero values in the struct you give to `targs.Run` will return an error.

## Dependencies

Use `targs.Deps` to run command dependencies exactly once per invocation:

```go
func Build() error {
    return targs.Deps(Test, Lint)
}
```

## File Checks

Use `targs.Newer` to check inputs against outputs (or an implicit cache when outputs are empty):

```go
needs, err := targs.Newer([]string{"**/*.go"}, []string{"bin/app"})
if err != nil {
    return err
}
if !needs {
    return nil
}
```

When outputs are empty, `Newer` compares the current matches and modtimes to a cached snapshot stored in the XDG cache directory.

## Watch Mode

Use `targs.Watch` to react to file additions, removals, and modifications:

```go
err := targs.Watch(ctx, []string{"**/*.go"}, targs.WatchOptions{}, func(changes targs.ChangeSet) error {
    return sh.Run("go", "test", "./...")
})
```

## Shell Helpers

Use `targs/sh` for simple shell execution helpers:

```go
import "targs/sh"

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
go get github.com/yourusername/targs
```
