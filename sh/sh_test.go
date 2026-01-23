package sh_test

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

	internal "github.com/toejough/targ/internal/sh"
	"github.com/toejough/targ/sh"
)

func TestEnableCleanup(_ *testing.T) {
	// EnableCleanup should be idempotent - calling multiple times is safe
	sh.EnableCleanup()
	sh.EnableCleanup() // Second call should not panic or cause issues
}

func TestExeSuffix(t *testing.T) {
	suffix := sh.ExeSuffix()
	if sh.IsWindows() {
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
	// Save and restore
	orig := internal.IsWindows

	defer func() { internal.IsWindows = orig }()

	// Inject Windows behavior
	internal.IsWindows = func() bool { return true }

	suffix := sh.ExeSuffix()
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
	_ = sh.IsWindows()
}

func TestKillAllProcesses(t *testing.T) {
	// Save and restore state
	internal.CleanupMu.Lock()

	origEnabled := internal.CleanupEnabled
	origProcs := internal.RunningProcs
	origKillFunc := internal.KillProcessFunc
	internal.CleanupEnabled = true
	internal.RunningProcs = make(map[*os.Process]struct{})

	internal.CleanupMu.Unlock()

	defer func() {
		internal.CleanupMu.Lock()

		internal.CleanupEnabled = origEnabled
		internal.RunningProcs = origProcs
		internal.KillProcessFunc = origKillFunc

		internal.CleanupMu.Unlock()
	}()

	// Track which processes were killed
	var killedProcs []*os.Process

	internal.KillProcessFunc = func(p *os.Process) {
		killedProcs = append(killedProcs, p)
	}

	// Create fake process entries
	proc1 := &os.Process{Pid: 1}
	proc2 := &os.Process{Pid: 2}

	// Register the processes via internal API
	internal.RegisterProcess(proc1)
	internal.RegisterProcess(proc2)

	// Kill all processes
	internal.KillAllProcesses()

	// Verify both processes were killed
	if len(killedProcs) != 2 {
		t.Errorf("expected 2 processes killed, got %d", len(killedProcs))
	}
}

func TestKillProcessFunc_Integration(t *testing.T) {
	// Start a real subprocess with its own process group
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err := cmd.Start()
	if err != nil {
		t.Fatalf("failed to start subprocess: %v", err)
	}

	// Save and get the actual kill function
	internal.CleanupMu.Lock()

	killFunc := internal.KillProcessFunc

	internal.CleanupMu.Unlock()

	// Call the real killProcessFunc to kill the process group
	killFunc(cmd.Process)

	// Clean up - wait for the process to exit (it should be killed)
	_ = cmd.Wait()
}

func TestOutputContext_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)

	go func() {
		_, err := sh.OutputContext(ctx, "sleep", "10")
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

	output, err := sh.OutputContext(ctx, "echo", "hello")
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

	output, err := sh.Output("combined")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for expected content (coverage mode may add extra output)
	if !strings.Contains(output, "stdout\n") || !strings.Contains(output, "stderr\n") {
		t.Fatalf("output missing expected content: %q", output)
	}
}

func TestQuoteArg_ViaRunV(t *testing.T) {
	// Test quoting behavior through RunV's verbose output
	restore := overrideExec(t)
	defer restore()

	var out bytes.Buffer

	internal.Stdout = &out
	internal.Stderr = &out

	// Test various quoting scenarios through the public API
	tests := []struct {
		args     []string
		contains string
	}{
		{[]string{"echo", "hello"}, "+ echo hello"},
		{[]string{"echo", "hello world"}, `+ echo "hello world"`},
		{[]string{"echo", ""}, `+ echo ""`},
	}

	for _, tc := range tests {
		out.Reset()

		err := sh.RunV(tc.args[0], tc.args[1:]...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(out.String(), tc.contains) {
			t.Errorf("expected output to contain %q, got %q", tc.contains, out.String())
		}
	}
}

func TestRunContextV_PrintsCommand(t *testing.T) {
	var out bytes.Buffer

	origStdout := internal.Stdout
	origStderr := internal.Stderr

	defer func() {
		internal.Stdout = origStdout
		internal.Stderr = origStderr
	}()

	internal.Stdout = &out
	internal.Stderr = &out

	ctx := context.Background()

	err := sh.RunContextV(ctx, "echo", "hello")
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
		errCh <- sh.RunContext(ctx, "sleep", "10")
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
	var out bytes.Buffer

	origStdout := internal.Stdout
	origStderr := internal.Stderr

	defer func() {
		internal.Stdout = origStdout
		internal.Stderr = origStderr
	}()

	internal.Stdout = &out
	internal.Stderr = &out

	ctx := context.Background()

	err := sh.RunContext(ctx, "echo", "hello")
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

	internal.Stdout = &out
	internal.Stderr = &out

	err := sh.RunV("echo", "hello")
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

	err := sh.Run("fail")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_Success(t *testing.T) {
	restore := overrideExec(t)
	defer restore()

	var out bytes.Buffer

	internal.Stdout = &out
	internal.Stderr = &out

	err := sh.Run("echo", "hello")
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

	if sh.IsWindows() {
		tests[0].expected = "myapp.exe"
		tests[2].expected = "path/to/myapp.exe"
	}

	for _, tc := range tests {
		result := sh.WithExeSuffix(tc.input)
		if result != tc.expected {
			t.Errorf("WithExeSuffix(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestWithExeSuffix_Windows(t *testing.T) {
	// Save and restore
	orig := internal.IsWindows

	defer func() { internal.IsWindows = orig }()

	// Inject Windows behavior
	internal.IsWindows = func() bool { return true }

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
		result := sh.WithExeSuffix(tc.input)
		if result != tc.expected {
			t.Errorf("WithExeSuffix(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

// Helper functions for subprocess testing

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

	prevExec := internal.ExecCommand
	prevStdout := internal.Stdout
	prevStderr := internal.Stderr
	prevStdin := internal.Stdin

	internal.ExecCommand = func(name string, args ...string) *exec.Cmd {
		cmdArgs := append([]string{"-test.run=TestHelperProcess", "--", name}, args...)
		cmd := exec.Command(os.Args[0], cmdArgs...)

		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

		return cmd
	}

	return func() {
		internal.ExecCommand = prevExec
		internal.Stdout = prevStdout
		internal.Stderr = prevStderr
		internal.Stdin = prevStdin
	}
}

func parseHelperArgs() (string, []string) {
	sep := findArgSeparator(os.Args)
	if sep == 0 || sep+1 >= len(os.Args) {
		return "", nil
	}

	return os.Args[sep+1], os.Args[sep+2:]
}

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
