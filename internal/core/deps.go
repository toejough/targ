package core

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

type depKey struct {
	kind    string
	id      uintptr
	typName string
}

type depTracker struct {
	ctx      context.Context
	mu       sync.Mutex
	done     map[depKey]error
	inFlight map[depKey]chan struct{}
}

var (
	depsMu      sync.Mutex
	currentDeps *depTracker
)

func newDepTracker(ctx context.Context) *depTracker {
	return &depTracker{
		ctx:      ctx,
		done:     make(map[depKey]error),
		inFlight: make(map[depKey]chan struct{}),
	}
}

// Deps executes each dependency exactly once per CLI run.
func Deps(targets ...interface{}) error {
	depsMu.Lock()
	tracker := currentDeps
	depsMu.Unlock()
	if tracker == nil {
		return fmt.Errorf("Deps must be called during targ.Run")
	}
	for _, target := range targets {
		if err := tracker.run(target); err != nil {
			return err
		}
	}
	return nil
}

// ResetDeps clears the dependency execution cache, allowing all targets
// to run again on subsequent Deps() calls. This is useful for watch mode
// where the same targets need to re-run on each file change.
func ResetDeps() {
	depsMu.Lock()
	defer depsMu.Unlock()
	if currentDeps != nil {
		currentDeps.done = make(map[depKey]error)
	}
}

// ParallelDeps executes dependencies in parallel, ensuring each target runs once.
func ParallelDeps(targets ...interface{}) error {
	depsMu.Lock()
	tracker := currentDeps
	depsMu.Unlock()
	if tracker == nil {
		return fmt.Errorf("ParallelDeps must be called during targ.Run")
	}
	if len(targets) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(targets))
	for _, target := range targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- tracker.run(target)
		}()
	}
	wg.Wait()
	close(errCh)
	var firstErr error
	for err := range errCh {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (d *depTracker) run(target interface{}) error {
	key, err := depKeyFor(target)
	if err != nil {
		return err
	}

	d.mu.Lock()
	if existing, ok := d.done[key]; ok {
		d.mu.Unlock()
		return existing
	}
	if ch, ok := d.inFlight[key]; ok {
		d.mu.Unlock()
		<-ch
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.done[key]
	}
	ch := make(chan struct{})
	d.inFlight[key] = ch
	d.mu.Unlock()

	err = d.execute(target)

	d.mu.Lock()
	d.done[key] = err
	delete(d.inFlight, key)
	close(ch)
	d.mu.Unlock()
	return err
}

func (d *depTracker) execute(target interface{}) error {
	node, err := parseTarget(target)
	if err != nil {
		return err
	}
	return node.execute(d.ctx, nil, RunOptions{})
}

func depKeyFor(target interface{}) (depKey, error) {
	if target == nil {
		return depKey{}, fmt.Errorf("dependency target cannot be nil")
	}
	v := reflect.ValueOf(target)
	switch v.Kind() {
	case reflect.Func:
		return depKey{kind: "func", id: v.Pointer(), typName: v.Type().String()}, nil
	case reflect.Ptr:
		if v.IsNil() {
			return depKey{}, fmt.Errorf("dependency target cannot be nil")
		}
		// Include type name to distinguish zero-sized structs with same address
		return depKey{kind: "ptr", id: v.Pointer(), typName: v.Type().String()}, nil
	default:
		return depKey{}, fmt.Errorf("dependency target must be func or pointer to struct")
	}
}

func withDepTracker(ctx context.Context, fn func() error) error {
	tracker := newDepTracker(ctx)
	depsMu.Lock()
	prev := currentDeps
	currentDeps = tracker
	depsMu.Unlock()
	defer func() {
		depsMu.Lock()
		currentDeps = prev
		depsMu.Unlock()
	}()
	return fn()
}
