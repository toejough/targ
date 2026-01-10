//go:build targ

// Package check provides build targets for testing, coverage, and linting.
package check

import "github.com/toejough/targ/sh"

// Test runs all tests with coverage.
type Test struct{}

func (t *Test) Description() string {
	return "Run all tests with coverage"
}

func (t *Test) Run() error {
	return sh.RunV("go", "test", "-coverprofile=coverage.out", "./...")
}

// Coverage displays the coverage report.
type Coverage struct {
	HTML bool `targ:"flag,desc=Open HTML report in browser"`
}

func (c *Coverage) Description() string {
	return "Display coverage report"
}

func (c *Coverage) Run() error {
	if c.HTML {
		return sh.RunV("go", "tool", "cover", "-html=coverage.out")
	}
	return sh.RunV("go", "tool", "cover", "-func=coverage.out")
}

// Lint runs the linter.
type Lint struct{}

func (l *Lint) Description() string {
	return "Run golangci-lint"
}

func (l *Lint) Run() error {
	return sh.RunV("golangci-lint", "run")
}

// All runs tests and lint.
type All struct{}

func (a *All) Description() string {
	return "Run tests and lint"
}

func (a *All) Run() error {
	t := &Test{}
	if err := t.Run(); err != nil {
		return err
	}
	l := &Lint{}
	return l.Run()
}
