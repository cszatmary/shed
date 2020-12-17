package util

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// FileOrDirExists returns true if the given path exists on the OS filesystem.
func FileOrDirExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// Fatal allows for handling fatal conditions in a program.
// It allows printing error details and then terminating the process.
type Fatal struct {
	// ShowErrorDetail controls whether or not errors should be printed
	// with full detail when ExitErrf is called.
	ShowErrorDetail bool
	// ErrWriter is where errors are written.
	// If not set, it defaults to os.Stderr.
	ErrWriter io.Writer
	// ExitFunc is the function called to terminate the program.
	// This defaults to os.Exit and generally should not be set directly.
	ExitFunc func(code int)
}

// ExitErrf prints the given message and error to stderr then exits the program.
// It supports printf like formatting.
func (f Fatal) ExitErrf(err error, format string, a ...interface{}) {
	// Set default values. No need for f to be a pointer receiver because this function
	// terminates the process, so the modifications don't need to be persisted.
	if f.ErrWriter == nil {
		f.ErrWriter = os.Stderr
	}
	if f.ExitFunc == nil {
		f.ExitFunc = os.Exit
	}

	fmt.Fprintf(f.ErrWriter, format, a...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintln(f.ErrWriter)
	}

	if err != nil {
		if f.ShowErrorDetail {
			fmt.Fprintf(f.ErrWriter, "Error: %+v\n", err)
		} else {
			fmt.Fprintf(f.ErrWriter, "Error: %s\n", err)
		}
	}

	f.ExitFunc(1)
}

// Exitf prints the given message to stderr then exits the program.
// It supports printf like formatting.
func (f Fatal) Exitf(format string, a ...interface{}) {
	f.ExitErrf(nil, format, a...)
}
