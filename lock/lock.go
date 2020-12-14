package lock

import (
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/TouchBistro/goutils/file"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

const lockfileName = "shed.lock"

type Tool struct {
	Version    string `json:"version"`
	ImportPath string `json:"-"`
}

// Name returns the name of the tool binary.
func (t Tool) Name() string {
	return path.Base(t.ImportPath)
}

func ParseTool(moduleName string) (Tool, error) {
	t := Tool{ImportPath: moduleName}

	// Check if version is provided
	if i := strings.IndexByte(moduleName, '@'); i != -1 {
		t.ImportPath = moduleName[:i]
		t.Version = moduleName[i+1:]
	}

	// Validate fields
	if err := module.CheckPath(t.ImportPath); err != nil {
		return t, errors.Wrapf(err, "invalid import path: %s", t.ImportPath)
	}

	if t.Version != "" && !semver.IsValid(t.Version) {
		return t, errors.Errorf("%s: invalid tool version: %s", t.ImportPath, t.Version)
	}

	return t, nil
}

// Lockfile represents a shed lockfile that contains
// information about installed tools.
type Lockfile struct {
	Tools map[string]Tool `json:"tools"`
	// Path the lockfile was read from, so we can easily write
	// back to the same file. Empty if a new lockfile.
	path string
}

// Read reads the shed lockfile.
func Read() (*Lockfile, error) {
	// TODO(@cszatmary): We should have a way to resolve the lockfile
	// in parent dirs up to the repo root
	path := lockfileName

	if !file.FileOrDirExists(path) {
		// Return a new empty lockfile if one doesn't exist
		log.Debug("No lockfile, creating new one")
		return &Lockfile{Tools: make(map[string]Tool)}, nil
	}

	log.Debugf("Reading lockfile: %s", path)

	f, err := os.Open(path)
	f.Name()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %s", path)
	}
	defer f.Close()

	lf := &Lockfile{path: path}
	err = json.NewDecoder(f).Decode(lf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode shed lockfile")
	}

	if lf.Tools == nil {
		lf.Tools = make(map[string]Tool)
	}

	// Parse all tools in lockfile and make sure there are no errors
	var errMessages []string
	for importPath, tool := range lf.Tools {
		parsedTool, err := ParseTool(importPath)
		if err != nil {
			errMessages = append(errMessages, err.Error())
			continue
		}

		parsedTool.Version = tool.Version
		lf.Tools[importPath] = parsedTool
	}

	if len(errMessages) > 0 {
		return nil, errors.Errorf("parse errors in shed lockfile: %s", strings.Join(errMessages, "\n"))
	}

	return lf, nil
}

// Write writes the shed lockfile.
func Write(lf *Lockfile) error {
	path := lf.path
	if path == "" {
		// New lockfile, create it in the current dir
		path = lockfileName
	}

	log.Debugf("Writing lockfile: %s", path)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return errors.Wrapf(err, "failed to create/open file %s", path)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(lf)
	if err != nil {
		return errors.Wrap(err, "failed to encode and write shed lockfile")
	}

	return nil
}
