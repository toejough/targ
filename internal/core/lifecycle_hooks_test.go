package core_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ/internal/core"
)

func TestOnStartOnStop(t *testing.T) {
	t.Parallel()

	t.Run("OnStartSetsHook", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var called bool

		target := core.Targ(func() {}).OnStart(func(_ context.Context, _ string) {
			called = true
		})

		hook := target.GetOnStart()
		g.Expect(hook).ToNot(BeNil())
		hook(context.Background(), "test")
		g.Expect(called).To(BeTrue())
	})

	t.Run("OnStopSetsHook", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		var got core.Result

		target := core.Targ(func() {}).OnStop(func(_ context.Context, _ string, result core.Result, _ time.Duration) {
			got = result
		})

		hook := target.GetOnStop()
		g.Expect(hook).ToNot(BeNil())
		hook(context.Background(), "test", core.Pass, 0)
		g.Expect(got).To(Equal(core.Pass))
	})

	t.Run("DefaultsAreNil", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		target := core.Targ(func() {})
		g.Expect(target.GetOnStart()).To(BeNil())
		g.Expect(target.GetOnStop()).To(BeNil())
	})
}
