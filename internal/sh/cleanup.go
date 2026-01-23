// Package internal provides shell execution utilities.
package internal

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Exported variables.
var (
	CleanupEnabled bool
	CleanupMu      sync.Mutex
	// KillProcessFunc is injectable for testing
	KillProcessFunc = func(*os.Process) {}
	RunningProcs    = make(map[*os.Process]struct{})
)

// EnableCleanup enables automatic cleanup of child processes on SIGINT/SIGTERM.
func EnableCleanup() {
	CleanupMu.Lock()
	defer CleanupMu.Unlock()

	if CleanupEnabled {
		return
	}

	CleanupEnabled = true

	if !signalInstalled {
		signalInstalled = true
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh
			KillAllProcesses()
			os.Exit(exitCodeSigInt)
		}()
	}
}

// KillAllProcesses kills all tracked processes.
func KillAllProcesses() {
	CleanupMu.Lock()

	procs := make([]*os.Process, 0, len(RunningProcs))
	for p := range RunningProcs {
		procs = append(procs, p)
	}

	CleanupMu.Unlock()

	for _, p := range procs {
		KillProcessFunc(p)
	}
}

// RegisterProcess adds a process to the cleanup list.
func RegisterProcess(p *os.Process) {
	CleanupMu.Lock()
	defer CleanupMu.Unlock()

	if CleanupEnabled {
		RunningProcs[p] = struct{}{}
	}
}

// UnregisterProcess removes a process from the cleanup list.
func UnregisterProcess(p *os.Process) {
	CleanupMu.Lock()
	defer CleanupMu.Unlock()

	delete(RunningProcs, p)
}

// unexported constants.
const (
	exitCodeSigInt = 130
)

// unexported variables.
var (
	signalInstalled bool
)
