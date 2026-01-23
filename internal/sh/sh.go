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

// Exported variables.
var (
	ExecCommand = exec.Command
	IsWindows   = func() bool { return runtime.GOOS == "windows" }
	Stderr      = io.Writer(os.Stderr)
	Stdin       = io.Reader(os.Stdin)
	Stdout      = io.Writer(os.Stdout)
)

// SafeBuffer is a thread-safe buffer for concurrent writes.
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

// ExeSuffix returns ".exe" on Windows, otherwise an empty string.
func ExeSuffix() string {
	if IsWindows() {
		return ".exe"
	}

	return ""
}

// FormatCommand formats a command with proper quoting for display
func FormatCommand(name string, args []string) string {
	parts := make([]string, 0, 1+len(args))

	parts = append(parts, quoteArg(name))
	for _, arg := range args {
		parts = append(parts, quoteArg(arg))
	}

	return strings.Join(parts, " ")
}

// Output executes a command and returns combined output.
func Output(name string, args ...string) (string, error) {
	cmd := ExecCommand(name, args...)
	cmd.Stdin = Stdin
	SetProcGroup(cmd)

	var buf SafeBuffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("starting command: %w", err)
	}

	RegisterProcess(cmd.Process)
	err = cmd.Wait()
	UnregisterProcess(cmd.Process)

	if err != nil {
		return buf.String(), fmt.Errorf("waiting for command: %w", err)
	}

	return buf.String(), nil
}

// QuoteArg quotes an argument for display (exported for testing)
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
func Run(name string, args ...string) error {
	cmd := ExecCommand(name, args...)
	cmd.Stdout = Stdout
	cmd.Stderr = Stderr
	cmd.Stdin = Stdin
	SetProcGroup(cmd)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	RegisterProcess(cmd.Process)
	err = cmd.Wait()
	UnregisterProcess(cmd.Process)

	if err != nil {
		return fmt.Errorf("waiting for command: %w", err)
	}

	return nil
}

// RunV executes a command and prints it first.
func RunV(name string, args ...string) error {
	_, _ = fmt.Fprintln(Stdout, "+", FormatCommand(name, args))
	return Run(name, args...)
}

// WithExeSuffix appends the OS-specific executable suffix if missing.
func WithExeSuffix(name string) string {
	if !IsWindows() {
		return name
	}

	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name
	}

	return name + ".exe"
}

func quoteArg(value string) string {
	return QuoteArg(value)
}
