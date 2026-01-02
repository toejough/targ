# Commander

Commander is a Go library for building CLIs with minimal configuration, combining the best of Mage (function-based discovery), go-arg (struct tags), and Cobra (subcommands).

## Features

- **Automatic Discovery**: Define commands as structs with a `Run` method.
- **Struct-based Arguments**: Define flags and arguments using struct tags.
- **Subcommands**: Use struct fields to create subcommands.
- **Build Tool Mode**: Run a folder of commands without writing a `main` function (Mage-style).

## Usage

### 1. Library Mode

Embed `commander` in your own main function.

```go
package main

import (
    "fmt"
    "commander"
)

type Greet struct {
    Name string `commander:"required"`
}

func (g *Greet) Run() {
    fmt.Printf("Hello %s\n", g.Name)
}

func main() {
    commander.Run(&Greet{})
}
```

With a single root, you run flags directly without a command name:

```bash
$ your-binary --name Alice
```

If you register multiple roots, you select a command name first (e.g. `your-binary greet --name Alice`).

### 2. Build Tool Mode (Mage-style)

Create a `command.go` file (name doesn't matter) in a directory. DO NOT define a `main` function.

```go
package main

import "fmt"

type Build struct {
    Target string `commander:"flag"`
}

func (b *Build) Run() {
    fmt.Printf("Building %s\n", b.Target)
}
```

Install the `commander` tool:
```bash
go install github.com/yourusername/commander/cmd/commander@latest
```

Run commands in that directory:
```bash
$ commander build -target prod
```

### Subcommands

Define subcommands using fields with the `commander:"subcommand"` tag.

```go
type Math struct {
    // Command: "add"
    Add *AddCmd `commander:"subcommand"`
    // Command: "run" (aliased)
    RunCmd *RunCmd `commander:"subcommand=run"`
}

func (m *Math) Run() {
    // This runs if you type just `math`
    fmt.Println("Math root")
}

type AddCmd struct {
    A int `commander:"positional"`
    B int `commander:"positional"`
}

func (a *AddCmd) Run() {
    fmt.Printf("%d + %d = %d\n", a.A, a.B, a.A+a.B)
}
```

When a root has subcommands, its `Run` method is used as the fallback when no subcommand is provided.

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
$ commander greet --help
Usage: greet [flags] [subcommand]

Greet the user.
This command prints a greeting message.

Flags:
...
```

### Tags

- `commander:"required"`: Flag is required.
- `commander:"desc=..."`: Description for help text (for flags).
- `commander:"name=..."`: Custom flag name.
- `commander:"short=..."`: Short flag alias (e.g., `short=n` for `-n`).
- `commander:"subcommand=..."`: Rename subcommand.
- `commander:"env=VAR_NAME"`: Default value from environment variable.
- `commander:"positional"`: Map positional arguments to this field.

## Shell Completion

To enable shell completion, generate the script and source it.

```bash
# Bash
source <(your-binary completion bash)

# Zsh
source <(your-binary completion zsh)

# Fish
your-binary completion fish | source
```

The completion supports:
- Commands and subcommands
- Flags (including short flags)

```bash
go get github.com/yourusername/commander
```
