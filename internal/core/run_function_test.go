package core

import (
	"context"
	"testing"
)

var (
	defaultFuncCalled bool
	helloWorldCalled  bool
)

var (
	multiSubOneCalls       int
	multiSubTwoCalls       int
	multiRootFlashCalls    int
	multiRootDiscoverCalls int
)

type multiSubOne struct{}
type multiSubTwo struct{}

type MultiSubRoot struct {
	One *multiSubOne `targ:"subcommand"`
	Two *multiSubTwo `targ:"subcommand"`
}

func (o *multiSubOne) Run() { multiSubOneCalls++ }
func (t *multiSubTwo) Run() { multiSubTwoCalls++ }

type firmwareRoot struct {
	FlashOnly *firmwareFlashOnly `targ:"subcommand=flash-only"`
}

func (f *firmwareRoot) Name() string { return "firmware" }

type firmwareFlashOnly struct{}

func (f *firmwareFlashOnly) Run() { multiRootFlashCalls++ }

type discoverRoot struct{}

func (d *discoverRoot) Name() string { return "discover" }

func (d *discoverRoot) Run() { multiRootDiscoverCalls++ }

func DefaultFunc() {
	defaultFuncCalled = true
}

func HelloWorld() {
	helloWorldCalled = true
}

func TestRunWithEnv_SingleFunction_DefaultCommand(t *testing.T) {
	defaultFuncCalled = false

	env := MockrunEnv(t)
	done := make(chan struct{})

	go func() {
		RunWithEnv(env.Mock, RunOptions{AllowDefault: true}, DefaultFunc)
		close(done)
	}()

	env.Method.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd"})
	<-done

	if !defaultFuncCalled {
		t.Fatal("expected function command to be called")
	}
}

func ContextFunc(ctx context.Context) {
	if ctx != nil {
		helloWorldCalled = true
	}
}

func TestRunWithEnv_ContextFunction(t *testing.T) {
	helloWorldCalled = false

	env := MockrunEnv(t)
	done := make(chan struct{})

	go func() {
		RunWithEnv(env.Mock, RunOptions{AllowDefault: true}, ContextFunc)
		close(done)
	}()

	env.Method.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd"})
	<-done

	if !helloWorldCalled {
		t.Fatal("expected context function command to be called")
	}
}

func TestRunWithEnv_MultipleTargets_FunctionByName(t *testing.T) {
	helloWorldCalled = false

	env := MockrunEnv(t)
	done := make(chan struct{})

	go func() {
		RunWithEnv(env.Mock, RunOptions{AllowDefault: true}, HelloWorld, &TestCmdStruct{})
		close(done)
	}()

	env.Method.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd", "hello-world"})
	<-done

	if !helloWorldCalled {
		t.Fatal("expected function command to be called")
	}
}

func TestRunWithEnv_SingleFunction_NoDefault(t *testing.T) {
	defaultFuncCalled = false

	env := MockrunEnv(t)
	done := make(chan struct{})

	go func() {
		RunWithEnv(env.Mock, RunOptions{AllowDefault: false}, DefaultFunc)
		close(done)
	}()

	env.Method.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd"})
	<-done

	if defaultFuncCalled {
		t.Fatal("expected function command not to be called without default")
	}
}

type FuncSubcommandRoot struct {
	Hello func() `targ:"subcommand"`
}

func TestRunWithEnv_FunctionSubcommand(t *testing.T) {
	called := false
	root := FuncSubcommandRoot{
		Hello: func() { called = true },
	}

	env := MockrunEnv(t)
	done := make(chan struct{})

	go func() {
		RunWithEnv(env.Mock, RunOptions{AllowDefault: true}, root)
		close(done)
	}()

	env.Method.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd", "hello"})
	<-done

	if !called {
		t.Fatal("expected function subcommand to be called")
	}
}

func TestRunWithEnv_MultipleSubcommands(t *testing.T) {
	multiSubOneCalls = 0
	multiSubTwoCalls = 0

	env := MockrunEnv(t)
	done := make(chan struct{})

	go func() {
		RunWithEnv(env.Mock, RunOptions{AllowDefault: true}, &MultiSubRoot{})
		close(done)
	}()

	env.Method.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd", "one", "two"})
	<-done

	if multiSubOneCalls != 1 {
		t.Fatalf("expected One to run once, got %d", multiSubOneCalls)
	}
	if multiSubTwoCalls != 1 {
		t.Fatalf("expected Two to run once, got %d", multiSubTwoCalls)
	}
}

func TestRunWithEnv_MultipleRoots_SubcommandThenRoot(t *testing.T) {
	multiRootFlashCalls = 0
	multiRootDiscoverCalls = 0

	env := MockrunEnv(t)
	done := make(chan struct{})

	go func() {
		RunWithEnv(env.Mock, RunOptions{AllowDefault: false}, &firmwareRoot{}, &discoverRoot{})
		close(done)
	}()

	env.Method.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd", "firmware", "flash-only", "discover"})
	<-done

	if multiRootFlashCalls != 1 {
		t.Fatalf("expected flash-only to run once, got %d", multiRootFlashCalls)
	}
	if multiRootDiscoverCalls != 1 {
		t.Fatalf("expected discover to run once, got %d", multiRootDiscoverCalls)
	}
}

func TestRunWithEnv_CaretResetsToRoot(t *testing.T) {
	multiSubOneCalls = 0
	multiRootDiscoverCalls = 0

	env := &executeEnv{args: []string{"cmd", "multi-sub-root", "one", "^", "discover"}}
	err := RunWithEnv(env, RunOptions{AllowDefault: false}, &MultiSubRoot{}, &discoverRoot{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if multiSubOneCalls != 1 {
		t.Fatalf("expected One to run once, got %d", multiSubOneCalls)
	}
	if multiRootDiscoverCalls != 1 {
		t.Fatalf("expected discover to run once, got %d", multiRootDiscoverCalls)
	}
}

func TestRunWithEnv_DisableHelp(t *testing.T) {
	// With DisableHelp, --help should be passed through as unknown flag
	env := &executeEnv{args: []string{"cmd", "--help"}}
	err := RunWithEnv(env, RunOptions{DisableHelp: true}, DefaultFunc)
	if err == nil {
		t.Fatal("expected error when help is disabled and --help is passed")
	}
	// Should error as unknown flag
	if exitErr, ok := err.(ExitError); !ok || exitErr.Code != 1 {
		t.Fatalf("expected ExitError with code 1, got %v", err)
	}
}

func TestRunWithEnv_DisableTimeout(t *testing.T) {
	// With DisableTimeout, --timeout should be passed through as unknown flag
	env := &executeEnv{args: []string{"cmd", "--timeout", "5m"}}
	err := RunWithEnv(env, RunOptions{DisableTimeout: true}, DefaultFunc)
	if err == nil {
		t.Fatal("expected error when timeout is disabled and --timeout is passed")
	}
}

func TestRunWithEnv_DisableCompletion(t *testing.T) {
	// With DisableCompletion, --completion should be passed through as unknown flag
	env := &executeEnv{args: []string{"cmd", "--completion", "bash"}}
	err := RunWithEnv(env, RunOptions{DisableCompletion: true}, DefaultFunc)
	if err == nil {
		t.Fatal("expected error when completion is disabled and --completion is passed")
	}
}
