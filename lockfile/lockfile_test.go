package lockfile_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/getshiphub/shed/lockfile"
	"github.com/getshiphub/shed/tool"
)

func TestLockfileGet(t *testing.T) {
	lf := &lockfile.Lockfile{}
	lf.PutTool(tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"})
	lf.PutTool(tool.Tool{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"})
	lf.PutTool(tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"})
	lf.PutTool(tool.Tool{ImportPath: "example.org/z/random/stringer/v2/cmd/stringer", Version: "v2.1.0"})

	tests := []struct {
		name     string
		toolName string
		wantTool tool.Tool
		wantErr  error
	}{
		{
			name:     "short name",
			toolName: "go-fish",
			wantTool: tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
			wantErr:  nil,
		},
		{
			name:     "import path",
			toolName: "golang.org/x/tools/cmd/stringer",
			wantTool: tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"},
			wantErr:  nil,
		},
		{
			name:     "import path with version",
			toolName: "github.com/golangci/golangci-lint/cmd/golangci-lint",
			wantTool: tool.Tool{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"},
			wantErr:  nil,
		},
		{
			name:     "import path with bucket collision",
			toolName: "example.org/z/random/stringer/v2/cmd/stringer",
			wantTool: tool.Tool{ImportPath: "example.org/z/random/stringer/v2/cmd/stringer", Version: "v2.1.0"},
			wantErr:  nil,
		},
		// Errors
		{
			name:     "short name multiple found",
			toolName: "stringer",
			wantTool: tool.Tool{},
			wantErr:  lockfile.ErrMultipleTools,
		},
		{
			name:     "not found short name",
			toolName: "stress",
			wantTool: tool.Tool{},
			wantErr:  lockfile.ErrNotFound,
		},
		{
			name:     "not found import path",
			toolName: "golang.org/x/tools/cmd/stress",
			wantTool: tool.Tool{},
			wantErr:  lockfile.ErrNotFound,
		},
		{
			name:     "not found bucket",
			toolName: "example.org/z/tools/cmd/stringer",
			wantTool: tool.Tool{},
			wantErr:  lockfile.ErrNotFound,
		},
		{
			name:     "incorrect version",
			toolName: "golang.org/x/tools/cmd/stringer@v0.1.0",
			wantTool: tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"},
			wantErr:  lockfile.ErrIncorrectVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl, err := lf.GetTool(tt.toolName)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("want err to match %v, got %v", tt.wantErr, err)
			}
			if tl != tt.wantTool {
				t.Errorf("got %+v, want %+v", tl, tt.wantTool)
			}
		})
	}
}

func TestLockfilePutReplace(t *testing.T) {
	lf := &lockfile.Lockfile{}
	lf.PutTool(tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"})
	tl, err := lf.GetTool("go-fish")
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}
	want := tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"}
	if tl != want {
		t.Errorf("got %+v, want %+v", tl, want)
	}

	// Replace
	lf.PutTool(tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v1.0.0"})
	tl, err = lf.GetTool("go-fish")
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}
	want = tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v1.0.0"}
	if tl != want {
		t.Errorf("got %+v, want %+v", tl, want)
	}
}

func TestLockfileDelete(t *testing.T) {
	lf := &lockfile.Lockfile{}
	lf.PutTool(tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"})
	lf.PutTool(tool.Tool{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"})
	lf.PutTool(tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"})
	lf.PutTool(tool.Tool{ImportPath: "example.org/z/random/stringer/v2/cmd/stringer", Version: "v2.1.0"})

	tests := []struct {
		name string
		tool tool.Tool
	}{
		{
			name: "single element in bucket",
			tool: tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
		},
		{
			name: "multiple elements in bucket",
			tool: tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"},
		},
		{
			name: "remainder in bucket",
			tool: tool.Tool{ImportPath: "example.org/z/random/stringer/v2/cmd/stringer", Version: "v2.1.0"},
		},
		{
			name: "does not exist",
			tool: tool.Tool{ImportPath: "example.org/z/random/stringer/v2/cmd/stringer", Version: "v2.1.0"},
		},
		{
			name: "does not exist in bucket",
			tool: tool.Tool{ImportPath: "golang.org/x/tools/cmd/golangci-lint", Version: "v0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lf.DeleteTool(tt.tool)

			_, err := lf.GetTool(tt.tool.ImportPath)
			if !errors.Is(err, lockfile.ErrNotFound) {
				t.Errorf("want err to match %v, got %v", lockfile.ErrNotFound, err)
			}
		})
	}
}

func TestLockfileIter(t *testing.T) {
	lf := &lockfile.Lockfile{}
	lf.PutTool(tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"})
	lf.PutTool(tool.Tool{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"})
	lf.PutTool(tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"})
	lf.PutTool(tool.Tool{ImportPath: "example.org/z/random/stringer/v2/cmd/stringer", Version: "v2.1.0"})

	want := []string{
		"example.org/z/random/stringer/v2/cmd/stringer",
		"github.com/cszatmary/go-fish",
		"github.com/golangci/golangci-lint/cmd/golangci-lint",
		"golang.org/x/tools/cmd/stringer",
	}
	var got []string

	it := lf.Iter()
	for it.Next() {
		tl := it.Value()
		got = append(got, tl.ImportPath)
	}

	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLockfileWriteTo(t *testing.T) {
	lf := &lockfile.Lockfile{}
	lf.PutTool(tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"})
	lf.PutTool(tool.Tool{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"})
	lf.PutTool(tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"})
	lf.PutTool(tool.Tool{ImportPath: "example.org/z/random/stringer/v2/cmd/stringer", Version: "v2.1.0"})

	buf := &bytes.Buffer{}
	n, err := lf.WriteTo(buf)
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}

	data := buf.Bytes()
	if int(n) != len(data) {
		t.Errorf("got %d bytes written, want %d", n, len(data))
	}

	want := map[string]interface{}{
		"tools": map[string]interface{}{
			"github.com/cszatmary/go-fish": map[string]interface{}{
				"version": "v0.1.0",
			},
			"github.com/golangci/golangci-lint/cmd/golangci-lint": map[string]interface{}{
				"version": "v1.33.0",
			},
			"golang.org/x/tools/cmd/stringer": map[string]interface{}{
				"version": "v0.0.0-20201211185031-d93e913c1a58",
			},
			"example.org/z/random/stringer/v2/cmd/stringer": map[string]interface{}{
				"version": "v2.1.0",
			},
		},
	}
	var got interface{}
	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParse(t *testing.T) {
	r := strings.NewReader(`{
		"tools": {
		  "github.com/cszatmary/go-fish": {
			"version": "v0.1.0"
		  },
		  "github.com/golangci/golangci-lint/cmd/golangci-lint": {
			"version": "v1.33.0"
		  },
		  "golang.org/x/tools/cmd/stringer": {
			"version": "v0.0.0-20201211185031-d93e913c1a58"
		  },
		  "example.org/z/random/stringer/v2/cmd/stringer": {
			"version": "v2.1.0"
		  }
		}
	  }`)
	lf, err := lockfile.Parse(r)
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}

	tl, err := lf.GetTool("github.com/cszatmary/go-fish")
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}
	want := tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"}
	if tl != want {
		t.Errorf("got %+v, want %+v", tl, want)
	}

	tl, err = lf.GetTool("github.com/golangci/golangci-lint/cmd/golangci-lint")
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}
	want = tool.Tool{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"}
	if tl != want {
		t.Errorf("got %+v, want %+v", tl, want)
	}

	tl, err = lf.GetTool("golang.org/x/tools/cmd/stringer")
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}
	want = tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"}
	if tl != want {
		t.Errorf("got %+v, want %+v", tl, want)
	}

	tl, err = lf.GetTool("example.org/z/random/stringer/v2/cmd/stringer")
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}
	want = tool.Tool{ImportPath: "example.org/z/random/stringer/v2/cmd/stringer", Version: "v2.1.0"}
	if tl != want {
		t.Errorf("got %+v, want %+v", tl, want)
	}
}
