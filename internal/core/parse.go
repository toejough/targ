package core

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// unexported variables.
var (
	stringSetterType    = reflect.TypeFor[interface{ Set(string) error }]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

type parseResult struct {
	remaining           []string
	subcommand          *commandNode
	positionalsComplete bool
}

// --- Argument parsing ---

type positionalSpec struct {
	field    reflect.StructField
	value    reflect.Value
	opts     TagOptions
	variadic bool
}

func collectFlagSpecs(chain []commandInstance) ([]*flagSpec, map[string]bool, error) {
	var specs []*flagSpec

	longNames := map[string]bool{}
	usedNames := map[string]bool{}

	for _, inst := range chain {
		if inst.node == nil || inst.node.Type == nil {
			continue
		}

		typ := inst.node.Type
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			fieldVal := inst.value.Field(i)

			spec, ok, err := flagSpecForField(inst.value, field, fieldVal)
			if err != nil {
				return nil, nil, err
			}

			if !ok || spec == nil {
				continue
			}

			if usedNames[spec.name] {
				return nil, nil, fmt.Errorf("flag %s already defined", spec.name)
			}

			usedNames[spec.name] = true
			if spec.short != "" {
				if usedNames[spec.short] {
					return nil, nil, fmt.Errorf("flag %s already defined", spec.short)
				}

				usedNames[spec.short] = true
			}

			longNames[spec.name] = true
			specs = append(specs, spec)
		}
	}

	return specs, longNames, nil
}

func collectPositionalSpecs(node *commandNode, inst reflect.Value) ([]positionalSpec, error) {
	if node == nil || node.Type == nil {
		return nil, nil
	}

	typ := node.Type

	var specs []positionalSpec

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		opts, ok, err := tagOptionsForField(inst, field)
		if err != nil {
			return nil, err
		}

		if !ok || opts.Kind != TagKindPositional {
			continue
		}

		if !field.IsExported() {
			return nil, fmt.Errorf("field %s must be exported", field.Name)
		}

		fieldVal := inst.Field(i)
		specs = append(specs, positionalSpec{
			field:    field,
			value:    fieldVal,
			opts:     opts,
			variadic: field.Type.Kind() == reflect.Slice,
		})
	}

	return specs, nil
}

func customSetter(fieldVal reflect.Value) (func(string) error, bool) {
	if fieldVal.CanAddr() {
		ptr := fieldVal.Addr()
		if ptr.Type().Implements(textUnmarshalerType) {
			return func(value string) error {
				u, ok := ptr.Interface().(encoding.TextUnmarshaler)
				if !ok {
					return errors.New("type assertion to TextUnmarshaler failed")
				}

				return u.UnmarshalText([]byte(value))
			}, true
		}

		if ptr.Type().Implements(stringSetterType) {
			return func(value string) error {
				s, ok := ptr.Interface().(interface{ Set(s string) error })
				if !ok {
					return errors.New("type assertion to Set(string) error failed")
				}

				return s.Set(value)
			}, true
		}
	}

	fieldType := fieldVal.Type()
	if fieldType.Implements(textUnmarshalerType) {
		return func(value string) error {
			next := reflect.New(fieldType).Elem()

			u, ok := next.Interface().(encoding.TextUnmarshaler)
			if !ok {
				return errors.New("type assertion to TextUnmarshaler failed")
			}

			err := u.UnmarshalText([]byte(value))
			if err != nil {
				return err
			}

			fieldVal.Set(next)

			return nil
		}, true
	}

	if fieldType.Implements(stringSetterType) {
		return func(value string) error {
			next := reflect.New(fieldType).Elem()

			s, ok := next.Interface().(interface{ Set(s string) error })
			if !ok {
				return errors.New("type assertion to Set(string) error failed")
			}

			err := s.Set(value)
			if err != nil {
				return err
			}

			fieldVal.Set(next)

			return nil
		}, true
	}

	return nil, false
}

// isInterleavedType checks if a type is Interleaved[T] by looking for Value and Position fields.
func isInterleavedType(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}
	// Check for our Interleaved struct signature: Value field and Position int field
	valueField, hasValue := t.FieldByName("Value")

	posField, hasPos := t.FieldByName("Position")
	if !hasValue || !hasPos {
		return false
	}
	// Position must be int
	if posField.Type.Kind() != reflect.Int {
		return false
	}
	// Value field exists (any type is fine)
	_ = valueField

	return true
}

func markFlagVisited(visited map[string]bool, spec *flagSpec) {
	visited[spec.name] = true
	if spec.short != "" {
		visited[spec.short] = true
	}
}

func parseCommandArgs(
	node *commandNode,
	inst reflect.Value,
	chain []commandInstance,
	args []string,
	visited map[string]bool,
	explicit bool,
	enforceRequired bool,
	allowIncomplete bool,
) (parseResult, error) {
	// Position counter for Interleaved slice tracking
	argPosition := 0

	return parseCommandArgsWithPosition(
		node,
		inst,
		chain,
		args,
		visited,
		explicit,
		enforceRequired,
		allowIncomplete,
		&argPosition,
	)
}

func parseCommandArgsWithPosition(
	node *commandNode,
	inst reflect.Value,
	chain []commandInstance,
	args []string,
	visited map[string]bool,
	explicit bool,
	enforceRequired bool,
	allowIncomplete bool,
	argPosition *int,
) (parseResult, error) {
	if visited == nil {
		visited = map[string]bool{}
	}

	specs, longNames, err := collectFlagSpecs(chain)
	if err != nil {
		return parseResult{}, err
	}

	expandedArgs, err := expandShortFlagGroups(args, specs)
	if err != nil {
		return parseResult{}, err
	}

	if err := validateLongFlagArgs(expandedArgs, longNames); err != nil {
		return parseResult{}, err
	}

	specByLong := map[string]*flagSpec{}
	specByShort := map[string]*flagSpec{}

	for _, spec := range specs {
		specByLong[spec.name] = spec
		if spec.short != "" {
			specByShort[spec.short] = spec
		}
	}

	posSpecs, err := collectPositionalSpecs(node, inst)
	if err != nil {
		return parseResult{}, err
	}

	posCounts := make([]int, len(posSpecs))

	posIndex := 0

	for i := 0; i < len(expandedArgs); i++ {
		arg := expandedArgs[i]
		if arg == "--" {
			if posIndex < len(posSpecs) && posSpecs[posIndex].variadic {
				posIndex++
			}

			continue
		}

		if strings.HasPrefix(arg, "-") && arg != "-" {
			if posIndex < len(posSpecs) && posSpecs[posIndex].variadic && posCounts[posIndex] > 0 {
				posIndex++
			}

			consumed, err := parseFlagArgWithPosition(
				arg,
				expandedArgs,
				i,
				specByLong,
				specByShort,
				visited,
				allowIncomplete,
				argPosition,
			)
			if err != nil {
				return parseResult{}, err
			}

			i += consumed

			continue
		}

		if posIndex < len(posSpecs) {
			spec := posSpecs[posIndex]
			if spec.variadic {
				err := setFieldWithPosition(spec.value, arg, argPosition)
				if err != nil {
					return parseResult{}, err
				}

				posCounts[posIndex]++

				continue
			}

			err := setFieldWithPosition(spec.value, arg, argPosition)
			if err != nil {
				return parseResult{}, err
			}

			posCounts[posIndex] = 1
			posIndex++

			continue
		}

		if node != nil && len(node.Subcommands) > 0 {
			if sub, ok := node.Subcommands[arg]; ok {
				return parseResult{
					remaining:           expandedArgs[i+1:],
					subcommand:          sub,
					positionalsComplete: true,
				}, nil
			}
		}

		if !explicit {
			return parseResult{}, fmt.Errorf("unknown command: %s", arg)
		}

		return parseResult{remaining: expandedArgs[i:]}, nil
	}

	for idx, spec := range posSpecs {
		if posCounts[idx] == 0 && spec.opts.Default != nil {
			err := setFieldFromString(spec.value, *spec.opts.Default)
			if err != nil {
				return parseResult{}, err
			}

			posCounts[idx] = 1
		}

		if enforceRequired && spec.opts.Required && posCounts[idx] == 0 {
			name := spec.opts.Name
			if name == "" {
				name = spec.field.Name
			}

			return parseResult{}, fmt.Errorf("missing required positional %s", name)
		}
	}

	complete := true

	for idx, spec := range posSpecs {
		if spec.opts.Required && posCounts[idx] == 0 {
			complete = false
			break
		}
	}

	return parseResult{positionalsComplete: complete}, nil
}

func parseFlagArgWithPosition(
	arg string,
	args []string,
	index int,
	specByLong map[string]*flagSpec,
	specByShort map[string]*flagSpec,
	visited map[string]bool,
	allowIncomplete bool,
	argPosition *int,
) (int, error) {
	if after, ok := strings.CutPrefix(arg, "--"); ok {
		name := after
		value := ""
		hasValue := false

		if eq := strings.Index(name, "="); eq >= 0 {
			value = name[eq+1:]
			name = name[:eq]
			hasValue = true
		}

		spec := specByLong[name]
		if spec == nil {
			return 0, fmt.Errorf("flag provided but not defined: --%s", name)
		}

		markFlagVisited(visited, spec)

		if hasValue {
			return 0, setFieldWithPosition(spec.value, value, argPosition)
		}

		return parseFlagValueWithPosition(spec, args, index, visited, allowIncomplete, argPosition)
	}

	name := strings.TrimPrefix(arg, "-")
	value := ""
	hasValue := false

	if eq := strings.Index(name, "="); eq >= 0 {
		value = name[eq+1:]
		name = name[:eq]
		hasValue = true
	}

	spec := specByShort[name]
	if spec == nil {
		return 0, fmt.Errorf("flag provided but not defined: -%s", name)
	}

	markFlagVisited(visited, spec)

	if hasValue {
		return 0, setFieldWithPosition(spec.value, value, argPosition)
	}

	return parseFlagValueWithPosition(spec, args, index, visited, allowIncomplete, argPosition)
}

func parseFlagValueWithPosition(
	spec *flagSpec,
	args []string,
	index int,
	visited map[string]bool,
	allowIncomplete bool,
	argPosition *int,
) (int, error) {
	if spec.value.Kind() == reflect.Bool {
		spec.value.SetBool(true)

		if argPosition != nil {
			*argPosition++
		}

		return 0, nil
	}

	if spec.value.Kind() == reflect.Slice {
		count := 0

		for i := index + 1; i < len(args); i++ {
			next := args[i]
			if next == "--" || strings.HasPrefix(next, "-") {
				break
			}

			err := setFieldWithPosition(spec.value, next, argPosition)
			if err != nil {
				return 0, err
			}

			count++
		}

		if count == 0 {
			if allowIncomplete && index+1 >= len(args) {
				return 0, nil
			}

			return 0, fmt.Errorf("flag needs an argument: --%s", spec.name)
		}

		return count, nil
	}

	if index+1 >= len(args) {
		if allowIncomplete {
			return 0, nil
		}

		return 0, fmt.Errorf("flag needs an argument: --%s", spec.name)
	}

	next := args[index+1]
	if next == "--" || strings.HasPrefix(next, "-") {
		return 0, fmt.Errorf("flag needs an argument: --%s", spec.name)
	}

	err := setFieldWithPosition(spec.value, next, argPosition)
	if err != nil {
		return 0, err
	}

	return 1, nil
}

func setFieldFromString(fieldVal reflect.Value, value string) error {
	return setFieldWithPosition(fieldVal, value, nil)
}

// setFieldWithPosition sets a field value, optionally tracking position for Interleaved slices.
// If pos is non-nil and the field is []Interleaved[T], the position is used and incremented.
func setFieldWithPosition(fieldVal reflect.Value, value string, pos *int) error {
	if setter, ok := customSetter(fieldVal); ok {
		if pos != nil {
			*pos++
		}

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
	case reflect.Slice:
		elemType := fieldVal.Type().Elem()
		if isInterleavedType(elemType) && pos != nil {
			// Create Interleaved[T] with position
			elem := reflect.New(elemType).Elem()
			valueField := elem.FieldByName("Value")
			posField := elem.FieldByName("Position")

			err := setFieldWithPosition(valueField, value, nil)
			if err != nil {
				return err
			}

			posField.SetInt(int64(*pos))
			*pos++

			fieldVal.Set(reflect.Append(fieldVal, elem))
		} else {
			elem := reflect.New(elemType).Elem()

			err := setFieldWithPosition(elem, value, pos)
			if err != nil {
				return err
			}

			fieldVal.Set(reflect.Append(fieldVal, elem))
		}
	case reflect.Map:
		// Initialize map if nil
		if fieldVal.IsNil() {
			fieldVal.Set(reflect.MakeMap(fieldVal.Type()))
		}
		// Parse key=value
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid map value %q, expected key=value", value)
		}

		keyVal := reflect.New(fieldVal.Type().Key()).Elem()
		valVal := reflect.New(fieldVal.Type().Elem()).Elem()

		err := setFieldWithPosition(keyVal, parts[0], nil)
		if err != nil {
			return err
		}

		err = setFieldWithPosition(valVal, parts[1], nil)
		if err != nil {
			return err
		}

		fieldVal.SetMapIndex(keyVal, valVal)
	default:
		return fmt.Errorf("unsupported value type %s", fieldVal.Type())
	}

	if pos != nil && fieldVal.Kind() != reflect.Slice {
		*pos++
	}

	return nil
}
