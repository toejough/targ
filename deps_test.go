package commander

import (
	"context"
	"fmt"
	"testing"
)

var depCount int

func depOnce() {
	depCount++
}

func depErr() error {
	depCount++
	return fmt.Errorf("boom")
}

type DepStruct struct {
	Called int
}

func (d *DepStruct) Run() {
	d.Called++
}

type DepRoot struct {
	Err bool
}

func (d *DepRoot) Run() error {
	if d.Err {
		return Deps(depErr)
	}
	return Deps(depOnce, depOnce)
}

func TestDepsRunsOnce(t *testing.T) {
	depCount = 0

	err := withDepTracker(context.Background(), func() error {
		node, parseErr := parseTarget(&DepRoot{})
		if parseErr != nil {
			return parseErr
		}
		return node.execute(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if depCount != 1 {
		t.Fatalf("expected dep to run once, got %d", depCount)
	}
}

func TestDepsErrorCached(t *testing.T) {
	depCount = 0
	err := withDepTracker(context.Background(), func() error {
		node, parseErr := parseTarget(&DepRoot{Err: true})
		if parseErr != nil {
			return parseErr
		}
		if runErr := node.execute(context.Background(), []string{"--err"}); runErr == nil {
			return fmt.Errorf("expected error")
		}
		if runErr := Deps(depErr); runErr == nil {
			return fmt.Errorf("expected error on second call")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if depCount != 1 {
		t.Fatalf("expected dep error to run once, got %d", depCount)
	}
}

func TestDepsStructRunsOnce(t *testing.T) {
	dep := &DepStruct{}
	err := withDepTracker(context.Background(), func() error {
		if runErr := Deps(dep, dep); runErr != nil {
			return runErr
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dep.Called != 1 {
		t.Fatalf("expected struct dep to run once, got %d", dep.Called)
	}
}
