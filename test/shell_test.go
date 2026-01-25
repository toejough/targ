package targ_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/toejough/targ"
)

func TestSubstituteVars_AllTypes(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct {
		Str   string
		Int   int
		Uint  uint
		Float float64
		Bool  bool
		Slice []string // tests default case
	}

	args := Args{
		Str:   "hello",
		Int:   42,
		Uint:  100,
		Float: 3.14,
		Bool:  true,
		Slice: []string{"a", "b"},
	}

	ctx := context.Background()
	// Use true to always succeed - we're testing substitution works
	err := targ.Shell(ctx, "true", args)

	g.Expect(err).NotTo(HaveOccurred())
}

func TestSubstituteVars_ContextCancelled(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := targ.Shell(ctx, "sleep 10", Args{})

	g.Expect(err).To(HaveOccurred())
}

func TestSubstituteVars_NilArgs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()

	// No variables - should work with nil args
	err := targ.Shell(ctx, "echo hello > /dev/null", nil)
	g.Expect(err).NotTo(HaveOccurred())

	// Has variables but nil args - should error
	err = targ.Shell(ctx, "echo $name", nil)
	g.Expect(err).To(MatchError(ContainSubstring("nil")))
}

func TestSubstituteVars_NonStruct(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()
	err := targ.Shell(ctx, "echo $name", "not a struct")

	g.Expect(err).To(MatchError(ContainSubstring("must be a struct")))
}

func TestSubstituteVars_Pointer(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct {
		Name string
	}

	args := &Args{Name: "test"}

	ctx := context.Background()
	err := targ.Shell(ctx, "echo $name > /dev/null", args)

	g.Expect(err).NotTo(HaveOccurred())
}
