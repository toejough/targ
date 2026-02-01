// TEST-030: Command internal properties - validates completion examples
// traces: ARCH-010

package core

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

func TestProperty_CompletionExampleWithGetenv(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		shell := rapid.StringMatching(`[a-z]+`).Draw(t, "shell")
		if shell == bashShell {
			shell = zshShell
		}

		fullPath := filepath.Join("/bin", shell)
		expect := "eval \"$(targ --completion)\""

		switch shell {
		case zshShell:
			expect = "source <(targ --completion)"
		case fishShell:
			expect = "targ --completion | source"
		}

		getenv := func(key string) string {
			if key == "SHELL" {
				return fullPath
			}

			return ""
		}

		example := completionExampleWithGetenv(getenv)
		g.Expect(example.Code).To(Equal(expect))

		emptyGetenv := func(_ string) string { return "" }
		g.Expect(detectCurrentShell(emptyGetenv)).To(Equal(bashShell))
	})
}
