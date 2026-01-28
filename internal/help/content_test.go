package help_test

import (
	"reflect"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"

	"github.com/toejough/targ/internal/help"
)

func TestContentBuilderExists(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// ContentBuilder should be exported and usable
	var _ help.ContentBuilder

	ct := reflect.TypeFor[help.ContentBuilder]()
	g.Expect(ct.NumField()).To(BeNumerically(">=", 9),
		"ContentBuilder should have at least 9 fields")
}

func TestExampleFieldTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	et := reflect.TypeFor[help.Example]()

	g.Expect(et.NumField()).To(Equal(2), "Example should have 2 fields")

	titleField, _ := et.FieldByName("Title")
	g.Expect(titleField.Type.Kind()).To(Equal(reflect.String))

	codeField, _ := et.FieldByName("Code")
	g.Expect(codeField.Type.Kind()).To(Equal(reflect.String))
}

func TestFlagFieldTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ft := reflect.TypeFor[help.Flag]()

	g.Expect(ft.NumField()).To(Equal(5), "Flag should have 5 fields")

	fields := map[string]reflect.Kind{
		"Long":        reflect.String,
		"Short":       reflect.String,
		"Desc":        reflect.String,
		"Placeholder": reflect.String,
		"Required":    reflect.Bool,
	}

	for name, kind := range fields {
		field, ok := ft.FieldByName(name)
		g.Expect(ok).To(BeTrue(), "Flag should have field %s", name)
		g.Expect(field.Type.Kind()).To(Equal(kind), "Flag.%s should be %v", name, kind)
	}
}

func TestFormatFieldTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ft := reflect.TypeFor[help.Format]()

	g.Expect(ft.NumField()).To(Equal(2), "Format should have 2 fields")

	nameField, _ := ft.FieldByName("Name")
	g.Expect(nameField.Type.Kind()).To(Equal(reflect.String))

	descField, _ := ft.FieldByName("Desc")
	g.Expect(descField.Type.Kind()).To(Equal(reflect.String))
}

func TestPositionalFieldTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	pt := reflect.TypeFor[help.Positional]()

	g.Expect(pt.NumField()).To(Equal(3), "Positional should have 3 fields")

	nameField, _ := pt.FieldByName("Name")
	g.Expect(nameField.Type.Kind()).To(Equal(reflect.String))

	placeholderField, _ := pt.FieldByName("Placeholder")
	g.Expect(placeholderField.Type.Kind()).To(Equal(reflect.String))

	requiredField, _ := pt.FieldByName("Required")
	g.Expect(requiredField.Type.Kind()).To(Equal(reflect.Bool))
}

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

func TestSubcommandFieldTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	st := reflect.TypeFor[help.Subcommand]()

	g.Expect(st.NumField()).To(Equal(2), "Subcommand should have 2 fields")

	nameField, _ := st.FieldByName("Name")
	g.Expect(nameField.Type.Kind()).To(Equal(reflect.String))

	descField, _ := st.FieldByName("Desc")
	g.Expect(descField.Type.Kind()).To(Equal(reflect.String))
}
