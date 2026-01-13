//go:build windows

package sh

import (
	"context"
	"os/exec"
)

// killProcessGroup kills the process on Windows.
// Note: This may not kill child processes - Job Objects would be needed
// for full process tree cleanup.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// runWithContext runs a command with context cancellation support.
// On Windows, this uses basic process termination.
// Note: Child processes may not be terminated - for full process tree
// cleanup, consider using Job Objects in a future enhancement.
func runWithContext(ctx context.Context, cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		killProcessGroup(cmd)
		<-done
		return ctx.Err()
	}
}

// setProcGroup is a no-op on Windows.
func setProcGroup(cmd *exec.Cmd) {
	// Windows doesn't use process groups in the same way as Unix.
	// Job Objects would be needed for full process tree management.
}
