package targ_test

import (
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

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

			target := targ.Targ(func(a Args) { got = a.Enabled })

			_, err := targ.Execute(
				[]string{"app", "--enabled=" + strconv.FormatBool(value)},
				target,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(value))
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

			target := targ.Targ(func(a Args) { got = a.Count })

			_, err := targ.Execute([]string{"app", "--count", strconv.Itoa(value)}, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(value))
		})
	})

	t.Run("PositionalFieldsCapture", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			value := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "value")

			type Args struct {
				File string `targ:"positional"`
			}

			var got string

			target := targ.Targ(func(a Args) { got = a.File })

			_, err := targ.Execute([]string{"app", value}, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(value))
		})
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

			target := targ.Targ(func(a Args) { got = a.Name })

			_, err := targ.Execute([]string{"app", "-n", value}, target)
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

			type Common struct {
				Verbose bool `targ:"flag,short=v"`
			}

			type Args struct {
				Common

				Env string `targ:"flag"`
			}

			var got Args

			target := targ.Targ(func(a Args) { got = a })

			args := []string{"app", "--env", envValue}
			if verboseValue {
				args = append(args, "--verbose")
			}

			_, err := targ.Execute(args, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got.Verbose).To(Equal(verboseValue))
			g.Expect(got.Env).To(Equal(envValue))
		})
	})

	t.Run("MapFlagsParseKeyValue", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			key := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "key")
			val := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "val")

			type Args struct {
				Labels map[string]string `targ:"flag"`
			}

			var got map[string]string

			target := targ.Targ(func(a Args) { got = a.Labels })

			_, err := targ.Execute([]string{"app", "--labels", key + "=" + val}, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(HaveKeyWithValue(key, val))
		})
	})

	t.Run("SliceFlagsAccumulate", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			values := rapid.SliceOfN(rapid.StringMatching(`[a-z]{3,10}`), 1, 5).Draw(t, "values")

			type Args struct {
				Tags []string `targ:"flag"`
			}

			var got []string

			target := targ.Targ(func(a Args) { got = a.Tags })

			args := make([]string, 1, 1+2*len(values))

			args[0] = "app"
			for _, v := range values {
				args = append(args, "--tags", v)
			}

			_, err := targ.Execute(args, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(values))
		})
	})

	t.Run("PositionalDefault", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Package string `targ:"positional,default=./..."`
		}

		var got string

		target := targ.Targ(func(a Args) { got = a.Package })

		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal("./..."))
	})

	t.Run("RequiredFlagMissing", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Name string `targ:"flag,required"`
		}

		target := targ.Targ(func(_ Args) {})

		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("GroupedBoolFlags", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool `targ:"flag,short=v"`
			Force   bool `targ:"flag,short=f"`
			Debug   bool `targ:"flag,short=d"`
		}

		var got Args

		target := targ.Targ(func(a Args) { got = a })

		_, err := targ.Execute([]string{"app", "-vfd"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got.Verbose).To(BeTrue())
		g.Expect(got.Force).To(BeTrue())
		g.Expect(got.Debug).To(BeTrue())
	})

	t.Run("GroupedNonBoolRejects", func(t *testing.T) {
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

	t.Run("SliceInvalidValue", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Counts []int `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {})

		_, err := targ.Execute([]string{"app", "--counts", "not-a-number"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("InterleavedFlagsPreservePosition", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			flagVal := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "flagVal")
			posVal := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "posVal")

			type Args struct {
				Name string `targ:"flag"`
				File string `targ:"positional"`
			}

			var got Args

			target := targ.Targ(func(a Args) { got = a })

			// Interleave: flag, positional, in mixed order
			_, err := targ.Execute([]string{"app", "--name", flagVal, posVal}, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got.Name).To(Equal(flagVal))
			g.Expect(got.File).To(Equal(posVal))

			// Also test: positional first, then flag
			var got2 Args
			target2 := targ.Targ(func(a Args) { got2 = a })
			_, err2 := targ.Execute([]string{"app", posVal, "--name", flagVal}, target2)
			g.Expect(err2).NotTo(HaveOccurred())
			g.Expect(got2.Name).To(Equal(flagVal))
			g.Expect(got2.File).To(Equal(posVal))
		})
	})

	t.Run("DefaultFlagValue", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Port int `targ:"flag,default=8080"`
		}

		var got int

		target := targ.Targ(func(a Args) { got = a.Port })

		// Execute without providing the flag - should use default
		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal(8080))
	})

	t.Run("EqualsSyntaxWorks", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			g := NewWithT(t)
			value := rapid.StringMatching(`[a-z0-9]{3,10}`).Draw(t, "value")

			type Args struct {
				Name string `targ:"flag"`
			}

			var got string

			target := targ.Targ(func(a Args) { got = a.Name })

			// Use equals syntax: --flag=value
			_, err := targ.Execute([]string{"app", "--name=" + value}, target)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(value))
		})
	})
}

// TestProperty_EnvVarBehavior tests environment variable fallback behavior.
// This cannot be parallel because t.Setenv modifies process environment.
func TestProperty_EnvVarBehavior(t *testing.T) {
	t.Run("EnvVarFallback", func(t *testing.T) {
		g := NewWithT(t)

		type Args struct {
			Host string `targ:"flag,env=TEST_HOST"`
		}

		var got string

		target := targ.Targ(func(a Args) { got = a.Host })

		t.Setenv("TEST_HOST", "localhost")

		_, err := targ.Execute([]string{"app"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal("localhost"))
	})

	t.Run("EnvVarOverriddenByFlag", func(t *testing.T) {
		g := NewWithT(t)

		type Args struct {
			Host string `targ:"flag,env=TEST_HOST2"`
		}

		var got string

		target := targ.Targ(func(a Args) { got = a.Host })

		t.Setenv("TEST_HOST2", "from-env")

		_, err := targ.Execute([]string{"app", "--host", "from-flag"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal("from-flag"))
	})
}
