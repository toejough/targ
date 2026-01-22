package targ_test

import (
	"testing"

	"github.com/toejough/targ"
)

type DefaultPositional struct {
	Pos string `targ:"positional,default=default_value"`
}

type InterleavedFlagsPositionals struct {
	Name  string `targ:"positional"`
	Count int    `targ:"flag,short=c"`
}

// --- Positional Arguments ---

type PositionalArgs struct {
	Src string `targ:"positional"`
	Dst string `targ:"positional"`
}

type RequiredPositional struct {
	Src string `targ:"positional,required"`
	Dst string `targ:"positional"`
}

func TestDefaultPositional(t *testing.T) {
	var gotPos string

	target := targ.Targ(func(args DefaultPositional) {
		gotPos = args.Pos
	})

	_, err := targ.Execute([]string{"app"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPos != "default_value" {
		t.Fatalf("expected default_value, got %q", gotPos)
	}
}

// --- Embedded Struct Flag Sharing ---
// (Replaces "persistent flags" from struct model)

func TestEmbeddedFlags_SharedAcrossTargets(t *testing.T) {
	type CommonFlags struct {
		Verbose bool `targ:"flag,short=v"`
	}

	type ChildArgs struct {
		CommonFlags

		Name string `targ:"flag"`
	}

	var (
		gotVerbose bool
		gotName    string
	)

	child := targ.Targ(func(args ChildArgs) {
		gotVerbose = args.Verbose
		gotName = args.Name
	}).Name("child")

	group := targ.NewGroup("parent", child)

	_, err := targ.Execute([]string{"app", "child", "--verbose", "--name", "ok"}, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !gotVerbose {
		t.Fatal("expected verbose flag to be set")
	}

	if gotName != "ok" {
		t.Fatalf("expected name='ok', got %q", gotName)
	}
}

func TestGroup_CustomNameRouting(t *testing.T) {
	var called string

	sub := targ.Targ(func() { called = "sub" }).Name("sub")
	custom := targ.Targ(func() { called = "custom" }).Name("custom")
	group := targ.NewGroup("parent", sub, custom)

	_, err := targ.Execute([]string{"app", "custom"}, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if called != "custom" {
		t.Fatal("expected custom to be called")
	}
}

// --- Group Routing ---

func TestGroup_SubcommandRouting(t *testing.T) {
	var called string

	sub := targ.Targ(func() { called = "sub" }).Name("sub")
	custom := targ.Targ(func() { called = "custom" }).Name("custom")
	group := targ.NewGroup("parent", sub, custom)

	_, err := targ.Execute([]string{"app", "sub"}, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if called != "sub" {
		t.Fatal("expected sub to be called")
	}
}

func TestInterleavedFlagsAndPositionals(t *testing.T) {
	var (
		gotName  string
		gotCount int
	)

	target := targ.Targ(func(args InterleavedFlagsPositionals) {
		gotName = args.Name
		gotCount = args.Count
	})

	_, err := targ.Execute([]string{"app", "Bob", "--count", "2"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotName != "Bob" {
		t.Fatalf("expected Name=Bob, got %q", gotName)
	}

	if gotCount != 2 {
		t.Fatalf("expected Count=2, got %d", gotCount)
	}
}

func TestPositionalArgs(t *testing.T) {
	var gotSrc, gotDst string

	target := targ.Targ(func(args PositionalArgs) {
		gotSrc = args.Src
		gotDst = args.Dst
	})

	_, err := targ.Execute([]string{"app", "source.txt", "dest.txt"}, target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotSrc != "source.txt" {
		t.Errorf("expected Src='source.txt', got '%s'", gotSrc)
	}

	if gotDst != "dest.txt" {
		t.Errorf("expected Dst='dest.txt', got '%s'", gotDst)
	}
}

func TestRequiredPositional(t *testing.T) {
	target := targ.Targ(func(_ RequiredPositional) {})

	_, err := targ.Execute([]string{"app"}, target)
	if err == nil {
		t.Fatal("expected error for missing required positional")
	}
}
