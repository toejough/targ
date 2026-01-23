package targ_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/toejough/targ"
	"github.com/toejough/targ/internal/core"
)

func TestExecuteRegisteredWithOptions_Subprocess(t *testing.T) {
	// Subprocess test pattern for code that calls os.Exit
	if os.Getenv("TEST_EXECUTE_WITH_OPTS") == "1" {
		// In subprocess: register a target and execute
		targ.Register(targ.Targ(func() {}).Name("opts-target"))
		targ.ExecuteRegisteredWithOptions(targ.RunOptions{Description: "Test description"})

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
		targ.Register(targ.Targ(func() {}).Name("basic-target"))
		targ.ExecuteRegistered()

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
	orig := core.GetRegistry()

	core.SetRegistry(nil)

	defer func() { core.SetRegistry(orig) }()

	// Register some targets
	target1 := targ.Targ(func() {})
	target2 := targ.Targ(func() {})
	targ.Register(target1, target2)

	reg := core.GetRegistry()
	if len(reg) != 2 {
		t.Fatalf("expected 2 targets in registry, got %d", len(reg))
	}
}

func TestRegister_Append(t *testing.T) {
	// Save original registry
	orig := core.GetRegistry()

	core.SetRegistry(nil)

	defer func() { core.SetRegistry(orig) }()

	// Register in two calls
	target1 := targ.Targ(func() {})
	target2 := targ.Targ(func() {})

	targ.Register(target1)
	targ.Register(target2)

	reg := core.GetRegistry()
	if len(reg) != 2 {
		t.Fatalf("expected 2 targets in registry, got %d", len(reg))
	}
}
