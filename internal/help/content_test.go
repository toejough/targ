package help_test

import (
	"reflect"
	"testing"

	"github.com/toejough/targ/internal/help"
	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

func TestPositionalFieldTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	p := help.Positional{}
	pt := reflect.TypeOf(p)

	g.Expect(pt.NumField()).To(Equal(3), "Positional should have 3 fields")

	nameField, _ := pt.FieldByName("Name")
	g.Expect(nameField.Type.Kind()).To(Equal(reflect.String))

	placeholderField, _ := pt.FieldByName("Placeholder")
	g.Expect(placeholderField.Type.Kind()).To(Equal(reflect.String))

	requiredField, _ := pt.FieldByName("Required")
	g.Expect(requiredField.Type.Kind()).To(Equal(reflect.Bool))
}

func TestFlagFieldTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	f := help.Flag{}
	ft := reflect.TypeOf(f)

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

	f := help.Format{}
	ft := reflect.TypeOf(f)

	g.Expect(ft.NumField()).To(Equal(2), "Format should have 2 fields")

	nameField, _ := ft.FieldByName("Name")
	g.Expect(nameField.Type.Kind()).To(Equal(reflect.String))

	descField, _ := ft.FieldByName("Desc")
	g.Expect(descField.Type.Kind()).To(Equal(reflect.String))
}

func TestSubcommandFieldTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	s := help.Subcommand{}
	st := reflect.TypeOf(s)

	g.Expect(st.NumField()).To(Equal(2), "Subcommand should have 2 fields")

	nameField, _ := st.FieldByName("Name")
	g.Expect(nameField.Type.Kind()).To(Equal(reflect.String))

	descField, _ := st.FieldByName("Desc")
	g.Expect(descField.Type.Kind()).To(Equal(reflect.String))
}

func TestExampleFieldTypes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	e := help.Example{}
	et := reflect.TypeOf(e)

	g.Expect(et.NumField()).To(Equal(2), "Example should have 2 fields")

	titleField, _ := et.FieldByName("Title")
	g.Expect(titleField.Type.Kind()).To(Equal(reflect.String))

	codeField, _ := et.FieldByName("Code")
	g.Expect(codeField.Type.Kind()).To(Equal(reflect.String))
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
