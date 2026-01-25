package targ_test

import (
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

type TextValue struct {
	Value string
}

func (tv *TextValue) UnmarshalText(text []byte) error {
	tv.Value = strings.ToUpper(string(text))
	return nil
}

// --- Tests ---

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
