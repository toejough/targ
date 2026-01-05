package targ

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
		runWithEnv(env.Interface(), RunOptions{AllowDefault: true}, DefaultFunc)
		close(done)
	}()

	env.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd"})
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
		runWithEnv(env.Interface(), RunOptions{AllowDefault: true}, ContextFunc)
		close(done)
	}()

	env.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd"})
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
		runWithEnv(env.Interface(), RunOptions{AllowDefault: true}, HelloWorld, &TestCmdStruct{})
		close(done)
	}()

	env.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd", "hello-world"})
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
		runWithEnv(env.Interface(), RunOptions{AllowDefault: false}, DefaultFunc)
		close(done)
	}()

	env.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd"})
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
		runWithEnv(env.Interface(), RunOptions{AllowDefault: true}, root)
		close(done)
	}()

	env.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd", "hello"})
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
		runWithEnv(env.Interface(), RunOptions{AllowDefault: true}, &MultiSubRoot{})
		close(done)
	}()

	env.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd", "one", "two"})
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
		runWithEnv(env.Interface(), RunOptions{AllowDefault: false}, &firmwareRoot{}, &discoverRoot{})
		close(done)
	}()

	env.Args.ExpectCalledWithExactly().InjectReturnValues([]string{"cmd", "firmware", "flash-only", "discover"})
	<-done

	if multiRootFlashCalls != 1 {
		t.Fatalf("expected flash-only to run once, got %d", multiRootFlashCalls)
	}
	if multiRootDiscoverCalls != 1 {
		t.Fatalf("expected discover to run once, got %d", multiRootDiscoverCalls)
	}
}
