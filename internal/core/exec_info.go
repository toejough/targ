package core

import "context"

// ExecInfo carries execution metadata through context.
type ExecInfo struct {
	Parallel   bool     // true if running in a parallel group
	Name       string   // target name, used as output prefix
	MaxNameLen int      // longest target name in the parallel group, for prefix padding
	Printer    *Printer // the printer for this parallel group (avoids global state races)
}

// GetExecInfo retrieves ExecInfo from context.
// Returns false if no ExecInfo is present (serial execution).
func GetExecInfo(ctx context.Context) (ExecInfo, bool) {
	info, ok := ctx.Value(execInfoKey{}).(ExecInfo)
	return info, ok
}

// WithExecInfo returns a new context carrying the given ExecInfo.
func WithExecInfo(ctx context.Context, info ExecInfo) context.Context {
	return context.WithValue(ctx, execInfoKey{}, info)
}

type execInfoKey struct{}
