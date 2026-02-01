// TEST-021: Help content data structures - validates example and content types
// traces: ARCH-007

package help_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/help"
)

func TestProperty_ExampleCanBeCreated(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		e := help.Example{
			Title: rapid.String().Draw(t, "title"),
			Code:  rapid.String().Draw(t, "code"),
		}
		_ = e.Title
	})
}

func TestProperty_FlagCanBeCreated(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		f := help.Flag{
			Long:        rapid.String().Draw(t, "long"),
			Short:       rapid.String().Draw(t, "short"),
			Desc:        rapid.String().Draw(t, "desc"),
			Placeholder: rapid.String().Draw(t, "placeholder"),
			Required:    rapid.Bool().Draw(t, "required"),
		}
		_ = f.Long
	})
}

func TestProperty_FormatCanBeCreated(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		f := help.Format{
			Name: rapid.String().Draw(t, "name"),
			Desc: rapid.String().Draw(t, "desc"),
		}
		_ = f.Name
	})
}

func TestProperty_PositionalCanBeCreated(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		p := help.Positional{
			Name:        rapid.String().Draw(t, "name"),
			Placeholder: rapid.String().Draw(t, "placeholder"),
			Required:    rapid.Bool().Draw(t, "required"),
		}
		// If we get here without panic, the struct is valid
		_ = p.Name
	})
}

func TestProperty_SubcommandCanBeCreated(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		s := help.Subcommand{
			Name: rapid.String().Draw(t, "name"),
			Desc: rapid.String().Draw(t, "desc"),
		}
		_ = s.Name
	})
}
