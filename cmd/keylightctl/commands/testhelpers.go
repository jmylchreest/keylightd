package commands

import (
	"bytes"
	"io"
	"os"
	"regexp"

	"github.com/pterm/pterm"
)

// captureStdout captures stdout during the execution of f, disables pterm color, and strips ANSI codes from the output.
func captureStdout(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Save original pterm settings and default table writer
	oldPrintColor := pterm.PrintColor
	oldOutput := pterm.Output
	oldDefaultTableWriter := pterm.DefaultTable.Writer

	pterm.PrintColor = false
	pterm.Output = true
	pterm.DefaultTable.Writer = w

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outC <- buf.String()
	}()

	f()

	w.Close()
	os.Stdout = oldStdout

	// Restore pterm
	pterm.PrintColor = oldPrintColor
	pterm.Output = oldOutput
	pterm.DefaultTable.Writer = oldDefaultTableWriter

	out := <-outC

	// Strip ANSI escape codes
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiRegex.ReplaceAllString(out, "")
}

// ClientContextKey is used for storing the client in context for tests and commands.
var ClientContextKey = &struct{}{}
