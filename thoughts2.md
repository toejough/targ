# how does the tool discover commands?

I think target discovery should work like it does for mage - identify the exported functions.

I think _arg_ discovery should be done via struct tags on a struct type that is passed into the function.

# how much config vs just works?

I'd like mostly just works, like mage. If you want to run it as a standalone binary instead, you need to register the
commands in a main function. Otherwise, it just discovers them automatically.

# what does the graduation path look like?

If we are starting with a function/struct tag based approach, our starting point might look like this:

```go
package main
import "github.com/my/tool"

// MyArgs defines the arguments for the Greet command.
type MyArgs struct {
    Name string `tool:"required, description=The name of the user,env=USER_NAME"`
    Age  int    `tool:"-a, --age, placeholder=YEARS, description=The age of the user"`
}

// Greet greets the user with their name and age.
func Greet(args MyArgs) {
    fmt.Printf("Hello, %s! You are %d years old.\n", args.Name, args.Age)
}

// OtherArgs defines the arguments for the Farewell command.
type OtherArgs struct {
    Name string `tool:"required, description=The name of the user,env=USER_NAME"`
}

// Farewell bids farewell to the user with their name.
func Farewell(args OtherArgs) {
    fmt.Printf("Goodbye, %s! See you next time.\n", args.Name)
}

// Optional main function to register commands. If omitted, the tool
// would automatically discover commands in the package, but the file itself would not be independently runnable. If included,
// it would use the provided commands instead, and only be runnable as a standalone binary (the tool would ignore it).
func main() {
    tool.Run(Greet, Farewell)
}
```

The tool would use reflection to discover the commands (Greet, Farewell) and their associated argument structs (MyArgs,
OtherArgs). It would parse the struct tags to determine which arguments are required, their descriptions, placeholders,
and any environment variable bindings. When the user runs the tool from the command line, it would automatically
generate help text and parse the provided arguments accordingly.

Without main, you could call it like this:

```bash
$ tool greet --name Alice --age 30
Hello, Alice! You are 30 years old.
$ tool farewell --name Bob
Goodbye, Bob! See you next time.
$ tool greet --name Charlie --age 25 farewell --name Charlie
Hello, Charlie! You are 25 years old.
Goodbye, Charlie! See you next time.
```

With main, you could call it like this:

```bash
$ go build -o standalone
$ standalone greet --name Alice --age 30
Hello, Alice! You are 30 years old.
$ standalone farewell --name Bob
Goodbye, Bob! See you next time.
$ standalone greet --name Charlie --age 25 farewell --name Charlie
Hello, Charlie! You are 25 years old.
Goodbye, Charlie! See you next time.
```

If you only supply a single target, or if only a single target has a struct that isn't considered a subcommand of
another struct, you would not need to specify the target name:

```bash
$ tool --name Alice --age 30
Hello, Alice! You are 30 years old.
```

# how deep do subcommands go?

I think we should support arbitrary depth of subcommands, as long as the struct tags can express the hierarchy clearly.

# devil's advocate

I can see contributing to mage. I think first I want to explore what is possible.

# approaches

I covered that above, I think. I want functions for the targets and structs with tags for the args. Having a method for
the struct might be interesting, but I think it adds complexity that isn't necessary for simple build targets, which is
a significant use case. (I don't want to have to write an empty struct and then a method on it just to have a target
that takes no args.)
