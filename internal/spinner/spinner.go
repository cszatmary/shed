package spinner

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var frames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner represents the state of the spinner.
type Spinner struct {
	interval time.Duration
	out      io.Writer
	mu       *sync.RWMutex
	// stopChan is used to stop the spinner
	stopChan chan struct{}
	active   bool
	// last string written to out
	lastOutput string
	// Prefix is the text written before the spinner
	Prefix string
	// Suffix is the text written after the spinner
	Suffix string
}

// Options allows for customization of a spinner.
type Options struct {
	// Interval is how often the spinner updates. This controls the speed of the spinner.
	// The default value is 100ms.
	Interval time.Duration
	// Out is where the spinner is written. The default value is os.Stderr.
	Out io.Writer
	// Suffix is the text written after the spinner
	Suffix string
}

// New creates a new spinner instance using the given options.
func New(opts Options) *Spinner {
	if opts.Interval == 0 {
		opts.Interval = 100 * time.Millisecond
	}
	if opts.Out == nil {
		opts.Out = os.Stderr
	}

	return &Spinner{
		interval: opts.Interval,
		out:      opts.Out,
		mu:       &sync.RWMutex{},
		stopChan: make(chan struct{}, 1),
		active:   false,
		Suffix:   opts.Suffix,
	}
}

// Start will start the spinner.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.mu.Unlock()
	go s.run()
}

// Stop stops the spinner if it is currently running.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return
	}

	s.active = false
	s.erase()
	s.stopChan <- struct{}{}
}

// run runs the spinner. It should be called in a separate goroutine because
// it will run forever until it receives a value on s.stopChan.
func (s *Spinner) run() {
	for {
		for i := 0; i < len(frames); i++ {
			select {
			case <-s.stopChan:
				return
			default:
				s.mu.Lock()
				if !s.active {
					s.mu.Unlock()
					return
				}
				s.erase()

				line := fmt.Sprintf("\r%s%s%s ", s.Prefix, frames[i], s.Suffix)
				fmt.Fprint(s.out, line)
				s.lastOutput = line
				d := s.interval

				s.mu.Unlock()
				time.Sleep(d)
			}
		}
	}
}

// erase deletes written characters. The caller must already hold s.lock.
func (s *Spinner) erase() {
	n := utf8.RuneCountInString(s.lastOutput)
	if runtime.GOOS == "windows" {
		clearString := "\r" + strings.Repeat(" ", n) + "\r"
		fmt.Fprint(s.out, clearString)
		s.lastOutput = ""
		return
	}

	// "\033[K" for macOS Terminal
	for _, c := range []string{"\b", "\127", "\b", "\033[K"} {
		fmt.Fprint(s.out, strings.Repeat(c, n))
	}
	// erases to end of line
	fmt.Fprintf(s.out, "\r\033[K")
	s.lastOutput = ""
}
