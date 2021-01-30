// Package cache handles managing the actual installing of tools.
// It handles downloading and building the go modules.
// Tools are stored in a cache on the OS filesystem so that they can
// be reused by other projects.
package cache

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/getshiphub/shed/internal/util"
	"github.com/getshiphub/shed/tool"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
)

// Cache manages tools in an OS filesystem directory.
type Cache struct {
	rootDir string
	// Used to download and build tools.
	goClient Go
	// For diagnostics.
	logger logrus.FieldLogger
}

// New creates a new Cache instance that uses the directory dir.
// Options can be provided to customize the Cache instance.
func New(dir string, opts ...Option) *Cache {
	c := &Cache{rootDir: dir}
	for _, opt := range opts {
		opt(c)
	}
	// Set defaults
	if c.goClient == nil {
		c.goClient = NewGo()
	}
	if c.logger == nil {
		// Logging is disabled by default, but we don't want to have to check
		// for nil all the time, so create a logger that logs to nowhere
		logger := logrus.New()
		logger.Out = ioutil.Discard
		c.logger = logger
	}
	return c
}

// Option is a function that takes a Cache instance and applies
// a configuration to it.
type Option func(*Cache)

// WithGo sets the Go client that should be used to download and build tools.
func WithGo(goClient Go) Option {
	return func(c *Cache) {
		c.goClient = goClient
	}
}

// WithLogger sets a logger that should be used for writing debug messages.
// By default no logging is done.
func WithLogger(logger logrus.FieldLogger) Option {
	return func(c *Cache) {
		c.logger = logger
	}
}

// Dir returns the OS filesystem directory used by this Cache.
func (c *Cache) Dir() string {
	return c.rootDir
}

// Clean removes the cache directory and all contents from the filesystem.
func (c *Cache) Clean() error {
	if err := os.RemoveAll(c.rootDir); err != nil {
		return errors.Wrapf(err, "cache: clean failed")
	}
	return nil
}

// toolsDir returns the path to the directory where tools are installed.
func (c *Cache) toolsDir() string {
	return filepath.Join(c.rootDir, "tools")
}

// Install installs the given tool. t must have ImportPath set, otherwise
// an error will be returned. If t.Version is empty, then the latest version
// of the tool will be installed. The returned tool will have Version set
// to the version that was installed.
func (c *Cache) Install(t tool.Tool) (tool.Tool, error) {
	// Make sure import path is set as it's required for download
	if t.ImportPath == "" {
		return t, errors.New("import path is required on module")
	}

	// Download step

	downloadedTool, err := c.download(t)
	if err != nil {
		return t, errors.WithMessagef(err, "failed to download tool: %s", t)
	}

	// Build step

	fp, err := downloadedTool.Filepath()
	if err != nil {
		return downloadedTool, err
	}
	baseDir := c.toolsDir()
	binDir := filepath.Join(baseDir, fp)

	bfp, err := downloadedTool.BinaryFilepath()
	if err != nil {
		return downloadedTool, err
	}
	binPath := filepath.Join(baseDir, bfp)

	// Check if already built
	if util.FileOrDirExists(binPath) {
		c.logger.WithFields(logrus.Fields{
			"tool": downloadedTool,
			"path": binPath,
		}).Debug("tool binary already exists, skipping build")
		return downloadedTool, nil
	}

	err = c.goClient.Build(downloadedTool.ImportPath, binPath, binDir)
	if err != nil {
		return downloadedTool, errors.WithMessagef(err, "failed to build tool: %s", downloadedTool)
	}

	c.logger.WithFields(logrus.Fields{
		"tool": downloadedTool,
		"path": binPath,
	}).Debug("tool built")
	return downloadedTool, nil
}

// download does half the work of Install. It is responsible for downloading the tool
// using go get -d. It does this by creating an empty go.mod which can then be used to install
// the desired tool. If no version is specified for the tool, the latest version will be resolved
// by go get.
//
// go.mod files are stored in a directory the is represented by the tool import path.
// For example if the import path is golang.org/x/tools/cmd/stringer then download will create
// BASE_DIR/golang.org/x/tools/cmd/stringer@VERSION/go.mod where BASE_DIR is the baseDir parameter
// and VERSION is the version of the tool (either explicit or resolved).
func (c *Cache) download(t tool.Tool) (tool.Tool, error) {
	// Get the path to where the tool will be installed
	// This is where the go.mod file will be
	fp, err := t.Filepath()
	if err != nil {
		return t, err
	}
	modDir := filepath.Join(c.toolsDir(), fp)
	modfilePath := filepath.Join(modDir, "go.mod")

	// If we have the version the process is pretty easy
	if t.HasSemver() {
		if util.FileOrDirExists(modfilePath) {
			// If go.mod already exists, make sure there's no issues with it
			data, err := ioutil.ReadFile(modfilePath)
			if err != nil {
				return t, errors.Wrapf(err, "failed to read file %q", modfilePath)
			}

			modFile, err := modfile.Parse(modfilePath, data, nil)
			if err != nil {
				return t, errors.Wrapf(err, "failed to parse go.mod file %q", modfilePath)
			}

			modfileOK := true
			// There should only be a single require, otherwise something is wrong
			if len(modFile.Require) != 1 {
				modfileOK = false
				c.logger.Debugf("expected 1 required statement in go.mod, found %d", len(modFile.Require))
			}

			mod := modFile.Require[0].Mod
			// Use contains since actual module could have less then what we are installing
			// Ex: golang.org/x/tools vs golang.org/x/tools/cmd/stringer
			if !strings.Contains(t.ImportPath, mod.Path) {
				modfileOK = false
				c.logger.WithFields(logrus.Fields{
					"expected": t.ImportPath,
					"received": mod.Path,
				}).Debug("incorrect dependency in go.mod")
			}

			if t.Version != mod.Version {
				modfileOK = false
				c.logger.WithFields(logrus.Fields{
					"expected": t.Version,
					"received": mod.Version,
				}).Debug("incorrect dependency version go.mod")
			}

			if modfileOK {
				c.logger.WithFields(logrus.Fields{
					"tool": t,
				}).Debug("tool already exists, skipping download")
				return t, nil
			}

			c.logger.WithFields(logrus.Fields{
				"tool": t,
			}).Debug("tool exists but issues found, re-downloading")

			if err := os.Remove(modfilePath); err != nil {
				return t, errors.Wrapf(err, "failed to remove file %q", modfilePath)
			}
		}

		if err := os.MkdirAll(modDir, 0o755); err != nil {
			return t, errors.Wrapf(err, "failed to create directory %q", modDir)
		}

		// Create empty go.mod file so we can install module
		// Can just use _ as the module name since this is a "fake" module
		err = createGoModFile("_", modDir)
		if err != nil {
			return t, err
		}

		// Download the module source. What's nice here is we leverage the power of
		// go get so we don't need to reinvent the module resolution & downloading.
		// Also we can reuse an existing download that's already cached.

		err = c.goClient.GetD(t.Module(), modDir)
		if err != nil {
			return t, err
		}

		c.logger.WithFields(logrus.Fields{
			"tool":    t,
			"srcPath": modDir,
		}).Debug("downloaded tool")
		return t, nil
	}

	// Don't have the version, this process is a bit more complicated because
	// we need to resolve the correct version.

	if err := os.MkdirAll(modDir, 0o755); err != nil {
		return t, errors.Wrapf(err, "failed to create directory %q", modDir)
	}

	// If modfile already exists, delete it and create a fresh one to be safe since
	// it's likely a leftover that wasn't cleaned up properly
	if util.FileOrDirExists(modfilePath) {
		err := os.Remove(modfilePath)
		if err != nil {
			return t, errors.Wrapf(err, "failed to remove file %q", modfilePath)
		}
	}

	// Create empty go.mod file so we can download the tool
	// Can just use _ as the module name since this is a "fake" module
	err = createGoModFile("_", modDir)
	if err != nil {
		return t, err
	}

	// Download the module source. This will do the heavy lifting to figure out
	// the correct version.
	err = c.goClient.GetD(t.Module(), modDir)
	if err != nil {
		return t, err
	}

	// Need to read go.mod file so we can figure out what version was installed
	data, err := ioutil.ReadFile(modfilePath)
	if err != nil {
		return t, errors.Wrapf(err, "failed to read file %q", modfilePath)
	}

	modFile, err := modfile.Parse(modfilePath, data, nil)
	if err != nil {
		return t, errors.Wrapf(err, "failed to parse go.mod file %q", modfilePath)
	}

	// There should only be a single require, otherwise we have a bug
	if len(modFile.Require) != 1 {
		return t, errors.Errorf("expected 1 required statement in go.mod, found %d", len(modFile.Require))
	}
	t.Version = modFile.Require[0].Mod.Version

	// We got the version, now we need to rename the dir so it includes the version
	vfp, err := t.Filepath()
	if err != nil {
		return t, err
	}

	modVersionDir := filepath.Join(c.toolsDir(), vfp)
	if util.FileOrDirExists(modVersionDir) {
		// This version was already installed
		// We can leave the current dir, since future latest installs will
		// make use of it
		return t, nil
	}

	err = os.Rename(modDir, modVersionDir)
	if err != nil {
		return t, errors.Wrapf(err, "failed to rename %q to %q", modDir, modVersionDir)
	}

	c.logger.WithFields(logrus.Fields{
		"tool": t,
		"path": modVersionDir,
	}).Debug("downloaded tool")
	return t, nil
}

// ToolPath returns the absolute path the the installed binary for the given tool.
// If the binary cannot be found, an error is returned.
func (c *Cache) ToolPath(t tool.Tool) (string, error) {
	baseDir := c.toolsDir()
	bfp, err := t.BinaryFilepath()
	if err != nil {
		return "", err
	}

	binPath := filepath.Join(baseDir, bfp)
	if !util.FileOrDirExists(binPath) {
		return "", errors.Errorf("binary for tool %s does not exist", t)
	}
	return binPath, nil
}
