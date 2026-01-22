package targ_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestTarg_AcceptsFunction(t *testing.T) {
	rapid.Check(t, func(_ *rapid.T) {
		g := NewWithT(t)

		// Create a no-op function
		fn := func() {}
		target := targ.Targ(fn)

		g.Expect(target).NotTo(BeNil())
		g.Expect(target.Fn()).NotTo(BeNil())
	})
}

func TestTarg_AcceptsString(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate a non-empty command string
		cmd := rapid.StringMatching(`[a-z]+ [a-z]+`).Draw(rt, "cmd")
		target := targ.Targ(cmd)

		g.Expect(target).NotTo(BeNil())
		g.Expect(target.Fn()).To(Equal(cmd))
	})
}

func TestTarg_PanicsOnEmptyString(t *testing.T) {
	g := NewWithT(t)

	g.Expect(func() {
		targ.Targ("")
	}).To(Panic())
}

func TestTarg_PanicsOnNil(t *testing.T) {
	g := NewWithT(t)

	g.Expect(func() {
		targ.Targ(nil)
	}).To(Panic())
}

func TestTarg_PanicsOnNonFuncNonString(t *testing.T) {
	g := NewWithT(t)

	g.Expect(func() {
		targ.Targ(42) // int is not func or string
	}).To(Panic())
}

func TestTarget_BuilderChainWithDepsAndTimeout(t *testing.T) {
	g := NewWithT(t)

	order := make([]string, 0)
	dep := targ.Targ(func() { order = append(order, "dep") })
	main := targ.Targ(func() { order = append(order, "main") }).
		Name("test").
		Description("test target").
		Deps(dep).
		Timeout(time.Second)

	err := main.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(order).To(Equal([]string{"dep", "main"}))
}

func TestTarget_BuilderChainsPreserveSettings(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.StringMatching(`[a-z]+`).Draw(rt, "name")
		desc := rapid.StringMatching(`[a-zA-Z ]+`).Draw(rt, "desc")

		target := targ.Targ(func() {}).Name(name).Description(desc)

		g.Expect(target.GetName()).To(Equal(name))
		g.Expect(target.GetDescription()).To(Equal(desc))
	})
}

func TestTarget_BuilderMethodsReturnSameTarget(t *testing.T) {
	g := NewWithT(t)

	original := targ.Targ(func() {})
	afterName := original.Name("test")
	afterDesc := afterName.Description("test desc")

	// All should be the same pointer
	g.Expect(afterName).To(BeIdenticalTo(original))
	g.Expect(afterDesc).To(BeIdenticalTo(original))
}

func TestTarget_DepsRunSerially(t *testing.T) {
	g := NewWithT(t)

	order := make([]string, 0)
	a := targ.Targ(func() { order = append(order, "a") })
	b := targ.Targ(func() { order = append(order, "b") })
	c := targ.Targ(func() { order = append(order, "c") }).Deps(a, b)

	err := c.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(order).To(Equal([]string{"a", "b", "c"}))
}

func TestTarget_DepsStopOnError(t *testing.T) {
	g := NewWithT(t)

	order := make([]string, 0)
	a := targ.Targ(func() { order = append(order, "a") })
	b := targ.Targ(func() error {
		order = append(order, "b")
		return errors.New("b failed")
	})
	c := targ.Targ(func() { order = append(order, "c") }).Deps(a, b)

	err := c.Run(context.Background())
	g.Expect(err).To(MatchError(ContainSubstring("b failed")))
	g.Expect(order).To(Equal([]string{"a", "b"}))
}

func TestTarget_ParallelDepsRunConcurrently(t *testing.T) {
	g := NewWithT(t)

	// Use channels to verify concurrent execution
	aStarted := make(chan struct{})
	bStarted := make(chan struct{})
	done := make(chan struct{})

	a := targ.Targ(func() {
		close(aStarted)
		<-done
	})
	b := targ.Targ(func() {
		close(bStarted)
		<-done
	})
	c := targ.Targ(func() {}).ParallelDeps(a, b)

	go func() {
		_ = c.Run(context.Background())
	}()

	// Both should start before either completes
	<-aStarted
	<-bStarted
	close(done)

	// Give the main goroutine time to complete
	g.Eventually(func() bool { return true }).Should(BeTrue())
}

func TestTarget_RunCallsFunction(t *testing.T) {
	g := NewWithT(t)

	called := false
	target := targ.Targ(func() {
		called = true
	})

	err := target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(called).To(BeTrue())
}

func TestTarget_RunPassesContext(t *testing.T) {
	g := NewWithT(t)

	var receivedValue any

	target := targ.Targ(func(ctx context.Context) {
		receivedValue = ctx.Value(testContextKey)
	})

	ctx := context.WithValue(context.Background(), testContextKey, "value")
	err := target.Run(ctx)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(receivedValue).To(Equal("value"))
}

func TestTarget_RunReturnsError(t *testing.T) {
	g := NewWithT(t)

	expectedErr := errors.New("test error")
	target := targ.Targ(func() error {
		return expectedErr
	})

	err := target.Run(context.Background())
	g.Expect(err).To(MatchError(expectedErr))
}

func TestTarget_RunWithArgs(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Value string
	}

	var received Args

	target := targ.Targ(func(args Args) {
		received = args
	})

	err := target.Run(context.Background(), Args{Value: "test"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(received.Value).To(Equal("test"))
}

func TestTarget_RunWithContextAndArgs(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Value string
	}

	var receivedCtxValue any

	var receivedArgs Args

	target := targ.Targ(func(ctx context.Context, args Args) error {
		receivedCtxValue = ctx.Value(testContextKey)
		receivedArgs = args

		return nil
	})

	ctx := context.WithValue(context.Background(), testContextKey, "value")
	err := target.Run(ctx, Args{Value: "test"})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(receivedCtxValue).To(Equal("value"))
	g.Expect(receivedArgs.Value).To(Equal("test"))
}

func TestTarget_ShellCommandExecution(t *testing.T) {
	g := NewWithT(t)

	// Simple shell command that should succeed
	target := targ.Targ("echo hello")
	err := target.Run(context.Background())
	g.Expect(err).ToNot(HaveOccurred())
}

func TestTarget_ShellCommandFails(t *testing.T) {
	g := NewWithT(t)

	// Command that should fail
	target := targ.Targ("exit 1")
	err := target.Run(context.Background())
	g.Expect(err).To(HaveOccurred())
}

func TestTarget_TimeoutCancelsExecution(t *testing.T) {
	g := NewWithT(t)

	target := targ.Targ(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}).Timeout(10 * time.Millisecond)

	err := target.Run(context.Background())
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
}

// unexported constants.
const (
	testContextKey contextKey = "key"
)

// contextKey is a typed key for context values in tests.
type contextKey string
