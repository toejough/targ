package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// ExitError is returned when a command exits with a non-zero code.
type ExitError struct {
	Code int
}

func (e ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// DetectShell returns the current shell name (bash, zsh, fish) or empty string.
func DetectShell() string {
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

// NewExecuteEnv returns a runEnv that captures output for testing.
func NewExecuteEnv(args []string) *executeEnv {
	return &executeEnv{args: args}
}

// NewOsEnv returns a runEnv that uses os.Args and real stdout/exit.
func NewOsEnv() runEnv {
	return osRunEnv{}
}

// RunWithEnv executes commands with a custom environment.
func RunWithEnv(env runEnv, opts RunOptions, targets ...any) error {
	exec := &runExecutor{
		env:        env,
		opts:       opts,
		args:       env.Args(),
		listFn:     doList,
		completeFn: doCompletion,
	}

	err := exec.setupContext()
	if err != nil {
		return err
	}

	if exec.cancelFunc != nil {
		defer exec.cancelFunc()
	}

	exec.extractHelpFlag()

	return withDepTracker(exec.ctx, func() error {
		err := exec.parseTargets(targets)
		if err != nil {
			return err
		}

		if len(exec.roots) == 0 {
			env.Println("No commands found.")
			return nil
		}

		exec.hasDefault = len(exec.roots) == 1 && opts.AllowDefault

		if len(exec.args) < 2 {
			return exec.handleNoArgs()
		}

		exec.rest = exec.args[1:]

		if handled, err := exec.handleSpecialCommands(); handled || err != nil {
			return err
		}

		if exec.hasDefault {
			return exec.executeDefault()
		}

		return exec.executeMultiRoot()
	})
}

// completeFunc is the function type for command completion.
type completeFunc func([]*commandNode, string) error

// executeEnv captures args and errors for testing.
type executeEnv struct {
	args   []string
	output strings.Builder
}

func (e *executeEnv) Args() []string {
	return e.args
}

func (e *executeEnv) Exit(_ int) {
	_ = 0 // No-op stub for coverage
}

// Output returns the captured output from command execution.
func (e *executeEnv) Output() string {
	return e.output.String()
}

func (e *executeEnv) Printf(format string, args ...any) {
	fmt.Fprintf(&e.output, format, args...)
}

func (e *executeEnv) Println(args ...any) {
	fmt.Fprintln(&e.output, args...)
}

// listCommandInfo represents a command in the __list output.
type listCommandInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// listFunc is the function type for listing commands.
type listFunc func([]*commandNode) error

// listOutput is the JSON structure returned by the __list command.
type listOutput struct {
	Commands []listCommandInfo `json:"commands"`
}

type osRunEnv struct{}

func (osRunEnv) Args() []string {
	return os.Args
}

func (osRunEnv) Exit(code int) {
	os.Exit(code)
}

func (osRunEnv) Printf(format string, args ...any) {
	fmt.Printf(format, args...)
}

func (osRunEnv) Println(args ...any) {
	fmt.Println(args...)
}

type runEnv interface {
	Args() []string
	Printf(format string, args ...any)
	Println(args ...any)
	Exit(code int)
}

// runExecutor holds state for executing commands.
type runExecutor struct {
	env        runEnv
	opts       RunOptions
	ctx        context.Context
	cancelFunc context.CancelFunc
	roots      []*commandNode
	args       []string
	rest       []string
	hasDefault bool
	listFn     listFunc     // injectable for testing, defaults to doList
	completeFn completeFunc // injectable for testing, defaults to doCompletion
}

// detectCompletionShell detects or extracts the shell for completion.
func (e *runExecutor) detectCompletionShell() string {
	if len(e.rest) > 1 && !strings.HasPrefix(e.rest[1], "-") {
		return e.rest[1]
	}

	return DetectShell()
}

// executeDefault executes commands against a single default root.
func (e *runExecutor) executeDefault() error {
	if len(e.rest) == 0 {
		if _, err := e.roots[0].executeWithParents(e.ctx, nil, nil, map[string]bool{}, false, e.opts); err != nil {
			e.env.Printf("Error: %v\n", err)
			return ExitError{Code: 1}
		}

		return nil
	}

	remaining := e.rest

	for len(remaining) > 0 {
		next, err := e.roots[0].executeWithParents(
			e.ctx,
			remaining,
			nil,
			map[string]bool{},
			false,
			e.opts,
		)
		if err != nil {
			e.env.Printf("Error: %v\n", err)
			return ExitError{Code: 1}
		}

		if len(next) == len(remaining) {
			e.env.Printf("Unknown command: %s\n", remaining[0])
			return ExitError{Code: 1}
		}

		remaining = next
	}

	return nil
}

// executeMultiRoot executes commands against multiple roots.
func (e *runExecutor) executeMultiRoot() error {
	remaining := e.rest

	for len(remaining) > 0 {
		matched := e.findMatchingRoot(remaining[0])

		if matched == nil {
			e.env.Printf("Unknown command: %s\n", remaining[0])
			printUsage(e.roots, e.opts)

			return ExitError{Code: 1}
		}

		next, err := matched.executeWithParents(
			e.ctx,
			remaining[1:],
			nil,
			map[string]bool{},
			true,
			e.opts,
		)
		if err != nil {
			e.env.Printf("Error: %v\n", err)
			return ExitError{Code: 1}
		}

		remaining = next
	}

	return nil
}

// extractHelpFlag checks for --help and enters help-only mode.
func (e *runExecutor) extractHelpFlag() {
	if e.opts.DisableHelp {
		return
	}

	if helpFound, remaining := extractHelpFlag(e.args); helpFound {
		e.opts.HelpOnly = true
		e.args = remaining
	}
}

// findMatchingRoot finds a root command matching the given name.
func (e *runExecutor) findMatchingRoot(name string) *commandNode {
	for _, root := range e.roots {
		if strings.EqualFold(root.Name, name) {
			return root
		}
	}

	return nil
}

// handleComplete handles the __complete hidden command.
func (e *runExecutor) handleComplete() error {
	if len(e.rest) > 1 {
		err := e.completeFn(e.roots, e.rest[1])
		if err != nil {
			e.env.Printf("Error: %v\n", err)
		}
	}

	return nil
}

// handleCompletionFlag handles --completion flag.
func (e *runExecutor) handleCompletionFlag() (bool, error) {
	if e.opts.DisableCompletion {
		return false, nil
	}

	if e.rest[0] == "--completion" {
		return true, e.printCompletion(e.detectCompletionShell())
	}

	if after, ok := strings.CutPrefix(e.rest[0], "--completion="); ok {
		return true, e.printCompletion(after)
	}

	return false, nil
}

// handleGlobalHelp handles global help when HelpOnly mode is set.
func (e *runExecutor) handleGlobalHelp() (bool, error) {
	if !e.opts.HelpOnly {
		return false, nil
	}

	if e.hasDefault {
		printCommandHelp(e.roots[0])
		printTargOptions(e.opts)
	} else {
		printUsage(e.roots, e.opts)
	}

	return true, nil
}

// handleList handles the __list hidden command.
func (e *runExecutor) handleList() error {
	err := e.listFn(e.roots)
	if err != nil {
		e.env.Printf("Error: %v\n", err)
		return ExitError{Code: 1}
	}

	return nil
}

// handleNoArgs handles the case when no command arguments are provided.
func (e *runExecutor) handleNoArgs() error {
	if e.hasDefault {
		err := e.roots[0].execute(e.ctx, nil, e.opts)
		if err != nil {
			e.env.Printf("Error: %v\n", err)
			return ExitError{Code: 1}
		}

		return nil
	}

	printUsage(e.roots, e.opts)

	return nil
}

// handleSpecialCommands handles __complete, __list, help, and completion flags.
func (e *runExecutor) handleSpecialCommands() (bool, error) {
	if e.rest[0] == "__complete" {
		return true, e.handleComplete()
	}

	if e.rest[0] == "__list" {
		return true, e.handleList()
	}

	if handled, err := e.handleGlobalHelp(); handled {
		return true, err
	}

	return e.handleCompletionFlag()
}

// parseTargets parses all targets into command nodes.
func (e *runExecutor) parseTargets(targets []any) error {
	e.roots = make([]*commandNode, 0, len(targets))

	for _, t := range targets {
		node, err := parseTarget(t)
		if err != nil {
			e.env.Printf("Error parsing target: %v\n", err)
			continue
		}

		e.roots = append(e.roots, node)
	}

	return nil
}

// printCompletion prints the completion script for the given shell.
func (e *runExecutor) printCompletion(shell string) error {
	if shell == "" {
		e.env.Println("Usage: --completion [bash|zsh|fish]")
		e.env.Println("Could not detect shell. Please specify one.")

		return ExitError{Code: 1}
	}

	err := PrintCompletionScript(shell, binaryName())
	if err != nil {
		e.env.Printf("Error: %v\n", err)
		return ExitError{Code: 1}
	}

	return nil
}

// setupContext creates the execution context with optional signal handling and timeout.
func (e *runExecutor) setupContext() error {
	e.ctx = context.Background()

	if _, ok := e.env.(osRunEnv); ok {
		ctx, cancel := signal.NotifyContext(e.ctx, os.Interrupt, syscall.SIGTERM)
		e.ctx = ctx
		e.cancelFunc = cancel
	}

	if e.opts.DisableTimeout {
		return nil
	}

	timeout, remaining, err := extractTimeout(e.args)
	if err != nil {
		e.env.Printf("Error: %v\n", err)
		return ExitError{Code: 1}
	}

	e.args = remaining

	if timeout > 0 {
		ctx, cancel := context.WithTimeout(e.ctx, timeout)
		e.ctx = ctx

		prevCancel := e.cancelFunc
		e.cancelFunc = func() {
			cancel()

			if prevCancel != nil {
				prevCancel()
			}
		}
	}

	return nil
}

// collectCommands recursively collects command info from a node and its subcommands.
func collectCommands(node *commandNode, prefix string, commands *[]listCommandInfo) {
	name := node.Name
	if prefix != "" {
		name = prefix + " " + name
	}

	*commands = append(*commands, listCommandInfo{
		Name:        name,
		Description: node.Description,
	})

	// Recursively collect subcommands
	for _, sub := range node.Subcommands {
		collectCommands(sub, name, commands)
	}
}

// doList outputs JSON with command names and descriptions to stdout.
func doList(roots []*commandNode) error {
	return doListTo(os.Stdout, roots)
}

// doListTo outputs JSON with command names and descriptions to the given writer.
func doListTo(w io.Writer, roots []*commandNode) error {
	output := listOutput{
		Commands: make([]listCommandInfo, 0),
	}

	for _, node := range roots {
		collectCommands(node, "", &output.Commands)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(output)
}

// extractHelpFlag checks if -h or --help is in args and returns remaining args.
func extractHelpFlag(args []string) (bool, []string) {
	var result []string

	helpFound := false

	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			helpFound = true
			continue
		}

		result = append(result, arg)
	}

	return helpFound, result
}

// extractTimeout looks for --timeout flag and returns the duration and remaining args.
func extractTimeout(args []string) (time.Duration, []string, error) {
	var (
		result  []string
		timeout time.Duration
	)

	skip := false

	for i, arg := range args {
		if skip {
			skip = false
			continue
		}

		if arg == "--timeout" {
			if i+1 >= len(args) {
				return 0, nil, errors.New("--timeout requires a duration value (e.g., 10m, 1h)")
			}

			d, err := time.ParseDuration(args[i+1])
			if err != nil {
				return 0, nil, fmt.Errorf("invalid timeout duration %q: %w", args[i+1], err)
			}

			timeout = d
			skip = true

			continue
		}

		if after, ok := strings.CutPrefix(arg, "--timeout="); ok {
			val := after

			d, err := time.ParseDuration(val)
			if err != nil {
				return 0, nil, fmt.Errorf("invalid timeout duration %q: %w", val, err)
			}

			timeout = d

			continue
		}

		result = append(result, arg)
	}

	return timeout, result, nil
}
