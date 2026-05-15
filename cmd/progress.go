package cmd

import (
	"fmt"
	"strings"
	"time"
)

// ── Progress bar ─────────────────────────────────────────────────

// progressBarWidth is the number of block characters in every progress bar.
const progressBarWidth = 24

// progressBar renders an in-place terminal progress bar using \r.
// Create with newProgressBar, call update to redraw, and done to finish.
type progressBar struct {
	total int
	width int
	start time.Time
}

func newProgressBar(total int) *progressBar {
	return &progressBar{total: total, width: progressBarWidth, start: time.Now()}
}

// renderBar returns a [████░░░░] string representing current/total progress.
// Rounding is applied so early progress is visible even at low percentages.
func renderBar(current, total, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("░", width) + "]"
	}
	// Use rounding division to avoid premature completion or invisible early progress.
	filled := (width*current + total/2) / total
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

// update redraws the progress bar at position current.
// It uses \r and the ANSI erase-line sequence to overwrite the current line.
func (p *progressBar) update(current int, label string) {
	bar := renderBar(current, p.total, p.width)
	elapsed := time.Since(p.start).Truncate(time.Second)
	fmt.Printf("\r\033[2K   %s %2d/%-2d  %s (%s elapsed)", bar, current, p.total, label, elapsed)
}

// clear erases the progress bar line and returns the cursor to column 0.
func (p *progressBar) clear() {
	fmt.Printf("\r\033[2K")
}

// done clears the bar and prints a completion message.
func (p *progressBar) done(msg string) {
	p.clear()
	fmt.Println("   " + msg)
}

// countdown shows a progress bar that ticks every second for n seconds,
// then clears the bar. Used for deterministic sleeps where the duration
// is known upfront (e.g. "Giving MySQL time to initialize").
func countdown(n int, label string) {
	bar := newProgressBar(n)
	for i := 0; i <= n; i++ {
		bar.update(i, label)
		if i < n {
			time.Sleep(time.Second)
		}
	}
	bar.clear()
}

// ── Spinner ───────────────────────────────────────────────────────

// spinChars is a rotating set of characters for indeterminate waits.
var spinChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinWhile runs fn in a goroutine and displays a spinner until it completes.
// On success it prints the elapsed time; on failure it clears the spinner line
// so the caller's error message lands on a clean line.
// Returns fn's error.
func spinWhile(label string, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()

	start := time.Now()
	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			elapsed := time.Since(start).Truncate(time.Second)
			fmt.Printf("\r\033[2K")
			if err == nil {
				fmt.Printf("   ✅ Done (%s)\n", elapsed)
			}
			return err
		case <-ticker.C:
			elapsed := time.Since(start).Truncate(time.Second)
			fmt.Printf("\r\033[2K   %s %s (%s elapsed)", spinChars[i%len(spinChars)], label, elapsed)
			i++
		}
	}
}
