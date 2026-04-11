package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/mattn/go-isatty"
)

// StartSpinner writes an animated spinner to stderr while work is in progress.
// Returns a stop function; call it when work is done to clear the spinner line.
// A no-op is returned when stderr is not an interactive terminal so piped and
// non-interactive invocations are unaffected.
func StartSpinner(label string) func() {
	if !isatty.IsTerminal(os.Stderr.Fd()) && !isatty.IsCygwinTerminal(os.Stderr.Fd()) {
		return func() {}
	}

	sp := spinner.MiniDot
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(sp.FPS)
		defer ticker.Stop()
		i := 0
		printFrame := func() {
			frame := StyleMuted.Render(sp.Frames[i%len(sp.Frames)])
			text := StyleMuted.Render(label)
			fmt.Fprintf(os.Stderr, "\r%s %s", frame, text)
		}
		printFrame()
		for {
			select {
			case <-stop:
				clearWidth := len(label) + 4
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", clearWidth))
				return
			case <-ticker.C:
				i++
				printFrame()
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}
