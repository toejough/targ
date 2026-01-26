//nolint:maintidx // Test functions with many subtests have low maintainability index by design
package targ_test

import (
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ"
)

// CustomSetter implements the Set(string) error interface for testing.
type CustomSetter struct {
	value string
}

// Set implements the flag.Value-like interface.
func (c *CustomSetter) Set(s string) error {
	c.value = "set:" + s
	return nil
}

// CustomText implements encoding.TextUnmarshaler for testing.
type CustomText struct {
	text string
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (c *CustomText) UnmarshalText(data []byte) error {
	c.text = strings.ToUpper(string(data))
	return nil
}

// APIServer is a named function with an acronym for testing camelToKebab.
func APIServer() {}

// HTTPHandler is another named function with an acronym.
func HTTPHandler() {}

// TestProperty_EnvVarBehavior tests environment variable fallback behavior.
// This cannot be parallel because t.Setenv modifies process environment.
//
//nolint:tparallel // Cannot use t.Parallel with t.Setenv - it modifies process environment
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

	t.Run("TextUnmarshalerType", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Value CustomText `targ:"flag"`
		}

		var got CustomText

		target := targ.Targ(func(a Args) { got = a.Value })

		_, err := targ.Execute([]string{"app", "--value", "hello"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got.text).To(Equal("HELLO")) // UnmarshalText uppercases
	})

	t.Run("StringSetterType", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Value CustomSetter `targ:"flag"`
		}

		var got CustomSetter

		target := targ.Targ(func(a Args) { got = a.Value })

		_, err := targ.Execute([]string{"app", "--value", "world"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got.value).To(Equal("set:world")) // Set prefixes with "set:"
	})
}

// TestProperty_HelpOutput tests help output formatting.
func TestProperty_HelpOutput(t *testing.T) {
	t.Parallel()

	t.Run("PositionalPlaceholderShowsInHelp", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			File string `targ:"positional,placeholder=FILENAME"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("FILENAME"))
	})

	t.Run("PositionalNameShowsWhenNoPlaceholder", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Source string `targ:"positional"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		result, err := targ.Execute([]string{"app", "--help"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		// Positional name appears in usage line
		g.Expect(result.Output).To(ContainSubstring("Source"))
	})
}

// TestProperty_NameDerivation tests automatic name derivation from function names.
func TestProperty_NameDerivation(t *testing.T) {
	t.Parallel()

	t.Run("AcronymFunctionDerivesCorrectKebabName", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// APIServer should become "api-server"
		target := targ.Targ(APIServer)
		other := targ.Targ(func() {}).Name("other")

		result, err := targ.Execute([]string{"app", "--help"}, target, other)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("api-server"))
	})

	t.Run("MultipleAcronymsFunctionDerivesCorrectKebabName", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// HTTPHandler should become "http-handler"
		target := targ.Targ(HTTPHandler)
		other := targ.Targ(func() {}).Name("other")

		result, err := targ.Execute([]string{"app", "--help"}, target, other)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("http-handler"))
	})
}

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

	t.Run("ShortFlagEqualsSyntaxWorks", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Name string `targ:"flag,short=n"`
		}

		var got string

		target := targ.Targ(func(a Args) { got = a.Name })

		// Use equals syntax with short flag: -n=value
		_, err := targ.Execute([]string{"app", "-n=hello"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal("hello"))
	})

	t.Run("SliceFlagStopsAtAnotherFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Files   []string `targ:"flag"`
			Verbose bool     `targ:"flag,short=v"`
		}

		var (
			files   []string
			verbose bool
		)

		target := targ.Targ(func(a Args) {
			files = a.Files
			verbose = a.Verbose
		})

		// Slice flag should stop when it encounters another flag
		result, err := targ.Execute([]string{"app", "--files", "a", "b", "-v"}, target)
		g.Expect(err).NotTo(HaveOccurred(), "output: %s", result.Output)
		g.Expect(files).To(Equal([]string{"a", "b"}))
		g.Expect(verbose).To(BeTrue())
	})

	t.Run("SliceFlagWithNoValuesReturnsError", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Files   []string `targ:"flag"`
			Verbose bool     `targ:"flag,short=v"`
		}

		target := targ.Targ(func(_ Args) {})

		// Slice flag immediately followed by another flag (no values)
		result, err := targ.Execute([]string{"app", "--files", "-v"}, target)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result.Output).To(ContainSubstring("files"))
	})

	t.Run("LongFlagWithSingleDashErrors", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Using -verbose instead of --verbose should error
		_, err := targ.Execute([]string{"app", "-verbose"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("LongFlagWithSingleDashEqualsErrors", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Output string `targ:"flag"`
		}

		target := targ.Targ(func(_ Args) {}).Name("cmd")

		// Using -output=file instead of --output=file should error
		_, err := targ.Execute([]string{"app", "-output=file"}, target)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("ShortFlagsAreAllowed", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Verbose bool `targ:"flag,short=v"`
		}

		var got bool

		target := targ.Targ(func(a Args) { got = a.Verbose }).Name("cmd")

		// -v is a valid short flag
		_, err := targ.Execute([]string{"app", "-v"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(BeTrue())
	})

	t.Run("FieldWithoutTargTagDefaultsToFlag", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		type Args struct {
			Name string // No targ tag - should default to --name flag
		}

		var got string

		target := targ.Targ(func(a Args) { got = a.Name })

		// Field without targ tag becomes a flag with lowercase name
		_, err := targ.Execute([]string{"app", "--name", "test"}, target)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(got).To(Equal("test"))
	})
}
