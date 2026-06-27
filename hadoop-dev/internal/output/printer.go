package output

import (
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/term"
)

const (
	reset  = "\033[0m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	dim    = "\033[2m"
	eraseL = "\033[2K"
)

// Printer is the single place all terminal progress output lives.
// Data output (tables, --format, --quiet) goes via cmd.OutOrStdout() separately.
type Printer struct {
	Verbose bool
	NoColor bool
	isTTY   bool
	w       io.Writer // stderr — progress never pollutes stdout
	midLine bool      // true when Begin wrote a partial line with no trailing \n
}

func New(verbose, noColor bool) *Printer {
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	}
	return &Printer{
		Verbose: verbose,
		NoColor: noColor,
		isTTY:   term.IsTerminal(int(os.Stderr.Fd())),
		w:       os.Stderr,
	}
}

func (p *Printer) c(code, s string) string {
	if p.NoColor {
		return s
	}
	return code + s + reset
}

// Header prints a title block (blank line, message, blank line).
func (p *Printer) Header(msg string) {
	p.endLine()
	fmt.Fprintf(p.w, "\n  %s\n\n", msg)
}

// Begin marks a step as in-progress. In non-verbose TTY mode it writes
// the label without a trailing newline so Done can overwrite it.
func (p *Printer) Begin(label string) {
	text := p.c(dim, label+"...")
	if p.Verbose {
		fmt.Fprintf(p.w, "  %s\n", text)
		p.midLine = false
		return
	}
	if p.isTTY {
		fmt.Fprintf(p.w, "  %-48s", text)
		p.midLine = true
		return
	}
	// non-TTY non-verbose: defer until Done
}

// Done completes the current step. detail is optional (e.g. "2/2").
func (p *Printer) Done(label, detail string) {
	check := p.c(green, "✓")
	suffix := "done"
	if detail != "" {
		suffix = "done (" + detail + ")"
	}

	if p.Verbose {
		fmt.Fprintf(p.w, "  %s %s\n", check, suffix)
		p.midLine = false
		return
	}
	if p.isTTY && p.midLine {
		fmt.Fprintf(p.w, "\r%s  %-48s%s %s\n", eraseL, p.c(dim, label+"..."), check, suffix)
		p.midLine = false
		return
	}
	// non-TTY: print the full collapsed line
	fmt.Fprintf(p.w, "  %-48s%s %s\n", p.c(dim, label+"..."), check, suffix)
}

// Fail marks the current step as failed.
func (p *Printer) Fail(label string) {
	cross := p.c(red, "✗")
	if p.isTTY && p.midLine {
		fmt.Fprintf(p.w, "\r%s  %-48s%s failed\n", eraseL, p.c(dim, label+"..."), cross)
		p.midLine = false
		return
	}
	fmt.Fprintf(p.w, "  %s %s failed\n", cross, label)
}

// Warn prints a ⚠ warning that is always visible (not gated on verbose).
func (p *Printer) Warn(msg string) {
	p.endLine()
	fmt.Fprintf(p.w, "  %s %s\n", p.c(yellow, "⚠"), msg)
}

// Sub prints an indented sub-step. Only shown in verbose mode.
func (p *Printer) Sub(msg string) {
	if !p.Verbose {
		return
	}
	fmt.Fprintf(p.w, "    %s\n", msg)
}

// Progress overwrites the current in-progress line with a count update.
// In non-verbose mode it uses \r to stay on one line (TTY only).
// In non-TTY or verbose mode it prints a new indented line.
func (p *Printer) Progress(label, detail string) {
	if p.isTTY && !p.Verbose {
		fmt.Fprintf(p.w, "\r%s  %-48s", eraseL, p.c(dim, label+" ("+detail+")..."))
		p.midLine = true
		return
	}
	if p.Verbose {
		fmt.Fprintf(p.w, "    %s\n", detail)
	}
	// non-TTY non-verbose: silence (avoid flooding CI logs)
}

// Elapsed prints the final "Cluster ready in X.Xs" line.
func (p *Printer) Elapsed(d time.Duration) {
	p.endLine()
	fmt.Fprintf(p.w, "\n  %s\n", p.c(green, fmt.Sprintf("Cluster ready in %.1fs", d.Seconds())))
}

// endLine ensures we are not mid-line before writing something that needs its own line.
func (p *Printer) endLine() {
	if p.midLine {
		fmt.Fprintln(p.w)
		p.midLine = false
	}
}
