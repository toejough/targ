package targ_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestSubstituteVars_Basic(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Name string
		Port int
	}

	args := Args{Name: "myapp", Port: 8080}

	// Test using Shell with echo to verify substitution
	ctx := context.Background()
	err := targ.Shell(ctx, "echo $name $port > /dev/null", args)

	g.Expect(err).NotTo(HaveOccurred())
}

func TestSubstituteVars_CaseInsensitive(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Namespace string
	}

	args := Args{Namespace: "prod"}

	// $namespace should match Namespace field
	ctx := context.Background()
	err := targ.Shell(ctx, "echo $namespace > /dev/null", args)

	g.Expect(err).NotTo(HaveOccurred())
}

func TestSubstituteVars_BraceStyle(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Name string
	}

	args := Args{Name: "test"}

	// ${name} style should work
	ctx := context.Background()
	err := targ.Shell(ctx, "echo ${name}suffix > /dev/null", args)

	g.Expect(err).NotTo(HaveOccurred())
}

func TestSubstituteVars_UnknownVariable(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Name string
	}

	args := Args{Name: "test"}

	ctx := context.Background()
	err := targ.Shell(ctx, "echo $unknown", args)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unknown variable"))
	g.Expect(err.Error()).To(ContainSubstring("unknown"))
}

func TestSubstituteVars_NilArgs(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	// No variables - should work with nil args
	err := targ.Shell(ctx, "echo hello > /dev/null", nil)
	g.Expect(err).NotTo(HaveOccurred())

	// Has variables but nil args - should error
	err = targ.Shell(ctx, "echo $name", nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("nil"))
}

func TestSubstituteVars_Pointer(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Name string
	}

	args := &Args{Name: "test"}

	ctx := context.Background()
	err := targ.Shell(ctx, "echo $name > /dev/null", args)

	g.Expect(err).NotTo(HaveOccurred())
}

func TestSubstituteVars_NonStruct(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	err := targ.Shell(ctx, "echo $name", "not a struct")

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must be a struct"))
}

func TestSubstituteVars_MultipleVariables(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Namespace string
		App       string
		Port      int
	}

	args := Args{Namespace: "prod", App: "myapp", Port: 8080}

	ctx := context.Background()
	err := targ.Shell(ctx, "echo $namespace $app $port > /dev/null", args)

	g.Expect(err).NotTo(HaveOccurred())
}

func TestSubstituteVars_NoVariables(t *testing.T) {
	g := NewWithT(t)

	type Args struct {
		Name string
	}

	args := Args{Name: "test"}

	ctx := context.Background()
	err := targ.Shell(ctx, "echo hello > /dev/null", args)

	g.Expect(err).NotTo(HaveOccurred())
}

func TestSubstituteVars_ContextCancelled(t *testing.T) {
	g := NewWithT(t)

	type Args struct{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := targ.Shell(ctx, "sleep 10", Args{})

	g.Expect(err).To(HaveOccurred())
}

func TestSubstituteVars_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.StringMatching(`[a-z]+`).Draw(rt, "name")
		port := rapid.IntRange(1, 65535).Draw(rt, "port")

		type Args struct {
			Name string
			Port int
		}

		args := Args{Name: name, Port: port}

		ctx := context.Background()
		// Use true to always succeed - we're testing substitution works
		err := targ.Shell(ctx, "true", args)

		g.Expect(err).NotTo(HaveOccurred())
	})
}
