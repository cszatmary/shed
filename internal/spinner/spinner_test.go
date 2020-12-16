package spinner_test

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cszatmary/shed/internal/spinner"
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

func containsAll(s string, chars string) bool {
	for _, r := range chars {
		if !strings.ContainsRune(s, r) {
			return false
		}
	}
	return true
}
