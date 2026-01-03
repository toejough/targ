package targs

import (
	"context"
	"testing"
)

var (
	defaultFuncCalled bool
	helloWorldCalled  bool
)

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
	Hello func() `targs:"subcommand"`
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
