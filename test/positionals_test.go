package targ_test

import (
	"testing"

	"github.com/toejough/targ"
)

type ChildCmd struct {
	Name   string `targ:"flag"`
	Called bool
}

func (h *ChildCmd) Run() {
	h.Called = true
}

type DefaultPositional struct {
	Pos string `targ:"positional,default=default_value"`
}

func (c *DefaultPositional) Run() {}

type DiscoveryChildB struct{}

// --- Discovery ---

type DiscoveryRootA struct {
	Sub *DiscoveryChildB `targ:"subcommand"`
}

type DiscoveryRootC struct{}

type InterleavedFlagsPositionals struct {
	Name  string `targ:"positional"`
	Count int    `targ:"flag,short=c"`
}

func (c *InterleavedFlagsPositionals) Run() {}

type ParentCmd struct {
	Sub    *SubCmd `targ:"subcommand"`
	Custom *SubCmd `targ:"subcommand=custom"`
}

// --- Persistent Flags ---

type PersistentRoot struct {
	Verbose bool      `targ:"flag,short=v"`
	Child   *ChildCmd `targ:"subcommand"`
}

// --- Positional Arguments ---

type PositionalArgs struct {
	Src string `targ:"positional"`
	Dst string `targ:"positional"`
}

func (c *PositionalArgs) Run() {}

type RequiredPositional struct {
	Src string `targ:"positional,required"`
	Dst string `targ:"positional"`
}

func (c *RequiredPositional) Run() {}

// --- Subcommands ---

type SubCmd struct {
	Verbose bool
	Called  bool
}

func (s *SubCmd) Run() {
	s.Called = true
}

func TestDefaultPositional(t *testing.T) {
	cmd := &DefaultPositional{}

	_, err := targ.Execute([]string{"app"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Pos != "default_value" {
		t.Fatalf("expected default_value, got %q", cmd.Pos)
	}
}

func TestDetectRootCommands(t *testing.T) {
	candidates := []any{
		&DiscoveryRootA{},
		&DiscoveryChildB{},
		&DiscoveryRootC{},
	}

	roots := targ.DetectRootCommands(candidates...)

	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}

	hasA := false
	hasC := false
	hasB := false

	for _, r := range roots {
		switch r.(type) {
		case *DiscoveryRootA:
			hasA = true
		case *DiscoveryRootC:
			hasC = true
		case *DiscoveryChildB:
			hasB = true
		}
	}

	if !hasA {
		t.Error("expected RootA to be detected")
	}

	if !hasC {
		t.Error("expected RootC to be detected")
	}

	if hasB {
		t.Error("ChildB should have been filtered out")
	}
}

func TestInterleavedFlagsAndPositionals(t *testing.T) {
	cmd := &InterleavedFlagsPositionals{}

	_, err := targ.Execute([]string{"app", "Bob", "--count", "2"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Name != "Bob" {
		t.Fatalf("expected Name=Bob, got %q", cmd.Name)
	}

	if cmd.Count != 2 {
		t.Fatalf("expected Count=2, got %d", cmd.Count)
	}
}

func TestPersistentFlagsInherited(t *testing.T) {
	root := &PersistentRoot{}

	_, err := targ.Execute([]string{"app", "child", "--verbose", "--name", "ok"}, root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !root.Verbose {
		t.Fatal("expected root verbose flag to be set from subcommand args")
	}

	if root.Child == nil || !root.Child.Called {
		t.Fatal("expected child to be called")
	}
}

func TestPositionalArgs(t *testing.T) {
	cmd := &PositionalArgs{}

	_, err := targ.Execute([]string{"app", "source.txt", "dest.txt"}, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Src != "source.txt" {
		t.Errorf("expected Src='source.txt', got '%s'", cmd.Src)
	}

	if cmd.Dst != "dest.txt" {
		t.Errorf("expected Dst='dest.txt', got '%s'", cmd.Dst)
	}
}

func TestRequiredPositional(t *testing.T) {
	cmd := &RequiredPositional{}

	_, err := targ.Execute([]string{"app"}, cmd)
	if err == nil {
		t.Fatal("expected error for missing required positional")
	}
}

func TestSubcommandCustomName(t *testing.T) {
	parent := &ParentCmd{}

	_, err := targ.Execute([]string{"app", "custom", "--verbose"}, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parent.Custom == nil || !parent.Custom.Called {
		t.Fatal("expected custom to be called")
	}
}

func TestSubcommands(t *testing.T) {
	parent := &ParentCmd{}

	_, err := targ.Execute([]string{"app", "sub", "--verbose"}, parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parent.Sub == nil || !parent.Sub.Called {
		t.Fatal("expected sub to be called")
	}

	if !parent.Sub.Verbose {
		t.Fatal("expected verbose to be set")
	}
}
