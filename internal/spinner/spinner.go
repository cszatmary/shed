package spinner

import (
	"bytes"
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
	// msg written on each frame
	msg string
	// total number of items
	count int
	// number of items completed
	completed int
	maxMsgLen int
	// buffer to keep track of message to write to w
	// these will be written on each call of erase
	// a list of debug messages that will be written
	// to debugw on the next frame
	msgBuf      *bytes.Buffer
	persistMsgs bool
}

// Options allows for customization of a spinner.
type Options struct {
	// Interval is how often the spinner updates. This controls the speed of the spinner.
	// The default value is 100ms.
	Interval time.Duration
	// Out is where the spinner is written. The default value is os.Stderr.
	Out io.Writer
	// Message is the text written after the spinner.
	Message string
	// Count is the total amount of items to track the progress of.
	Count int
	// MaxMessageLength is the max length of the message that is written. If the message is
	// longer then this length it will be truncated. The default max length is 80.
	MaxMessageLength int
	// PersistMessages is whether or not messages should be persisted to Out when the message
	// is updated. By default messages are not persisted and are replaced.
	PersistMessages bool
}

// New creates a new spinner instance using the given options.
func New(opts Options) *Spinner {
	if opts.Interval == 0 {
		opts.Interval = 100 * time.Millisecond
	}
	if opts.Out == nil {
		opts.Out = os.Stderr
	}
	if opts.MaxMessageLength == 0 {
		opts.MaxMessageLength = 80
	}
	s := &Spinner{
		interval:    opts.Interval,
		out:         opts.Out,
		mu:          &sync.RWMutex{},
		stopChan:    make(chan struct{}, 1),
		active:      false,
		count:       opts.Count,
		maxMsgLen:   opts.MaxMessageLength,
		msgBuf:      &bytes.Buffer{},
		persistMsgs: opts.PersistMessages,
	}
	s.setMsg(opts.Message)
	return s
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
	s.stopChan <- struct{}{}
	// Persist last msg before we do the final erase.
	// Need to do this manually since we aren't using setMsg
	s.persistMsg()
	s.erase()
}

// Inc increments the progress of the spinner. If the spinner
// has already reached full progress, Inc does nothing.
func (s *Spinner) Inc() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.completed >= s.count {
		return
	}
	s.completed++
}

// UpdateMessage changes the current message being shown by the spinner.
func (s *Spinner) UpdateMessage(m string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setMsg(m)
}

// setMsg sets the spinner message to m. If m is longer then s.maxMsgLen it will
// be truncated. If m is empty, setMsg will do nothing.
// The caller must already hold s.lock.
func (s *Spinner) setMsg(m string) {
	if m == "" {
		return
	}
	// Make sure there is no trailing newline or it will mess up the spinner
	if m[len(m)-1] == '\n' {
		m = m[:len(m)-1]
	}
	// Truncate msg if it's too long
	const ellipses = "..."
	if len(m)-len(ellipses) > s.maxMsgLen {
		m = m[:s.maxMsgLen-len(ellipses)] + ellipses
	}
	// Make sure message has a leading space to pad between it and the spinner icon
	if m[0] != ' ' {
		m = " " + m
	}
	s.persistMsg()
	s.msg = m
}

// persistMsg will handle persisting msg if required. The caller must already hold s.lock.
func (s *Spinner) persistMsg() {
	if !s.persistMsgs || s.msg == "" {
		return
	}
	// The message should always be written on it's own line so make sure there is a newline before
	if s.msgBuf.Len() > 0 && s.msgBuf.Bytes()[s.msgBuf.Len()-1] != '\n' {
		s.msgBuf.WriteByte('\n')
	}
	// Drop first char since it's a space
	s.msgBuf.WriteString(s.msg[1:])
	s.msgBuf.WriteByte('\n')
}

// Write writes p to the spinner's writer after the current frame has been erased.
// Write will always immediately return successfully as p is first written to an internal buffer.
// The actual writing of the data to the spinner's writer will not occur until the appropriate time
// during the spinner animation.
//
// Write will add a newline to the end of p in order to ensure that it does not interfere with
// the spinner animation.
func (s *Spinner) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.msgBuf.Write(p)
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

				line := fmt.Sprintf("\r%s%s ", frames[i], s.msg)
				if s.count > 1 {
					line += fmt.Sprintf("(%d/%d) ", s.completed, s.count)
				}
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
	} else {
		// "\033[K" for macOS Terminal
		for _, c := range []string{"\b", "\127", "\b", "\033[K"} {
			fmt.Fprint(s.out, strings.Repeat(c, n))
		}
		// erases to end of line
		fmt.Fprintf(s.out, "\r\033[K")
	}
	if s.msgBuf.Len() > 0 {
		if s.msgBuf.Bytes()[s.msgBuf.Len()-1] != '\n' {
			s.msgBuf.WriteByte('\n')
		}
		// Ignore error because there's nothing we can really do about it
		_, _ = s.msgBuf.WriteTo(s.out)
	}
	s.lastOutput = ""
}
