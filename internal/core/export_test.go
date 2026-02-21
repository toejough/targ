package core

// Exported variables.
var (
	BuildPositionalPartsForTest  = buildPositionalParts
	CallerPackagePathForTest     = callerPackagePath
	ChainExampleForTest          = chainExample
	ConvertExamplesForTest       = convertExamples
	ExtractPackagePathForTest    = extractPackagePath
	ParseGroupLikeForTest        = parseGroupLike
	ParseTargetLikeForTest       = parseTargetLike
	PositionalDisplayNameForTest = positionalDisplayName
	PrintCommandHelpForTest      = printCommandHelp
	ResolveMoreInfoTextForTest   = resolveMoreInfoText
)

// Test-only exports for use by core_test package tests.
// These follow Go's standard export_test.go pattern (see Go stdlib).

// CommandNodeForTest is a type alias for the unexported commandNode type.
type CommandNodeForTest = commandNode

// PositionalHelpForTest is a type alias for the unexported positionalHelp type.
type PositionalHelpForTest = positionalHelp

// NewReportedErrorForTest wraps an error in a reportedError for testing.
func NewReportedErrorForTest(err error) error {
	return reportedError{err: err}
}

// NewTargetForTest creates a Target with unexported fields set for testing.
func NewTargetForTest(name, desc, sourcePkg string, nameOverridden bool) *Target {
	return &Target{
		name:           name,
		description:    desc,
		sourcePkg:      sourcePkg,
		nameOverridden: nameOverridden,
	}
}

// NewTargetGroupForTest creates a TargetGroup with unexported fields set for testing.
func NewTargetGroupForTest(name string, members []any) *TargetGroup {
	return &TargetGroup{
		name:    name,
		members: members,
	}
}
