package targs

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

func parseTarget(t interface{}) (*CommandNode, error) {
	if t == nil {
		return nil, fmt.Errorf("nil target")
	}

	v := reflect.ValueOf(t)
	if v.Kind() == reflect.Func {
		return parseFunc(v)
	}

	return parseStruct(t)
}

func parseFunc(v reflect.Value) (*CommandNode, error) {
	typ := v.Type()
	if typ.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected func, got %v", typ.Kind())
	}

	if err := validateFuncType(typ); err != nil {
		return nil, err
	}

	name := functionName(v)
	if name == "" {
		return nil, fmt.Errorf("unable to determine function name")
	}

	return &CommandNode{
		Name:        camelToKebab(name),
		Func:        v,
		Subcommands: make(map[string]*CommandNode),
	}, nil
}

func validateFuncType(typ reflect.Type) error {
	if typ.NumIn() > 1 {
		return fmt.Errorf("function command must be niladic or accept context")
	}
	if typ.NumIn() == 1 && !isContextType(typ.In(0)) {
		return fmt.Errorf("function command must accept context.Context")
	}
	if typ.NumOut() == 0 {
		return nil
	}
	if typ.NumOut() == 1 && isErrorType(typ.Out(0)) {
		return nil
	}
	return fmt.Errorf("function command must return only error")
}

func isContextType(t reflect.Type) bool {
	return t == reflect.TypeOf((*context.Context)(nil)).Elem()
}

func isErrorType(t reflect.Type) bool {
	return t.Implements(reflect.TypeOf((*error)(nil)).Elem())
}

func functionName(v reflect.Value) string {
	fn := runtime.FuncForPC(v.Pointer())
	if fn == nil {
		return ""
	}

	name := fn.Name()
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, "-fm")
	return name
}
