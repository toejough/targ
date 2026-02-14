// TEST-030: Command internal properties - validates completion examples
// traces: ARCH-010

package core

import (
	"path/filepath"
	"reflect"
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

// TestProperty_StructFieldNameToKebabCase validates that multi-word struct field
// names are converted to kebab-case for flag names (GH-6).
func TestProperty_StructFieldNameToKebabCase(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		// Test multi-word field names
		type testStruct struct {
			MinRetrievals     int
			HTTPPort          int
			BaselinePattern   string
			CoverageThreshold float64
		}

		inst := testStruct{}
		val := reflect.ValueOf(inst)
		typ := val.Type()

		// MinRetrievals → min-retrievals
		field, _ := typ.FieldByName("MinRetrievals")
		opts, err := tagOptionsForField(val, field)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(opts.Name).To(Equal("min-retrievals"))

		// HTTPPort → http-port (handles acronyms)
		field, _ = typ.FieldByName("HTTPPort")
		opts, err = tagOptionsForField(val, field)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(opts.Name).To(Equal("http-port"))

		// BaselinePattern → baseline-pattern
		field, _ = typ.FieldByName("BaselinePattern")
		opts, err = tagOptionsForField(val, field)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(opts.Name).To(Equal("baseline-pattern"))

		// CoverageThreshold → coverage-threshold
		field, _ = typ.FieldByName("CoverageThreshold")
		opts, err = tagOptionsForField(val, field)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(opts.Name).To(Equal("coverage-threshold"))
	})
}
