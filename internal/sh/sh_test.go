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
		err = internal.RunContextWithIO(ctx, env, "sh", []string{"-c", "ps -o pgid= -p $$"})
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
		err = internal.RunContextWithIO(ctx, env, "sh", []string{"-c", "ps -o pgid= -p $$"})
		g.Expect(err).ToNot(gomega.HaveOccurred())

		childPGID, err := strconv.Atoi(strings.TrimSpace(buf.String()))
		g.Expect(err).ToNot(gomega.HaveOccurred())
		g.Expect(childPGID).ToNot(gomega.Equal(parentPGID),
			fmt.Sprintf("background child PGID %d should differ from parent PGID %d", childPGID, parentPGID))
	})
}
