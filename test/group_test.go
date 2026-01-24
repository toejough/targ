package targ_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

func TestGroup_AcceptsMixedMembers(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func() {})
	inner := targ.Group("inner", targ.Targ(func() {}))
	outer := targ.Group("outer", target, inner)

	g.Expect(outer.GetMembers()).To(HaveLen(2))
}

func TestGroup_AcceptsNestedGroups(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func() {})
	inner := targ.Group("inner", target)
	outer := targ.Group("outer", inner)

	g.Expect(outer.GetMembers()).To(HaveLen(1))
	g.Expect(outer.GetMembers()[0]).To(Equal(inner))
}

func TestGroup_AcceptsTargetMembers(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target1 := targ.Targ(func() {})
	target2 := targ.Targ(func() {})

	group := targ.Group("test", target1, target2)

	g.Expect(group.GetMembers()).To(HaveLen(2))
	g.Expect(group.GetMembers()[0]).To(Equal(target1))
	g.Expect(group.GetMembers()[1]).To(Equal(target2))
}

func TestGroup_AcceptsValidName(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate a valid group name (lowercase, starts with letter)
		name := rapid.StringMatching(`[a-z][a-z0-9-]{0,10}`).Draw(rt, "name")
		group := targ.Group(name)

		g.Expect(group).NotTo(BeNil())
		g.Expect(group.GetName()).To(Equal(name))
	})
}

func TestGroup_DeepNesting(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		depth := rapid.IntRange(1, 10).Draw(rt, "depth")

		// Build nested groups
		var current any = targ.Targ(func() {})

		for i := range depth {
			name := fmt.Sprintf("g%d", i)
			current = targ.Group(name, current)
		}

		// Verify we can traverse to the bottom
		group, ok := current.(*targ.TargetGroup)
		g.Expect(ok).To(BeTrue(), "expected *targ.TargetGroup")

		for range depth - 1 {
			members := group.GetMembers()
			g.Expect(members).To(HaveLen(1))

			group, ok = members[0].(*targ.TargetGroup)
			g.Expect(ok).To(BeTrue(), "expected *targ.TargetGroup")
		}

		// Final level should have the target
		members := group.GetMembers()
		g.Expect(members).To(HaveLen(1))
		_, isTarget := members[0].(*targ.Target)
		g.Expect(isTarget).To(BeTrue())
	})
}

func TestGroup_PanicsOnEmptyName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	g.Expect(func() {
		targ.Group("")
	}).To(Panic())
}

func TestGroup_PanicsOnInvalidMemberType(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	g.Expect(func() {
		targ.Group("test", "not a target")
	}).To(Panic())

	g.Expect(func() {
		targ.Group("test", 42)
	}).To(Panic())

	g.Expect(func() {
		targ.Group("test", func() {}) // raw func, not *Target
	}).To(Panic())
}

func TestGroup_PanicsOnInvalidName(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate invalid names (uppercase, special chars, starts with number)
		invalidNames := []string{
			rapid.StringMatching(`[A-Z][a-z]*`).Draw(rt, "uppercase"),
			rapid.StringMatching(`[0-9][a-z]*`).Draw(rt, "starts-with-number"),
			rapid.StringMatching(`[a-z]*[!@#$%]+`).Draw(rt, "special-chars"),
		}

		for _, name := range invalidNames {
			if name == "" {
				continue // skip empty, tested separately
			}

			g.Expect(func() {
				targ.Group(name)
			}).To(Panic(), fmt.Sprintf("name %q should be invalid", name))
		}
	})
}
