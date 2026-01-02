# What's your primary use case?

I want to be able to add local build commands to my development workflow, and have first-class support for everything
I'd expect in a CLI. This includes:

- Parsing command-line arguments (positional, long flags, short flags, variadic)
- Handling environment variables
- Providing help and usage information
- Supporting subcommands
- shell completion integration (bash, zsh, fish)

I'd like to be able to just add target functions to a file, and have the tool find them and use them as CLI commands
(similar to mage).

A secondary use case is to have a way to transform these commands into a standalone CLI, such as perhaps by:

- deleting all non-target functions and types
- adding a main function that calls a library function that accepts the remaining target `commander.Use(target)`

It doesn't have to be that way, that's just an example I'm thinking of.

I want the configuration of the CLI to be as minimal and clear as possible, which is what I really like about
alex-flint/go-args. I want the addition of subcommands to be as simple as adding more target functions, like mage. And I
want built-in support for shell completion, like cobra.

# What specific frustration triggered this?

I wanted to add some issue tracker commands to a project, but was forced into awkward command patterns and a lack of CLI
completion support because of mage's limitations (no flags, variadic args, or completion support). Check out the
magefile in ~/repos/personal/imptest for an example of what I ended up with.

# Who's the target user?

Primarily myself, but also other go developers who want a better way to manage local build and development commands.
