package targ_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
	"github.com/toejough/targ/internal/core"
)

//nolint:paralleltest // Uses targ.Register which modifies global state
func TestExecuteRegisteredWithOptions_Subprocess(t *testing.T) {
	// Subprocess test pattern for code that calls os.Exit
	if os.Getenv("TEST_EXECUTE_WITH_OPTS") == "1" {
		// In subprocess: register a target and execute
		targ.Register(targ.Targ(func() {}).Name("opts-target"))
		targ.ExecuteRegisteredWithOptions(targ.RunOptions{Description: "Test description"})

		return
	}

	g := NewWithT(t)

	if len(os.Args) == 0 {
		t.Fatal("os.Args is empty")
	}

	// Spawn subprocess with --help to verify it runs
	cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteRegisteredWithOptions_Subprocess$")

	cmd.Env = append(os.Environ(), "TEST_EXECUTE_WITH_OPTS=1")
	cmd.Args = append(cmd.Args, "--", "--help")
	output, err := cmd.CombinedOutput()

	g.Expect(err).NotTo(HaveOccurred(), "ExecuteRegisteredWithOptions failed: %s", output)
	g.Expect(string(output)).To(ContainSubstring("opts-target"))
}

//nolint:paralleltest // Uses targ.Register which modifies global state
func TestExecuteRegistered_Subprocess(t *testing.T) {
	// Subprocess test pattern for code that calls os.Exit
	if os.Getenv("TEST_EXECUTE_BASIC") == "1" {
		targ.Register(targ.Targ(func() {}).Name("basic-target"))
		targ.ExecuteRegistered()

		return
	}

	g := NewWithT(t)

	if len(os.Args) == 0 {
		t.Fatal("os.Args is empty")
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteRegistered_Subprocess$")

	cmd.Env = append(os.Environ(), "TEST_EXECUTE_BASIC=1")
	cmd.Args = append(cmd.Args, "--", "--help")
	output, err := cmd.CombinedOutput()

	g.Expect(err).NotTo(HaveOccurred(), "ExecuteRegistered failed: %s", output)
	g.Expect(string(output)).To(ContainSubstring("basic-target"))
}

//nolint:paralleltest // Modifies global registry via core.SetRegistry
func TestRegister(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Save and restore registry
		orig := core.GetRegistry()

		core.SetRegistry(nil)

		defer func() { core.SetRegistry(orig) }()

		// Generate random number of targets (1-10)
		count := rapid.IntRange(1, 10).Draw(rt, "count")

		targets := make([]any, count)
		for i := range targets {
			targets[i] = targ.Targ(func() {})
		}

		targ.Register(targets...)

		reg := core.GetRegistry()
		g.Expect(reg).To(HaveLen(count))
	})
}

//nolint:paralleltest // Modifies global registry via core.SetRegistry
func TestRegister_Append(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Save and restore registry
		orig := core.GetRegistry()

		core.SetRegistry(nil)

		defer func() { core.SetRegistry(orig) }()

		// Generate random counts for two register calls
		count1 := rapid.IntRange(1, 5).Draw(rt, "count1")
		count2 := rapid.IntRange(1, 5).Draw(rt, "count2")

		targets1 := make([]any, count1)
		for i := range targets1 {
			targets1[i] = targ.Targ(func() {})
		}

		targets2 := make([]any, count2)
		for i := range targets2 {
			targets2[i] = targ.Targ(func() {})
		}

		targ.Register(targets1...)
		targ.Register(targets2...)

		reg := core.GetRegistry()
		g.Expect(reg).To(HaveLen(count1 + count2))
	})
}

// Subprocess test with random target name
//
//nolint:paralleltest // Uses targ.Register which modifies global state
func TestExecuteRegistered_RandomName_Subprocess(t *testing.T) {
	targetName := os.Getenv("TEST_TARGET_NAME")
	if targetName != "" {
		targ.Register(targ.Targ(func() {}).Name(targetName))
		targ.ExecuteRegistered()

		return
	}

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		if len(os.Args) == 0 {
			t.Fatal("os.Args is empty")
		}

		// Generate valid target name
		name := rapid.StringMatching(`[a-z][a-z0-9-]{2,10}`).Draw(rt, "name")

		cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteRegistered_RandomName_Subprocess$")

		cmd.Env = append(os.Environ(), "TEST_TARGET_NAME="+name)
		cmd.Args = append(cmd.Args, "--", "--help")
		output, err := cmd.CombinedOutput()

		g.Expect(err).NotTo(HaveOccurred(), "failed: %s", output)
		g.Expect(strings.ToLower(string(output))).To(ContainSubstring(name))
	})
}
