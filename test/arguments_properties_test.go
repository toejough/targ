package targ_test

import (
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

//nolint:funlen // subtest container
func TestProperty_StructTagParsing(t *testing.T) {
	t.Parallel()

	t.Run("BoolFlagsAcceptExplicitValues", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			value := rapid.Bool().Draw(t, "value")

			type Args struct {
				Enabled bool `targ:"flag"`
			}

			var got bool

			target := targ.Targ(func(args Args) { got = args.Enabled })

			_, err := targ.Execute(
				[]string{"app", "--enabled=" + strconv.FormatBool(value)},
				target,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(value))
		})
	})

	t.Run("EmbeddedStructsFlatten", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			verboseValue := rapid.Bool().Draw(t, "verbose")
			envValue := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "env")

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
	})

	t.Run("IntFlagsParseNumericValues", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			value := rapid.IntRange(0, 10000).Draw(t, "value")

			type Args struct {
				Count int `targ:"flag"`
			}

			var got int

			target := targ.Targ(func(args Args) { got = args.Count })

			_, err := targ.Execute([]string{"app", "--count", strconv.Itoa(value)}, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(value))
		})
	})

	t.Run("MapFieldsParseKeyValueSyntax", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			key := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "key")
			value := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "value")

			type Args struct {
				Labels map[string]string `targ:"flag"`
			}

			var got map[string]string

			target := targ.Targ(func(args Args) { got = args.Labels })

			_, err := targ.Execute([]string{"app", "--labels", key + "=" + value}, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(HaveKeyWithValue(key, value))
		})
	})

	t.Run("PositionalDefaultsApplied", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Package string `targ:"positional,default=./..."`
		}

		var got string

		target := targ.Targ(func(args Args) { got = args.Package })

		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal("./..."))
	})

	t.Run("PositionalFieldsCaptureOrderedArgs", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			value := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "value")

			type Args struct {
				File string `targ:"positional"`
			}

			var got string

			target := targ.Targ(func(args Args) { got = args.File })

			_, err := targ.Execute([]string{"app", value}, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(value))
		})
	})

	t.Run("RequiredFieldsErrorIfMissing", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Name string `targ:"flag,required"`
		}

		target := targ.Targ(func(_ Args) {})
		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("ShortFlagGroupsExpand", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool `targ:"flag,short=v"`
			Force   bool `targ:"flag,short=f"`
			Debug   bool `targ:"flag,short=d"`
		}

		var got Args

		target := targ.Targ(func(args Args) { got = args })

		_, err := targ.Execute([]string{"app", "-vfd"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got.Verbose).To(BeTrue())
		g.Expect(got.Force).To(BeTrue())
		g.Expect(got.Debug).To(BeTrue())
	})

	t.Run("ShortFlagGroupsRejectNonBool", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool `targ:"flag,short=v"`
			Count   int  `targ:"flag,short=c"`
			Force   bool `targ:"flag,short=f"`
		}

		target := targ.Targ(func(_ Args) {})
		_, err := targ.Execute([]string{"app", "-vcf"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("ShortFlagsWork", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			value := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "value")

			type Args struct {
				Name string `targ:"flag,short=n"`
			}

			var got string

			target := targ.Targ(func(args Args) { got = args.Name })

			_, err := targ.Execute([]string{"app", "-n", value}, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(value))
		})
	})

	t.Run("SliceFieldsAccumulateRepeatedFlags", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			values := rapid.SliceOfN(rapid.StringMatching(`[a-z]{3,10}`), 1, 5).Draw(t, "values")

			type Args struct {
				Tags []string `targ:"flag"`
			}

			var got []string

			target := targ.Targ(func(args Args) { got = args.Tags })

			cliArgs := make([]string, 0, 1+2*len(values))

			cliArgs = append(cliArgs, "app")
			for _, v := range values {
				cliArgs = append(cliArgs, "--tags", v)
			}

			_, err := targ.Execute(cliArgs, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(values))
		})
	})

	t.Run("SliceFlagInvalidValueReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Counts []int `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})
		_, err := targ.Execute([]string{"app", "--counts", "not-a-number"}, target)
		g.Expect(err).To(HaveOccurred())
	})
}
