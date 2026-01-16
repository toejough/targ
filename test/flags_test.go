package targ_test

import (
	"strings"
	"testing"

	"github.com/toejough/targ"
)

// --- Flag Parsing Tests ---

type CustomFlagName struct {
	User string `targ:"name=user_name"`
}

func (c *CustomFlagName) Run() {}

type CustomTypeFlags struct {
	Name TextValue   `targ:"flag"`
	Nick SetterValue `targ:"flag"`
}

func (c *CustomTypeFlags) Run() {}

type DefaultEnvFlags struct {
	Name string `targ:"default=Alice,env=TEST_DEFAULT_NAME_FLAG"`
}

func (c *DefaultEnvFlags) Run() {}

type DefaultFlags struct {
	Name    string `targ:"default=Alice"`
	Count   int    `targ:"default=42"`
	Enabled bool   `targ:"default=true"`
}

func (c *DefaultFlags) Run() {}

type EnvFlag struct {
	User string `targ:"env=TEST_USER_FLAG"`
}

func (c *EnvFlag) Run() {}

type RequiredFlag struct {
	ID string `targ:"required"`
}

func (c *RequiredFlag) Run() {}

type RequiredShortFlag struct {
	Name string `targ:"required,short=n"`
}

func (c *RequiredShortFlag) Run() {}

type SetterValue struct {
	Value string
}

func (s *SetterValue) Set(value string) error {
	s.Value = value + "!"
	return nil
}

type ShortBoolFlags struct {
	Verbose bool `targ:"flag,short=v"`
	Force   bool `targ:"flag,short=f"`
}

func (c *ShortBoolFlags) Run() {}

type ShortFlags struct {
	Name string `targ:"flag,short=n"`
	Age  int    `targ:"flag,short=a"`
}

func (c *ShortFlags) Run() {}

type ShortMixedFlags struct {
	Verbose bool   `targ:"flag,short=v"`
	Name    string `targ:"flag,short=n"`
}

func (c *ShortMixedFlags) Run() {}

// --- TagOptions Override ---

type TagOptionsOverride struct {
	Mode string `targ:"flag,enum=dev|prod"`
}

func (c *TagOptionsOverride) Run() {}

func (c *TagOptionsOverride) TagOptions(
	field string,
	opts targ.TagOptions,
) (targ.TagOptions, error) {
	if field == "Mode" {
		opts.Name = "stage"
		opts.Short = "s"
	}

	return opts, nil
}

// --- Custom Types ---

type TextValue struct {
	Value string
}

func (tv *TextValue) UnmarshalText(text []byte) error {
	tv.Value = strings.ToUpper(string(text))
	return nil
}

func TestCustomFlagName(t *testing.T) {
	cmd := &CustomFlagName{}

	_, err := targ.Execute([]string{"app", "--user_name", "Bob"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.User != "Bob" {
		t.Errorf("expected User='Bob', got '%s'", cmd.User)
	}
}

func TestCustomTypes(t *testing.T) {
	cmd := &CustomTypeFlags{}

	_, err := targ.Execute([]string{"app", "--name", "alice", "--nick", "bob"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Name.Value != "ALICE" {
		t.Fatalf("expected ALICE via UnmarshalText, got %q", cmd.Name.Value)
	}

	if cmd.Nick.Value != "bob!" {
		t.Fatalf("expected bob! via Set, got %q", cmd.Nick.Value)
	}
}

func TestDefaultEnvFlags_EnvOverrides(t *testing.T) {
	t.Setenv("TEST_DEFAULT_NAME_FLAG", "Bob")

	cmd := &DefaultEnvFlags{}

	_, err := targ.Execute([]string{"app"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Name != "Bob" {
		t.Errorf("expected Name='Bob', got '%s'", cmd.Name)
	}
}

func TestDefaultFlags(t *testing.T) {
	cmd := &DefaultFlags{}

	_, err := targ.Execute([]string{"app"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Name != "Alice" {
		t.Errorf("expected Name='Alice', got '%s'", cmd.Name)
	}

	if cmd.Count != 42 {
		t.Errorf("expected Count=42, got %d", cmd.Count)
	}

	if !cmd.Enabled {
		t.Error("expected Enabled=true")
	}
}

func TestEnvFlag(t *testing.T) {
	t.Setenv("TEST_USER_FLAG", "EnvAlice")

	cmd := &EnvFlag{}

	_, err := targ.Execute([]string{"app"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.User != "EnvAlice" {
		t.Errorf("expected User='EnvAlice', got '%s'", cmd.User)
	}
}

func TestLongFlagsRequireDoubleDash(t *testing.T) {
	cmd := &ShortFlags{}

	_, err := targ.Execute([]string{"app", "-name", "Alice"}, cmd)
	if err == nil {
		t.Fatal("expected error for single-dash long flag")
	}
	// Note: Error may be wrapped in ExitError; we just verify an error occurred
}

func TestRequiredFlag(t *testing.T) {
	cmd := &RequiredFlag{}

	_, err := targ.Execute([]string{"app"}, cmd)
	if err == nil {
		t.Fatal("expected error for missing required flag")
	}
}

func TestRequiredShortFlagErrorIncludesShort(t *testing.T) {
	cmd := &RequiredShortFlag{}

	result, err := targ.Execute([]string{"app"}, cmd)
	if err == nil {
		t.Fatal("expected error for missing required flag")
	}
	// Error message should mention both --name and -n
	if !strings.Contains(result.Output, "--name") || !strings.Contains(result.Output, "-n") {
		t.Fatalf("expected error to mention --name and -n, got: %q", result.Output)
	}
}

func TestShortFlagGroups(t *testing.T) {
	cmd := &ShortBoolFlags{}

	_, err := targ.Execute([]string{"app", "-vf"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cmd.Verbose || !cmd.Force {
		t.Fatalf("expected both flags set, got verbose=%v force=%v", cmd.Verbose, cmd.Force)
	}
}

func TestShortFlagGroupsRejectValueFlags(t *testing.T) {
	cmd := &ShortMixedFlags{}

	_, err := targ.Execute([]string{"app", "-vn"}, cmd)
	if err == nil {
		t.Fatal("expected error for grouped short flag with value")
	}
}

func TestShortFlags(t *testing.T) {
	cmd := &ShortFlags{}

	_, err := targ.Execute([]string{"app", "-n", "Alice", "-a", "30"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Name != "Alice" {
		t.Errorf("expected Name='Alice', got '%s'", cmd.Name)
	}

	if cmd.Age != 30 {
		t.Errorf("expected Age=30, got %d", cmd.Age)
	}
}

func TestTagOptionsOverride(t *testing.T) {
	cmd := &TagOptionsOverride{}

	_, err := targ.Execute([]string{"app", "--stage", "alpha"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Mode != "alpha" {
		t.Fatalf("expected mode=alpha, got %q", cmd.Mode)
	}
}
