package commander

import (
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

	if err := validateNiladicFuncType(typ); err != nil {
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

func validateNiladicFuncType(typ reflect.Type) error {
	if typ.NumIn() != 0 || typ.NumOut() != 0 {
		return fmt.Errorf("function command must be niladic")
	}
	return nil
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
