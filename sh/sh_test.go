package sh

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestEnableCleanup(t *testing.T) {
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

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	sep := 0

	for i, arg := range args {
		if arg == "--" {
			sep = i
			break
		}
	}

	if sep == 0 || sep+1 >= len(args) {
		os.Exit(2)
	}

	cmd := args[sep+1]
	switch cmd {
	case "echo":
		if sep+2 < len(args) {
			_, _ = os.Stdout.WriteString(args[sep+2])
		}

		os.Exit(0)
	case "combined":
		_, _ = os.Stdout.WriteString("stdout\n")
		_, _ = os.Stderr.WriteString("stderr\n")

		os.Exit(0)
	case "fail":
		_, _ = os.Stderr.WriteString("fail\n")

		os.Exit(1)
	case "sleep":
		// Sleep for a long time (used for cancellation tests)
		time.Sleep(10 * time.Second)
		os.Exit(0)
	default:
		os.Exit(0)
	}
}

func TestIsWindows(t *testing.T) {
	// Just verify it returns a bool and doesn't panic
	_ = IsWindows()
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

	if output != "stdout\nstderr\n" {
		t.Fatalf("unexpected output: %q", output)
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
