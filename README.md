# Commander

Commander is a Go library for building CLIs with minimal configuration, combining the best of Mage (function-based discovery), go-arg (struct tags), and Cobra (subcommands).

## Features

- **Automatic Discovery**: Define commands as structs with a `Run` method.
- **Struct-based Arguments**: Define flags and arguments using struct tags.
- **Subcommands**: Use struct methods to create subcommands (e.g., `remote add`).
- **Environment Variables**: Bind flags to environment variables.
- **Positional Arguments**: Support for positional and variadic arguments.

## Usage

### Basic Command

Define commands as structs with a `Run` method. The struct fields define the arguments.

```go
package main

import (
    "fmt"
    "commander"
)

type Greet struct {
    Name string `commander:"required,desc=Name of the person"`
    Age  int    `commander:"name=age,desc=Age of the person"`
}

func (g *Greet) Run() {
    fmt.Printf("Hello %s (%d)\n", g.Name, g.Age)
}

func main() {
    commander.Run(&Greet{})
}
```

Run it:
```bash
$ go run main.go greet --name Alice --age 30
```

### Subcommands

Define subcommands using methods on a struct. The arguments are passed as a struct to the method.

```go
type Math struct{}

type AddArgs struct {
    A int `commander:"positional"`
    B int `commander:"positional"`
}

func (m Math) Add(args AddArgs) {
    fmt.Printf("%d + %d = %d\n", args.A, args.B, args.A+args.B)
}

func main() {
    commander.Run(Math{})
}
```

Run it:
```bash
$ go run main.go math add 10 20
```

### Tags

- `commander:"required"`: Flag is required.
- `commander:"desc=..."`: Description for help text.
- `commander:"name=..."`: Custom flag name.
- `commander:"env=VAR_NAME"`: Default value from environment variable.
- `commander:"positional"`: Map positional arguments to this field.

## Installation

```bash
go get github.com/yourusername/commander
```
