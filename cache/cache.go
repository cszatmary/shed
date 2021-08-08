// Package cache handles managing the actual installing of tools.
// It handles downloading and building the go modules.
// Tools are stored in a cache on the OS filesystem so that they can
// be reused by other projects.
package cache

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cszatmary/shed/errors"
	"github.com/cszatmary/shed/internal/util"
	"github.com/cszatmary/shed/tool"
	"github.com/sirupsen/logrus"
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
		logger.Out = io.Discard
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
		return errors.New(errors.IO, "cache clean failed", errors.Op("Cache.Clean"), err)
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
//
// The provided context is used to terminate the install if the context becomes
// done before the install completes on its own.
func (c *Cache) Install(ctx context.Context, t tool.Tool) (tool.Tool, error) {
	const op = errors.Op("Cache.Install")
	select {
	case <-ctx.Done():
		return t, ctx.Err()
	default:
	}

	// Make sure import path is set as it's required for download
	if t.ImportPath == "" {
		return t, errors.New(errors.Internal, "import path is missing from tool")
	}

	// Download step

	downloadedTool, err := c.download(ctx, op, t)
	if err != nil {
		return t, errors.New(fmt.Sprintf("failed to download tool %s", t), op, err)
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

	err = c.goClient.Build(ctx, downloadedTool.ImportPath, binPath, binDir)
	if err != nil {
		return downloadedTool, errors.New(fmt.Sprintf("failed to build tool %s", downloadedTool), op, err)
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
func (c *Cache) download(ctx context.Context, op errors.Op, t tool.Tool) (tool.Tool, error) {
	// Get the path to where the tool will be installed. This is where the go.mod file will be.
	fp, err := t.Filepath()
	if err != nil {
		return t, err
	}
	modDir := filepath.Join(c.toolsDir(), fp)
	modfilePath := filepath.Join(modDir, "go.mod")

	// If we have the version see if the tool already exists and whether or not we need to re-download it.
	// If any validations fail, the tool will be re-downloaded. This allows shed to recover from a bad state.
	if t.HasSemver() {
		modFile, err := readGoModFile(op, errors.BadState, modfilePath, t)
		if modFile != nil {
			// Perform some additional validations specific to download
			mod := modFile.Require[0].Mod
			modfileOk := true
			if t.Version != mod.Version {
				modfileOk = false
				c.logger.WithFields(logrus.Fields{
					"expected": t.Version,
					"received": mod.Version,
				}).Debug("incorrect dependency version go.mod")
			}
			if modfileOk {
				c.logger.WithFields(logrus.Fields{
					"tool": t,
				}).Debug("tool already exists, skipping download")
				return t, nil
			}
			// Invalid modfile, fallthrough to error case below
		}
		if modFile == nil && err == nil {
			c.logger.WithFields(logrus.Fields{
				"tool": t,
			}).Debug("tool does not exist, downloading")
		} else {
			fields := logrus.Fields{"tool": t}
			if err != nil {
				fields["error"] = err
			}
			c.logger.WithFields(fields).Debug("tool exists but issues found, re-downloading")
		}
	}

	// Start download process

	if err := os.MkdirAll(modDir, 0o755); err != nil {
		return t, errors.New(errors.IO, fmt.Sprintf("failed to create directory %q", modDir), op, err)
	}

	// If modfile already exists, delete it and create a fresh one.
	// The existing modfile is either a leftover that wasn't cleaned up properly,
	// or it was found to be invalid above so we need to start from scratch.
	if err := os.RemoveAll(modfilePath); err != nil {
		return t, errors.New(errors.IO, fmt.Sprintf("failed to remove file %q", modfilePath), op, err)
	}

	// Create empty go.mod file so we can download the tool.
	// Can just use _ as the module name since this is a "fake" module.
	if err := createGoModFile(ctx, op, "_", modDir); err != nil {
		return t, err
	}

	// Download the module source. What's nice here is we leverage the power of
	// go get so we don't need to reinvent the module resolution & downloading.
	// Also we can reuse an existing download that's already cached.
	if err := c.goClient.GetD(ctx, t.Module(), modDir); err != nil {
		return t, err
	}

	// Need to read go.mod file so we can figure out what version was installed
	modFile, err := readGoModFile(op, errors.Internal, modfilePath, t)
	if err != nil {
		return t, err
	}
	if modFile == nil {
		// Shouldn't happen, but handle just to be safe.
		return t, errors.New(errors.Internal, fmt.Sprintf("modfile is missing for installed tool %s", t), op)
	}

	version := modFile.Require[0].Mod.Version
	toolDir := modDir
	if t.HasSemver() {
		// Make sure we actually got the version we asked for
		if version != t.Version {
			return t, errors.New(errors.Internal, fmt.Sprintf("incorrect version of tool %s was installed, got %s", t, version), op)
		}
	} else {
		// We got the version, now we need to rename the dir so it includes the version
		t.Version = version
		vfp, err := t.Filepath()
		if err != nil {
			return t, err
		}

		modVersionDir := filepath.Join(c.toolsDir(), vfp)
		if !util.FileOrDirExists(modVersionDir) {
			if err := os.Rename(modDir, modVersionDir); err != nil {
				return t, errors.New(errors.IO, fmt.Sprintf("failed to rename %q to %q", modDir, modVersionDir), op, err)
			}
		}
		// If a dir already exists for this version do nothing.
		// We can leave the current dir since future installs might make use of it.
		toolDir = modVersionDir
	}

	c.logger.WithFields(logrus.Fields{
		"tool": t,
		"path": toolDir,
	}).Debug("downloaded tool")
	return t, nil
}

// ToolPath returns the absolute path the the installed binary for the given tool.
// If the binary cannot be found, an error is returned.
func (c *Cache) ToolPath(t tool.Tool) (string, error) {
	bfp, err := t.BinaryFilepath()
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(c.toolsDir(), bfp)
	if !util.FileOrDirExists(binPath) {
		return "", errors.New(
			errors.NotInstalled,
			fmt.Sprintf("binary for tool %s does not exist", t),
			errors.Op("Cache.ToolPath"),
		)
	}
	return binPath, nil
}

// FindUpdate checks if there is a newer version available for tool t.
// If no newer version is found, an empty string is returned.
func (c *Cache) FindUpdate(ctx context.Context, t tool.Tool) (string, error) {
	const op = errors.Op("Cache.FindUpdate")
	fp, err := t.Filepath()
	if err != nil {
		return "", err
	}

	c.logger.WithFields(logrus.Fields{
		"tool": t,
	}).Debug("finding module that tool belongs to")
	dir := filepath.Join(c.toolsDir(), fp)
	modfilePath := filepath.Join(dir, "go.mod")
	modFile, err := readGoModFile(op, errors.BadState, modfilePath, t)
	if err != nil {
		return "", err
	}
	if modFile == nil {
		return "", errors.New(errors.NotInstalled, fmt.Sprintf("tool %s does not exist", t), op)
	}

	m := modFile.Require[0].Mod
	c.logger.WithFields(logrus.Fields{
		"tool":   t,
		"module": m,
	}).Debug("finding latest version of tool")
	gm, err := c.goClient.ListU(ctx, m.Path, dir)
	if err != nil {
		return "", errors.New(fmt.Sprintf("failed to list module update for %s", m.Path), op, err)
	}
	if gm.Update == nil {
		return "", nil
	}
	return gm.Update.Version, nil
}
