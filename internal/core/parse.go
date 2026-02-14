package core

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// unexported constants.
const (
	keyValueParts = 2
)

// unexported variables.
var (
	errFlagAlreadyDefined        = errors.New("flag already defined")
	errFlagNeedsArgument         = errors.New("flag needs an argument")
	errFlagNotDefined            = errors.New("flag provided but not defined")
	errInvalidMapValue           = errors.New("invalid map value, expected key=value")
	errMissingRequiredPositional = errors.New("missing required positional")
	errStringSetterFailed        = errors.New("type assertion to Set(string) error failed")
	errTextUnmarshalerFailed     = errors.New("type assertion to TextUnmarshaler failed")
	errUnknownCommand            = errors.New("unknown command")
	errUnsupportedValueType      = errors.New("unsupported value type")
	//nolint:gochecknoglobals,inamedparam // reflect type for flag.Value interface
	stringSetterType = reflect.TypeFor[interface{ Set(string) error }]()
	//nolint:gochecknoglobals // reflect type for text unmarshaling
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

type parseContext struct {
	node            *commandNode
	expandedArgs    []string
	specByLong      map[string]*flagSpec
	specByShort     map[string]*flagSpec
	posSpecs        []positionalSpec
	posCounts       []int
	visited         map[string]bool
	explicit        bool
	allowIncomplete bool
	argPosition     *int
	posIndex        int
}

// advanceVariadicPositional moves past a variadic positional on "--".
func (ctx *parseContext) advanceVariadicPositional() {
	if ctx.posIndex < len(ctx.posSpecs) && ctx.posSpecs[ctx.posIndex].variadic {
		ctx.posIndex++
	}
}

// parseArg processes a single argument at the given index.
func (ctx *parseContext) parseArg(i int) (*parseResult, int, error) {
	arg := ctx.expandedArgs[i]

	if arg == "--" {
		ctx.advanceVariadicPositional()
		return nil, 0, nil
	}

	if strings.HasPrefix(arg, "-") && arg != "-" {
		return ctx.parseFlag(i, arg)
	}

	return ctx.parsePositionalOrSubcommand(i, arg)
}

// parseArgs processes all arguments in the context.
func (ctx *parseContext) parseArgs() (parseResult, error) {
	for i := 0; i < len(ctx.expandedArgs); i++ {
		result, consumed, err := ctx.parseArg(i)
		if err != nil {
			return parseResult{}, err
		}

		if result != nil {
			return *result, nil
		}

		i += consumed
	}

	return parseResult{}, nil
}

// parseFlag handles flag arguments.
func (ctx *parseContext) parseFlag(i int, arg string) (*parseResult, int, error) {
	if ctx.posIndex < len(ctx.posSpecs) &&
		ctx.posSpecs[ctx.posIndex].variadic &&
		ctx.posCounts[ctx.posIndex] > 0 {
		ctx.posIndex++
	}

	consumed, err := parseFlagArgWithPosition(
		arg,
		ctx.expandedArgs,
		i,
		ctx.specByLong,
		ctx.specByShort,
		ctx.visited,
		ctx.allowIncomplete,
		ctx.argPosition,
	)

	return nil, consumed, err
}

// parsePositional handles a positional argument.
func (ctx *parseContext) parsePositional(arg string) (*parseResult, int, error) {
	spec := ctx.posSpecs[ctx.posIndex]

	err := setFieldWithPosition(spec.value, arg, ctx.argPosition)
	if err != nil {
		return nil, 0, err
	}

	ctx.posCounts[ctx.posIndex]++

	if !spec.variadic {
		ctx.posIndex++
	}

	return nil, 0, nil
}

// parsePositionalOrSubcommand handles positional args and subcommand detection.
func (ctx *parseContext) parsePositionalOrSubcommand(i int, arg string) (*parseResult, int, error) {
	if ctx.posIndex < len(ctx.posSpecs) {
		return ctx.parsePositional(arg)
	}

	return ctx.trySubcommandOrUnknown(i, arg)
}

// trySubcommandOrUnknown attempts to match a subcommand or handle unknown arg.
func (ctx *parseContext) trySubcommandOrUnknown(i int, arg string) (*parseResult, int, error) {
	if ctx.node != nil && len(ctx.node.Subcommands) > 0 {
		if sub, ok := ctx.node.Subcommands[arg]; ok {
			return &parseResult{
				remaining:           ctx.expandedArgs[i+1:],
				subcommand:          sub,
				positionalsComplete: true,
			}, 0, nil
		}
	}

	if !ctx.explicit {
		return nil, 0, fmt.Errorf("%w: %s", errUnknownCommand, arg)
	}

	return &parseResult{remaining: ctx.expandedArgs[i:]}, 0, nil
}

type parseResult struct {
	remaining           []string
	subcommand          *commandNode
	positionalsComplete bool
}

type positionalSpec struct {
	field    reflect.StructField
	value    reflect.Value
	opts     TagOptions
	variadic bool
}

func addressableCustomSetter(fieldVal reflect.Value) (func(string) error, bool) {
	if !fieldVal.CanAddr() {
		return nil, false
	}

	ptr := fieldVal.Addr()
	if ptr.Type().Implements(textUnmarshalerType) {
		return func(value string) error {
			u, ok := ptr.Interface().(encoding.TextUnmarshaler)
			if !ok {
				return errTextUnmarshalerFailed
			}

			return u.UnmarshalText([]byte(value))
		}, true
	}

	if ptr.Type().Implements(stringSetterType) {
		return func(value string) error {
			s, ok := ptr.Interface().(interface{ Set(s string) error })
			if !ok {
				return errStringSetterFailed
			}

			return s.Set(value)
		}, true
	}

	return nil, false
}

// appendInterleavedElement appends an Interleaved[T] element with position tracking.
func appendInterleavedElement(
	fieldVal reflect.Value,
	elemType reflect.Type,
	value string,
	pos *int,
) error {
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

	return nil
}

// applyPositionalDefaults applies defaults and validates required positionals.
func applyPositionalDefaults(
	posSpecs []positionalSpec,
	posCounts []int,
	enforceRequired bool,
) error {
	for idx, spec := range posSpecs {
		if posCounts[idx] == 0 && spec.opts.Default != nil {
			err := setFieldFromString(spec.value, *spec.opts.Default)
			if err != nil {
				return err
			}

			posCounts[idx] = 1
		}

		if enforceRequired && spec.opts.Required && posCounts[idx] == 0 {
			return missingPositionalError(spec)
		}
	}

	return nil
}

// buildSpecMaps creates lookup maps for flag specs.
func buildSpecMaps(specs []*flagSpec) (map[string]*flagSpec, map[string]*flagSpec) {
	specByLong := map[string]*flagSpec{}
	specByShort := map[string]*flagSpec{}

	for _, spec := range specs {
		specByLong[spec.name] = spec
		if spec.short != "" {
			specByShort[spec.short] = spec
		}
	}

	return specByLong, specByShort
}

func collectFieldFlagSpec(
	inst commandInstance,
	field reflect.StructField,
	fieldVal reflect.Value,
	usedNames map[string]bool,
) (*flagSpec, bool, error) {
	spec, ok, err := flagSpecForField(inst.value, field, fieldVal)
	if err != nil {
		return nil, false, err
	}

	if !ok || spec == nil {
		return nil, false, nil
	}

	err = registerFlagName(spec, usedNames)
	if err != nil {
		return nil, false, err
	}

	return spec, true, nil
}

func collectFlagSpecs(chain []commandInstance) ([]*flagSpec, map[string]bool, error) {
	var specs []*flagSpec

	longNames := map[string]bool{}
	usedNames := map[string]bool{}

	for _, inst := range chain {
		instSpecs, err := collectInstanceFlagSpecs(inst, usedNames)
		if err != nil {
			return nil, nil, err
		}

		for _, spec := range instSpecs {
			longNames[spec.name] = true
			specs = append(specs, spec)
		}
	}

	return specs, longNames, nil
}

func collectInstanceFlagSpecs(
	inst commandInstance,
	usedNames map[string]bool,
) ([]*flagSpec, error) {
	if inst.node == nil || inst.node.Type == nil {
		return nil, nil
	}

	return collectStructFlagSpecs(inst, inst.node.Type, inst.value, usedNames)
}

func collectPositionalSpecs(node *commandNode, inst reflect.Value) ([]positionalSpec, error) {
	if node == nil || node.Type == nil {
		return nil, nil
	}

	return collectStructPositionalSpecs(node.Type, inst)
}

func collectStructFlagSpecs(
	inst commandInstance,
	typ reflect.Type,
	val reflect.Value,
	usedNames map[string]bool,
) ([]*flagSpec, error) {
	var specs []*flagSpec

	for i := range typ.NumField() {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Handle embedded (anonymous) structs by recursing into them
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			embeddedSpecs, err := collectStructFlagSpecs(inst, field.Type, fieldVal, usedNames)
			if err != nil {
				return nil, err
			}

			specs = append(specs, embeddedSpecs...)

			continue
		}

		spec, ok, err := collectFieldFlagSpec(inst, field, fieldVal, usedNames)
		if err != nil {
			return nil, err
		}

		if ok {
			specs = append(specs, spec)
		}
	}

	return specs, nil
}

func collectStructPositionalSpecs(typ reflect.Type, inst reflect.Value) ([]positionalSpec, error) {
	specs := make([]positionalSpec, 0, typ.NumField())

	for i := range typ.NumField() {
		field := typ.Field(i)
		fieldVal := inst.Field(i)

		// Handle embedded (anonymous) structs by recursing into them
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			embeddedSpecs, err := collectStructPositionalSpecs(field.Type, fieldVal)
			if err != nil {
				return nil, err
			}

			specs = append(specs, embeddedSpecs...)

			continue
		}

		opts, err := tagOptionsForField(inst, field)
		if err != nil {
			return nil, err
		}

		if opts.Kind != TagKindPositional {
			continue
		}

		if !field.IsExported() {
			return nil, fmt.Errorf("%w: %s", errFieldNotExported, field.Name)
		}

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
	return addressableCustomSetter(fieldVal)
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

// missingPositionalError creates an error for a missing required positional.
func missingPositionalError(spec positionalSpec) error {
	name := spec.opts.Name
	if name == "" {
		name = spec.field.Name
	}

	return fmt.Errorf("%w: %s", errMissingRequiredPositional, name)
}

func parseBoolFlagValue(spec *flagSpec, argPosition *int) (int, error) {
	spec.value.SetBool(true)

	if argPosition != nil {
		*argPosition++
	}

	return 0, nil
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
	ctx, err := prepareParseContext(
		node,
		inst,
		chain,
		args,
		visited,
		explicit,
		allowIncomplete,
		argPosition,
	)
	if err != nil {
		return parseResult{}, err
	}

	result, err := ctx.parseArgs()
	if err != nil {
		return parseResult{}, err
	}

	if result.subcommand != nil || len(result.remaining) > 0 {
		return result, nil
	}

	err = applyPositionalDefaults(ctx.posSpecs, ctx.posCounts, enforceRequired)
	if err != nil {
		return parseResult{}, err
	}

	return parseResult{positionalsComplete: positionalsComplete(ctx.posSpecs, ctx.posCounts)}, nil
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
			return 0, fmt.Errorf("%w: --%s", errFlagNotDefined, name)
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
		return 0, fmt.Errorf("%w: -%s", errFlagNotDefined, name)
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
	_ map[string]bool,
	allowIncomplete bool,
	argPosition *int,
) (int, error) {
	if spec.value.Kind() == reflect.Bool {
		return parseBoolFlagValue(spec, argPosition)
	}

	if spec.value.Kind() == reflect.Slice {
		return parseSliceFlagValue(spec, args, index, allowIncomplete, argPosition)
	}

	return parseSingleFlagValue(spec, args, index, allowIncomplete, argPosition)
}

func parseSingleFlagValue(
	spec *flagSpec,
	args []string,
	index int,
	allowIncomplete bool,
	argPosition *int,
) (int, error) {
	if index+1 >= len(args) {
		if allowIncomplete {
			return 0, nil
		}

		return 0, fmt.Errorf("%w: --%s", errFlagNeedsArgument, spec.name)
	}

	next := args[index+1]
	if next == "--" || strings.HasPrefix(next, "-") {
		return 0, fmt.Errorf("%w: --%s", errFlagNeedsArgument, spec.name)
	}

	err := setFieldWithPosition(spec.value, next, argPosition)
	if err != nil {
		return 0, err
	}

	return 1, nil
}

func parseSliceFlagValue(
	spec *flagSpec,
	args []string,
	index int,
	allowIncomplete bool,
	argPosition *int,
) (int, error) {
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

		return 0, fmt.Errorf("%w: --%s", errFlagNeedsArgument, spec.name)
	}

	return count, nil
}

// positionalsComplete checks if all required positionals have been filled.
func positionalsComplete(posSpecs []positionalSpec, posCounts []int) bool {
	for idx, spec := range posSpecs {
		if spec.opts.Required && posCounts[idx] == 0 {
			return false
		}
	}

	return true
}

// prepareParseContext sets up the parsing context with specs and expanded args.
func prepareParseContext(
	node *commandNode,
	inst reflect.Value,
	chain []commandInstance,
	args []string,
	visited map[string]bool,
	explicit bool,
	allowIncomplete bool,
	argPosition *int,
) (*parseContext, error) {
	if visited == nil {
		visited = map[string]bool{}
	}

	specs, longNames, err := collectFlagSpecs(chain)
	if err != nil {
		return nil, err
	}

	expandedArgs, err := expandShortFlagGroups(args, specs)
	if err != nil {
		return nil, err
	}

	err = validateLongFlagArgs(expandedArgs, longNames)
	if err != nil {
		return nil, err
	}

	specByLong, specByShort := buildSpecMaps(specs)

	posSpecs, err := collectPositionalSpecs(node, inst)
	if err != nil {
		return nil, err
	}

	return &parseContext{
		node:            node,
		expandedArgs:    expandedArgs,
		specByLong:      specByLong,
		specByShort:     specByShort,
		posSpecs:        posSpecs,
		posCounts:       make([]int, len(posSpecs)),
		visited:         visited,
		explicit:        explicit,
		allowIncomplete: allowIncomplete,
		argPosition:     argPosition,
	}, nil
}

func registerFlagName(spec *flagSpec, usedNames map[string]bool) error {
	if usedNames[spec.name] {
		return fmt.Errorf("%w: %s", errFlagAlreadyDefined, spec.name)
	}

	usedNames[spec.name] = true

	if spec.short != "" {
		if usedNames[spec.short] {
			return fmt.Errorf("%w: %s", errFlagAlreadyDefined, spec.short)
		}

		usedNames[spec.short] = true
	}

	return nil
}

// setBoolField parses and sets a boolean field.
func setBoolField(fieldVal reflect.Value, value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("parsing bool %q: %w", value, err)
	}

	fieldVal.SetBool(parsed)

	return nil
}

// setFieldByKind handles the type-specific field setting logic.
func setFieldByKind(fieldVal reflect.Value, value string, pos *int) error {
	switch fieldVal.Kind() { //nolint:exhaustive // default handles unsupported types
	case reflect.String:
		fieldVal.SetString(value)
	case reflect.Int:
		return setIntField(fieldVal, value)
	case reflect.Bool:
		return setBoolField(fieldVal, value)
	case reflect.Float64:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float64 value %q: %w", value, err)
		}
		fieldVal.SetFloat(v)
	case reflect.Slice:
		return setSliceField(fieldVal, value, pos)
	case reflect.Map:
		return setMapField(fieldVal, value)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedValueType, fieldVal.Type())
	}

	return nil
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

	err := setFieldByKind(fieldVal, value, pos)
	if err != nil {
		return err
	}

	if pos != nil && fieldVal.Kind() != reflect.Slice {
		*pos++
	}

	return nil
}

// setIntField parses and sets an integer field.
func setIntField(fieldVal reflect.Value, value string) error {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing int %q: %w", value, err)
	}

	fieldVal.SetInt(parsed)

	return nil
}

// setMapField parses a key=value pair and sets it in a map field.
func setMapField(fieldVal reflect.Value, value string) error {
	if fieldVal.IsNil() {
		fieldVal.Set(reflect.MakeMap(fieldVal.Type()))
	}

	parts := strings.SplitN(value, "=", keyValueParts)
	if len(parts) != keyValueParts {
		return fmt.Errorf("%w: %q", errInvalidMapValue, value)
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

	return nil
}

// setSliceField appends a value to a slice field, handling interleaved types.
func setSliceField(fieldVal reflect.Value, value string, pos *int) error {
	elemType := fieldVal.Type().Elem()
	if isInterleavedType(elemType) && pos != nil {
		return appendInterleavedElement(fieldVal, elemType, value, pos)
	}

	elem := reflect.New(elemType).Elem()

	err := setFieldWithPosition(elem, value, pos)
	if err != nil {
		return err
	}

	fieldVal.Set(reflect.Append(fieldVal, elem))

	return nil
}
