package targ_test

import (
	"strings"
	"testing"

	"github.com/toejough/targ"
)

// --- Args struct types for Target functions ---

type CustomFlagNameArgs struct {
	User string `targ:"name=user_name"`
}

type CustomTypeFlagsArgs struct {
	Name TextValue   `targ:"flag"`
	Nick SetterValue `targ:"flag"`
}

type DefaultEnvFlagsArgs struct {
	Name string `targ:"default=Alice,env=TEST_DEFAULT_NAME_FLAG"`
}

type DefaultFlagsArgs struct {
	Name    string `targ:"default=Alice"`
	Count   int    `targ:"default=42"`
	Enabled bool   `targ:"default=true"`
}

type EnvFlagArgs struct {
	User string `targ:"env=TEST_USER_FLAG"`
}

type RequiredFlagArgs struct {
	ID string `targ:"required"`
}

type RequiredShortFlagArgs struct {
	Name string `targ:"required,short=n"`
}

type ShortBoolFlagsArgs struct {
	Verbose bool `targ:"flag,short=v"`
	Force   bool `targ:"flag,short=f"`
}

type ShortFlagsArgs struct {
	Name string `targ:"flag,short=n"`
	Age  int    `targ:"flag,short=a"`
}

type ShortMixedFlagsArgs struct {
	Verbose bool   `targ:"flag,short=v"`
	Name    string `targ:"flag,short=n"`
}

// TagOptionsOverrideArgs tests static tag attributes (replacing dynamic TagOptions method).
// Instead of runtime override, we use static tag attributes.
type TagOptionsOverrideArgs struct {
	Mode string `targ:"flag,name=stage,short=s,enum=dev|prod"`
}

// --- Custom unmarshaler types (these stay - they're not commands) ---

type SetterValue struct {
	Value string
}

func (s *SetterValue) Set(value string) error {
	s.Value = value + "!"
	return nil
}

type TextValue struct {
	Value string
}

func (tv *TextValue) UnmarshalText(text []byte) error {
	tv.Value = strings.ToUpper(string(text))
	return nil
}

// --- Tests ---

func TestCustomFlagName(t *testing.T) {
	var gotUser string

	target := targ.Targ(func(args CustomFlagNameArgs) {
		gotUser = args.User
	})

	_, err := targ.Execute([]string{"app", "--user_name", "Bob"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotUser != "Bob" {
		t.Errorf("expected User='Bob', got '%s'", gotUser)
	}
}

func TestCustomTypes(t *testing.T) {
	var gotName TextValue
	var gotNick SetterValue

	target := targ.Targ(func(args CustomTypeFlagsArgs) {
		gotName = args.Name
		gotNick = args.Nick
	})

	_, err := targ.Execute([]string{"app", "--name", "alice", "--nick", "bob"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotName.Value != "ALICE" {
		t.Fatalf("expected ALICE via UnmarshalText, got %q", gotName.Value)
	}

	if gotNick.Value != "bob!" {
		t.Fatalf("expected bob! via Set, got %q", gotNick.Value)
	}
}

func TestDefaultEnvFlags_EnvOverrides(t *testing.T) {
	t.Setenv("TEST_DEFAULT_NAME_FLAG", "Bob")

	var gotName string

	target := targ.Targ(func(args DefaultEnvFlagsArgs) {
		gotName = args.Name
	})

	_, err := targ.Execute([]string{"app"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotName != "Bob" {
		t.Errorf("expected Name='Bob', got '%s'", gotName)
	}
}

func TestDefaultFlags(t *testing.T) {
	var gotName string
	var gotCount int
	var gotEnabled bool

	target := targ.Targ(func(args DefaultFlagsArgs) {
		gotName = args.Name
		gotCount = args.Count
		gotEnabled = args.Enabled
	})

	_, err := targ.Execute([]string{"app"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotName != "Alice" {
		t.Errorf("expected Name='Alice', got '%s'", gotName)
	}

	if gotCount != 42 {
		t.Errorf("expected Count=42, got %d", gotCount)
	}

	if !gotEnabled {
		t.Error("expected Enabled=true")
	}
}

func TestEnvFlag(t *testing.T) {
	t.Setenv("TEST_USER_FLAG", "EnvAlice")

	var gotUser string

	target := targ.Targ(func(args EnvFlagArgs) {
		gotUser = args.User
	})

	_, err := targ.Execute([]string{"app"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotUser != "EnvAlice" {
		t.Errorf("expected User='EnvAlice', got '%s'", gotUser)
	}
}

func TestLongFlagsRequireDoubleDash(t *testing.T) {
	target := targ.Targ(func(_ ShortFlagsArgs) {})

	_, err := targ.Execute([]string{"app", "-name", "Alice"}, target)
	if err == nil {
		t.Fatal("expected error for single-dash long flag")
	}
	// Note: Error may be wrapped in ExitError; we just verify an error occurred
}

func TestRequiredFlag(t *testing.T) {
	target := targ.Targ(func(_ RequiredFlagArgs) {})

	_, err := targ.Execute([]string{"app"}, target)
	if err == nil {
		t.Fatal("expected error for missing required flag")
	}
}

func TestRequiredShortFlagErrorIncludesShort(t *testing.T) {
	target := targ.Targ(func(_ RequiredShortFlagArgs) {})

	result, err := targ.Execute([]string{"app"}, target)
	if err == nil {
		t.Fatal("expected error for missing required flag")
	}
	// Error message should mention both --name and -n
	if !strings.Contains(result.Output, "--name") || !strings.Contains(result.Output, "-n") {
		t.Fatalf("expected error to mention --name and -n, got: %q", result.Output)
	}
}

func TestShortFlagGroups(t *testing.T) {
	var gotVerbose, gotForce bool

	target := targ.Targ(func(args ShortBoolFlagsArgs) {
		gotVerbose = args.Verbose
		gotForce = args.Force
	})

	_, err := targ.Execute([]string{"app", "-vf"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !gotVerbose || !gotForce {
		t.Fatalf("expected both flags set, got verbose=%v force=%v", gotVerbose, gotForce)
	}
}

func TestShortFlagGroupsRejectValueFlags(t *testing.T) {
	target := targ.Targ(func(_ ShortMixedFlagsArgs) {})

	_, err := targ.Execute([]string{"app", "-vn"}, target)
	if err == nil {
		t.Fatal("expected error for grouped short flag with value")
	}
}

func TestShortFlags(t *testing.T) {
	var gotName string
	var gotAge int

	target := targ.Targ(func(args ShortFlagsArgs) {
		gotName = args.Name
		gotAge = args.Age
	})

	_, err := targ.Execute([]string{"app", "-n", "Alice", "-a", "30"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotName != "Alice" {
		t.Errorf("expected Name='Alice', got '%s'", gotName)
	}

	if gotAge != 30 {
		t.Errorf("expected Age=30, got %d", gotAge)
	}
}

func TestTagOptionsOverride(t *testing.T) {
	// Note: This test now uses static tag attributes instead of a dynamic TagOptions method.
	// The behavior is the same (--stage flag with short -s), just configured statically.
	var gotMode string

	target := targ.Targ(func(args TagOptionsOverrideArgs) {
		gotMode = args.Mode
	})

	_, err := targ.Execute([]string{"app", "--stage", "alpha"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMode != "alpha" {
		t.Fatalf("expected mode=alpha, got %q", gotMode)
	}
}
