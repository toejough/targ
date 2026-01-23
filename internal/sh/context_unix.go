//go:build !windows

package internal

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

// KillProcessGroup kills the process and all its children.
func KillProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		// Kill the entire process group (negative PID)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// SetProcGroup configures the command to run in its own process group.
func SetProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// runWithContext runs a command with context cancellation support.
// On Unix, it uses process groups to kill the entire process tree.
func runWithContext(ctx context.Context, cmd *exec.Cmd) error {
	SetProcGroup(cmd)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("starting command: %w", err)
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

		return fmt.Errorf("command cancelled: %w", ctx.Err())
	}
}
