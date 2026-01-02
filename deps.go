package commander

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

type depKey struct {
	kind string
	id   uintptr
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
		return fmt.Errorf("Deps must be called during commander.Run")
	}
	for _, target := range targets {
		if err := tracker.run(target); err != nil {
			return err
		}
	}
	return nil
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
	return node.execute(d.ctx, nil)
}

func depKeyFor(target interface{}) (depKey, error) {
	if target == nil {
		return depKey{}, fmt.Errorf("dependency target cannot be nil")
	}
	v := reflect.ValueOf(target)
	switch v.Kind() {
	case reflect.Func:
		return depKey{kind: "func", id: v.Pointer()}, nil
	case reflect.Ptr:
		if v.IsNil() {
			return depKey{}, fmt.Errorf("dependency target cannot be nil")
		}
		return depKey{kind: "ptr", id: v.Pointer()}, nil
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
