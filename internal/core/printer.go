package core

import "io"

// Printer serializes output from parallel targets through a single goroutine.
// Targets send complete lines to the channel; the printer goroutine writes them
// to the output writer sequentially, guaranteeing line atomicity.
type Printer struct {
	ch   chan string
	done chan struct{}
	out  io.Writer
}

// NewPrinter creates a Printer that writes to out with the given channel buffer size.
// Starts the printer goroutine immediately.
func NewPrinter(out io.Writer, bufSize int) *Printer {
	p := &Printer{
		ch:   make(chan string, bufSize),
		done: make(chan struct{}),
		out:  out,
	}

	go p.run()

	return p
}

// Close drains remaining lines and waits for the printer goroutine to exit.
func (p *Printer) Close() {
	close(p.ch)
	<-p.done
}

// Send queues a line for printing. Blocks only if the channel buffer is full.
func (p *Printer) Send(line string) {
	p.ch <- line
}

func (p *Printer) run() {
	defer close(p.done)

	for line := range p.ch {
		_, _ = io.WriteString(p.out, line)
	}
}
