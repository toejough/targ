package targ_test

import (
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// Property: Boolean flags accept explicit true/false values
func TestProperty_StructTagParsing_BoolFlagsAcceptExplicitValues(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		value := rapid.Bool().Draw(rt, "value")

		type Args struct {
			Enabled bool `targ:"flag"`
		}

		var got bool

		target := targ.Targ(func(args Args) {
			got = args.Enabled
		})

		_, err := targ.Execute([]string{"app", "--enabled=" + strconv.FormatBool(value)}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal(value))
	})
}

// Property: Embedded structs flatten their fields
func TestProperty_StructTagParsing_EmbeddedStructsFlatten(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		verboseValue := rapid.Bool().Draw(rt, "verbose")
		envValue := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "env")

		type CommonArgs struct {
			Verbose bool `targ:"flag,short=v"`
		}

		type DeployArgs struct {
			CommonArgs

			Env string `targ:"flag"`
		}

		var (
			gotVerbose bool
			gotEnv     string
		)

		target := targ.Targ(func(args DeployArgs) {
			gotVerbose = args.Verbose
			gotEnv = args.Env
		})

		cliArgs := []string{"app", "--env", envValue}
		if verboseValue {
			cliArgs = append(cliArgs, "--verbose")
		}

		_, err := targ.Execute(cliArgs, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(gotVerbose).To(Equal(verboseValue))
		g.Expect(gotEnv).To(Equal(envValue))
	})
}

// Property: Integer flags parse numeric values
func TestProperty_StructTagParsing_IntFlagsParseNumericValues(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		value := rapid.IntRange(0, 10000).Draw(rt, "value")

		type Args struct {
			Count int `targ:"flag"`
		}

		var got int

		target := targ.Targ(func(args Args) {
			got = args.Count
		})

		_, err := targ.Execute([]string{"app", "--count", strconv.Itoa(value)}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal(value))
	})
}

// Property: Map fields parse key=value syntax
func TestProperty_StructTagParsing_MapFieldsParseKeyValueSyntax(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		key := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "key")
		value := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "value")

		type Args struct {
			Labels map[string]string `targ:"flag"`
		}

		var got map[string]string

		target := targ.Targ(func(args Args) {
			got = args.Labels
		})

		_, err := targ.Execute([]string{"app", "--labels", key + "=" + value}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(HaveKeyWithValue(key, value))
	})
}

// Property: Positional args have default values applied
func TestProperty_StructTagParsing_PositionalDefaultsApplied(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct {
		Package string `targ:"positional,default=./..."`
	}

	var got string

	target := targ.Targ(func(args Args) {
		got = args.Package
	})

	// Without providing the positional, default should be applied
	_, err := targ.Execute([]string{"app"}, target)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal("./..."))
}

// Property: Positional fields capture ordered arguments
func TestProperty_StructTagParsing_PositionalFieldsCaptureOrderedArgs(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		value := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "value")

		type Args struct {
			File string `targ:"positional"`
		}

		var got string

		target := targ.Targ(func(args Args) {
			got = args.File
		})

		_, err := targ.Execute([]string{"app", value}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal(value))
	})
}

// Property: Required fields error if missing
func TestProperty_StructTagParsing_RequiredFieldsErrorIfMissing(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct {
		Name string `targ:"flag,required"`
	}

	target := targ.Targ(func(_ Args) {})

	_, err := targ.Execute([]string{"app"}, target)
	g.Expect(err).To(HaveOccurred())
}

// Property: Short flag groups expand to individual boolean flags
func TestProperty_StructTagParsing_ShortFlagGroupsExpand(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct {
		Verbose bool `targ:"flag,short=v"`
		Force   bool `targ:"flag,short=f"`
		Debug   bool `targ:"flag,short=d"`
	}

	var got Args

	target := targ.Targ(func(args Args) {
		got = args
	})

	// Grouped short flags like -vfd should expand to -v -f -d
	_, err := targ.Execute([]string{"app", "-vfd"}, target)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got.Verbose).To(BeTrue())
	g.Expect(got.Force).To(BeTrue())
	g.Expect(got.Debug).To(BeTrue())
}

// Property: Short flag groups with non-bool flag return error
func TestProperty_StructTagParsing_ShortFlagGroupsRejectNonBool(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	type Args struct {
		Verbose bool `targ:"flag,short=v"`
		Count   int  `targ:"flag,short=c"`
		Force   bool `targ:"flag,short=f"`
	}

	target := targ.Targ(func(_ Args) {})

	// Grouped flags with non-bool (count) should error
	_, err := targ.Execute([]string{"app", "-vcf"}, target)
	g.Expect(err).To(HaveOccurred())
}

// Property: Short flags work with single letter
func TestProperty_StructTagParsing_ShortFlagsWork(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		value := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "value")

		type Args struct {
			Name string `targ:"flag,short=n"`
		}

		var got string

		target := targ.Targ(func(args Args) {
			got = args.Name
		})

		_, err := targ.Execute([]string{"app", "-n", value}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal(value))
	})
}

// Property: Slice fields accumulate repeated flags
func TestProperty_StructTagParsing_SliceFieldsAccumulateRepeatedFlags(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		g := NewWithT(t)

		// Generate random flag values
		values := rapid.SliceOfN(
			rapid.StringMatching(`[a-z]{3,10}`),
			1, 5,
		).Draw(rt, "values")

		type Args struct {
			Tags []string `targ:"flag"`
		}

		var got []string

		target := targ.Targ(func(args Args) {
			got = args.Tags
		})

		// Build args: --tags v1 --tags v2 ...
		cliArgs := make([]string, 0, 1+2*len(values))

		cliArgs = append(cliArgs, "app")
		for _, v := range values {
			cliArgs = append(cliArgs, "--tags", v)
		}

		_, err := targ.Execute(cliArgs, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal(values))
	})
}
