package core_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestExecInfo(t *testing.T) {
	t.Parallel()

	t.Run("RoundTrip", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: true,
			Name:     "build",
		})

		info, ok := core.GetExecInfo(ctx)
		g.Expect(ok).To(BeTrue())
		g.Expect(info.Parallel).To(BeTrue())
		g.Expect(info.Name).To(Equal("build"))
	})

	t.Run("MissingReturnsFalse", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		_, ok := core.GetExecInfo(context.Background())
		g.Expect(ok).To(BeFalse())
	})

	t.Run("SerialMode", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		ctx := core.WithExecInfo(context.Background(), core.ExecInfo{
			Parallel: false,
			Name:     "test",
		})

		info, ok := core.GetExecInfo(ctx)
		g.Expect(ok).To(BeTrue())
		g.Expect(info.Parallel).To(BeFalse())
		g.Expect(info.Name).To(Equal("test"))
	})
}
