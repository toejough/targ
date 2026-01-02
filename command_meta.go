package commander

import (
	"reflect"
	"strings"
)

func getCommandName(v reflect.Value, typ reflect.Type) string {
	name := callStringMethod(v, typ, "CommandName")
	if name == "" {
		return ""
	}
	return camelToKebab(name)
}

func getDescription(v reflect.Value, typ reflect.Type) string {
	desc := callStringMethod(v, typ, "Description")
	return strings.TrimSpace(desc)
}

func callStringMethod(v reflect.Value, typ reflect.Type, method string) string {
	if m := methodValue(v, typ, method); m.IsValid() {
		if m.Type().NumIn() == 0 && m.Type().NumOut() == 1 && m.Type().Out(0).Kind() == reflect.String {
			out := m.Call(nil)
			if len(out) == 1 {
				if s, ok := out[0].Interface().(string); ok {
					return strings.TrimSpace(s)
				}
			}
		}
	}
	return ""
}

func methodValue(v reflect.Value, typ reflect.Type, method string) reflect.Value {
	if v.IsValid() {
		if m := v.MethodByName(method); m.IsValid() {
			return m
		}
		if v.CanAddr() {
			if m := v.Addr().MethodByName(method); m.IsValid() {
				return m
			}
		}
	}
	ptr := reflect.New(typ)
	if m := ptr.MethodByName(method); m.IsValid() {
		return m
	}
	if m := ptr.Elem().MethodByName(method); m.IsValid() {
		return m
	}
	return reflect.Value{}
}
