package core

// Exported variables.
var (
	CallerPackagePathForTest    = callerPackagePath
	ExtractPackagePathForTest   = extractPackagePath
	HasRemoteTargetsForTest     = hasRemoteTargets
	ParseGroupLikeForTest       = parseGroupLike
	ParseTargetLikeForTest      = parseTargetLike
	PrintTopLevelCommandForTest = printTopLevelCommand
)

// Test-only exports for use by core_test package tests.
// These follow Go's standard export_test.go pattern (see Go stdlib).

// CommandNodeForTest is a type alias for the unexported commandNode type.
type CommandNodeForTest = commandNode

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
