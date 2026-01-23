package sh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// ExeSuffix returns ".exe" on Windows, otherwise an empty string.
func ExeSuffix() string {
	if isWindows() {
		return ".exe"
	}

	return ""
}

// IsWindows reports whether the current OS is Windows.
func IsWindows() bool {
	return isWindows()
}

// Output executes a command and returns combined output.
func Output(name string, args ...string) (string, error) {
	cmd := execCommand(name, args...)
	cmd.Stdin = stdin
	setProcGroup(cmd)

	// Use Start/Wait instead of CombinedOutput so we can register the process
	var buf safeBuffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("starting command: %w", err)
	}

	registerProcess(cmd.Process)
	err = cmd.Wait()
	unregisterProcess(cmd.Process)

	if err != nil {
		return buf.String(), fmt.Errorf("waiting for command: %w", err)
	}

	return buf.String(), nil
}

// Run executes a command streaming stdout/stderr.
func Run(name string, args ...string) error {
	cmd := execCommand(name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin
	setProcGroup(cmd)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	registerProcess(cmd.Process)
	err = cmd.Wait()
	unregisterProcess(cmd.Process)

	if err != nil {
		return fmt.Errorf("waiting for command: %w", err)
	}

	return nil
}

// RunV executes a command and prints it first.
func RunV(name string, args ...string) error {
	_, _ = fmt.Fprintln(stdout, "+", formatCommand(name, args))
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

// unexported variables.
var (
	execCommand = exec.Command
	isWindows   = func() bool { return runtime.GOOS == "windows" }
	stderr      = io.Writer(os.Stderr)
	stdin       = io.Reader(os.Stdin)
	stdout      = io.Writer(os.Stdout)
)

func formatCommand(name string, args []string) string {
	parts := make([]string, 0, 1+len(args))

	parts = append(parts, quoteArg(name))
	for _, arg := range args {
		parts = append(parts, quoteArg(arg))
	}

	return strings.Join(parts, " ")
}

func quoteArg(value string) string {
	if value == "" {
		return `""`
	}

	if strings.ContainsAny(value, " \t\n\"") {
		return strconv.Quote(value)
	}

	return value
}
