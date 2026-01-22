package targ

import "testing"

func TestExecuteRegisteredWithOptions_Exists(t *testing.T) {
	// Verify ExecuteRegisteredWithOptions is callable (prevents deadcode removal).
	// We can't fully test it because it calls os.Exit, but we verify the function exists
	// and has the right signature by taking its address.
	_ = t // use t to avoid unused parameter lint error
	_ = ExecuteRegisteredWithOptions
}

func TestExecuteRegistered_Exists(t *testing.T) {
	// Verify ExecuteRegistered is callable (prevents deadcode removal).
	// We can't fully test it because it calls os.Exit.
	_ = t // use t to avoid unused parameter lint error
	_ = ExecuteRegistered
}

func TestRegister(t *testing.T) {
	// Save original registry
	orig := registry
	registry = nil

	defer func() { registry = orig }()

	// Register some targets
	target1 := Targ(func() {})
	target2 := Targ(func() {})
	Register(target1, target2)

	if len(registry) != 2 {
		t.Fatalf("expected 2 targets in registry, got %d", len(registry))
	}
}

func TestRegister_Append(t *testing.T) {
	// Save original registry
	orig := registry
	registry = nil

	defer func() { registry = orig }()

	// Register in two calls
	target1 := Targ(func() {})
	target2 := Targ(func() {})

	Register(target1)
	Register(target2)

	if len(registry) != 2 {
		t.Fatalf("expected 2 targets in registry, got %d", len(registry))
	}
}
