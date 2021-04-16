package spinner_test

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getshiphub/shed/internal/spinner"
)

type syncBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(data)
}

func TestSpinner(t *testing.T) {
	out := &syncBuffer{}
	s := spinner.New(spinner.Options{
		Interval: 100 * time.Millisecond,
		Out:      out,
	})

	s.Start()
	time.Sleep(500 * time.Millisecond)
	s.Stop()

	// wait a bit because the spinner still has to erase before stopping
	time.Sleep(100 * time.Millisecond)
	got := out.String()
	// Should be 5 frames since we ran for 500ms and it's 1 frame per 100ms
	want := "⠋⠙⠹⠸⠼"
	// Check that frames were actually written
	if !containsAll(got, want) {
		t.Errorf("got %q, want to contain all %q", got, "")
	}
}

func TestSpinnerCounter(t *testing.T) {
	const count = 3
	out := &syncBuffer{}
	s := spinner.New(spinner.Options{
		Interval: 10 * time.Millisecond,
		Out:      out,
		Message:  "Cloning repos",
		Count:    count,
	})
	s.Start()
	for i := 1; i < count+1; i++ {
		time.Sleep(15 * time.Millisecond)
		s.Inc()
	}
	s.Stop()

	// wait a bit because the spinner still has to erase before stopping
	time.Sleep(25 * time.Millisecond)
	got := out.String()

	// Should be at least 4 frames
	wantFrames := "⠋⠙⠹⠸"
	if !containsAll(got, wantFrames) {
		t.Errorf("got %q, want to contain all %q", got, wantFrames)
	}

	// Asserting the output is a bit tricky because of the special erase codes written
	// to erase the text in terminals.
	// Just make sure that the text we expect appears in the output
	wantMsgs := []string{
		"Cloning repos (0/3)",
		"Cloning repos (1/3)",
		"Cloning repos (2/3)",
	}
	for _, wantMsg := range wantMsgs {
		if !strings.Contains(got, wantMsg) {
			t.Errorf("got %q, want to contain %q", got, wantMsg)
		}
	}
}

func TestSpinnerUpdateMessage(t *testing.T) {
	out := &syncBuffer{}
	s := spinner.New(spinner.Options{
		Interval: 10 * time.Millisecond,
		Out:      out,
		Message:  "Cloning repos",
	})
	s.Start()
	time.Sleep(15 * time.Millisecond)
	s.UpdateMessage("Updating repos")
	time.Sleep(15 * time.Millisecond)
	s.UpdateMessage("Cleaning up")
	time.Sleep(15 * time.Millisecond)
	s.Stop()

	// wait a bit because the spinner still has to erase before stopping
	time.Sleep(25 * time.Millisecond)
	got := out.String()

	// Should be at least 4 frames
	wantFrames := "⠋⠙⠹⠸"
	if !containsAll(got, wantFrames) {
		t.Errorf("got %q, want to contain all %q", got, wantFrames)
	}

	wantMsgs := []string{
		"Cloning repos",
		"Updating repos",
		"Cleaning up",
	}
	for _, wantMsg := range wantMsgs {
		if !strings.Contains(got, wantMsg) {
			t.Errorf("got %q, want to contain %q", got, wantMsg)
		}
	}
}

func TestSpinnerPersist(t *testing.T) {
	const count = 3
	out := &syncBuffer{}
	s := spinner.New(spinner.Options{
		Interval:        10 * time.Millisecond,
		Out:             out,
		Message:         "Cloning repos",
		Count:           count,
		PersistMessages: true,
	})
	s.Start()
	for i := 1; i < count+1; i++ {
		time.Sleep(15 * time.Millisecond)
		s.Inc()
		s.UpdateMessage(fmt.Sprintf("Cloned repo %d", i))
	}
	s.Stop()

	// wait a bit because the spinner still has to erase before stopping
	time.Sleep(50 * time.Millisecond)
	got := out.String()

	// Should be at least 4 frames
	wantFrames := "⠋⠙⠹⠸"
	if !containsAll(got, wantFrames) {
		t.Errorf("got %q, want to contain all %q", got, wantFrames)
	}

	wantMsgs := []string{
		"Cloning repos (0/3)",
		"Cloning repos\n",
		"Cloned repo 1 (1/3)",
		"Cloned repo 1\n",
		"Cloned repo 2 (2/3)",
		"Cloned repo 2\n",
		"Cloned repo 3\n",
	}
	for _, wantMsg := range wantMsgs {
		if !strings.Contains(got, wantMsg) {
			t.Errorf("got %q, want to contain %q", got, wantMsg)
		}
	}
}

func TestSpinnerWrite(t *testing.T) {
	const count = 3
	out := &syncBuffer{}
	s := spinner.New(spinner.Options{
		Interval: 10 * time.Millisecond,
		Out:      out,
		Message:  "Cloning repos",
		Count:    count,
	})
	s.Start()
	for i := 1; i < count+1; i++ {
		time.Sleep(15 * time.Millisecond)
		s.Inc()
		fmt.Fprintf(s, "debug stuff %d", i)
	}
	s.Stop()

	// wait a bit because the spinner still has to erase before stopping
	time.Sleep(50 * time.Millisecond)
	got := out.String()

	// Should be at least 4 frames
	wantFrames := "⠋⠙⠹⠸"
	if !containsAll(got, wantFrames) {
		t.Errorf("got %q, want to contain all %q", got, wantFrames)
	}

	wantMsgs := []string{
		"debug stuff 1\n",
		"debug stuff 2\n",
		"debug stuff 3\n",
		"Cloning repos",
	}
	for _, wantMsg := range wantMsgs {
		if !strings.Contains(got, wantMsg) {
			t.Errorf("got %q, want to contain %q", got, wantMsg)
		}
	}
}

func TestSpinnerMaxMessageLength(t *testing.T) {
	out := &syncBuffer{}
	s := spinner.New(spinner.Options{
		Interval:         10 * time.Millisecond,
		Out:              out,
		Message:          "This message is way too long",
		MaxMessageLength: 15,
	})
	s.Start()
	time.Sleep(25 * time.Millisecond)
	s.Stop()

	// wait a bit because the spinner still has to erase before stopping
	time.Sleep(25 * time.Millisecond)
	got := out.String()

	// Should be at least 2 frames
	wantFrames := "⠋⠙"
	if !containsAll(got, wantFrames) {
		t.Errorf("got %q, want to contain all %q", got, wantFrames)
	}

	wantMsg := "This message..."
	if !strings.Contains(got, wantMsg) {
		t.Errorf("got %q, want to contain %q", got, wantMsg)
	}
}

func containsAll(s string, chars string) bool {
	for _, r := range chars {
		if !strings.ContainsRune(s, r) {
			return false
		}
	}
	return true
}
