package sh

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// EnableCleanup enables automatic cleanup of child processes on SIGINT/SIGTERM.
// Call this once at program startup to ensure Ctrl-C kills all spawned processes.
func EnableCleanup() {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()

	if cleanupEnabled {
		return
	}
	cleanupEnabled = true

	if !signalInstalled {
		signalInstalled = true
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			killAllProcesses()
			os.Exit(130) // 128 + SIGINT(2)
		}()
	}
}

// unexported variables.
var (
	cleanupEnabled  bool
	cleanupMu       sync.Mutex
	runningProcs    = make(map[*os.Process]struct{})
	signalInstalled bool
)

// killAllProcesses kills all tracked processes.
func killAllProcesses() {
	cleanupMu.Lock()
	procs := make([]*os.Process, 0, len(runningProcs))
	for p := range runningProcs {
		procs = append(procs, p)
	}
	cleanupMu.Unlock()

	for _, p := range procs {
		killProcess(p)
	}
}

// registerProcess adds a process to the cleanup list.
func registerProcess(p *os.Process) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	if cleanupEnabled {
		runningProcs[p] = struct{}{}
	}
}

// unregisterProcess removes a process from the cleanup list.
func unregisterProcess(p *os.Process) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	delete(runningProcs, p)
}
