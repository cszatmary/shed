package tool_test

import (
	"path/filepath"
	"testing"

	"github.com/cszatmary/shed/tool"
)

func TestTool(t *testing.T) {
	tests := []struct {
		name               string
		tool               tool.Tool
		wantName           string
		wantModule         string
		wantFilepath       string
		wantBinaryFilepath string
	}{
		{
			name:               "root module",
			tool:               tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
			wantName:           "go-fish",
			wantModule:         "github.com/cszatmary/go-fish@v0.1.0",
			wantFilepath:       filepath.FromSlash("github.com/cszatmary/go-fish@v0.1.0"),
			wantBinaryFilepath: filepath.FromSlash("github.com/cszatmary/go-fish@v0.1.0/go-fish"),
		},
		{
			name:               "no version",
			tool:               tool.Tool{ImportPath: "github.com/cszatmary/go-fish"},
			wantName:           "go-fish",
			wantModule:         "github.com/cszatmary/go-fish",
			wantFilepath:       filepath.FromSlash("github.com/cszatmary/go-fish"),
			wantBinaryFilepath: filepath.FromSlash("github.com/cszatmary/go-fish/go-fish"),
		},
		{
			name:               "nested import path",
			tool:               tool.Tool{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"},
			wantName:           "golangci-lint",
			wantModule:         "github.com/golangci/golangci-lint/cmd/golangci-lint@v1.33.0",
			wantFilepath:       filepath.FromSlash("github.com/golangci/golangci-lint/cmd/golangci-lint@v1.33.0"),
			wantBinaryFilepath: filepath.FromSlash("github.com/golangci/golangci-lint/cmd/golangci-lint@v1.33.0/golangci-lint"),
		},
		{
			name:               "pseudo-version",
			tool:               tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"},
			wantName:           "stringer",
			wantModule:         "golang.org/x/tools/cmd/stringer@v0.0.0-20201211185031-d93e913c1a58",
			wantFilepath:       filepath.FromSlash("golang.org/x/tools/cmd/stringer@v0.0.0-20201211185031-d93e913c1a58"),
			wantBinaryFilepath: filepath.FromSlash("golang.org/x/tools/cmd/stringer@v0.0.0-20201211185031-d93e913c1a58/stringer"),
		},
		{
			name:               "escaped path",
			tool:               tool.Tool{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.2.2"},
			wantName:           "ejson",
			wantModule:         "github.com/Shopify/ejson/cmd/ejson@v1.2.2",
			wantFilepath:       filepath.FromSlash("github.com/!shopify/ejson/cmd/ejson@v1.2.2"),
			wantBinaryFilepath: filepath.FromSlash("github.com/!shopify/ejson/cmd/ejson@v1.2.2/ejson"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.tool.Name()
			if name != tt.wantName {
				t.Errorf("got %s, want %s", name, tt.wantName)
			}

			module := tt.tool.Module()
			if module != tt.wantModule {
				t.Errorf("got %s, want %s", module, tt.wantModule)
			}

			fp, err := tt.tool.Filepath()
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}
			if fp != tt.wantFilepath {
				t.Errorf("got %s, want %s", fp, tt.wantFilepath)
			}

			bfp, err := tt.tool.BinaryFilepath()
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}
			if bfp != tt.wantBinaryFilepath {
				t.Errorf("got %s, want %s", bfp, tt.wantBinaryFilepath)
			}
		})
	}
}

func TestToolFilepathError(t *testing.T) {
	tests := []struct {
		name string
		tool tool.Tool
	}{
		{
			name: "invalid domain",
			tool: tool.Tool{ImportPath: "golang/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"},
		},
		{
			name: "invalid version",
			tool: tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.!.0-20201211185031-d93e913c1a58"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.tool.Filepath()
			if err == nil {
				t.Error("Filepath: want non-nil error, got nil")
			}

			_, err = tt.tool.BinaryFilepath()
			if err == nil {
				t.Error("BinaryFilepath: want non-nil error, got nil")
			}
		})
	}
}

func TestToolString(t *testing.T) {
	tl := tool.Tool{ImportPath: "golang/x/tools/cmd/stringer", Version: "v0.0.1"}
	s := tl.String()
	want := "golang/x/tools/cmd/stringer@v0.0.1"
	if s != want {
		t.Errorf("got %s, want %s", s, want)
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name   string
		module string
		want   tool.Tool
	}{
		{
			name:   "root module",
			module: "github.com/cszatmary/go-fish@v0.1.0",
			want:   tool.Tool{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
		},
		{
			name:   "no version",
			module: "github.com/cszatmary/go-fish",
			want:   tool.Tool{ImportPath: "github.com/cszatmary/go-fish"},
		},
		{
			name:   "nested import path",
			module: "github.com/golangci/golangci-lint/cmd/golangci-lint@v1.33.0",
			want:   tool.Tool{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"},
		},
		{
			name:   "pseudo-version",
			module: "golang.org/x/tools/cmd/stringer@v0.0.0-20201211185031-d93e913c1a58",
			want:   tool.Tool{ImportPath: "golang.org/x/tools/cmd/stringer", Version: "v0.0.0-20201211185031-d93e913c1a58"},
		},
		{
			name:   "escaped path",
			module: "github.com/Shopify/ejson/cmd/ejson@v1.2.2",
			want:   tool.Tool{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.2.2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl, err := tool.Parse(tt.module)
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}
			if tl != tt.want {
				t.Errorf("got %+v, want %+v", tl, tt.want)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name   string
		module string
	}{
		{
			name:   "invalid domain",
			module: "golang/x/tools/cmd/stringer@v0.0.0-20201211185031-d93e913c1a58",
		},
		{
			name:   "invalid version",
			module: "golang.org/x/tools/cmd/stringer@v0..0-20201211185031-d93e913c1a58",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Parse(tt.module)
			if err == nil {
				t.Error("want non-nil error, got nil")
			}
		})
	}
}
