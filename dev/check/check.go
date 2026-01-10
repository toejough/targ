//go:build targ

// Package check provides build targets for testing, coverage, and linting.
package check

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/toejough/targ/sh"
)

// Test runs all tests.
type Test struct{}

func (t *Test) Description() string {
	return "Run all tests"
}

func (t *Test) Run() error {
	return sh.RunV("go", "test", "./...")
}

// Coverage runs tests with coverage and displays a summary.
type Coverage struct {
	HTML bool `targ:"flag,desc=Open HTML coverage report in browser"`
}

func (c *Coverage) Description() string {
	return "Run tests with coverage analysis"
}

func (c *Coverage) Run() error {
	if err := sh.RunV("go", "test", "-coverprofile=coverage.out", "./..."); err != nil {
		return err
	}

	if c.HTML {
		return sh.RunV("go", "tool", "cover", "-html=coverage.out")
	}

	return sh.RunV("go", "tool", "cover", "-func=coverage.out")
}

// Lint runs the linter.
type Lint struct{}

func (l *Lint) Description() string {
	return "Run linter (golangci-lint or go vet)"
}

func (l *Lint) Run() error {
	if _, err := exec.LookPath("golangci-lint"); err == nil {
		return sh.RunV("golangci-lint", "run")
	}
	fmt.Fprintln(os.Stderr, "golangci-lint not found, using go vet")
	return sh.RunV("go", "vet", "./...")
}

// All runs tests, coverage, and lint.
type All struct{}

func (a *All) Description() string {
	return "Run tests, coverage, and lint"
}

func (a *All) Run() error {
	t := &Test{}
	if err := t.Run(); err != nil {
		return err
	}
	cov := &Coverage{}
	if err := cov.Run(); err != nil {
		return err
	}
	l := &Lint{}
	return l.Run()
}
