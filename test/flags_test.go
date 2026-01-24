package targ_test

import (
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// --- Args struct types for Target functions ---

type CustomFlagNameArgs struct {
	User string `targ:"name=user_name"`
}

type CustomTypeFlagsArgs struct {
	Name TextValue   `targ:"flag"`
	Nick SetterValue `targ:"flag"`
}

type DefaultEnvFlagsArgs struct {
	Name string `targ:"default=Alice,env=TEST_DEFAULT_NAME_FLAG"`
}

type DefaultFlagsArgs struct {
	Name    string `targ:"default=Alice"`
	Count   int    `targ:"default=42"`
	Enabled bool   `targ:"default=true"`
}

type EnvFlagArgs struct {
	User string `targ:"env=TEST_USER_FLAG"`
}

type RequiredFlagArgs struct {
	ID string `targ:"required"`
}

type RequiredShortFlagArgs struct {
	Name string `targ:"required,short=n"`
}

// --- Custom unmarshaler types (these stay - they're not commands) ---

type SetterValue struct {
	Value string
}

func (s *SetterValue) Set(value string) error {
	s.Value = value + "!"
	return nil
}

type ShortBoolFlagsArgs struct {
	Verbose bool `targ:"flag,short=v"`
	Force   bool `targ:"flag,short=f"`
}

type ShortFlagsArgs struct {
	Name string `targ:"flag,short=n"`
	Age  int    `targ:"flag,short=a"`
}

type ShortMixedFlagsArgs struct {
	Verbose bool   `targ:"flag,short=v"`
	Name    string `targ:"flag,short=n"`
}

// TagOptionsOverrideArgs tests static tag attributes
type TagOptionsOverrideArgs struct {
	Mode string `targ:"flag,name=stage,short=s,enum=dev|prod"`
}

type TextValue struct {
	Value string
}

func (tv *TextValue) UnmarshalText(text []byte) error {
	tv.Value = strings.ToUpper(string(text))
	return nil
}

// --- Tests ---

func TestCustomFlagName(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		user := rapid.StringMatching(`[A-Z][a-z]{2,10}`).Draw(rt, "user")

		var gotUser string

		target := targ.Targ(func(args CustomFlagNameArgs) {
			gotUser = args.User
		})

		_, err := targ.Execute([]string{"app", "--user_name", user}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotUser).To(Equal(user))
	})
}

func TestCustomTypes(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "name")
		nick := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "nick")

		var (
			gotName TextValue
			gotNick SetterValue
		)

		target := targ.Targ(func(args CustomTypeFlagsArgs) {
			gotName = args.Name
			gotNick = args.Nick
		})

		_, err := targ.Execute([]string{"app", "--name", name, "--nick", nick}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotName.Value).To(Equal(strings.ToUpper(name)))
		g.Expect(gotNick.Value).To(Equal(nick + "!"))
	})
}

func TestDefaultEnvFlags_EnvOverrides(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		envValue := rapid.StringMatching(`[A-Z][a-z]{2,10}`).Draw(rt, "envValue")
		t.Setenv("TEST_DEFAULT_NAME_FLAG", envValue)

		var gotName string

		target := targ.Targ(func(args DefaultEnvFlagsArgs) {
			gotName = args.Name
		})

		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotName).To(Equal(envValue))
	})
}

func TestDefaultFlags(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	var (
		gotName    string
		gotCount   int
		gotEnabled bool
	)

	target := targ.Targ(func(args DefaultFlagsArgs) {
		gotName = args.Name
		gotCount = args.Count
		gotEnabled = args.Enabled
	})

	_, err := targ.Execute([]string{"app"}, target)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(gotName).To(Equal("Alice"))
	g.Expect(gotCount).To(Equal(42))
	g.Expect(gotEnabled).To(BeTrue())
}

func TestDefaultFlags_OverrideWithValues(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.StringMatching(`[A-Z][a-z]{2,10}`).Draw(rt, "name")
		count := rapid.IntRange(0, 1000).Draw(rt, "count")
		enabled := rapid.Bool().Draw(rt, "enabled")

		var (
			gotName    string
			gotCount   int
			gotEnabled bool
		)

		target := targ.Targ(func(args DefaultFlagsArgs) {
			gotName = args.Name
			gotCount = args.Count
			gotEnabled = args.Enabled
		})

		args := []string{"app", "--name", name, "--count", strconv.Itoa(count)}
		if enabled {
			args = append(args, "--enabled")
		} else {
			args = append(args, "--enabled=false")
		}

		_, err := targ.Execute(args, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotName).To(Equal(name))
		g.Expect(gotCount).To(Equal(count))
		g.Expect(gotEnabled).To(Equal(enabled))
	})
}

func TestEnvFlag(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		envValue := rapid.StringMatching(`[A-Z][a-z]{2,10}`).Draw(rt, "envValue")
		t.Setenv("TEST_USER_FLAG", envValue)

		var gotUser string

		target := targ.Targ(func(args EnvFlagArgs) {
			gotUser = args.User
		})

		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotUser).To(Equal(envValue))
	})
}

func TestLongFlagsRequireDoubleDash(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.StringMatching(`[A-Z][a-z]{2,10}`).Draw(rt, "name")
		target := targ.Targ(func(_ ShortFlagsArgs) {})

		_, err := targ.Execute([]string{"app", "-name", name}, target)
		g.Expect(err).To(HaveOccurred())
	})
}

func TestRequiredFlag(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func(_ RequiredFlagArgs) {})

	_, err := targ.Execute([]string{"app"}, target)
	g.Expect(err).To(HaveOccurred())
}

func TestRequiredFlag_Provided(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		id := rapid.StringMatching(`[a-z0-9]{5,20}`).Draw(rt, "id")

		var gotID string

		target := targ.Targ(func(args RequiredFlagArgs) {
			gotID = args.ID
		})

		_, err := targ.Execute([]string{"app", "--id", id}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotID).To(Equal(id))
	})
}

func TestRequiredShortFlagErrorIncludesShort(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func(_ RequiredShortFlagArgs) {})

	result, err := targ.Execute([]string{"app"}, target)
	g.Expect(err).To(HaveOccurred())
	g.Expect(result.Output).To(ContainSubstring("--name"))
	g.Expect(result.Output).To(ContainSubstring("-n"))
}

func TestShortFlagGroups(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Randomly select which flags to set
		setVerbose := rapid.Bool().Draw(rt, "setVerbose")
		setForce := rapid.Bool().Draw(rt, "setForce")

		var gotVerbose, gotForce bool

		target := targ.Targ(func(args ShortBoolFlagsArgs) {
			gotVerbose = args.Verbose
			gotForce = args.Force
		})

		args := []string{"app"}
		if setVerbose && setForce {
			args = append(args, "-vf")
		} else if setVerbose {
			args = append(args, "-v")
		} else if setForce {
			args = append(args, "-f")
		}

		_, err := targ.Execute(args, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotVerbose).To(Equal(setVerbose))
		g.Expect(gotForce).To(Equal(setForce))
	})
}

func TestShortFlagGroupsRejectValueFlags(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	target := targ.Targ(func(_ ShortMixedFlagsArgs) {})

	_, err := targ.Execute([]string{"app", "-vn"}, target)
	g.Expect(err).To(HaveOccurred())
}

func TestShortFlags(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		name := rapid.StringMatching(`[A-Z][a-z]{2,10}`).Draw(rt, "name")
		age := rapid.IntRange(0, 120).Draw(rt, "age")

		var (
			gotName string
			gotAge  int
		)

		target := targ.Targ(func(args ShortFlagsArgs) {
			gotName = args.Name
			gotAge = args.Age
		})

		_, err := targ.Execute([]string{"app", "-n", name, "-a", strconv.Itoa(age)}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotName).To(Equal(name))
		g.Expect(gotAge).To(Equal(age))
	})
}

func TestTagOptionsOverride(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		mode := rapid.SampledFrom([]string{"dev", "prod"}).Draw(rt, "mode")

		var gotMode string

		target := targ.Targ(func(args TagOptionsOverrideArgs) {
			gotMode = args.Mode
		})

		_, err := targ.Execute([]string{"app", "--stage", mode}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotMode).To(Equal(mode))
	})
}

func TestTagOptionsOverride_ShortFlag(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		mode := rapid.SampledFrom([]string{"dev", "prod"}).Draw(rt, "mode")

		var gotMode string

		target := targ.Targ(func(args TagOptionsOverrideArgs) {
			gotMode = args.Mode
		})

		_, err := targ.Execute([]string{"app", "-s", mode}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotMode).To(Equal(mode))
	})
}
