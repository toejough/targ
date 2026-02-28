// TEST-032: Create code generation
package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/runner"
)

func TestCreateCodegenWithRegister(t *testing.T) {
	t.Parallel()

	t.Run("GeneratesInitWithRegister", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Create temp dir with valid package name
		tmpDir := t.TempDir()
		testDir := filepath.Join(tmpDir, "testpkg")
		err := os.Mkdir(testDir, 0o755)
		g.Expect(err).ToNot(HaveOccurred())

		// Create targ file
		opts := runner.CreateOptions{
			Name:     "test",
			ShellCmd: "echo hello",
		}

		targFile, err := runner.FindOrCreateTargFile(testDir)
		g.Expect(err).ToNot(HaveOccurred())

		err = runner.AddTargetToFileWithOptions(targFile, opts)
		g.Expect(err).ToNot(HaveOccurred())

		// Read generated file
		content, err := os.ReadFile(targFile)
		g.Expect(err).ToNot(HaveOccurred())

		fileContent := string(content)
		t.Logf("Generated file content:\n%s", fileContent)

		// Should contain init() with targ.Register()
		g.Expect(fileContent).To(ContainSubstring("func init() {"))
		g.Expect(fileContent).To(ContainSubstring("targ.Register("))

		// Check if it has the var pattern OR inline pattern
		hasVarPattern := strings.Contains(fileContent, `var Test = targ.Targ("echo hello")`)
		hasInlinePattern := strings.Contains(fileContent, `targ.Register(targ.Targ("echo hello")`)

		g.Expect(hasVarPattern || hasInlinePattern).To(BeTrue(),
			"should generate either var+register or inline register pattern")

		// According to ISSUE-021, it should use inline pattern in init()
		// Let's verify the expected pattern is present
		g.Expect(fileContent).
			To(ContainSubstring(`targ.Register(targ.Targ("echo hello").Name("test"))`),
				"should generate inline targ.Register(targ.Targ(...).Name(...)) pattern in init()")
	})
}
