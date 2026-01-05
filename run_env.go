package targ

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type runEnv interface {
	Args() []string
	Printf(format string, args ...any)
	Println(args ...any)
	Exit(code int)
}

type osRunEnv struct{}

func (osRunEnv) Args() []string {
	return os.Args
}

func (osRunEnv) Printf(format string, args ...any) {
	fmt.Printf(format, args...)
}

func (osRunEnv) Println(args ...any) {
	fmt.Println(args...)
}

func (osRunEnv) Exit(code int) {
	os.Exit(code)
}

func runWithEnv(env runEnv, opts RunOptions, targets ...interface{}) {
	ctx := context.Background()
	if _, ok := env.(osRunEnv); ok {
		rootCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer cancel()
		ctx = rootCtx
	}
	_ = withDepTracker(ctx, func() error {
		roots := []*CommandNode{}
		for _, t := range targets {
			node, err := parseTarget(t)
			if err != nil {
				env.Printf("Error parsing target: %v\n", err)
				continue
			}
			roots = append(roots, node)
		}

		if len(roots) == 0 {
			env.Println("No commands found.")
			return nil
		}

		singleRoot := len(roots) == 1
		hasDefault := singleRoot && opts.AllowDefault
		args := env.Args()
		if len(args) < 2 {
			if hasDefault {
				if err := roots[0].execute(ctx, nil); err != nil {
					env.Printf("Error: %v\n", err)
					env.Exit(1)
				}
				return nil
			}
			printUsage(roots)
			return nil
		}

		rest := args[1:]

		// 1. Check for runtime completion request (Hidden command)
		if rest[0] == "__complete" {
			// usage: __complete "entire command line"
			if len(rest) > 1 {
				if err := doCompletion(roots, rest[1]); err != nil {
					env.Printf("Error: %v\n", err)
				}
			}
			return nil
		}

		// 2. Check for completion script generation
		if rest[0] == "--completion" {
			shell := ""
			if len(rest) > 1 {
				shell = rest[1]
			}
			if shell == "" {
				shell = detectShell()
			}
			if shell == "" {
				env.Println("Usage: --completion [bash|zsh|fish]")
				return nil
			}
			binName := args[0]
			// Determine binary name from path
			if idx := strings.LastIndex(binName, "/"); idx != -1 {
				binName = binName[idx+1:]
			}
			if idx := strings.LastIndex(binName, "\\"); idx != -1 {
				binName = binName[idx+1:]
			}
			if err := PrintCompletionScript(shell, binName); err != nil {
				env.Printf("Unsupported shell: %s. Supported: bash, zsh, fish\n", shell)
			}
			return nil
		}

		// Handle global help
		if rest[0] == "-h" || rest[0] == "--help" {
			if hasDefault {
				printCommandHelp(roots[0])
				printTargOptions()
			} else {
				printUsage(roots)
			}
			return nil
		}

		if hasDefault {
			if len(rest) == 0 {
				if _, err := roots[0].executeWithParents(ctx, nil, nil, map[string]bool{}, false); err != nil {
					env.Printf("Error: %v\n", err)
					env.Exit(1)
				}
				return nil
			}
			remaining := rest
			for len(remaining) > 0 {
				next, err := roots[0].executeWithParents(ctx, remaining, nil, map[string]bool{}, false)
				if err != nil {
					env.Printf("Error: %v\n", err)
					env.Exit(1)
					return nil
				}
				if len(next) == len(remaining) {
					env.Printf("Unknown command: %s\n", remaining[0])
					env.Exit(1)
					return nil
				}
				remaining = next
			}
			return nil
		}

		remaining := rest
		for len(remaining) > 0 {
			var matched *CommandNode
			for _, root := range roots {
				if strings.EqualFold(root.Name, remaining[0]) {
					matched = root
					break
				}
			}

			if matched == nil {
				env.Printf("Unknown command: %s\n", remaining[0])
				printUsage(roots)
				env.Exit(1)
				return nil
			}

			next, err := matched.executeWithParents(ctx, remaining[1:], nil, map[string]bool{}, true)
			if err != nil {
				env.Printf("Error: %v\n", err)
				env.Exit(1)
				return nil
			}
			remaining = next
		}
		return nil
	})
}

func detectShell() string {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		return ""
	}
	base := shell
	if idx := strings.LastIndex(base, "/"); idx != -1 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndex(base, "\\"); idx != -1 {
		base = base[idx+1:]
	}
	switch base {
	case "bash", "zsh", "fish":
		return base
	default:
		return ""
	}
}
