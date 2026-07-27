package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// ExecuteEnv is a RunEnv implementation that captures output for testing.
type ExecuteEnv struct {
	args     []string
	output   strings.Builder
	exitCode int
	env      map[string]string // For testing environment variables
}

// NewExecuteEnv returns a RunEnv that captures output for testing.
func NewExecuteEnv(args []string) *ExecuteEnv {
	return &ExecuteEnv{args: args, env: make(map[string]string)}
}

// Args returns the command line arguments.
func (e *ExecuteEnv) Args() []string {
	return e.args
}

// BinaryName returns the binary name for test environments,
// derived from args[0], or "app" when no args were provided.
func (e *ExecuteEnv) BinaryName() string {
	if len(e.args) > 0 {
		return e.args[0]
	}

	return "app"
}

// Exit captures the exit code for testing instead of actually exiting.
func (e *ExecuteEnv) Exit(code int) {
	e.exitCode = code
}

// ExitCode returns the captured exit code (0 if Exit was never called).
func (e *ExecuteEnv) ExitCode() int {
	return e.exitCode
}

// Getenv returns the value of an environment variable.
// First checks the test environment map (set via SetEnv), then falls back to os.Getenv.
// This allows tests to use either SetEnv for isolation or t.Setenv for convenience.
func (e *ExecuteEnv) Getenv(key string) string {
	if v, ok := e.env[key]; ok {
		return v
	}

	return os.Getenv(key)
}

// Getwd returns the current working directory.
func (e *ExecuteEnv) Getwd() (string, error) {
	//nolint:wrapcheck // Thin wrapper passthrough to OS - wrapping adds untestable code paths
	return os.Getwd()
}

// Output returns the captured output from command execution.
func (e *ExecuteEnv) Output() string {
	return e.output.String()
}

// Printf writes formatted output to the captured buffer.
func (e *ExecuteEnv) Printf(format string, args ...any) {
	fmt.Fprintf(&e.output, format, args...)
}

// Println writes a line to the captured buffer.
func (e *ExecuteEnv) Println(args ...any) {
	fmt.Fprintln(&e.output, args...)
}

// SetEnv sets an environment variable for testing.
func (e *ExecuteEnv) SetEnv(key, value string) {
	e.env[key] = value
}

// Stdout returns a writer for stdout output.
// For test environments, this returns the captured output buffer.
func (e *ExecuteEnv) Stdout() io.Writer {
	return &e.output
}

// SupportsSignals returns false for test environments.
func (e *ExecuteEnv) SupportsSignals() bool {
	return false
}

// ExitError represents a non-zero exit code from command execution.
type ExitError struct {
	Code int
}

func (e ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// RunEnv abstracts the runtime environment for testing.
type RunEnv interface {
	Args() []string
	Printf(format string, args ...any)
	Println(args ...any)
	Exit(code int)
	// Stdout returns a writer for stdout output (help text, usage, etc.).
	// Production implementations return os.Stdout; test mocks return a buffer.
	Stdout() io.Writer
	// SupportsSignals returns true if signal handling should be enabled.
	// Production implementations return true; test mocks return false.
	SupportsSignals() bool
	// Getenv returns the value of an environment variable.
	Getenv(key string) string
	// Getwd returns the current working directory.
	Getwd() (string, error)
	// BinaryName returns the name of the executable for help/completion output.
	BinaryName() string
}

// Execute runs commands with the given args and returns results instead of exiting.
// This is useful for testing. Args should include the program name as the first element.
func Execute(args []string, targets ...any) (ExecuteResult, error) {
	return ExecuteWithOptions(args, RunOptions{AllowDefault: true}, targets...)
}

// ExecuteWithOptions runs commands with given args and options, returning results.
// This is useful for testing. Args should include the program name as the first element.
func ExecuteWithOptions(
	args []string,
	opts RunOptions,
	targets ...any,
) (ExecuteResult, error) {
	env := NewExecuteEnv(args)

	// Copy test environment variables if provided
	for k, v := range opts.Env {
		env.SetEnv(k, v)
	}

	err := RunWithEnv(env, opts, targets...)

	return ExecuteResult{Output: env.Output(), ExitCode: env.ExitCode()}, err
}

// RunWithEnv executes commands with a custom environment.
// It sets up output writers and environment functions from env,
// then delegates to runWithEnvInternal for the actual execution logic.
func RunWithEnv(env RunEnv, opts RunOptions, targets ...any) error {
	// Set stdout, binary name, getenv, and getwd from environment (unless caller provided them)
	opts.Stdout = env.Stdout()
	opts.BinaryName = env.BinaryName()

	if opts.Getenv == nil {
		opts.Getenv = env.Getenv
	}

	if opts.Getwd == nil {
		opts.Getwd = env.Getwd
	}

	exec := &runExecutor{
		env:        env,
		opts:       opts,
		args:       env.Args(),
		listFn:     doListTo,
		completeFn: doCompletion,
	}

	err := runWithEnvInternal(exec, env, opts, targets...)
	if err != nil {
		// Call Exit so test environments can capture the exit code
		var exitErr ExitError
		if errors.As(err, &exitErr) {
			env.Exit(exitErr.Code)
		} else {
			env.Exit(1)
		}
	}

	return err
}

// unexported constants.
const (
	minArgsWithCommand = 2
)

// unexported variables.
var (
	errTimeoutRequiresDuration = errors.New("--timeout requires a duration value (e.g., 10m, 1h)")
)

type completeFunc func(io.Writer, []*commandNode, string) error

type listCommandInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type listFunc func(io.Writer, []*commandNode) error

type listOutput struct {
	Commands []listCommandInfo `json:"commands"`
}

// parallelUnit is one CLI target scheduled by runUnitsParallel. It resolves
// from an explicit root/glob match, or - in default mode - a subcommand name
// to run against the sole root.
type parallelUnit struct {
	node     *commandNode
	name     string
	args     []string
	explicit bool
}

type runExecutor struct {
	env        RunEnv
	opts       RunOptions
	ctx        context.Context //nolint:containedctx // stored for command execution
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

	return detectShellFromPath(e.env.Getenv("SHELL"))
}

// executeGlobPattern handles execution of glob-matched targets.
func (e *runExecutor) executeGlobPattern(name string, opts RunOptions) error {
	matches := e.findMatchingRootsGlob(name)
	if len(matches) == 0 {
		e.env.Printf("No targets match pattern: %s\n", name)
		return ExitError{Code: 1}
	}

	for _, matched := range matches {
		_, err := matched.executeWithParents(
			e.ctx,
			nil, // No args passed to glob-expanded targets
			nil,
			map[string]bool{},
			true,
			opts,
		)
		if err != nil {
			e.env.Printf("Error: %v\n", err)
			return ExitError{Code: 1}
		}
	}

	return nil
}

func (e *runExecutor) executeRoots() error {
	if e.opts.Overrides.Parallel {
		return e.runUnitsParallel()
	}

	// A caller that already asked for a single non-executing pass gets exactly
	// that. Promoting it to resolve-then-execute would run the target under a
	// caller-supplied ResolveOnly, and would fire every HelpOnly-gated print
	// once per pass.
	if e.opts.HelpOnly || e.opts.ResolveOnly {
		return e.walkRoots(e.opts.ResolveOnly)
	}

	// Resolve the whole chain before running any of it: a target must not fire
	// its deps or its command for an invocation that is going to be rejected.
	err := e.walkRoots(true)
	if err != nil {
		return err
	}

	return e.walkRoots(false)
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

// extractOverrides parses runtime override flags from args.
func (e *runExecutor) extractOverrides() error {
	overrides, remaining, err := ExtractOverrides(e.args)
	if err != nil {
		return err
	}

	e.opts.Overrides = overrides
	e.args = remaining

	return nil
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

// findMatchingRootsGlob finds all root commands matching a glob pattern.
func (e *runExecutor) findMatchingRootsGlob(pattern string) []*commandNode {
	matches := make([]*commandNode, 0)

	for _, root := range e.roots {
		if matchesGlob(root.Name, pattern) {
			matches = append(matches, root)
		}

		// For ** patterns, also include subcommands
		if after, ok := strings.CutPrefix(pattern, "**"); ok {
			subMatches := expandRecursive(root, after)
			matches = append(matches, subMatches...)
		}
	}

	return matches
}

// handleComplete handles the __complete hidden command.
// Note: completeFn (doCompletion) effectively cannot return errors in normal usage
// because TagOptions errors during parsing cause the chain to be empty, and all
// suggestion functions check for empty chain before calling tagOptionsForField.
func (e *runExecutor) handleComplete() {
	if len(e.rest) > 1 {
		_ = e.completeFn(e.env.Stdout(), e.roots, e.rest[1])
	}
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
// Returns true if help was printed and command processing should stop.
func (e *runExecutor) handleGlobalHelp() bool {
	if !e.opts.HelpOnly {
		return false
	}

	// If arg names something dispatch can resolve, let command execution
	// handle help. In default mode any non-flag arg resolves against the
	// sole root (runWithEnvInternal prepends its name before dispatch), so
	// walking there - and printing whatever nodes it visits along the way -
	// is the same thing multi-root already does when arg matches a root.
	if len(e.rest) > 0 && !strings.HasPrefix(e.rest[0], "-") {
		if e.hasDefault {
			return false // Let command execution handle help
		}

		for _, root := range e.roots {
			if strings.EqualFold(root.Name, e.rest[0]) {
				return false // Let command execution handle help
			}
		}
	}

	// Show global help
	if e.hasDefault {
		printCommandHelp(e.env.Stdout(), e.roots[0], e.opts)
	} else {
		printUsage(e.env.Stdout(), e.roots, e.opts)
	}

	return true
}

// handleList handles the __list hidden command.
// Error handling is omitted because listFn only fails on write errors,
// which are unrecoverable at this level anyway.
//
//nolint:unparam // Returns nil for interface consistency; errors unrecoverable at this level
func (e *runExecutor) handleList() error {
	_ = e.listFn(e.env.Stdout(), e.roots)
	return nil
}

// handleNoArgs handles the case when no command arguments are provided.
func (e *runExecutor) handleNoArgs() error {
	if e.hasDefault {
		err := e.roots[0].execute(e.ctx, nil, e.opts)
		if err != nil {
			var re reportedError
			if !errors.As(err, &re) {
				e.env.Printf("Error: %v\n", err)
			}

			return ExitError{Code: 1}
		}

		return nil
	}

	printUsage(e.env.Stdout(), e.roots, e.opts)

	return nil
}

// handleSpecialCommands handles __complete, __list, help, and completion flags.
func (e *runExecutor) handleSpecialCommands() (bool, error) {
	if e.rest[0] == "__complete" {
		e.handleComplete()
		return true, nil
	}

	if e.rest[0] == "__list" {
		return true, e.handleList()
	}

	if e.handleGlobalHelp() {
		return true, nil
	}

	return e.handleCompletionFlag()
}

// parseTargets parses all targets into command nodes.
func (e *runExecutor) parseTargets(targets []any) error {
	e.roots = make([]*commandNode, 0, len(targets))
	seenNames := make(map[string]bool)

	for _, t := range targets {
		node, err := parseTarget(t)
		if err != nil {
			e.env.Printf("Error parsing target: %v\n", err)
			continue
		}

		// Check for duplicate names at the root level
		if seenNames[node.Name] {
			e.env.Printf("Error: duplicate target name %q\n", node.Name)
			return ExitError{Code: 1}
		}

		seenNames[node.Name] = true

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

	err := PrintCompletionScriptTo(e.env.Stdout(), shell, e.env.BinaryName())
	if err != nil {
		e.env.Printf("Error: %v\n", err)
		return ExitError{Code: 1}
	}

	return nil
}

// resolveUnitDefault resolves a unit against the default root and appends it to
// units if successful, or returns an error (either a genuine resolve error or
// unknown-command error).
func (e *runExecutor) resolveUnitDefault(units *[]parallelUnit, unit *parallelUnit, arg string) error {
	// Resolve before the fan-out rather than after: a unit that
	// consumes none of its args names no real subcommand of the sole
	// root, and nothing should run for an invocation that will fail.
	resolveOpts := e.opts
	resolveOpts.ResolveOnly = true

	next, resolveErr := unit.node.executeWithParents(
		e.ctx, unit.args, nil, map[string]bool{}, unit.explicit, resolveOpts)
	if resolveErr != nil {
		e.env.Printf("Error: %v\n", resolveErr)

		return ExitError{Code: 1}
	}

	if len(next) == len(unit.args) {
		e.env.Printf("Error: %v\n", fmt.Errorf("%w: %s", errUnknownCommand, arg))

		return ExitError{Code: 1}
	}

	*units = append(*units, *unit)

	return nil
}

// resolveUnits maps each non-flag arg in e.rest to a parallelUnit: glob
// expansion first, then an explicit root match, then - in default mode only
// - a subcommand name to run against the sole root. An arg matching none of
// these is an unknown-command usage error.
func (e *runExecutor) resolveUnits() ([]parallelUnit, error) {
	units := make([]parallelUnit, 0, len(e.rest))

	for _, arg := range e.rest {
		if strings.HasPrefix(arg, "-") {
			continue
		}

		if isGlobPattern(arg) {
			matches := e.findMatchingRootsGlob(arg)
			if len(matches) == 0 {
				e.env.Printf("No targets match pattern: %s\n", arg)
				return nil, ExitError{Code: 1}
			}

			for _, matched := range matches {
				units = append(units, parallelUnit{node: matched, name: matched.Name, explicit: true})
			}

			continue
		}

		if matched := e.findMatchingRoot(arg); matched != nil {
			units = append(units, parallelUnit{node: matched, name: arg, explicit: true})
			continue
		}

		if e.hasDefault {
			unit := parallelUnit{
				node:     e.roots[0],
				name:     arg,
				args:     []string{arg},
				explicit: false,
			}

			err := e.resolveUnitDefault(&units, &unit, arg)
			if err != nil {
				return nil, err
			}

			continue
		}

		e.env.Printf("Unknown command: %s\n", arg)
		printUsage(e.env.Stdout(), e.roots, e.opts)

		return nil, ExitError{Code: 1}
	}

	return units, nil
}

// runUnitsParallel resolves e.rest into units, then runs them concurrently
// bounded by parallelCap, sharing one printer and one fail-fast cancellation.
// It is the single fan-out shared by both default and multi-root -p dispatch.
func (e *runExecutor) runUnitsParallel() error {
	units, err := e.resolveUnits()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()

	maxNameLen := 0
	for _, u := range units {
		if len(u.name) > maxNameLen {
			maxNameLen = len(u.name)
		}
	}

	// Set up printer for parallel output
	const printerBufferMultiplier = 10

	printer := NewPrinter(e.opts.Stdout, len(units)*printerBufferMultiplier)

	resultCh := make(chan unitResultMsg, len(units))
	results := make([]TargetResult, len(units))

	e.startUnits(ctx, units, results, resultCh, printer, maxNameLen)

	firstErrIdx, firstErr := collectUnitResults(resultCh, results, cancel)

	printUnitResults(ctx, results, firstErrIdx, printer, maxNameLen, e.opts.Stdout)

	if firstErr != nil {
		return ExitError{Code: 1}
	}

	return nil
}

// setupContext creates the execution context with optional signal handling and timeout.
func (e *runExecutor) setupContext() error {
	// Use provided context if available, otherwise background
	if e.opts.Context != nil {
		e.ctx = e.opts.Context
	} else {
		e.ctx = context.Background()
	}

	// Thread the output writer through context so Print/Printf use it
	// instead of a global variable (avoids races in parallel tests).
	e.ctx = WithExecInfo(e.ctx, ExecInfo{Output: e.opts.Stdout})

	if e.env.SupportsSignals() {
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

// startUnits launches one goroutine per unit, bounded by a semaphore sized
// to parallelCap so top-level -p fan-out is capped the same way a parallel
// dep group is (#26). Acquire precedes the "starting..." print, so a queued
// unit does not announce itself before it runs; a unit that finds the run
// already canceled (a sibling failed) skips starting entirely (fail-fast).
func (e *runExecutor) startUnits(
	ctx context.Context,
	units []parallelUnit,
	results []TargetResult,
	resultCh chan<- unitResultMsg,
	printer *Printer,
	maxNameLen int,
) {
	sem := make(chan struct{}, max(1, parallelCap(len(units), runtime.GOMAXPROCS(0))))

	for i, unit := range units {
		results[i].Name = unit.name

		go func(idx int, unit parallelUnit) {
			sem <- struct{}{}
			defer func() { <-sem }()

			// Skip queued units once the run is canceled (fail-fast):
			// starting fresh work after a sibling failed would defeat the mode.
			if ctx.Err() != nil {
				resultCh <- unitResultMsg{index: idx, err: ctx.Err()}
				return
			}

			tctx := WithExecInfo(ctx, ExecInfo{
				Parallel:   true,
				Name:       unit.name,
				MaxNameLen: maxNameLen,
				Printer:    printer,
				Output:     e.opts.Stdout,
			})

			Print(tctx, "starting...\n")

			start := time.Now()

			_, err := unit.node.executeWithParents(
				tctx, unit.args, nil, map[string]bool{}, unit.explicit, e.opts)

			resultCh <- unitResultMsg{index: idx, err: err, duration: time.Since(start)}
		}(i, unit)
	}
}

// walkRoots runs the chain once. With resolveOnly set, every target parses its
// args and hands back its leftovers without executing, so an unresolvable
// token is reported before the first target runs.
func (e *runExecutor) walkRoots(resolveOnly bool) error {
	opts := e.opts
	opts.ResolveOnly = resolveOnly

	remaining := e.rest

	for len(remaining) > 0 {
		if remaining[0] == "^" {
			remaining = remaining[1:]

			continue
		}

		name := remaining[0]

		if isGlobPattern(name) {
			err := e.executeGlobPattern(name, opts)
			if err != nil {
				return err
			}

			remaining = remaining[1:]

			continue
		}

		matched := e.findMatchingRoot(name)
		if matched == nil {
			e.env.Printf("Unknown command: %s\n", name)
			printUsage(e.env.Stdout(), e.roots, e.opts)

			return ExitError{Code: 1}
		}

		next, err := matched.executeWithParents(
			e.ctx,
			remaining[1:],
			nil,
			map[string]bool{},
			!e.hasDefault,
			opts,
		)
		if err != nil {
			var re reportedError
			if !errors.As(err, &re) {
				e.env.Printf("Error: %v\n", err)
			}

			return ExitError{Code: 1}
		}

		remaining = next
	}

	return nil
}

// unitResultMsg carries one parallelUnit's outcome back from its goroutine
// to collectUnitResults.
type unitResultMsg struct {
	index    int
	err      error
	duration time.Duration
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

// collectUnitResults receives one unitResultMsg per unit, records it into
// results, and cancels the run on the first error so queued units can skip
// starting (see startUnits). Returns the index of the first failing unit
// (or -1 if none failed) and its error - error last, per staticcheck ST1008.
func collectUnitResults(
	resultCh <-chan unitResultMsg,
	results []TargetResult,
	cancel context.CancelFunc,
) (int, error) {
	var firstErr error

	firstErrIdx := -1

	for range results {
		msg := <-resultCh
		results[msg.index].Err = msg.err
		results[msg.index].Duration = msg.duration

		if msg.err != nil && firstErr == nil {
			firstErr = msg.err
			firstErrIdx = msg.index

			cancel()
		}
	}

	return firstErrIdx, firstErr
}

// detectShellFromPath extracts the shell name from a SHELL path.
// Accepts the value of the SHELL environment variable.
func detectShellFromPath(shell string) string {
	shell = strings.TrimSpace(shell)
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

	err := enc.Encode(output)
	if err != nil {
		return fmt.Errorf("encoding list output: %w", err)
	}

	return nil
}

// expandRecursive returns all subcommands under a node matching a suffix pattern.
func expandRecursive(node *commandNode, suffix string) []*commandNode {
	matches := make([]*commandNode, 0)

	for _, sub := range node.Subcommands {
		// If suffix is empty or "/*", match all
		if suffix == "" || suffix == "/" || suffix == "/*" {
			matches = append(matches, sub)
		} else if strings.HasPrefix(suffix, "/") && matchesGlob(sub.Name, strings.TrimPrefix(suffix, "/")) {
			matches = append(matches, sub)
		}

		// Always recurse for **
		subMatches := expandRecursive(sub, suffix)
		matches = append(matches, subMatches...)
	}

	return matches
}

// extractHelpFlag checks if -h or --help is in args and returns remaining args.
func extractHelpFlag(args []string) (bool, []string) {
	result := make([]string, 0, len(args))

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
	result := make([]string, 0, len(args))
	timeout := time.Duration(0)

	skip := false

	for i, arg := range args {
		if skip {
			skip = false
			continue
		}

		if arg == "--timeout" {
			if i+1 >= len(args) {
				return 0, nil, errTimeoutRequiresDuration
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

// isGlobPattern checks if a string contains glob metacharacters.
func isGlobPattern(s string) bool {
	return strings.Contains(s, "*")
}

// matchesGlob checks if a name matches a glob pattern.
// Supports * (any characters) at start, end, or both.
func matchesGlob(name, pattern string) bool {
	// Handle ** and * (match everything)
	if pattern == "**" || pattern == "*" {
		return true
	}

	// Handle patterns like "*test*" (contains)
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		middle := pattern[1 : len(pattern)-1]
		return strings.Contains(strings.ToLower(name), strings.ToLower(middle))
	}

	// Handle patterns like "*-unit" (suffix match)
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix))
	}

	// Handle patterns like "test-*" (prefix match)
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))
	}

	// No wildcards - exact match
	return strings.EqualFold(name, pattern)
}

// printUnitResults classifies each unit's result, prints its error text (if
// any) and stop message with the parallel prefix, then drains the printer
// and prints the run summary.
func printUnitResults(
	ctx context.Context,
	results []TargetResult,
	firstErrIdx int,
	printer *Printer,
	maxNameLen int,
	out io.Writer,
) {
	for i := range results {
		isFirst := i == firstErrIdx
		results[i].Status = ClassifyResult(results[i].Err, isFirst)

		tctx := WithExecInfo(ctx, ExecInfo{
			Parallel:   true,
			Name:       results[i].Name,
			MaxNameLen: maxNameLen,
			Printer:    printer,
			Output:     out,
		})

		// Print error text with prefix before the stop message
		if results[i].Err != nil && !errors.Is(results[i].Err, context.Canceled) {
			Printf(tctx, "Error: %v\n", results[i].Err)
		}

		Printf(tctx, "%s (%s)\n", results[i].Status, results[i].Duration.Round(time.Millisecond))
	}

	// Drain printer and print summary
	printer.Close()

	summary := FormatSummary(results)
	if summary != "" {
		_, _ = fmt.Fprintln(out, "\n"+summary)
	}
}

// runWithEnvInternal contains the actual execution logic.
//
//nolint:cyclop // Sequential flow of distinct steps; splitting would obscure logic
func runWithEnvInternal(exec *runExecutor, env RunEnv, opts RunOptions, targets ...any) error {
	err := exec.setupContext()
	if err != nil {
		return err
	}

	if exec.cancelFunc != nil {
		defer exec.cancelFunc()
	}

	exec.extractHelpFlag()

	err = exec.extractOverrides()
	if err != nil {
		env.Printf("Error: %v\n", err)
		return ExitError{Code: 1}
	}

	err = exec.parseTargets(targets)
	if err != nil {
		return err
	}

	if len(exec.roots) == 0 {
		env.Println("No commands found.")
		return nil
	}

	exec.hasDefault = len(exec.roots) == 1 && opts.AllowDefault
	exec.opts.HasDefault = exec.hasDefault

	if len(exec.args) < minArgsWithCommand {
		return exec.handleNoArgs()
	}

	exec.rest = exec.args[1:]

	handled, err := exec.handleSpecialCommands()
	if handled || err != nil {
		return err
	}

	// A default root is addressable by name like any other root, so hand the
	// one dispatch path an arg list that already names it.
	if exec.hasDefault && !exec.opts.Overrides.Parallel &&
		!strings.EqualFold(exec.rest[0], exec.roots[0].Name) {
		exec.rest = append([]string{exec.roots[0].Name}, exec.rest...)
	}

	return exec.executeRoots()
}
