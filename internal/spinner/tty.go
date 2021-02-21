package spinner

import (
	"fmt"

	"github.com/mattn/go-isatty"
)

// TTYSpinner is a wrapper over a spinner that handles whether or not
// the spinner's output is a tty. If out is a tty, it functions the same
// as a Spinner. If out is not a tty, then the spinner will simply
// write messages to it without the spinner animation.
type TTYSpinner struct {
	*Spinner
	isaTTY bool
}

type fder interface {
	Fd() uintptr
}

// NewTTY creates a new TTYSpinner instance.
func NewTTY(opts Options) *TTYSpinner {
	s := &TTYSpinner{Spinner: New(opts)}
	if f, ok := s.out.(fder); ok {
		s.isaTTY = isatty.IsTerminal(f.Fd())
	}
	if !s.isaTTY {
		// Persisting messages isn't allowed if not a tty, since messages
		// are not erased, and are by definition persisted.
		s.Spinner.persistMsgs = false
	}
	return s
}

// Start starts the spinner if out is a tty, otherwise it writes the message to out directly.
func (s *TTYSpinner) Start() {
	if s.isaTTY {
		s.Spinner.Start()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeMsg()
}

// UpdateMessage either updates the spinner message, or writes m directy to out
// depending on whether or not out is a tty.
func (s *TTYSpinner) UpdateMessage(m string) {
	if s.isaTTY {
		s.Spinner.UpdateMessage(m)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setMsg(m)
	s.writeMsg()
}

func (s *TTYSpinner) Write(p []byte) (int, error) {
	if s.isaTTY {
		return s.Spinner.Write(p)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.out.Write(p)
	if err != nil {
		return n, err
	}
	if p[len(p)-1] == '\n' {
		return n, nil
	}
	m, err := s.out.Write([]byte{'\n'})
	return n + m, err
}

func (s *TTYSpinner) writeMsg() {
	if s.msg == "" {
		return
	}
	// First char is always a space
	fmt.Fprintln(s.out, s.msg[1:])
}
