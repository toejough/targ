package sh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
)

// OutputContext executes a command and returns combined output, with context support.
// When ctx is cancelled, the process and all its children are killed.
func OutputContext(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin

	// Capture combined output
	var buf safeBuffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	done := make(chan error, 1)

	setProcGroup(cmd)

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("starting command: %w", err)
	}

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return buf.String(), err
	case <-ctx.Done():
		killProcessGroup(cmd)
		<-done

		return buf.String(), fmt.Errorf("command cancelled: %w", ctx.Err())
	}
}

// RunContext executes a command with context support.
// When ctx is cancelled, the process and all its children are killed.
func RunContext(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin

	return runWithContext(ctx, cmd)
}

// RunContextV executes a command, prints it first, with context support.
// When ctx is cancelled, the process and all its children are killed.
func RunContextV(ctx context.Context, name string, args ...string) error {
	_, _ = fmt.Fprintln(stdout, "+", formatCommand(name, args))
	return RunContext(ctx, name, args...)
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n, err := b.buf.Write(p)
	if err != nil {
		return n, fmt.Errorf("writing to buffer: %w", err)
	}

	return n, nil
}
