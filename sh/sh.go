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

var (
	stdout      io.Writer = os.Stdout
	stderr      io.Writer = os.Stderr
	stdin       io.Reader = os.Stdin
	execCommand           = exec.Command
)

// Run executes a command streaming stdout/stderr.
func Run(name string, args ...string) error {
	cmd := execCommand(name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin
	return cmd.Run()
}

// RunV executes a command and prints it first.
func RunV(name string, args ...string) error {
	fmt.Fprintln(stdout, "+", formatCommand(name, args))
	return Run(name, args...)
}

// Output executes a command and returns combined output.
func Output(name string, args ...string) (string, error) {
	cmd := execCommand(name, args...)
	cmd.Stdin = stdin
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// IsWindows reports whether the current OS is Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// ExeSuffix returns ".exe" on Windows, otherwise an empty string.
func ExeSuffix() string {
	if IsWindows() {
		return ".exe"
	}
	return ""
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
