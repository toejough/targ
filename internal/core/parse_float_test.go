// TEST-031: Parse float64 flags
package core_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

func TestFloat64FlagParsing(t *testing.T) {
	t.Parallel()

	t.Run("ParsesFloat64Flag", func(t *testing.T) {
		t.Parallel()

		g := NewWithT(t)

		// Define struct with float64 field
		type Args struct {
			Threshold float64 `targ:"flag,name=threshold"`
		}

		var receivedArgs Args

		fn := func(args Args) {
			receivedArgs = args
		}

		// Create target
		target := targ.Targ(fn).Name("test")

		// Execute with args containing float value
		args := []string{"test", "--threshold", "3.14"}
		_, err := targ.Execute(args, target)

		// Should parse successfully
		g.Expect(err).ToNot(HaveOccurred(), "parsing float64 flag should not error")
		g.Expect(receivedArgs.Threshold).To(Equal(3.14), "float64 value should be parsed correctly")
	})
}
