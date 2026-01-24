package targ_test

import (
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

type DefaultPositional struct {
	Pos string `targ:"positional,default=default_value"`
}

type InterleavedFlagsPositionals struct {
	Name  string `targ:"positional"`
	Count int    `targ:"flag,short=c"`
}

type PositionalArgs struct {
	Src string `targ:"positional"`
	Dst string `targ:"positional"`
}

type RequiredPositional struct {
	Src string `targ:"positional,required"`
	Dst string `targ:"positional"`
}

func TestDefaultPositional(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var gotPos string

	target := targ.Targ(func(args DefaultPositional) {
		gotPos = args.Pos
	})

	_, err := targ.Execute([]string{"app"}, target)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotPos).To(Equal("default_value"))
}

func TestDefaultPositional_OverrideWithValue(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		value := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "value")

		var gotPos string

		target := targ.Targ(func(args DefaultPositional) {
			gotPos = args.Pos
		})

		_, err := targ.Execute([]string{"app", value}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotPos).To(Equal(value))
	})
}

// --- Embedded Struct Flag Sharing ---

func TestEmbeddedFlags_SharedAcrossTargets(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		type CommonFlags struct {
			Verbose bool `targ:"flag,short=v"`
		}

		type ChildArgs struct {
			CommonFlags

			Name string `targ:"flag"`
		}

		name := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "name")
		verbose := rapid.Bool().Draw(rt, "verbose")

		var (
			gotVerbose bool
			gotName    string
		)

		child := targ.Targ(func(args ChildArgs) {
			gotVerbose = args.Verbose
			gotName = args.Name
		}).Name("child")

		group := targ.Group("parent", child)

		args := []string{"app", "child", "--name", name}
		if verbose {
			args = append(args, "--verbose")
		}

		_, err := targ.Execute(args, group)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotVerbose).To(Equal(verbose))
		g.Expect(gotName).To(Equal(name))
	})
}

func TestGroup_CustomNameRouting(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate two distinct names
		name1 := rapid.StringMatching(`[a-z]{3,6}`).Draw(rt, "name1")
		name2 := rapid.StringMatching(`[a-z]{7,10}`).Draw(rt, "name2")
		targetName := rapid.SampledFrom([]string{name1, name2}).Draw(rt, "target")

		var called string

		sub := targ.Targ(func() { called = name1 }).Name(name1)
		custom := targ.Targ(func() { called = name2 }).Name(name2)
		group := targ.Group("parent", sub, custom)

		_, err := targ.Execute([]string{"app", targetName}, group)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(called).To(Equal(targetName))
	})
}

// --- Group Routing ---

func TestGroup_SubcommandRouting(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "name")

		var called bool

		sub := targ.Targ(func() { called = true }).Name(name)
		group := targ.Group("parent", sub)

		_, err := targ.Execute([]string{"app", name}, group)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(called).To(BeTrue())
	})
}

func TestInterleavedFlagsAndPositionals(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.StringMatching(`[A-Z][a-z]{2,8}`).Draw(rt, "name")
		count := rapid.IntRange(0, 100).Draw(rt, "count")

		var (
			gotName  string
			gotCount int
		)

		target := targ.Targ(func(args InterleavedFlagsPositionals) {
			gotName = args.Name
			gotCount = args.Count
		})

		_, err := targ.Execute([]string{"app", name, "--count", strconv.Itoa(count)}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotName).To(Equal(name))
		g.Expect(gotCount).To(Equal(count))
	})
}

func TestPositionalArgs(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		src := rapid.StringMatching(`[a-z]+\.txt`).Draw(rt, "src")
		dst := rapid.StringMatching(`[a-z]+\.txt`).Draw(rt, "dst")

		var gotSrc, gotDst string

		target := targ.Targ(func(args PositionalArgs) {
			gotSrc = args.Src
			gotDst = args.Dst
		})

		_, err := targ.Execute([]string{"app", src, dst}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotSrc).To(Equal(src))
		g.Expect(gotDst).To(Equal(dst))
	})
}

func TestRequiredPositional(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func(_ RequiredPositional) {})

	_, err := targ.Execute([]string{"app"}, target)
	g.Expect(err).To(HaveOccurred())
}

func TestRequiredPositional_Provided(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		src := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "src")

		var gotSrc string

		target := targ.Targ(func(args RequiredPositional) {
			gotSrc = args.Src
		})

		_, err := targ.Execute([]string{"app", src}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotSrc).To(Equal(src))
	})
}
