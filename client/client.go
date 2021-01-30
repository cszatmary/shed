// Package client provides the high level API for using shed.
package client

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/getshiphub/shed/cache"
	"github.com/getshiphub/shed/lockfile"
	"github.com/getshiphub/shed/tool"
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

// NewShed creates a new Shed instance. Options can be provided to customize the created Shed instance.
//
// By default, the lockfile path used is './shed.lock' and the cache directory is 'os.UserCacheDir()/shed'.
func NewShed(opts ...Option) (*Shed, error) {
	s := &Shed{}
	for _, opt := range opts {
		opt(s)
	}

	// Set defaults
	if s.lockfilePath == "" {
		s.lockfilePath = "shed.lock"
	}
	if s.logger == nil {
		// Logging is disabled by default, but we don't want to have to check
		// for nil all the time, so create a logger that logs to nowhere
		logger := logrus.New()
		logger.Out = ioutil.Discard
		s.logger = logger
	}
	if s.cache == nil {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			return nil, errors.Wrap(err, "failed to find user cache directory")
		}
		s.cache = cache.New(filepath.Join(userCacheDir, "shed"), cache.WithLogger(s.logger))
	}

	f, err := os.Open(s.lockfilePath)
	if os.IsNotExist(err) {
		// No lockfile, create an empty one
		s.lf = &lockfile.Lockfile{}
		return s, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %s", s.lockfilePath)
	}
	defer f.Close()

	s.lf, err = lockfile.Parse(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse lockfile %s", s.lockfilePath)
	}
	return s, nil
}

// Option is a function that takes a Shed instance and applies a configuration to it.
type Option func(*Shed)

// WithLockfilePath sets the path to lockfile.
func WithLockfilePath(lfp string) Option {
	return func(s *Shed) {
		s.lockfilePath = lfp
	}
}

// WithLogger sets a logger that should be used for writing debug messages.
// By default no logging is done.
func WithLogger(logger logrus.FieldLogger) Option {
	return func(s *Shed) {
		s.logger = logger
	}
}

// WithCache sets the Cache instance to use for installing tools.
func WithCache(c *cache.Cache) Option {
	return func(s *Shed) {
		s.cache = c
	}
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
	if _, err = s.lf.WriteTo(f); err != nil {
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
		// This also serves to validate the the given tool name is a valid module name
		// Use ParseLax since the version might be a query that should be passed to go get.
		t, err := tool.ParseLax(toolName)
		if err != nil {
			errs = append(errs, errors.WithMessagef(err, "invalid tool name %s", toolName))
			continue
		}

		existing, err := s.lf.GetTool(toolName)
		if errors.Is(err, lockfile.ErrIncorrectVersion) {
			if !allowUpdates {
				err := errors.Errorf("trying to install %s, but %s is in the lockfile", t, existing.Version)
				errs = append(errs, err)
				continue
			}
		} else if err != nil && !errors.Is(err, lockfile.ErrNotFound) {
			// Shouldn't happen, but handle just to be safe
			errs = append(errs, errors.WithMessagef(err, "failed to check if tool exists in lockfile: %s", t))
			continue
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

// Uninstall uninstalls the given tools. This only removes them from the lockfile.
// The actual tool binaries are not removed, since they might be used by other projects.
// To remove the actual binaries, use CleanCache.
func (s *Shed) Uninstall(toolNames ...string) error {
	var tools []tool.Tool
	var errs lockfile.ErrorList
	for _, toolName := range toolNames {
		t, err := s.lf.GetTool(toolName)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		tools = append(tools, t)
	}
	if len(errs) > 0 {
		return errs
	}

	for _, t := range tools {
		s.logger.Debugf("Uninstalling tool: %v", t)
		s.lf.DeleteTool(t)
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
