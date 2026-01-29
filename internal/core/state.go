package core

import (
	"fmt"
	"os"
)

// RegistryState holds mutable registry/deregistration state for testing and DI.
type RegistryState struct {
	registry           []any
	deregistrations    []Deregistration
	registryResolved   bool
	mainModuleProvider func() (string, bool)
}

// NewRegistryState creates a new isolated registry state.
func NewRegistryState() *RegistryState {
	return &RegistryState{
		mainModuleProvider: getMainModule,
	}
}

// DeregisterFrom queues a package path for deregistration on this state.
func (s *RegistryState) DeregisterFrom(packagePath string) error {
	if s.registryResolved {
		return errDeregisterAfterResolution
	}

	if packagePath == "" {
		return errEmptyPackagePath
	}

	for _, dereg := range s.deregistrations {
		if dereg.PackagePath == packagePath {
			return nil
		}
	}

	s.deregistrations = append(s.deregistrations, Deregistration{
		PackagePath: packagePath,
		RegistryLen: len(s.registry),
	})

	return nil
}

// ExecuteWithResolution resolves registry for this state and executes via RunWithEnv.
func (s *RegistryState) ExecuteWithResolution(env RunEnv, opts RunOptions) error {
	resolved, deregisteredPkgs, err := s.resolveRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		env.Exit(1)

		return err
	}

	opts.DeregisteredPackages = deregisteredPkgs

	return RunWithEnv(env, opts, resolved...)
}

// GetDeregistrations returns the current deregistrations queue.
func (s *RegistryState) GetDeregistrations() []Deregistration {
	return s.deregistrations
}

// GetRegistry returns the current registry.
func (s *RegistryState) GetRegistry() []any {
	return s.registry
}

// RegisterTarget adds targets to this state's registry.
func (s *RegistryState) RegisterTarget(targets ...any) {
	s.RegisterTargetWithSkip(CallerSkipPublicAPI, targets...)
}

// RegisterTargetWithSkip adds targets to this state's registry with caller skip.
func (s *RegistryState) RegisterTargetWithSkip(skip int, targets ...any) {
	sourcePkg, _ := callerPackagePath(skip)

	for _, item := range targets {
		if target, ok := item.(*Target); ok {
			if target.sourcePkg == "" && sourcePkg != "" {
				target.sourcePkg = sourcePkg
			}
		}

		if group, ok := item.(*TargetGroup); ok {
			if group.sourcePkg == "" && sourcePkg != "" {
				group.sourcePkg = sourcePkg
			}
		}
	}

	s.registry = append(s.registry, targets...)
}

// ResetDeregistrations clears the deregistration queue.
func (s *RegistryState) ResetDeregistrations() {
	s.deregistrations = nil
}

// ResetResolved clears the resolved flag.
func (s *RegistryState) ResetResolved() {
	s.registryResolved = false
}

// ResolveRegistryForTest exposes registry resolution for tests.
func (s *RegistryState) ResolveRegistryForTest() ([]any, []string, error) {
	return s.resolveRegistry()
}

// SetMainModuleProvider overrides main module provider (for tests).
func (s *RegistryState) SetMainModuleProvider(fn func() (string, bool)) {
	s.mainModuleProvider = fn
}

// SetRegistry replaces the registry (for tests).
func (s *RegistryState) SetRegistry(targets []any) {
	s.registry = targets
}

// unexported variables.
var (
	defaultState = NewRegistryState() //nolint:gochecknoglobals // default shared state for public API.
)
