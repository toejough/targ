package targ

import (
	"fmt"
	"reflect"
	"strings"
)

type positionalSpec struct {
	field    reflect.StructField
	value    reflect.Value
	opts     TagOptions
	variadic bool
}

type parseResult struct {
	remaining           []string
	subcommand          *CommandNode
	positionalsComplete bool
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
			if !ok {
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

func collectPositionalSpecs(node *CommandNode, inst reflect.Value) ([]positionalSpec, error) {
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

func parseCommandArgs(
	node *CommandNode,
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
	return parseCommandArgsWithPosition(node, inst, chain, args, visited, explicit, enforceRequired, allowIncomplete, &argPosition)
}

func parseCommandArgsWithPosition(
	node *CommandNode,
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
			consumed, err := parseFlagArgWithPosition(arg, expandedArgs, i, specByLong, specByShort, visited, allowIncomplete, argPosition)
			if err != nil {
				return parseResult{}, err
			}
			i += consumed
			continue
		}

		if posIndex < len(posSpecs) {
			spec := posSpecs[posIndex]
			if spec.variadic {
				if err := setFieldWithPosition(spec.value, arg, argPosition); err != nil {
					return parseResult{}, err
				}
				posCounts[posIndex]++
				continue
			}
			if err := setFieldWithPosition(spec.value, arg, argPosition); err != nil {
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
			if err := setFieldFromString(spec.value, *spec.opts.Default); err != nil {
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

func parseFlagArg(
	arg string,
	args []string,
	index int,
	specByLong map[string]*flagSpec,
	specByShort map[string]*flagSpec,
	visited map[string]bool,
	allowIncomplete bool,
) (int, error) {
	return parseFlagArgWithPosition(arg, args, index, specByLong, specByShort, visited, allowIncomplete, nil)
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
	if strings.HasPrefix(arg, "--") {
		name := strings.TrimPrefix(arg, "--")
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

func parseFlagValue(spec *flagSpec, args []string, index int, visited map[string]bool, allowIncomplete bool) (int, error) {
	return parseFlagValueWithPosition(spec, args, index, visited, allowIncomplete, nil)
}

func parseFlagValueWithPosition(spec *flagSpec, args []string, index int, visited map[string]bool, allowIncomplete bool, argPosition *int) (int, error) {
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
			if err := setFieldWithPosition(spec.value, next, argPosition); err != nil {
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
	if err := setFieldWithPosition(spec.value, next, argPosition); err != nil {
		return 0, err
	}
	return 1, nil
}

func markFlagVisited(visited map[string]bool, spec *flagSpec) {
	visited[spec.name] = true
	if spec.short != "" {
		visited[spec.short] = true
	}
}
