//go:build !windows

package internal_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/onsi/gomega"

	internal "github.com/toejough/targ/internal/sh"
)

func TestProperty_ForegroundProcessGroup(t *testing.T) {
	t.Parallel()

	t.Run("DefaultShellEnvSetsForegroundTrue", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		env := internal.DefaultShellEnv()

		g.Expect(env.Foreground).To(gomega.BeTrue())
	})

	t.Run("ForegroundCommandInheritsParentProcessGroup", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		parentPGID, err := syscall.Getpgid(os.Getpid())
		g.Expect(err).ToNot(gomega.HaveOccurred())

		env := internal.DefaultShellEnv()

		var buf internal.SafeBuffer

		env.Stdout = &buf

		ctx := context.Background()
		err = internal.RunContextWithIO(ctx, env, "sh", []string{"-c", "ps -o pgid= -p $$"}, nil)
		g.Expect(err).ToNot(gomega.HaveOccurred())

		childPGID, err := strconv.Atoi(strings.TrimSpace(buf.String()))
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(childPGID).To(gomega.Equal(parentPGID),
			fmt.Sprintf("foreground child PGID %d should equal parent PGID %d", childPGID, parentPGID))
	})

	t.Run("BackgroundCommandGetsOwnProcessGroup", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		parentPGID, err := syscall.Getpgid(os.Getpid())
		g.Expect(err).ToNot(gomega.HaveOccurred())

		env := internal.DefaultShellEnv()
		env.Foreground = false

		var buf internal.SafeBuffer

		env.Stdout = &buf

		ctx := context.Background()
		err = internal.RunContextWithIO(ctx, env, "sh", []string{"-c", "ps -o pgid= -p $$"}, nil)
		g.Expect(err).ToNot(gomega.HaveOccurred())

		childPGID, err := strconv.Atoi(strings.TrimSpace(buf.String()))
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(childPGID).ToNot(gomega.Equal(parentPGID),
			fmt.Sprintf("background child PGID %d should differ from parent PGID %d", childPGID, parentPGID))
	})
}

func TestProperty_SubprocessEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("EnvvOverridesAreVisibleToTheChild", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		envv := append(os.Environ(), "TARG_SH_OVERRIDE_PROBE=overridden")

		out, err := internal.OutputContext(
			t.Context(), "sh", []string{"-c", "printf %s \"$TARG_SH_OVERRIDE_PROBE\""}, nil, envv,
		)

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal("overridden"))
	})

	t.Run("EnvvDoesNotStripTheRestOfTheEnvironment", func(t *testing.T) {
		t.Parallel()
		g := gomega.NewWithT(t)

		envv := append(os.Environ(), "TARG_SH_UNRELATED_PROBE=set")

		out, err := internal.OutputContext(
			t.Context(), "sh", []string{"-c", "printf %s \"$PATH\""}, nil, envv,
		)

		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(out).To(gomega.Equal(os.Getenv("PATH")))
	})
}

// TestSubprocessEnvironmentInheritsParent stands alone rather than joining the
// property above, because two rules collide: t.Setenv panics when the test or
// any parent is parallel, while tparallel requires a test with parallel subtests
// to be parallel itself. A serial top-level test with no subtests satisfies both.
func TestSubprocessEnvironmentInheritsParent(t *testing.T) {
	g := gomega.NewWithT(t)

	t.Setenv("TARG_SH_INHERIT_PROBE", "inherited")

	out, err := internal.OutputContext(
		t.Context(), "sh", []string{"-c", "printf %s \"$TARG_SH_INHERIT_PROBE\""}, nil, nil,
	)

	g.Expect(err).ToNot(gomega.HaveOccurred())
	g.Expect(out).To(gomega.Equal("inherited"))
}
