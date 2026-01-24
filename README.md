# Targ

<p align="center">
  <img src="assets/targ.png" alt="Targ logo" width="500">
</p>

Build CLIs and run build targets with minimal configuration. Inspired by Mage, go-arg, and Cobra.

## Quick Reference

| Want to...          | Do this                                               |
| ------------------- | ----------------------------------------------------- |
| Run build targets   | `//go:build targ` files + `targ <command>`            |
| Define a target     | `var Build = targ.Targ(build)`                        |
| Add flags/args      | Function with struct parameter + `targ:"..."` tags    |
| Shell command target| `var Tidy = targ.Targ("go mod tidy")`                 |
| Run shell commands  | `targ.Run("go", "build")` or `targ.RunContext(ctx, ...)`  |
| Skip unchanged work | `targ.Newer(inputs, outputs)` or `targ.Checksum(...)` |
| Watch for changes   | `targ.Watch(ctx, patterns, opts, callback)`           |
| Run deps once       | `targ.Deps(A, B, C)` or `.Deps(A, B)`                 |
| Scaffold a target   | `targ --create build` or `targ --create tidy "go mod tidy"` |

## Installation

```bash
# Build tool (run targets)
go install github.com/toejough/targ/cmd/targ@latest

# Library (embed in your binary)
go get github.com/toejough/targ
```

## From Build Targets to Dedicated CLI

Targ makes it easy to start with simple build targets and evolve to a full CLI. The same code works in both modes.

### Stage 1: String Commands

Scaffold targets from the command line:

```bash
targ --create test "go test -race $package"
targ --create lint "golangci-lint run --fix $path"
```

This creates a targ file with string command targets. Variables like `$package` become CLI flags:

```bash
targ test --package=./...
targ test -p ./cmd/...           # short flags auto-generated
targ lint --path=./internal/...
```

The generated file looks like:

```go
//go:build targ

package dev

import "github.com/toejough/targ"

func init() {
    targ.Register(
        targ.Targ("go test -race $package").Name("test"),
        targ.Targ("golangci-lint run --fix $path").Name("lint"),
    )
}
```

Commands run via the system shell, so pipes and shell features work:

```go
targ.Targ("go test -coverprofile=coverage.out $package && go tool cover -html=coverage.out")
```

### Stage 2: Programmatic Flags

Need conditional logic or computed values? Use a function with a struct parameter:

```go
//go:build targ
// ↑ Build tag: only compiled when running `targ` command, ignored by `go build`

package dev

import "github.com/toejough/targ"

// Register targets at init time so targ can discover them
func init() {
    targ.Register(
        targ.Targ(build).Description("Compile the project"),
        targ.Targ(test).Description("Run tests"),
    )
}

// Struct fields become CLI flags. Tags control flag behavior:
// - flag: explicitly mark as flag (default for struct fields)
// - short=X: single-letter alias (-o instead of --output)
// - default=X: value when flag not provided
// - desc=X: help text shown in --help
type BuildArgs struct {
    Output  string `targ:"flag,short=o,default=myapp,desc=Output binary name"`
    Verbose bool   `targ:"flag,short=v,desc=Verbose output"`
}

// Function receives parsed args. Use values for conditional logic.
func build(args BuildArgs) error {
    cmdArgs := []string{"build", "-o", args.Output}
    if args.Verbose {
        cmdArgs = append(cmdArgs, "-v")
    }
    return targ.Run("go", append(cmdArgs, "./...")...)
}

type TestArgs struct {
    Cover bool `targ:"flag,desc=Enable coverage"`
}

func test(args TestArgs) error {
    cmdArgs := []string{"test"}
    if args.Cover {
        cmdArgs = append(cmdArgs, "-cover")
    }
    return targ.Run("go", append(cmdArgs, "./...")...)
}
```

```bash
targ build --output=myapp --verbose
targ test --cover
```

### Stage 3: Dedicated Binary

Ready to ship? Remove the build tag and switch to main:

```go
package main

import "github.com/toejough/targ"

func main() {
    targ.Main(
        targ.Targ(build).Description("Compile the project"),
        targ.Targ(test).Description("Run tests"),
    )
}

// ... same function definitions as Stage 2
```

```bash
go build -o mytool .
./mytool build --verbose
./mytool test --cover
```

### Multi-Directory Layout

In build tool mode, discovery is recursive. Commands are namespaced by earliest unique path:

```text
repo/
  tools/
    issues/
      targets.go      # //go:build targ → targ issues <cmd>
    deploy/
      staging.go      # //go:build targ → targ deploy staging <cmd>
      prod.go         # //go:build targ → targ deploy prod <cmd>
```

If only one tagged file exists, commands appear at the root (no namespace prefix).

## Target Builder

Configure targets with builder methods:

```go
var Build = targ.Targ(build).
    Name("build").              // CLI name (default: function name in kebab-case)
    Description("Build the app"). // Help text
    Deps(Generate, Compile).    // Run dependencies first (serial)
    ParallelDeps(Lint, Test).   // Run dependencies first (parallel)
    Cache("**/*.go", "go.mod"). // Skip if files unchanged
    Watch("**/*.go").           // Re-run on file changes
    Timeout(5 * time.Minute).   // Execution timeout
    Times(3).                   // Run multiple times
    Retry().                    // Continue on failure
    Backoff(time.Second, 2.0)   // Exponential backoff between retries
```

| Method | Description |
|--------|-------------|
| `.Name(s)` | Override CLI command name |
| `.Description(s)` | Help text |
| `.Deps(targets...)` | Serial dependencies |
| `.ParallelDeps(targets...)` | Parallel dependencies |
| `.Cache(patterns...)` | Skip if files unchanged |
| `.CacheDir(dir)` | Cache checksum directory |
| `.Watch(patterns...)` | Re-run on file changes |
| `.Timeout(d)` | Execution timeout |
| `.Times(n)` | Number of iterations |
| `.Retry()` | Continue despite failures |
| `.Backoff(initial, factor)` | Exponential backoff |
| `.While(fn)` | Run while predicate is true |

## Tags

Configure struct fields with `targ:"..."` tags:

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

Combine with commas: `targ:"positional,required,enum=dev|prod"`

## Groups

Use `targ.NewGroup` to organize targets into named groups:

```go
func init() {
    add := targ.Targ(func(args struct {
        A, B int `targ:"positional"`
    }) {
        fmt.Printf("%d\n", args.A+args.B)
    }).Name("add")

    multiply := targ.Targ(func(args struct {
        A, B int `targ:"positional"`
    }) {
        fmt.Printf("%d\n", args.A*args.B)
    }).Name("multiply")

    targ.Register(targ.NewGroup("math", add, multiply))
}
```

```bash
targ math add 2 3      # 5
targ math multiply 2 3 # 6
```

## Function Signatures

Target functions support these signatures:

- `func()`
- `func() error`
- `func(ctx context.Context)`
- `func(ctx context.Context) error`
- `func(args T)` where T is a struct
- `func(args T) error`
- `func(ctx context.Context, args T)`
- `func(ctx context.Context, args T) error`

## Command Names

Names are derived from function names, converted to kebab-case:

| Definition        | Command     |
| ----------------- | ----------- |
| `func BuildAll()` | `build-all` |
| `func RunTests()` | `run-tests` |

Override with `.Name()`:

```go
targ.Targ(build).Name("compile")
```

## Dependencies

### Using targ.Deps()

Run dependencies exactly once per invocation. By default, Deps **fails fast** - stops on first error:

```go
func build() error {
    return targ.Deps(Generate, Compile)  // stops if Generate fails
}

func test() error {
    return targ.Deps(Build)  // Build only runs once even if called multiple times
}
```

```bash
targ test lint  # runs Generate, Compile, Build, Test, then Lint - each only once
```

Options can be mixed with targets:

```go
// Parallel execution (fail-fast, cancels siblings on error)
func ci() error {
    return targ.Deps(Test, Lint, targ.Parallel())
}

// Run all even if some fail
func checkAll() error {
    return targ.Deps(Lint, Test, Vet, targ.Parallel(), targ.ContinueOnError())
}

// Pass context for cancellation
func watch(ctx context.Context) error {
    return targ.Watch(ctx, []string{"**/*.go"}, targ.WatchOptions{}, func(_ targ.ChangeSet) error {
        targ.ResetDeps()
        return targ.Deps(Tidy, Lint, Test, targ.WithContext(ctx))
    })
}
```

| Option | Effect |
|--------|--------|
| `targ.Parallel()` | Run concurrently instead of sequentially |
| `targ.ContinueOnError()` | Run all targets, return first error |
| `targ.WithContext(ctx)` | Pass context to targets (for cancellation) |

### Using .Deps() Builder

For static dependencies, use the builder method:

```go
targ.Targ(test).Deps(build)           // serial
targ.Targ(ci).ParallelDeps(test, lint)  // parallel
```

## Shell Helpers

Run commands with `targ.Run` and friends:

```go
err := targ.Run("go", "build", "./...")           // inherit stdout/stderr
err := targ.RunV("go", "test", "./...")           // print command first
out, err := targ.Output("go", "env", "GOMOD")     // capture output
```

For cancellable commands (e.g., in watch mode), use context variants. When cancelled, the entire process tree is killed:

```go
err := targ.RunContext(ctx, "go", "test", "./...")
err := targ.RunContextV(ctx, "golangci-lint", "run")
out, err := targ.OutputContext(ctx, "go", "list", "./...")
```

## File Checks

Skip work when files haven't changed:

```go
needs, err := targ.Newer([]string{"**/*.go"}, []string{"bin/app"})
if !needs {
    return nil  // outputs are up to date
}
```

Content-based checking (when modtimes aren't reliable):

```go
changed, err := targ.Checksum([]string{"**/*.go"}, ".cache/build.sum")
if !changed {
    return nil
}
```

## Watch Mode

### Manual Watch

React to file changes:

```go
func watch(ctx context.Context) error {
    return targ.Watch(ctx, []string{"**/*.go"}, targ.WatchOptions{}, func(_ targ.ChangeSet) error {
        targ.ResetDeps()  // clear dep cache so targets run again
        return targ.RunContext(ctx, "go", "test", "./...")
    })
}
```

### Builder Watch

Use `.Watch()` for declarative watch mode:

```go
targ.Targ(test).Watch("**/*.go", "**/*_test.go")
```

When run with watch patterns, the target re-runs automatically on file changes.

## Shell Completion

```bash
source <(your-binary --completion bash)   # Bash
source <(your-binary --completion zsh)    # Zsh
your-binary --completion fish | source    # Fish
```

Supports commands, subcommands, flags, and enum values.

## Example Help Output

```
$ targ build --help
Compile the project

Source: dev/targets.go:42

Usage: build [flags]

Flags:
  -o, --output    Output binary name (default: myapp)
  -v, --verbose   Verbose output
  -h, --help      Show this help

Execution:
  Deps: generate, compile (serial)
  Cache: **/*.go, go.mod
```

## Dynamic Tag Options

Override tag options at runtime by implementing `TagOptions` on your args struct:

```go
type DeployArgs struct {
    Env string `targ:"positional,enum=dev|prod"`
}

func (d DeployArgs) TagOptions(field string, opts targ.TagOptions) (targ.TagOptions, error) {
    if field == "Env" {
        opts.Enum = strings.Join(loadEnvsFromConfig(), "|")
    }
    return opts, nil
}

deploy := targ.Targ(func(args DeployArgs) error {
    // deploy to args.Env
    return nil
})
```

Useful for loading enum values from config, conditional required fields, or environment-specific defaults.

## Patterns

### Conditional Build

```go
func build() error {
    needs, _ := targ.Newer([]string{"**/*.go"}, []string{"bin/app"})
    if !needs {
        fmt.Println("up to date")
        return nil
    }
    return targ.Run("go", "build", "-o", "bin/app", "./...")
}
```

### CI Pipeline

```go
func ci() error {
    if err := targ.Deps(generate); err != nil {
        return err
    }
    if err := targ.Deps(build, lint, targ.Parallel()); err != nil {
        return err
    }
    return targ.Deps(test)
}
```

### Testing Commands

```go
func TestDeploy(t *testing.T) {
    deploy := targ.Targ(func(args DeployArgs) error { /* ... */ return nil })
    result, err := targ.Execute([]string{"app", "deploy", "prod", "--force"}, deploy)
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
type CatArgs struct {
    Files []string `targ:"positional,required"`
}

func init() {
    targ.Register(targ.Targ(func(args CatArgs) error {
        for _, f := range args.Files {
            // process each file
        }
        return nil
    }).Name("cat"))
}
```

```bash
targ cat file1.txt file2.txt file3.txt
```

### Ordered Repeated Flags

When flag order matters (e.g., include/exclude filters), use `[]targ.Interleaved[T]`:

```go
type FilterArgs struct {
    Include []targ.Interleaved[string] `targ:"flag,short=i"`
    Exclude []targ.Interleaved[string] `targ:"flag,short=e"`
}

func init() {
    targ.Register(targ.Targ(func(args FilterArgs) error {
        type rule struct {
            include bool
            pattern string
            pos     int
        }
        var rules []rule
        for _, inc := range args.Include {
            rules = append(rules, rule{true, inc.Value, inc.Position})
        }
        for _, exc := range args.Exclude {
            rules = append(rules, rule{false, exc.Value, exc.Position})
        }
        sort.Slice(rules, func(i, j int) bool {
            return rules[i].pos < rules[j].pos
        })
        // rules now in original command-line order
        return nil
    }).Name("filter"))
}
```

```bash
targ filter -i "*.go" -e "vendor/*" -i "*.md"
# Processes in order: include *.go, exclude vendor/*, include *.md
```

## When to Use Targ

| Need                                | Tool         |
| ----------------------------------- | ------------ |
| Build targets + CLI parsing         | **Targ**     |
| Simple build targets only           | Targ or Mage |
| Simple struct-to-flags mapping      | go-arg       |
| Complex CLI with plugins/middleware | Cobra        |

**Targ's sweet spot**: Build automation that can evolve into a full CLI, or CLI parsing with minimal boilerplate.

## Build Tool Flags

| Flag                        | Description                                  |
| --------------------------- | -------------------------------------------- |
| `--no-cache`                | Force rebuild of the build tool binary       |
| `--keep`                    | Keep generated bootstrap file for inspection |
| `--create NAME [CMD]`       | Create a new target (function or shell)      |
| `--completion [bash\|zsh\|fish]` | Print shell completion script           |

### Quick Target Scaffolding

Use `--create` to add targets:

```bash
targ --create build                    # creates function target
targ --create tidy "go mod tidy"       # creates shell command target
targ --create lint --deps=fmt,tidy     # with dependencies
targ --create test --cache="**/*.go"   # with caching
```

Kebab-case names are converted to PascalCase (`run-tests` → `RunTests`).

## Cache Management

Targ caches compiled binaries in `~/.cache/targ/`. The cache is invalidated when source files or `go.mod`/`go.sum` change.

```bash
targ --no-cache <command>   # force rebuild
rm -rf ~/.cache/targ/       # clear all cached binaries
```
