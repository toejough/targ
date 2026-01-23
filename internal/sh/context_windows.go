//go:build windows

package internal

import (
	"context"
	"os/exec"
)

// KillProcessGroup kills the process on Windows.
// Note: This may not kill child processes - Job Objects would be needed
// for full process tree cleanup.
func KillProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// SetProcGroup is a no-op on Windows.
func SetProcGroup(cmd *exec.Cmd) {
	// Windows doesn't use process groups in the same way as Unix.
	// Job Objects would be needed for full process tree management.
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
		KillProcessGroup(cmd)
		<-done
		return ctx.Err()
	}
}
