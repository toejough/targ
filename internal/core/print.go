package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// FormatPrefix creates a right-padded prefix like "[build] " or "[test]  ".
// maxLen is the length of the longest target name in the parallel group.
// If maxLen is less than len(name), no extra padding is added.
func FormatPrefix(name string, maxLen int) string {
	padding := maxLen - len(name)
	if padding <= 0 {
		return "[" + name + "] "
	}

	return "[" + name + "] " + strings.Repeat(" ", padding)
}

// Print writes output, prefixed with [name] if running in parallel mode.
func Print(ctx context.Context, args ...any) {
	text := fmt.Sprint(args...)
	info, ok := GetExecInfo(ctx)

	if !ok || !info.Parallel || info.Printer == nil {
		_, _ = fmt.Fprint(printOutput, text)
		return
	}

	prefix := FormatPrefix(info.Name, info.MaxNameLen)
	sendPrefixed(info.Printer, prefix, text)
}

// Printf writes formatted output, prefixed with [name] if running in parallel mode.
func Printf(ctx context.Context, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	info, ok := GetExecInfo(ctx)

	if !ok || !info.Parallel || info.Printer == nil {
		_, _ = fmt.Fprint(printOutput, text)
		return
	}

	prefix := FormatPrefix(info.Name, info.MaxNameLen)
	sendPrefixed(info.Printer, prefix, text)
}

// SetPrintOutput sets the default output writer for serial mode.
// Passing nil resets to os.Stdout.
func SetPrintOutput(w io.Writer) {
	if w == nil {
		printOutput = os.Stdout
	} else {
		printOutput = w
	}
}

// unexported variables.
var (
	printOutput io.Writer = os.Stdout //nolint:gochecknoglobals // intentional test seam
)

func sendPrefixed(p *Printer, prefix, text string) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		// Skip empty last element from trailing \n
		if i == len(lines)-1 && line == "" {
			continue
		}

		p.Send(prefix + line + "\n")
	}
}
