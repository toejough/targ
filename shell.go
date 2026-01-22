package targ

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/toejough/targ/sh"
)

// Shell executes a shell command with variable substitution from struct fields.
// Variables are specified as $name in the command string and are replaced with
// the corresponding field value from the args struct.
//
// Example:
//
//	type DeployArgs struct {
//	    Namespace string
//	    File      string
//	}
//	err := targ.Shell(ctx, "kubectl apply -n $namespace -f $file", args)
//
// Field names are matched case-insensitively (e.g., $namespace matches Namespace).
// Unknown variables return an error.
func Shell(ctx context.Context, cmd string, args any) error {
	substituted, err := substituteVars(cmd, args)
	if err != nil {
		return err
	}

	err = sh.RunContext(ctx, "sh", "-c", substituted)
	if err != nil {
		return fmt.Errorf("shell command failed: %w", err)
	}

	return nil
}

// varPattern matches $var or ${var} style variables.
var varPattern = regexp.MustCompile(`\$\{?([a-zA-Z_][a-zA-Z0-9_]*)\}?`)

// substituteVars replaces $var placeholders with values from the args struct.
func substituteVars(cmd string, args any) (string, error) {
	if args == nil {
		// No args - check if there are any variables that need substitution
		matches := varPattern.FindAllStringSubmatch(cmd, -1)
		if len(matches) > 0 {
			return "", fmt.Errorf("variable $%s has no value (args is nil)", matches[0][1])
		}

		return cmd, nil
	}

	// Build a map of lowercase field names to values
	values, err := structToMap(args)
	if err != nil {
		return "", err
	}

	var errs []string

	result := varPattern.ReplaceAllStringFunc(cmd, func(match string) string {
		// Extract the variable name (without $ and optional braces)
		submatch := varPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}

		varName := strings.ToLower(submatch[1])
		if val, ok := values[varName]; ok {
			return val
		}

		errs = append(errs, varName)

		return match
	})

	if len(errs) > 0 {
		return "", fmt.Errorf("unknown variable(s): $%s", strings.Join(errs, ", $"))
	}

	return result, nil
}

// structToMap converts a struct to a map of lowercase field names to string values.
func structToMap(args any) (map[string]string, error) {
	v := reflect.ValueOf(args)

	// Handle pointer to struct
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, fmt.Errorf("args is nil pointer")
		}

		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("args must be a struct, got %T", args)
	}

	t := v.Type()
	values := make(map[string]string)

	for i := range t.NumField() {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		fieldVal := v.Field(i)
		strVal := formatValue(fieldVal)
		values[strings.ToLower(field.Name)] = strVal
	}

	return values, nil
}

// formatValue converts a reflect.Value to its string representation.
func formatValue(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", v.Float())
	case reflect.Bool:
		return fmt.Sprintf("%t", v.Bool())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}
