package client_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/getshiphub/shed/cache"
	"github.com/getshiphub/shed/client"
	"github.com/getshiphub/shed/internal/util"
	"github.com/getshiphub/shed/lockfile"
	"github.com/getshiphub/shed/tool"
)

func TestClientCache(t *testing.T) {
	td := t.TempDir()
	s, err := client.NewShed(client.Options{
		CacheDir: td,
	})
	if err != nil {
		t.Fatalf("failed to create shed client %v", err)
	}

	if s.CacheDir() != td {
		t.Errorf("got %s, want %s", s.CacheDir(), td)
	}
	if !util.FileOrDirExists(s.CacheDir()) {
		t.Errorf("expected %s to exist, but it doesn't", s.CacheDir())
	}

	err = s.CleanCache()
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}
	if util.FileOrDirExists(s.CacheDir()) {
		t.Errorf("expected %s to not exist, but it exists", s.CacheDir())
	}
}

var availableTools = map[string]map[string]string{
	"github.com/cszatmary/go-fish": {
		"v0.1.0": "v0.1.0",
		"22d10c9b658df297b17b33c836a60fb943ef5a5f": "v0.0.0-20201203230243-22d10c9b658d",
	},
	"github.com/golangci/golangci-lint/cmd/golangci-lint": {
		"v1.33.0": "v1.33.0",
		"v1.28.3": "v1.28.3",
	},
	"golang.org/x/tools/cmd/stringer": {
		"v0.0.0-20201211185031-d93e913c1a58": "v0.0.0-20201211185031-d93e913c1a58",
	},
	"github.com/Shopify/ejson/cmd/ejson": {
		"v1.2.2": "v1.2.2",
		"v1.1.0": "v1.1.0",
	},
	"example.org/z/random/stringer/v2/cmd/stringer": {
		"v2.1.0": "v2.1.0",
	},
}

func createLockfile(t *testing.T, path string, tools []tool.Tool) {
	lf := &lockfile.Lockfile{}
	for _, tl := range tools {
		lf.PutTool(tl)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("failed to create %s, %v", path, err)
	}
	defer f.Close()

	_, err = lf.WriteTo(f)
	if err != nil {
		t.Fatalf("failed to write lockfile, %v", err)
	}
}

func readLockfile(t *testing.T, path string) *lockfile.Lockfile {
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open file %s, %v", path, err)
	}
	defer f.Close()
	lf, err := lockfile.Parse(f)
	if err != nil {
		t.Fatalf("failed to parse lockfile at %s, %v", path, err)
	}
	return lf
}

func TestInstall(t *testing.T) {
	tests := []struct {
		name          string
		lockfileTools []tool.Tool
		installTools  []string
		allowUpdates  bool
		wantTools     []tool.Tool
	}{
		{
			name:          "install latest",
			lockfileTools: nil,
			installTools: []string{
				"github.com/cszatmary/go-fish",
				"github.com/golangci/golangci-lint/cmd/golangci-lint",
				"github.com/Shopify/ejson/cmd/ejson",
			},
			allowUpdates: false,
			wantTools: []tool.Tool{
				{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
				{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"},
				{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.2.2"},
			},
		},
		{
			name:          "install specific versions",
			lockfileTools: nil,
			installTools: []string{
				"github.com/cszatmary/go-fish@22d10c9b658df297b17b33c836a60fb943ef5a5f",
				"github.com/golangci/golangci-lint/cmd/golangci-lint@v1.28.3",
				"github.com/Shopify/ejson/cmd/ejson@v1.1.0",
			},
			allowUpdates: false,
			wantTools: []tool.Tool{
				{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.0.0-20201203230243-22d10c9b658d"},
				{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.28.3"},
				{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.1.0"},
			},
		},
		{
			name: "install from lockfile",
			lockfileTools: []tool.Tool{
				{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
				{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.28.3"},
				{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.1.0"},
			},
			installTools: nil,
			allowUpdates: false,
			wantTools: []tool.Tool{
				{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
				{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.28.3"},
				{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.1.0"},
			},
		},
		{
			name: "update tool",
			lockfileTools: []tool.Tool{
				{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
				{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.28.3"},
				{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.1.0"},
			},
			installTools: []string{
				"github.com/golangci/golangci-lint/cmd/golangci-lint@v1.33.0",
			},
			allowUpdates: true,
			wantTools: []tool.Tool{
				{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
				{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"},
				{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.1.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := t.TempDir()
			lockfilePath := filepath.Join(td, "shed.lock")
			mockGo, err := cache.NewMockGo(availableTools)
			if err != nil {
				t.Fatalf("failed to create mock go %v", err)
			}

			// Only create lockfile if tools not nil, so we can test cases
			// where the lockfile doesn't exist.
			if tt.lockfileTools != nil {
				createLockfile(t, lockfilePath, tt.lockfileTools)
			}
			s, err := client.NewShed(client.Options{
				LockfilePath: lockfilePath,
				CacheDir:     td,
				Go:           mockGo,
			})
			if err != nil {
				t.Fatalf("failed to create shed client %v", err)
			}

			err = s.Install(tt.allowUpdates, tt.installTools...)
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			lf := readLockfile(t, lockfilePath)
			for _, wantTool := range tt.wantTools {
				tl, err := lf.GetTool(wantTool.ImportPath)
				if err != nil {
					t.Errorf("want nil error, got %v", err)
				}
				if tl != wantTool {
					t.Errorf("got %+v, want %+v", tl, wantTool)
				}
				// ToolPath will return an error if the binary does not exist
				_, err = s.ToolPath(wantTool.ImportPath)
				if err != nil {
					t.Errorf("want nil error, got %v", err)
				}
			}
		})
	}
}

func TestInstallError(t *testing.T) {
	td := t.TempDir()
	lockfilePath := filepath.Join(td, "shed.lock")
	mockGo, err := cache.NewMockGo(availableTools)
	if err != nil {
		t.Fatalf("failed to create mock go %v", err)
	}

	createLockfile(t, lockfilePath, []tool.Tool{
		{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.1.0"},
	})
	s, err := client.NewShed(client.Options{
		LockfilePath: lockfilePath,
		CacheDir:     td,
		Go:           mockGo,
	})
	if err != nil {
		t.Fatalf("failed to create shed client %v", err)
	}

	err = s.Install(
		false,
		"github.com/cszatmary/go-fish",
		"golangci-lint",
		"github.com/Shopify/ejson/cmd/ejson@v1.2.2",
	)
	errList, ok := err.(lockfile.ErrorList)
	if !ok {
		t.Errorf("want error to be lockfile.ErrorList, got %s: %T", err, err)
	}
	wantLen := 2
	if len(errList) != wantLen {
		t.Errorf("got %d errors, want %d", len(errList), wantLen)
	}
}

func TestUninstall(t *testing.T) {
	td := t.TempDir()
	lockfilePath := filepath.Join(td, "shed.lock")
	createLockfile(t, lockfilePath, []tool.Tool{
		{ImportPath: "github.com/cszatmary/go-fish", Version: "v0.1.0"},
		{ImportPath: "github.com/golangci/golangci-lint/cmd/golangci-lint", Version: "v1.33.0"},
		{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.2.2"},
	})
	s, err := client.NewShed(client.Options{
		LockfilePath: lockfilePath,
		CacheDir:     td,
	})
	if err != nil {
		t.Fatalf("failed to create shed client %v", err)
	}

	uninstallTools := []string{"go-fish", "golangci-lint"}
	err = s.Uninstall(uninstallTools...)
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}

	lf := readLockfile(t, lockfilePath)
	for _, tn := range uninstallTools {
		_, err := lf.GetTool(tn)
		if !errors.Is(err, lockfile.ErrNotFound) {
			t.Errorf("want ErrNotFound, got %v", err)
		}
		// ToolPath will return an error if the binary does not exist
		_, err = s.ToolPath(tn)
		if err == nil {
			t.Error("want non-nil error, got nil")
		}
	}

	tl, err := lf.GetTool("ejson")
	if err != nil {
		t.Errorf("want nil error, got %v", err)
	}
	wantTool := tool.Tool{ImportPath: "github.com/Shopify/ejson/cmd/ejson", Version: "v1.2.2"}
	if tl != wantTool {
		t.Errorf("got %+v, want %+v", tl, wantTool)
	}
}
