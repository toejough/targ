package core

import (
	"context"
	"os"

	internalsh "github.com/toejough/targ/internal/sh"
)

// Command is a subprocess invocation with optional per-invocation environment
// overrides. Build one with Cmd, then run it with Run, RunV, or Output.
//
// Every terminal takes a context, so a Command always routes through the
// parallel printer when its target is running in parallel mode.
type Command struct {
	args []string
	env  []string
	name string
}

// Env adds an environment variable for this command only. It is repeatable, and
// the last value declared for a key wins. Declared variables are added to the
// inherited environment — they do not replace it.
func (c *Command) Env(key, value string) *Command {
	c.env = append(c.env, key+"="+value)

	return c
}

// Output runs the command and returns its combined output.
func (c *Command) Output(ctx context.Context) (string, error) {
	return internalsh.OutputContext(ctx, c.name, c.args, os.Stdin, c.environ())
}

// Run executes the command, streaming stdout/stderr.
// In parallel mode, output is routed through the parallel printer.
func (c *Command) Run(ctx context.Context) error {
	env, pw := parallelShellEnv(ctx)

	err := internalsh.RunContextWithIO(ctx, env, c.name, c.args, c.environ())

	if pw != nil {
		pw.Flush()
	}

	return err
}

// RunV executes the command, printing it first.
// In parallel mode, output is routed through the parallel printer.
func (c *Command) RunV(ctx context.Context) error {
	env, pw := parallelShellEnv(ctx)

	err := internalsh.RunContextV(ctx, env, c.name, c.args, c.environ())

	if pw != nil {
		pw.Flush()
	}

	return err
}

// environ returns the child's environment. It returns nil when no overrides are
// declared, so exec inherits the parent's environment unchanged — that is what
// keeps this builder a no-op for callers that declare no environment.
//
// os/exec uses the last value for a duplicated key, so appending the overrides
// after os.Environ() gives both override-wins and later-call-wins.
func (c *Command) environ() []string {
	if len(c.env) == 0 {
		return nil
	}

	return append(os.Environ(), c.env...)
}

// Cmd creates a Command for the named program with the given arguments.
//
//	Cmd("golangci-lint", "run").Env("GOLANGCI_LINT_CACHE", dir).Run(ctx)
func Cmd(name string, args ...string) *Command {
	return &Command{name: name, args: args, env: nil}
}

// OutputContext executes a command and returns combined output, with context support.
func OutputContext(ctx context.Context, name string, args ...string) (string, error) {
	return Cmd(name, args...).Output(ctx)
}
