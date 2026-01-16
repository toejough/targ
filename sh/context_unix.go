//go:build !windows

package sh

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

// killProcessGroup kills the process and all its children.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		// Kill the entire process group (negative PID)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// runWithContext runs a command with context cancellation support.
// On Unix, it uses process groups to kill the entire process tree.
func runWithContext(ctx context.Context, cmd *exec.Cmd) error {
	setProcGroup(cmd)

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
		killProcessGroup(cmd)
		<-done

		return fmt.Errorf("command cancelled: %w", ctx.Err())
	}
}

// setProcGroup configures the command to run in its own process group.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
