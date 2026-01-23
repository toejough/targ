package targ

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestExecuteRegisteredWithOptions_Subprocess(t *testing.T) {
	// Subprocess test pattern for code that calls os.Exit
	if os.Getenv("TEST_EXECUTE_WITH_OPTS") == "1" {
		// In subprocess: register a target and execute
		Register(Targ(func() {}).Name("opts-target"))
		ExecuteRegisteredWithOptions(RunOptions{Description: "Test description"})

		return
	}

	if len(os.Args) == 0 {
		t.Fatal("os.Args is empty")
	}

	// Spawn subprocess with --help to verify it runs
	cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteRegisteredWithOptions_Subprocess$")

	cmd.Env = append(os.Environ(), "TEST_EXECUTE_WITH_OPTS=1")
	cmd.Args = append(cmd.Args, "--", "--help")
	output, err := cmd.CombinedOutput()
	// Should exit 0 with --help
	if err != nil {
		t.Fatalf("ExecuteRegisteredWithOptions failed: %v\nOutput: %s", err, output)
	}

	// Verify the registered target appears in help output
	if !strings.Contains(string(output), "opts-target") {
		t.Errorf("Expected help output to contain opts-target, got: %s", output)
	}
}

func TestExecuteRegistered_Subprocess(t *testing.T) {
	// Subprocess test pattern for code that calls os.Exit
	if os.Getenv("TEST_EXECUTE_BASIC") == "1" {
		Register(Targ(func() {}).Name("basic-target"))
		ExecuteRegistered()

		return
	}

	if len(os.Args) == 0 {
		t.Fatal("os.Args is empty")
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteRegistered_Subprocess$")

	cmd.Env = append(os.Environ(), "TEST_EXECUTE_BASIC=1")
	cmd.Args = append(cmd.Args, "--", "--help")
	output, err := cmd.CombinedOutput()
	// Should exit 0 with --help
	if err != nil {
		t.Fatalf("ExecuteRegistered failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(string(output), "basic-target") {
		t.Errorf("Expected help output to contain basic-target, got: %s", output)
	}
}

func TestRegister(t *testing.T) {
	// Save original registry
	orig := registry
	registry = nil

	defer func() { registry = orig }()

	// Register some targets
	target1 := Targ(func() {})
	target2 := Targ(func() {})
	Register(target1, target2)

	if len(registry) != 2 {
		t.Fatalf("expected 2 targets in registry, got %d", len(registry))
	}
}

func TestRegister_Append(t *testing.T) {
	// Save original registry
	orig := registry
	registry = nil

	defer func() { registry = orig }()

	// Register in two calls
	target1 := Targ(func() {})
	target2 := Targ(func() {})

	Register(target1)
	Register(target2)

	if len(registry) != 2 {
		t.Fatalf("expected 2 targets in registry, got %d", len(registry))
	}
}
