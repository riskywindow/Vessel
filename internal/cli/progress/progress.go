// Package progress provides terminal progress indicators for long-running operations.
package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/briandowns/spinner"
)

// Spinner wraps a terminal spinner for long-running operations.
type Spinner struct {
	s       *spinner.Spinner
	message string
	mu      sync.Mutex
	writer  io.Writer
}

// NewSpinner creates a new spinner with a message.
func NewSpinner(message string) *Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " " + message
	s.Writer = os.Stderr
	return &Spinner{s: s, message: message, writer: os.Stderr}
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	s.s.Start()
}

// Stop stops the spinner.
func (s *Spinner) Stop() {
	s.s.Stop()
}

// Success stops the spinner and shows a success message.
func (s *Spinner) Success(msg string) {
	s.s.Stop()
	fmt.Fprintf(s.writer, "  \u2713 %s\n", msg)
}

// Fail stops the spinner and shows a failure message.
func (s *Spinner) Fail(msg string) {
	s.s.Stop()
	fmt.Fprintf(s.writer, "  \u2717 %s\n", msg)
}

// UpdateMessage changes the spinner message.
func (s *Spinner) UpdateMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.s.Suffix = " " + msg
}

// ProgressBar shows a progress bar for operations with known size.
type ProgressBar struct {
	total    int64
	current  int64
	width    int
	message  string
	mu       sync.Mutex
	lastDraw time.Time
	writer   io.Writer
}

// NewProgressBar creates a new progress bar.
func NewProgressBar(total int64, message string) *ProgressBar {
	return &ProgressBar{
		total:   total,
		width:   40,
		message: message,
		writer:  os.Stderr,
	}
}

// Add adds to the current progress.
func (p *ProgressBar) Add(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current += n

	// Rate limit redraws to avoid flickering.
	if time.Since(p.lastDraw) < 50*time.Millisecond {
		return
	}
	p.lastDraw = time.Now()
	p.draw()
}

// Set sets the current progress.
func (p *ProgressBar) Set(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = n
	p.draw()
}

// Finish completes the progress bar.
func (p *ProgressBar) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = p.total
	p.draw()
	fmt.Fprintln(p.writer)
}

// Percent returns the current completion percentage (0-100).
func (p *ProgressBar) Percent() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.total == 0 {
		return 0
	}
	pct := float64(p.current) / float64(p.total) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

func (p *ProgressBar) draw() {
	if p.total == 0 {
		return
	}
	pct := float64(p.current) / float64(p.total)
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(p.width))

	bar := ""
	for i := 0; i < p.width; i++ {
		if i < filled {
			bar += "\u2588"
		} else {
			bar += "\u2591"
		}
	}

	fmt.Fprintf(p.writer, "\r%s [%s] %.0f%%", p.message, bar, pct*100)
}

// ProgressWriter wraps an io.Writer and tracks bytes written as progress.
type ProgressWriter struct {
	w   io.Writer
	bar *ProgressBar
}

// NewProgressWriter creates a writer that updates a progress bar on each write.
func NewProgressWriter(w io.Writer, total int64, message string) *ProgressWriter {
	return &ProgressWriter{
		w:   w,
		bar: NewProgressBar(total, message),
	}
}

// Write implements io.Writer, forwarding bytes and updating the progress bar.
func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	pw.bar.Add(int64(n))
	return n, err
}

// Finish completes the underlying progress bar.
func (pw *ProgressWriter) Finish() {
	pw.bar.Finish()
}
