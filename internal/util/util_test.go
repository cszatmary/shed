package util_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pkg/errors"

	"github.com/getshiphub/shed/internal/util"
)

func TestFileOrDirExists(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"dir exists", filepath.Join("."), true},
		// Test that this file exists, how meta!
		{"file exists", "util_test.go", true},
		{"file exists", "if_this_exists_all_hope_is_lost.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := util.FileOrDirExists(tt.path)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

type mockExit struct {
	code int
}

func (me *mockExit) Exit(code int) {
	me.code = code
}

func TestFatalExitf(t *testing.T) {
	buf := &bytes.Buffer{}
	me := mockExit{}
	fatal := util.Fatal{ErrWriter: buf, ExitFunc: me.Exit}

	fatal.Exitf("%d failures", 3)
	if me.code != 1 {
		t.Errorf("got error code %d, expected 1", me.code)
	}
	out := buf.String()
	if out != "3 failures\n" {
		t.Errorf("got output '%s', expected '3 failures\n'", out)
	}
}

func TestExitErrf(t *testing.T) {
	buf := &bytes.Buffer{}
	me := mockExit{}
	fatal := util.Fatal{ErrWriter: buf, ExitFunc: me.Exit}

	err := errors.New("err everything broke")
	fatal.ExitErrf(err, "%d failures", 3)
	if me.code != 1 {
		t.Errorf("got error code %d, expected 1", me.code)
	}
	out := buf.String()
	if out != "3 failures\nError: err everything broke\n" {
		t.Errorf("got output '%s', expected '3 failures\nError: err everything broke\n'", out)
	}
}

func TestExitErrStackf(t *testing.T) {
	buf := &bytes.Buffer{}
	me := mockExit{}
	fatal := util.Fatal{ShowErrorDetail: true, ErrWriter: buf, ExitFunc: me.Exit}

	err := errors.New("err everything broke")
	fatal.ExitErrf(err, "%d failures", 3)
	if me.code != 1 {
		t.Errorf("got error code %d, expected 1", me.code)
	}

	expected := "3 failures\n" +
		"Error: err everything broke\n" +
		"github.com/getshiphub/shed/internal/util_test.TestExitErrStack\n" +
		"\t.+"
	testFormatRegexp(t, 0, err, buf.String(), expected)
}

// Taken from https://github.com/pkg/errors/blob/614d223910a179a466c1767a985424175c39b465/format_test.go#L387
// Helper to test string with regexp
func testFormatRegexp(t *testing.T, n int, arg interface{}, format, want string) {
	t.Helper()
	got := fmt.Sprintf(format, arg)
	gotLines := strings.SplitN(got, "\n", -1)
	wantLines := strings.SplitN(want, "\n", -1)

	if len(wantLines) > len(gotLines) {
		t.Errorf("test %d: wantLines(%d) > gotLines(%d):\n got: %q\nwant: %q", n+1, len(wantLines), len(gotLines), got, want)
		return
	}

	for i, w := range wantLines {
		match, err := regexp.MatchString(w, gotLines[i])
		if err != nil {
			t.Fatal(err)
		}
		if !match {
			t.Errorf("test %d: line %d: fmt.Sprintf(%q, err):\n got: %q\nwant: %q", n+1, i+1, format, got, want)
		}
	}
}
