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
	"sync"
	"syscall"
	"time"
)

// ExecuteEnv is a RunEnv implementation that captures output for testing.
type ExecuteEnv struct {
	args   []string
	output strings.Builder
}

// NewExecuteEnv returns a RunEnv that captures output for testing.
func NewExecuteEnv(args []string) *ExecuteEnv {
	return &ExecuteEnv{args: args}
}

// Args returns the command line arguments.
func (e *ExecuteEnv) Args() []string {
	return e.args
}

// Exit is a no-op for testing environments.
func (e *ExecuteEnv) Exit(_ int) {
	_ = 0 // No-op stub for coverage
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
	err := RunWithEnv(env, opts, targets...)

	return ExecuteResult{Output: env.Output()}, err
}

// RunWithEnv executes commands with a custom environment.
//
//nolint:cyclop // Entry point orchestrating setup, flag extraction, and execution paths
func RunWithEnv(env RunEnv, opts RunOptions, targets ...any) error {
	// Set stdout for help output if not already set
	opts.Stdout = env.Stdout()

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

	if len(exec.args) < minArgsWithCommand {
		return exec.handleNoArgs()
	}

	exec.rest = exec.args[1:]

	handled, err := exec.handleSpecialCommands()
	if handled || err != nil {
		return err
	}

	if exec.hasDefault {
		return exec.executeDefault()
	}

	return exec.executeMultiRoot()
}

// unexported constants.
const (
	globExpansionMultiplier = 10 // Extra capacity for glob expansions in parallel mode
	minArgsWithCommand      = 2
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

type listFunc func([]*commandNode) error

type listOutput struct {
	Commands []listCommandInfo `json:"commands"`
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

	return DetectShell()
}

// executeDefault executes commands against a single default root.
func (e *runExecutor) executeDefault() error {
	if len(e.rest) == 0 {
		_, err := e.roots[0].executeWithParents(e.ctx, nil, nil, map[string]bool{}, false, e.opts)
		if err != nil {
			e.env.Printf("Error: %v\n", err)
			return ExitError{Code: 1}
		}

		return nil
	}

	// If parallel mode, run targets concurrently
	if e.opts.Overrides.Parallel {
		return e.executeDefaultParallel()
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

// executeDefaultParallel runs targets in parallel for default (single root) mode.
func (e *runExecutor) executeDefaultParallel() error {
	// Create cancellable context for fail-fast behavior
	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()

	var wg sync.WaitGroup

	errCh := make(chan error, len(e.rest))

	for _, arg := range e.rest {
		// Skip flags
		if strings.HasPrefix(arg, "-") {
			continue
		}

		wg.Add(1)

		go func(cmdName string) {
			defer wg.Done()

			_, err := e.roots[0].executeWithParents(
				ctx,
				[]string{cmdName},
				nil,
				map[string]bool{},
				false,
				e.opts,
			)
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", cmdName, err)

				cancel() // Cancel siblings on first error
			}
		}(arg)
	}

	wg.Wait()
	close(errCh)

	// Return first error
	for err := range errCh {
		e.env.Printf("Error: %v\n", err)
		return ExitError{Code: 1}
	}

	return nil
}

// executeGlobPattern handles execution of glob-matched targets.
func (e *runExecutor) executeGlobPattern(name string) error {
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
			e.opts,
		)
		if err != nil {
			e.env.Printf("Error: %v\n", err)
			return ExitError{Code: 1}
		}
	}

	return nil
}

// executeMultiRoot executes commands against multiple roots.
func (e *runExecutor) executeMultiRoot() error {
	if e.opts.Overrides.Parallel {
		return e.executeMultiRootParallel()
	}

	remaining := e.rest

	for len(remaining) > 0 {
		if remaining[0] == "^" {
			remaining = remaining[1:]
			continue
		}

		name := remaining[0]

		if isGlobPattern(name) {
			err := e.executeGlobPattern(name)
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

// executeMultiRootParallel runs targets in parallel for multi-root mode.
func (e *runExecutor) executeMultiRootParallel() error {
	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()

	var wg sync.WaitGroup

	errCh := make(chan error, len(e.rest)*globExpansionMultiplier)

	for _, arg := range e.rest {
		if strings.HasPrefix(arg, "-") {
			continue
		}

		if isGlobPattern(arg) {
			err := e.launchGlobTargets(ctx, arg, &wg, errCh, cancel)
			if err != nil {
				return err
			}

			continue
		}

		matched := e.findMatchingRoot(arg)
		if matched == nil {
			e.env.Printf("Unknown command: %s\n", arg)
			printUsage(e.env.Stdout(), e.roots, e.opts)

			return ExitError{Code: 1}
		}

		e.launchTarget(ctx, matched, arg, &wg, errCh, cancel)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		e.env.Printf("Error: %v\n", err)
		return ExitError{Code: 1}
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
func (e *runExecutor) handleComplete() {
	if len(e.rest) > 1 {
		err := e.completeFn(os.Stdout, e.roots, e.rest[1])
		if err != nil {
			e.env.Println(err.Error())
		}
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

	// For multi-root mode: if arg matches a root command, let command handle help
	// This allows `targ <cmd> --help` to show command-specific help
	if !e.hasDefault && len(e.rest) > 0 && !strings.HasPrefix(e.rest[0], "-") {
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

// launchGlobTargets launches parallel execution for all targets matching a glob pattern.
func (e *runExecutor) launchGlobTargets(
	ctx context.Context,
	pattern string,
	wg *sync.WaitGroup,
	errCh chan<- error,
	cancel context.CancelFunc,
) error {
	matches := e.findMatchingRootsGlob(pattern)
	if len(matches) == 0 {
		e.env.Printf("No targets match pattern: %s\n", pattern)
		return ExitError{Code: 1}
	}

	for _, matched := range matches {
		e.launchTarget(ctx, matched, matched.Name, wg, errCh, cancel)
	}

	return nil
}

// launchTarget launches a single target in a goroutine for parallel execution.
func (e *runExecutor) launchTarget(
	ctx context.Context,
	node *commandNode,
	name string,
	wg *sync.WaitGroup,
	errCh chan<- error,
	cancel context.CancelFunc,
) {
	wg.Go(func() {
		_, err := node.executeWithParents(ctx, nil, nil, map[string]bool{}, true, e.opts)
		if err != nil {
			errCh <- fmt.Errorf("%s: %w", name, err)

			cancel()
		}
	})
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
