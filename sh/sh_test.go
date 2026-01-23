package sh

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestEnableCleanup(_ *testing.T) {
	// EnableCleanup should be idempotent - calling multiple times is safe
	EnableCleanup()
	EnableCleanup() // Second call should not panic or cause issues
}

func TestExeSuffix(t *testing.T) {
	suffix := ExeSuffix()
	if IsWindows() {
		if suffix != ".exe" {
			t.Errorf("expected .exe on Windows, got %q", suffix)
		}
	} else {
		if suffix != "" {
			t.Errorf("expected empty string on non-Windows, got %q", suffix)
		}
	}
}

func TestExeSuffix_Windows(t *testing.T) {
	// Mock Windows behavior
	origIsWindows := isWindows
	isWindows = func() bool { return true }

	defer func() { isWindows = origIsWindows }()

	suffix := ExeSuffix()
	if suffix != ".exe" {
		t.Errorf("ExeSuffix() = %q, want .exe on Windows", suffix)
	}
}

func TestHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	cmd, cmdArgs := parseHelperArgs()
	if cmd == "" {
		os.Exit(2)
	}

	runHelperCommand(cmd, cmdArgs)
}

func TestIsWindows(_ *testing.T) {
	// Just verify it returns a bool and doesn't panic
	_ = IsWindows()
}

func TestKillAllProcesses(t *testing.T) {
	// Save and restore state
	cleanupMu.Lock()

	origEnabled := cleanupEnabled
	origProcs := runningProcs
	origKillFunc := killProcessFunc
	cleanupEnabled = true
	runningProcs = make(map[*os.Process]struct{})

	cleanupMu.Unlock()

	defer func() {
		cleanupMu.Lock()

		cleanupEnabled = origEnabled
		runningProcs = origProcs
		killProcessFunc = origKillFunc

		cleanupMu.Unlock()
	}()

	// Track which processes were killed
	var killedProcs []*os.Process

	killProcessFunc = func(p *os.Process) {
		killedProcs = append(killedProcs, p)
	}

	// Create fake process entries (we don't need real processes, just pointers)
	proc1 := &os.Process{Pid: 1}
	proc2 := &os.Process{Pid: 2}

	// Register the processes
	registerProcess(proc1)
	registerProcess(proc2)

	// Kill all processes
	killAllProcesses()

	// Verify both processes were killed
	if len(killedProcs) != 2 {
		t.Errorf("expected 2 processes killed, got %d", len(killedProcs))
	}
}

func TestKillProcessFunc_Integration(t *testing.T) {
	// Start a real subprocess with its own process group
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start subprocess: %v", err)
	}

	// Call the real killProcessFunc to kill the process group
	killProcessFunc(cmd.Process)

	// Clean up - wait for the process to exit (it should be killed)
	_ = cmd.Wait()
}

func TestOutputContext_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)

	go func() {
		_, err := OutputContext(ctx, "sleep", "10")
		errCh <- err
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel should kill immediately
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Error("command did not terminate after cancel")
	}
}

func TestOutputContext_Success(t *testing.T) {
	ctx := context.Background()

	output, err := OutputContext(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "hello") {
		t.Fatalf("expected output to contain hello, got %q", output)
	}
}

func TestOutput_ReturnsCombinedOutput(t *testing.T) {
	restore := overrideExec(t)
	defer restore()

	output, err := Output("combined")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for expected content (coverage mode may add extra output)
	if !strings.Contains(output, "stdout\n") || !strings.Contains(output, "stderr\n") {
		t.Fatalf("output missing expected content: %q", output)
	}
}

func TestQuoteArg_EmptyString(t *testing.T) {
	result := quoteArg("")
	if result != `""` {
		t.Errorf("quoteArg(\"\") = %q, want %q", result, `""`)
	}
}

func TestQuoteArg_SimpleString(t *testing.T) {
	result := quoteArg("hello")
	if result != "hello" {
		t.Errorf("quoteArg(\"hello\") = %q, want \"hello\"", result)
	}
}

func TestQuoteArg_StringWithNewline(t *testing.T) {
	result := quoteArg("hello\nworld")
	if !strings.HasPrefix(result, "\"") {
		t.Errorf("quoteArg with newline should be quoted, got %q", result)
	}
}

func TestQuoteArg_StringWithQuote(t *testing.T) {
	result := quoteArg(`hello"world`)
	if !strings.HasPrefix(result, "\"") {
		t.Errorf("quoteArg with quote should be quoted, got %q", result)
	}
}

func TestQuoteArg_StringWithSpace(t *testing.T) {
	result := quoteArg("hello world")
	if !strings.HasPrefix(result, "\"") {
		t.Errorf("quoteArg(\"hello world\") = %q, should be quoted", result)
	}
}

func TestQuoteArg_StringWithTab(t *testing.T) {
	result := quoteArg("hello\tworld")
	if !strings.HasPrefix(result, "\"") {
		t.Errorf("quoteArg with tab should be quoted, got %q", result)
	}
}

func TestRunContextV_PrintsCommand(t *testing.T) {
	restore := overrideStdio(t)
	defer restore()

	var out bytes.Buffer

	stdout = &out
	stderr = &out

	ctx := context.Background()

	err := RunContextV(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "+ echo hello") {
		t.Fatalf("expected verbose prefix, got %q", out.String())
	}
}

func TestRunContext_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)

	go func() {
		errCh <- RunContext(ctx, "sleep", "10")
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel should kill immediately
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Error("command did not terminate after cancel")
	}
}

func TestRunContext_Success(t *testing.T) {
	restore := overrideStdio(t)
	defer restore()

	var out bytes.Buffer

	stdout = &out
	stderr = &out

	ctx := context.Background()

	err := RunContext(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "hello") {
		t.Fatalf("expected output to contain hello, got %q", out.String())
	}
}

func TestRunV_PrintsCommand(t *testing.T) {
	restore := overrideExec(t)
	defer restore()

	var out bytes.Buffer

	stdout = &out
	stderr = &out

	err := RunV("echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "+ echo hello") {
		t.Fatalf("expected verbose prefix, got %q", out.String())
	}
}

func TestRun_ReturnsError(t *testing.T) {
	restore := overrideExec(t)
	defer restore()

	err := Run("fail")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_Success(t *testing.T) {
	restore := overrideExec(t)
	defer restore()

	var out bytes.Buffer

	stdout = &out
	stderr = &out

	err := Run("echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "hello") {
		t.Fatalf("expected output to contain hello, got %q", out.String())
	}
}

func TestWithExeSuffix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"myapp", "myapp"},
		{"myapp.exe", "myapp.exe"},
		{"path/to/myapp", "path/to/myapp"},
	}

	if IsWindows() {
		tests[0].expected = "myapp.exe"
		tests[2].expected = "path/to/myapp.exe"
	}

	for _, tc := range tests {
		result := WithExeSuffix(tc.input)
		if result != tc.expected {
			t.Errorf("WithExeSuffix(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestWithExeSuffix_Windows(t *testing.T) {
	// Mock Windows behavior
	origIsWindows := isWindows
	isWindows = func() bool { return true }

	defer func() { isWindows = origIsWindows }()

	tests := []struct {
		input    string
		expected string
	}{
		{"myapp", "myapp.exe"},
		{"myapp.exe", "myapp.exe"},
		{"path/to/myapp", "path/to/myapp.exe"},
		{"myapp.EXE", "myapp.EXE"}, // Case-insensitive suffix check
	}

	for _, tc := range tests {
		result := WithExeSuffix(tc.input)
		if result != tc.expected {
			t.Errorf("WithExeSuffix(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

// findArgSeparator finds the position of "--" separator in args.
func findArgSeparator(args []string) int {
	for i, arg := range args {
		if arg == "--" {
			return i
		}
	}

	return 0
}

func helperCombined() {
	_, _ = os.Stdout.WriteString("stdout\n")
	_, _ = os.Stderr.WriteString("stderr\n")

	os.Exit(0)
}

func helperEcho(args []string) {
	if len(args) > 0 {
		_, _ = os.Stdout.WriteString(args[0])
	}

	os.Exit(0)
}

func helperFail() {
	_, _ = os.Stderr.WriteString("fail\n")

	os.Exit(1)
}

func helperSleep() {
	time.Sleep(10 * time.Second)
	os.Exit(0)
}

func overrideExec(t *testing.T) func() {
	t.Helper()

	prevExec := execCommand
	prevStdout := stdout
	prevStderr := stderr
	prevStdin := stdin

	execCommand = func(name string, args ...string) *exec.Cmd {
		cmdArgs := append([]string{"-test.run=TestHelperProcess", "--", name}, args...)
		cmd := exec.Command(os.Args[0], cmdArgs...)

		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

		return cmd
	}

	return func() {
		execCommand = prevExec
		stdout = prevStdout
		stderr = prevStderr
		stdin = prevStdin
	}
}

func overrideStdio(t *testing.T) func() {
	t.Helper()

	prevStdout := stdout
	prevStderr := stderr
	prevStdin := stdin

	return func() {
		stdout = prevStdout
		stderr = prevStderr
		stdin = prevStdin
	}
}

// parseHelperArgs extracts the command and args from test helper invocation.
func parseHelperArgs() (string, []string) {
	sep := findArgSeparator(os.Args)
	if sep == 0 || sep+1 >= len(os.Args) {
		return "", nil
	}

	return os.Args[sep+1], os.Args[sep+2:]
}

// runHelperCommand executes the appropriate helper command.
func runHelperCommand(cmd string, args []string) {
	switch cmd {
	case "echo":
		helperEcho(args)
	case "combined":
		helperCombined()
	case "fail":
		helperFail()
	case "sleep":
		helperSleep()
	default:
		os.Exit(0)
	}
}
