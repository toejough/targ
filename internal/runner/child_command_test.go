package runner_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/runner"
)

func TestChildCommand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	cmd := runner.ChildCommand(context.Background(), "/cache/bin/targ_abc", "targ", "build", "-v")

	g.Expect(cmd.Path).To(Equal("/cache/bin/targ_abc"))
	g.Expect(cmd.Args).To(Equal([]string{"targ", "build", "-v"}))
	g.Expect(cmd.Env).To(BeNil())
}
