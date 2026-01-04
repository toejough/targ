package targ

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
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
		node, parseErr := parseTarget(&DepRoot{})
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

func TestParallelDepsRunsConcurrently(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	worker := func() {
		started <- struct{}{}
		<-release
	}

	err := withDepTracker(context.Background(), func() error {
		done := make(chan error, 1)
		go func() {
			done <- ParallelDeps(worker, func() { worker() })
		}()

		timeout := time.After(200 * time.Millisecond)
		for i := 0; i < 2; i++ {
			select {
			case <-started:
			case <-timeout:
				close(release)
				return fmt.Errorf("expected both tasks to start concurrently")
			}
		}
		close(release)
		return <-done
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParallelDepsSharesDependencies(t *testing.T) {
	depCount = 0
	var runCount int32
	dep := func() { depCount++ }
	target := func() error {
		if err := Deps(dep); err != nil {
			return err
		}
		atomic.AddInt32(&runCount, 1)
		return nil
	}

	err := withDepTracker(context.Background(), func() error {
		return ParallelDeps(target, func() error { return target() })
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if depCount != 1 {
		t.Fatalf("expected shared dep to run once, got %d", depCount)
	}
	if runCount != 2 {
		t.Fatalf("expected both targets to run, got %d", runCount)
	}
}

func TestParallelDepsReturnsError(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	bad := func() error { return fmt.Errorf("boom") }
	waiter := func() error {
		started <- struct{}{}
		<-release
		return nil
	}

	err := withDepTracker(context.Background(), func() error {
		done := make(chan error, 1)
		go func() {
			done <- ParallelDeps(bad, waiter)
		}()
		select {
		case <-started:
		case <-time.After(200 * time.Millisecond):
			close(release)
			return fmt.Errorf("expected waiter to start")
		}
		close(release)
		return <-done
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
