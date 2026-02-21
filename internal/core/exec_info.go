package core

import (
	"context"
	"io"
	"os"
)

// ExecInfo carries execution metadata through context.
type ExecInfo struct {
	Parallel   bool      // true if running in a parallel group
	Name       string    // target name, used as output prefix
	MaxNameLen int       // longest target name in the parallel group, for prefix padding
	Printer    *Printer  // the printer for this parallel group (avoids global state races)
	Output     io.Writer // default output writer (nil means os.Stdout)
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

// outputFromContext returns the output writer from the context's ExecInfo,
// falling back to os.Stdout if not set.
func outputFromContext(ctx context.Context) io.Writer {
	info, ok := ctx.Value(execInfoKey{}).(ExecInfo)
	if ok && info.Output != nil {
		return info.Output
	}

	return os.Stdout
}
