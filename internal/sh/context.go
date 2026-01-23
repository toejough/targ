package internal

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// OutputContext executes a command and returns combined output, with context support.
// When ctx is cancelled, the process and all its children are killed.
func OutputContext(
	ctx context.Context,
	name string,
	args []string,
	stdin io.Reader,
) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin

	// Capture combined output
	var buf SafeBuffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	done := make(chan error, 1)

	SetProcGroup(cmd)

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
		KillProcessGroup(cmd)
		<-done

		return buf.String(), fmt.Errorf("command cancelled: %w", ctx.Err())
	}
}

// RunContextV runs a command with context support, printing it first.
func RunContextV(ctx context.Context, env *ShellEnv, name string, args []string) error {
	if env == nil {
		env = DefaultShellEnv()
	}

	_, _ = fmt.Fprintln(env.Stdout, "+", FormatCommand(name, args))

	return RunContextWithIO(ctx, env, name, args)
}

// RunContextWithIO runs a command with context support and custom IO.
func RunContextWithIO(ctx context.Context, env *ShellEnv, name string, args []string) error {
	if env == nil {
		env = DefaultShellEnv()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = env.Stdout
	cmd.Stderr = env.Stderr
	cmd.Stdin = env.Stdin

	return runWithContext(ctx, cmd)
}
