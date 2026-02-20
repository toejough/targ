package core

import "strings"

// PrefixWriter is an io.Writer that buffers partial lines and sends
// complete, prefixed lines to a Printer. Used as stdout/stderr for
// shell commands in parallel mode.
type PrefixWriter struct {
	prefix  string
	printer *Printer
	buf     strings.Builder
}

// NewPrefixWriter creates a PrefixWriter that prefixes each line and sends to printer.
func NewPrefixWriter(prefix string, printer *Printer) *PrefixWriter {
	return &PrefixWriter{
		prefix:  prefix,
		printer: printer,
	}
}

// Flush sends any remaining partial line to the Printer with a trailing newline.
func (w *PrefixWriter) Flush() {
	if w.buf.Len() > 0 {
		w.printer.Send(w.prefix + w.buf.String() + "\n")
		w.buf.Reset()
	}
}

// Write implements io.Writer. Buffers partial lines, sends complete lines to Printer.
func (w *PrefixWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.buf.Write(p)

	for {
		line, ok := w.extractLine()
		if !ok {
			break
		}

		w.printer.Send(w.prefix + line + "\n")
	}

	return n, nil
}

// extractLine extracts one complete line (up to \n) from the buffer.
// Returns false if no complete line is available.
func (w *PrefixWriter) extractLine() (string, bool) {
	s := w.buf.String()
	line, rest, ok := strings.Cut(s, "\n")

	if !ok {
		return "", false
	}

	w.buf.Reset()
	w.buf.WriteString(rest)

	return line, true
}
