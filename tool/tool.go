// Package tool defines the tool.Tool type which allows for the manipulation
// of various properties of a tool.
package tool

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

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

// HasSemver reports whether t.Version is a valid semantic version.
// HasSemver requires t.Version to be a full semantic version. It does
// not allow shorthands like vMAJOR or vMAJOR.MINOR.
func (t Tool) HasSemver() bool {
	// Compare against canonical to make sure it isn't a shorthand.
	return semver.IsValid(t.Version) && t.Version == semver.Canonical(t.Version)
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
		return "", fmt.Errorf("tool: failed to escape path %q: %w", t.ImportPath, err)
	}

	if t.Version != "" {
		escapedVersion, err := module.EscapeVersion(t.Version)
		if err != nil {
			return "", fmt.Errorf("tool: failed to escape version %q: %w", t.Version, err)
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
		return "", err
	}
	return filepath.Join(fp, t.Name()), nil
}

// Parse parses the given tool name and returns a tool containing the
// import path and version. name must be a valid import path and a version
// with the format 'IMPORT_PATH@VERSION'. This format is the same as what would be
// pass to a command like 'go get'. The version must be a valid semantic version
// and it must be prefixed with 'v' (ex: 'v1.2.3'). If a shorthand semantic version
// is used, it will be canonicalized (ex: 'v1' will become 'v1.0.0').
func Parse(name string) (Tool, error) {
	return parseTool(name, true)
}

// ParseLax is like Parse but does not check that the version is a valid semantic version.
// It is used when downloading and resolving tools using 'go get'. This is because
// go get allows module queries, which is where a version is resolved based on a
// branch name, commit SHA, version range, etc.
// See https://golang.org/cmd/go/#hdr-Module_queries for more details on module queries.
// Unlike Parse, ParseLax will not canonicalize shorthand semantic verions and will
// instead leave them as is.
//
// ParseLax allows the version to be omitted in which case it is assumed to mean
// the latest version. That is, 'golang/x/tools/cmd/stringer' is functionally
// equivalent to 'golang/x/tools/cmd/stringer@latest'.
func ParseLax(name string) (Tool, error) {
	return parseTool(name, false)
}

func parseTool(name string, strict bool) (Tool, error) {
	t := Tool{ImportPath: name}

	// Check if a version/query is provided
	if i := strings.IndexByte(name, '@'); i != -1 {
		t.ImportPath = name[:i]
		t.Version = name[i+1:]

		// Make sure there isn't a dangling '@'
		if t.Version == "" {
			return t, fmt.Errorf("tool: missing version after '@'")
		}
	}

	// Validations
	if err := module.CheckPath(t.ImportPath); err != nil {
		return t, fmt.Errorf("tool: invalid import path %q: %w", t.ImportPath, err)
	}
	// Version validation is ignored if not strict
	if !strict {
		return t, nil
	}

	if !semver.IsValid(t.Version) {
		return t, fmt.Errorf("tool: invalid version %q: not a semantic version", t.Version)
	}
	// The semver package allows vMAJOR and vMAJOR.MINOR as shorthands.
	// Use the canonical version to ensure it is a full semantic version.
	canonical := semver.Canonical(t.Version)
	if t.Version != canonical {
		t.Version = canonical
	}
	return t, nil
}
