// Package client provides the high level API for using shed.
package client

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/TouchBistro/goutils/file"
	"github.com/cszatmary/shed/cache"
	"github.com/cszatmary/shed/lockfile"
	"github.com/cszatmary/shed/tool"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Shed provides the API for managing tool dependencies with shed.
type Shed struct {
	cache        *cache.Cache
	lf           *lockfile.Lockfile
	lockfilePath string
	logger       logrus.FieldLogger
}

// Options allows for custom configuration of a new Shed instance.
type Options struct {
	// The path to the lockfile. If omitted, it will default to './shed.lock'.
	LockfilePath string
	// A logger to write any debug information to. If omitted, logging will be disabled.
	Logger logrus.FieldLogger
	// The directory where tools should be installed and cached.
	// If omitted it will default to 'os.UserCacheDir/shed'.
	CacheDir string
}

// NewShed creates a new Shed instance.
func NewShed(opts Options) (*Shed, error) {
	if opts.LockfilePath == "" {
		opts.LockfilePath = "shed.lock"
	}

	if opts.Logger == nil {
		// Create a logger that logs to nothing to disable logging
		l := logrus.New()
		l.SetOutput(ioutil.Discard)
		opts.Logger = l
	}

	if opts.CacheDir == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find user cache directory")
		}
		opts.CacheDir = filepath.Join(userCacheDir, "shed")
	}

	lf := &lockfile.Lockfile{}
	if file.FileOrDirExists(opts.LockfilePath) {
		f, err := os.Open(opts.LockfilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open file %s", opts.LockfilePath)
		}
		defer f.Close()

		lf, err = lockfile.Parse(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse lockfile %s", opts.LockfilePath)
		}
	}

	return &Shed{
		cache:        cache.New(opts.CacheDir, opts.Logger),
		lf:           lf,
		lockfilePath: opts.LockfilePath,
		logger:       opts.Logger,
	}, nil
}

// CacheDir returns the OS filesystem directory where the shed cache is located.
func (s *Shed) CacheDir() string {
	return s.cache.Dir()
}

// CleanCache removes the cache directory and all contents from the filesystem.
func (s *Shed) CleanCache() error {
	return s.cache.Clean()
}

func (s *Shed) writeLockfile() error {
	f, err := os.OpenFile(s.lockfilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return errors.Wrapf(err, "failed to create/open file %s", s.lockfilePath)
	}
	defer f.Close()

	_, err = s.lf.WriteTo(f)
	if err != nil {
		return errors.Wrapf(err, "failed to write lockfile to %s", s.lockfilePath)
	}
	return nil
}

// Install installs zero or more given tools and add them to the lockfile.
// It also checks if any tools in the lockfile are not installed and installs
// them if so.
//
// If a tool name is provided with a version and the same tool already exists in the
// lockfile with a different version, then Install will return an error, unless allowUpdates
// is set in which case the given tool version will overwrite the one in the lockfile.
func (s *Shed) Install(allowUpdates bool, toolNames ...string) error {
	// Collect all the tools that need to be installed.
	// Merge the given tools with what exists in the lockfile.
	seenTools := make(map[string]bool)
	var tools []tool.Tool

	var errs lockfile.ErrorList
	for _, toolName := range toolNames {
		t, err := tool.Parse(toolName)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		existingTool, err := s.lf.GetTool(t.ImportPath)
		switch {
		case errors.Is(err, lockfile.ErrNotFound):
			// New tool, will be installed
		case errors.Is(err, lockfile.ErrIncorrectVersion):
			if !allowUpdates {
				err := errors.Errorf("trying to install version %s but lockfile has version %s for tool %s", t.Version, existingTool.Version, t.ImportPath)
				errs = append(errs, err)
				continue
			}
		default:
			// Shouldn't happen, but handle just to be safe
			return errors.WithMessagef(err, "failed to check if tool exists in lockfile: %s", t)
		}
		seenTools[t.ImportPath] = true
		tools = append(tools, t)
	}

	if len(errs) > 0 {
		return errs
	}

	// Take union with lockfile
	it := s.lf.Iter()
	for it.Next() {
		t := it.Value()
		if ok := seenTools[t.ImportPath]; !ok {
			tools = append(tools, t)
		}
	}

	// Sort the tools so they are always installed in the same order
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].ImportPath < tools[j].ImportPath
	})

	for _, t := range tools {
		s.logger.Debugf("Installing tool: %v", t)
		installedTool, err := s.cache.Install(t)
		if err != nil {
			return errors.WithMessagef(err, "failed to install tool %s", t)
		}
		s.lf.PutTool(installedTool)
	}

	if err := s.writeLockfile(); err != nil {
		return err
	}
	return nil
}

// ToolPath returns the absolute path to the binary of the tool if it is installed.
// If the tool cannot be found, or toolName is invalid, an error will be returned.
func (s *Shed) ToolPath(toolName string) (string, error) {
	t, err := s.lf.GetTool(toolName)
	if err != nil {
		return "", err
	}
	return s.cache.ToolPath(t)
}
