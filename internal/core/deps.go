package core

import (
	"context"
	"errors"
	"reflect"
	"sync"
)

// DepsOption configures Deps behavior.
type DepsOption interface {
	applyDeps(*depsConfig)
}

// ContinueOnError runs all dependencies even if one fails.
// Without this option, Deps fails fast (cancels remaining on first error).
func ContinueOnError() DepsOption { return continueOnErrorOpt{} }

// Deps executes dependencies, each exactly once per CLI run.
// Options can be mixed with targets in any order:
//
//	targ.Deps(A, B, C)                              // serial, fail-fast
//	targ.Deps(A, B, C, targ.Parallel())             // parallel, fail-fast
//	targ.Deps(A, B, C, targ.ContinueOnError())      // serial, run all
//	targ.Deps(A, B, targ.Parallel(), targ.WithContext(ctx))
func Deps(args ...any) error {
	depsMu.Lock()

	tracker := currentDeps

	depsMu.Unlock()

	if tracker == nil {
		return errors.New("Deps must be called during targ.Run")
	}

	// Separate options from targets
	var (
		cfg     depsConfig
		targets []any
	)

	for _, arg := range args {
		if opt, ok := arg.(DepsOption); ok {
			opt.applyDeps(&cfg)
		} else {
			targets = append(targets, arg)
		}
	}

	// Use tracker's context if none specified
	ctx := cfg.ctx
	if ctx == nil {
		ctx = tracker.ctx
	}

	if cfg.parallel {
		return parallelRun(tracker, ctx, targets, cfg.continueOnError)
	}

	return serialRun(tracker, ctx, targets, cfg.continueOnError)
}

// Parallel runs dependencies concurrently instead of sequentially.
func Parallel() DepsOption { return parallelOpt{} }

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

// WithContext passes a custom context to dependencies.
// Useful for cancellation in watch mode.
func WithContext(ctx context.Context) DepsOption { return withContextOpt{ctx} }

// unexported variables.
var (
	currentDeps *depTracker
	depsMu      sync.Mutex
)

type continueOnErrorOpt struct{}

func (continueOnErrorOpt) applyDeps(c *depsConfig) { c.continueOnError = true }

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

func (d *depTracker) execute(ctx context.Context, target any) error {
	node, err := parseTarget(target)
	if err != nil {
		return err
	}

	return node.execute(ctx, nil, RunOptions{})
}

func (d *depTracker) run(ctx context.Context, target any) error {
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

	err = d.execute(ctx, target)

	d.mu.Lock()
	d.done[key] = err
	delete(d.inFlight, key)
	close(ch)
	d.mu.Unlock()

	return err
}

type depsConfig struct {
	parallel        bool
	continueOnError bool
	ctx             context.Context
}

type parallelOpt struct{}

func (parallelOpt) applyDeps(c *depsConfig) { c.parallel = true }

type withContextOpt struct{ ctx context.Context }

func (o withContextOpt) applyDeps(c *depsConfig) { c.ctx = o.ctx }

func depKeyFor(target any) (depKey, error) {
	if target == nil {
		return depKey{}, errors.New("dependency target cannot be nil")
	}

	v := reflect.ValueOf(target)
	switch v.Kind() {
	case reflect.Func:
		return depKey{kind: "func", id: v.Pointer(), typName: v.Type().String()}, nil
	case reflect.Ptr:
		if v.IsNil() {
			return depKey{}, errors.New("dependency target cannot be nil")
		}
		// Include type name to distinguish zero-sized structs with same address
		return depKey{kind: "ptr", id: v.Pointer(), typName: v.Type().String()}, nil
	default:
		return depKey{}, errors.New("dependency target must be func or pointer to struct")
	}
}

func newDepTracker(ctx context.Context) *depTracker {
	return &depTracker{
		ctx:      ctx,
		done:     make(map[depKey]error),
		inFlight: make(map[depKey]chan struct{}),
	}
}

func parallelRun(
	tracker *depTracker,
	ctx context.Context,
	targets []any,
	continueOnError bool,
) error {
	if len(targets) == 0 {
		return nil
	}

	// For fail-fast, create a cancellable context
	runCtx := ctx

	var cancel context.CancelFunc
	if !continueOnError {
		runCtx, cancel = context.WithCancel(ctx)
		defer cancel()
	}

	var wg sync.WaitGroup

	errCh := make(chan error, len(targets))

	for _, target := range targets {
		wg.Go(func() {
			err := tracker.run(runCtx, target)
			if err != nil {
				errCh <- err

				if !continueOnError && cancel != nil {
					cancel() // cancel siblings on first error
				}
			}
		})
	}

	wg.Wait()
	close(errCh)

	// Return first error
	for err := range errCh {
		return err
	}

	return nil
}

func serialRun(
	tracker *depTracker,
	ctx context.Context,
	targets []any,
	continueOnError bool,
) error {
	var firstErr error

	for _, target := range targets {
		// Check for cancellation before each target
		select {
		case <-ctx.Done():
			if firstErr != nil {
				return firstErr
			}

			return ctx.Err()
		default:
		}

		err := tracker.run(ctx, target)
		if err != nil {
			if !continueOnError {
				return err
			}

			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
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
