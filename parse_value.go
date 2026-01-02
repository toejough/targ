package commander

import (
	"encoding"
	"errors"
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	stringSetterType    = reflect.TypeOf((*interface{ Set(string) error })(nil)).Elem()
)

type stringFlagValue struct {
	set func(string) error
	str func() string
}

func (s *stringFlagValue) String() string {
	if s.str == nil {
		return ""
	}
	return s.str()
}

func (s *stringFlagValue) Set(value string) error {
	if s.set == nil {
		return errors.New("no setter defined")
	}
	return s.set(value)
}

func customSetter(fieldVal reflect.Value) (func(string) error, bool) {
	if fieldVal.CanAddr() {
		ptr := fieldVal.Addr()
		if ptr.Type().Implements(textUnmarshalerType) {
			return func(value string) error {
				return ptr.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value))
			}, true
		}
		if ptr.Type().Implements(stringSetterType) {
			return func(value string) error {
				return ptr.Interface().(interface{ Set(string) error }).Set(value)
			}, true
		}
	}

	fieldType := fieldVal.Type()
	if fieldType.Implements(textUnmarshalerType) {
		return func(value string) error {
			next := reflect.New(fieldType).Elem()
			if err := next.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value)); err != nil {
				return err
			}
			fieldVal.Set(next)
			return nil
		}, true
	}
	if fieldType.Implements(stringSetterType) {
		return func(value string) error {
			next := reflect.New(fieldType).Elem()
			if err := next.Interface().(interface{ Set(string) error }).Set(value); err != nil {
				return err
			}
			fieldVal.Set(next)
			return nil
		}, true
	}

	return nil, false
}

func setFieldFromString(fieldVal reflect.Value, value string) error {
	if setter, ok := customSetter(fieldVal); ok {
		return setter(value)
	}

	switch fieldVal.Kind() {
	case reflect.String:
		fieldVal.SetString(value)
	case reflect.Int:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		fieldVal.SetInt(parsed)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		fieldVal.SetBool(parsed)
	default:
		return fmt.Errorf("unsupported value type %s", fieldVal.Type())
	}

	return nil
}

func fieldSupportsTextUnmarshal(fieldType reflect.Type) bool {
	if fieldType.Implements(textUnmarshalerType) {
		return true
	}
	if fieldType.Kind() != reflect.Ptr {
		return reflect.PointerTo(fieldType).Implements(textUnmarshalerType)
	}
	return false
}

func fieldSupportsStringSetter(fieldType reflect.Type) bool {
	if fieldType.Implements(stringSetterType) {
		return true
	}
	if fieldType.Kind() != reflect.Ptr {
		return reflect.PointerTo(fieldType).Implements(stringSetterType)
	}
	return false
}

func registerHelpFlag(fs *flag.FlagSet, fieldType reflect.Type, name string, shortName string, usage string) {
	switch fieldType.Kind() {
	case reflect.String:
		fs.StringVar(new(string), name, "", usage)
		if shortName != "" {
			fs.StringVar(new(string), shortName, "", usage)
		}
	case reflect.Int:
		fs.IntVar(new(int), name, 0, usage)
		if shortName != "" {
			fs.IntVar(new(int), shortName, 0, usage)
		}
	case reflect.Bool:
		fs.BoolVar(new(bool), name, false, usage)
		if shortName != "" {
			fs.BoolVar(new(bool), shortName, false, usage)
		}
	default:
		if fieldSupportsTextUnmarshal(fieldType) || fieldSupportsStringSetter(fieldType) {
			value := &stringFlagValue{
				set: func(string) error { return nil },
				str: func() string { return "" },
			}
			fs.Var(value, name, usage)
			if shortName != "" {
				fs.Var(value, shortName, usage)
			}
		}
	}
}

func normalizedDescription(desc string) string {
	return strings.TrimSpace(desc)
}
