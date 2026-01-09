# The actual output

```
❯ targ --help
targ is a build-tool runner that discovers tagged commands and executes them.

Usage: targ [--no-cache] [--keep] [args]

Flags:
    --no-cache disable cached build tool binaries
    --keep keep generated bootstrap file
    --completion [bash|zsh|fish]

More info: https://github.com/toejough/targ#readme
targ is a build-tool runner that discovers tagged commands and executes them.

Usage: targ [--no-cache] [--keep] [args]

Flags:
    --no-cache disable cached build tool binaries
    --keep keep generated bootstrap file
    --completion [bash|zsh|fish]

More info: https://github.com/toejough/targ#readme

Commands:
    create
    dedupe
    list
    move
    update
    validate

```

# Desired output

```
❯ targ --help
targ is a build-tool runner that discovers tagged commands and executes them.

Usage: targ [FLAGS...] COMMAND [COMMAND_ARGS...]

Flags:
    --no-cache                      disable cached build tool binaries
    --keep                          keep generated bootstrap file
    --completion [bash|zsh|fish]    print completion script for specified shell. Uses the current shell if none is
                                    specified. The output should be eval'd/sourced in the shell to enable completions.                                    (e.g. 'targ --completion fish | source')
    --help                          Print help information

Commands:
    create      <command description>
    dedupe      <command description>
    list        <command description>
    move        <command description>
    update      <command description>
    validate    <command description>

More info: https://github.com/toejough/targ#readme
```
