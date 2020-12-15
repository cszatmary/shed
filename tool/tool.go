// Package tool defines the tool.Tool type which allows for the manipulation
// of various properties of a tool.
package tool

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// Tool represents a tool managed by shed.
// In most cases this corresponds to a Go module.
type Tool struct {
	// ImportPath is the Go import path for the tool.
	// This includes the full path to the tool, not just the module.
	// Ex: For the stringer tool the import path is
	// golang/x/tools/cmd/stringer not golang/x/tools.
	ImportPath string
	// The version of the tool. This correspeonds to the version of
	// the Go module the tool belongs to. If version is empty,
	// it significes that the latest version is desired where allowed.
	Version string
}

// Name returns the name of the tool. This is the name of the
// binary produced. It is the last component of the import path.
func (t Tool) Name() string {
	return path.Base(t.ImportPath)
}

// Module returns the module name suitable for commands like 'go get'.
// This is the import path plus the version, if it exists, with the
// format 'ImportPath@Version'. If Version is empty, Module just
// returns ImportPath.
func (t Tool) Module() string {
	if t.Version == "" {
		return t.ImportPath
	}
	return t.ImportPath + "@" + t.Version
}

// String returns a string representation of the tool.
func (t Tool) String() string {
	// While this may seem shallow, String serves a different purpose
	// than Module and is therefore distinct. Module clearly represents
	// the intent to get the module name, whereas String is meant to
	// produce a string representation suitable for logging.
	return t.Module()
}

// Filepath returns the relative OS filesystem path represented by this tool.
// The escape rules required for import paths on are followed.
// For details on escaped paths see:
// https://pkg.go.dev/golang.org/x/mod@v0.4.0/module#hdr-Escaped_Paths
func (t Tool) Filepath() (string, error) {
	escapedPath, err := module.EscapePath(t.ImportPath)
	if err != nil {
		return "", errors.Wrapf(err, "tool: failed to escape path %q", t.ImportPath)
	}

	if t.Version != "" {
		escapedVersion, err := module.EscapeVersion(t.Version)
		if err != nil {
			return "", errors.Wrapf(err, "tool: failed to escape version %q", t.Version)
		}
		escapedPath += "@" + escapedVersion
	}

	return filepath.FromSlash(escapedPath), nil
}

// BinaryFilepath returns the relative OS filesystem path to the tool binary.
// This is the Filepath joined with the Name.
func (t Tool) BinaryFilepath() (string, error) {
	fp, err := t.Filepath()
	if err != nil {
		return "", errors.WithMessage(err, "tool: failed to get filepath")
	}
	return filepath.Join(fp, t.Name()), nil
}

// Parse parse the given tool name. Name must be a valid import path,
// optionally with a version. If a version is provided, the format must be
// 'ImportPath@Version', just like what would be passed to a command like 'go get'.
func Parse(name string) (Tool, error) {
	t := Tool{ImportPath: name}

	// Check if version is provided
	if i := strings.IndexByte(name, '@'); i != -1 {
		t.ImportPath = name[:i]
		t.Version = name[i+1:]
	}

	// Validations
	if err := module.CheckPath(t.ImportPath); err != nil {
		return t, errors.Wrapf(err, "tool: invalid import path: %q", t.ImportPath)
	}

	if t.Version != "" && !semver.IsValid(t.Version) {
		return t, errors.Errorf("tool: invalid version: %q", t.Version)
	}

	return t, nil
}
