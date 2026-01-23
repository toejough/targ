// Package internal provides shell execution utilities.
package internal

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// CleanupManager manages process cleanup on signals.
type CleanupManager struct {
	mu              sync.Mutex
	enabled         bool
	signalInstalled bool
	runningProcs    map[*os.Process]struct{}
	killFunc        func(*os.Process)
}

// NewCleanupManager creates a new CleanupManager with the given kill function.
func NewCleanupManager(killFunc func(*os.Process)) *CleanupManager {
	return &CleanupManager{
		runningProcs: make(map[*os.Process]struct{}),
		killFunc:     killFunc,
	}
}

// EnableCleanup enables automatic cleanup of child processes on SIGINT/SIGTERM.
func (m *CleanupManager) EnableCleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.enabled {
		return
	}

	m.enabled = true

	if !m.signalInstalled {
		m.signalInstalled = true
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh
			m.KillAllProcesses()
			os.Exit(exitCodeSigInt)
		}()
	}
}

// KillAllProcesses kills all tracked processes.
func (m *CleanupManager) KillAllProcesses() {
	m.mu.Lock()

	procs := make([]*os.Process, 0, len(m.runningProcs))
	for p := range m.runningProcs {
		procs = append(procs, p)
	}

	m.mu.Unlock()

	for _, p := range procs {
		m.killFunc(p)
	}
}

// RegisterProcess adds a process to the cleanup list.
func (m *CleanupManager) RegisterProcess(p *os.Process) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.enabled {
		m.runningProcs[p] = struct{}{}
	}
}

// UnregisterProcess removes a process from the cleanup list.
func (m *CleanupManager) UnregisterProcess(p *os.Process) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.runningProcs, p)
}

// unexported constants.
const (
	exitCodeSigInt = 130
)
