package errors_test

import (
	"fmt"
	"testing"

	"github.com/cszatmary/shed/errors"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		format string
		want   string
	}{
		{
			name:   "unspecified error",
			err:    errors.New("something blew up"),
			format: "%s",
			want:   "something blew up",
		},
		{
			name: "string format",
			err: errors.New(
				errors.IO,
				"unable to create go.mod",
				errors.Op("Cache.Install"),
				fmt.Errorf("dir not exist"),
			),
			format: "%s",
			want:   "I/O error: unable to create go.mod: dir not exist",
		},
		{
			name: "detailed format",
			err: errors.New(
				errors.IO,
				"unable to create go.mod",
				errors.Op("Cache.Install"),
				fmt.Errorf("dir not exist"),
			),
			format: "%+v",
			want:   "Cache.Install: I/O error: unable to create go.mod: dir not exist",
		},
		{
			name: "detailed format with nested error",
			err: errors.New(
				errors.BadState,
				"cannot find tool",
				errors.Op("Shed.ToolPath"),
				errors.New(
					errors.NotInstalled,
					"no binary for tool stringer",
					errors.Op("Cache.ToolPath"),
					fmt.Errorf("file not exist"),
				),
			),
			format: "%+v",
			want:   "Shed.ToolPath: bad state: cannot find tool:\n\tCache.ToolPath: tool not installed: no binary for tool stringer: file not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := fmt.Sprintf(tt.format, tt.err)
			if s != tt.want {
				t.Errorf("got\n\t%s\nwant\n\t%s", s, tt.want)
			}
		})
	}
}

func TestRoot(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want *errors.Error
	}{
		{
			name: "nil error",
			err:  nil,
			want: nil,
		},
		{
			name: "non *Error",
			err:  fmt.Errorf("boom"),
			want: nil,
		},
		{
			name: "is an *Error",
			err:  errors.New(errors.IO, "unable to create go.mod", errors.Op("Cache.Install")),
			want: &errors.Error{
				Kind:   errors.IO,
				Reason: "unable to create go.mod",
				Op:     "Cache.Install",
			},
		},
		{
			name: "nested *Error",
			err: errors.New(
				errors.BadState,
				"cannot find tool",
				errors.Op("Shed.ToolPath"),
				errors.New(
					errors.NotInstalled,
					"no binary for tool stringer",
					errors.Op("Cache.ToolPath"),
				),
			),
			want: &errors.Error{
				Kind:   errors.NotInstalled,
				Reason: "no binary for tool stringer",
				Op:     "Cache.ToolPath",
			},
		},
		{
			name: "nested inside non *Error",
			err: fmt.Errorf("failed to find tool: %w", errors.New(
				errors.BadState,
				"cannot find tool",
				errors.Op("Shed.ToolPath"),
				errors.New(
					errors.NotInstalled,
					"no binary for tool stringer",
					errors.Op("Cache.ToolPath"),
				),
			)),
			want: &errors.Error{
				Kind:   errors.NotInstalled,
				Reason: "no binary for tool stringer",
				Op:     "Cache.ToolPath",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errors.Root(tt.err)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got\n\t%+v\nwant nil", got)
				}
				return // passed
			}
			if *got != *tt.want {
				t.Errorf("got\n\t%+v\nwant\n\t%+v", got, tt.want)
			}
		})
	}
}
