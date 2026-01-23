package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// SafeBuffer is a thread-safe buffer for concurrent writes.
type SafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *SafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func (b *SafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n, err := b.buf.Write(p)
	if err != nil {
		return n, fmt.Errorf("writing to buffer: %w", err)
	}

	return n, nil
}

// ShellEnv provides shell execution environment for dependency injection.
type ShellEnv struct {
	ExecCommand func(string, ...string) *exec.Cmd
	IsWindows   func() bool
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	Cleanup     *CleanupManager
}

// DefaultShellEnv returns the standard OS implementations with the default cleanup manager.
func DefaultShellEnv() *ShellEnv {
	return &ShellEnv{
		ExecCommand: exec.Command,
		IsWindows:   func() bool { return runtime.GOOS == "windows" },
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Cleanup:     defaultCleanup,
	}
}

// EnableCleanup enables automatic cleanup of child processes on SIGINT/SIGTERM.
func EnableCleanup() {
	defaultCleanup.EnableCleanup()
}

// ExeSuffix returns ".exe" on Windows, otherwise an empty string.
func ExeSuffix(env *ShellEnv) string {
	if env == nil {
		env = DefaultShellEnv()
	}

	if env.IsWindows() {
		return ".exe"
	}

	return ""
}

// FormatCommand formats a command with proper quoting for display.
func FormatCommand(name string, args []string) string {
	parts := make([]string, 0, 1+len(args))

	parts = append(parts, quoteArg(name))
	for _, arg := range args {
		parts = append(parts, quoteArg(arg))
	}

	return strings.Join(parts, " ")
}

// IsWindowsOS reports whether the current OS is Windows.
func IsWindowsOS() bool {
	return runtime.GOOS == "windows"
}

// Output executes a command and returns combined output.
func Output(env *ShellEnv, name string, args ...string) (string, error) {
	if env == nil {
		env = DefaultShellEnv()
	}

	cmd := env.ExecCommand(name, args...)
	cmd.Stdin = env.Stdin
	SetProcGroup(cmd)

	var buf SafeBuffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("starting command: %w", err)
	}

	env.Cleanup.RegisterProcess(cmd.Process)
	err = cmd.Wait()
	env.Cleanup.UnregisterProcess(cmd.Process)

	if err != nil {
		return buf.String(), fmt.Errorf("waiting for command: %w", err)
	}

	return buf.String(), nil
}

// QuoteArg quotes an argument for display (exported for testing).
func QuoteArg(value string) string {
	if value == "" {
		return `""`
	}

	if strings.ContainsAny(value, " \t\n\"") {
		return strconv.Quote(value)
	}

	return value
}

// Run executes a command streaming stdout/stderr.
func Run(env *ShellEnv, name string, args ...string) error {
	if env == nil {
		env = DefaultShellEnv()
	}

	cmd := env.ExecCommand(name, args...)
	cmd.Stdout = env.Stdout
	cmd.Stderr = env.Stderr
	cmd.Stdin = env.Stdin
	SetProcGroup(cmd)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	env.Cleanup.RegisterProcess(cmd.Process)
	err = cmd.Wait()
	env.Cleanup.UnregisterProcess(cmd.Process)

	if err != nil {
		return fmt.Errorf("waiting for command: %w", err)
	}

	return nil
}

// RunV executes a command and prints it first.
func RunV(env *ShellEnv, name string, args ...string) error {
	if env == nil {
		env = DefaultShellEnv()
	}

	_, _ = fmt.Fprintln(env.Stdout, "+", FormatCommand(name, args))

	return Run(env, name, args...)
}

// WithExeSuffix appends the OS-specific executable suffix if missing.
func WithExeSuffix(env *ShellEnv, name string) string {
	if env == nil {
		env = DefaultShellEnv()
	}

	if !env.IsWindows() {
		return name
	}

	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name
	}

	return name + ".exe"
}

// unexported variables.
var (
	//nolint:gochecknoglobals // singleton pattern for shared cleanup state
	defaultCleanup = NewCleanupManager(PlatformKillProcess)
)

func quoteArg(value string) string {
	return QuoteArg(value)
}
