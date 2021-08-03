// Package client provides the high level API for using shed.
package client

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/getshiphub/shed/cache"
	"github.com/getshiphub/shed/internal/util"
	"github.com/getshiphub/shed/lockfile"
	"github.com/getshiphub/shed/tool"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const LockfileName = "shed.lock"

// noneVersion is a special module version that signifies the module should be removed.
const noneVersion = "none"

// ResolveLockfilePath resolves the path to the nearest shed lockfile starting at dir.
// It will keep searching parent directories until either a lockfile is found,
// or the root directory is reached. If no lockfile is found, an empty string will be returned.
func ResolveLockfilePath(dir string) string {
	// "" is synonymous with "."
	// This makes sure we do at least one check in the current directory
	if dir == "" {
		dir = "."
	}
	var prev string
	for dir != prev {
		p := filepath.Join(dir, LockfileName)
		if util.FileOrDirExists(p) {
			return p
		}
		prev = dir
		dir = filepath.Dir(dir)
	}
	return ""
}

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
		s.lockfilePath = LockfileName
	}
	if s.logger == nil {
		// Logging is disabled by default, but we don't want to have to check
		// for nil all the time, so create a logger that logs to nowhere
		logger := logrus.New()
		logger.Out = io.Discard
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

// Install computes a set of tools that should be installed. It can be given zero or
// more tools as arguments. These will be unioned with the tools in the lockfile
// to produce a final set of tools to install. Install will return an InstallSet instance
// which can be used to perform the actual installation.
//
// Install does not modify any state, therefore, if you wish to abort the install simply
// discard the returned InstallSet.
//
// All tool names provided must be full import paths, not binary names.
// If a tool name is invalid, Install will return an error.
func (s *Shed) Install(toolNames ...string) (*InstallSet, error) {
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
		seenTools[t.ImportPath] = true
		tools = append(tools, t)
	}
	if len(errs) > 0 {
		return nil, errs
	}

	// Take union with lockfile
	it := s.lf.Iter()
	for it.Next() {
		t := it.Value()
		if ok := seenTools[t.ImportPath]; !ok {
			tools = append(tools, t)
		}
	}
	return &InstallSet{s: s, tools: tools}, nil
}

// InstallSet represents a set of tools that are to be installed.
// To perform the installation call the Apply method.
// To abort the install, simply discard the InstallSet object.
type InstallSet struct {
	s        *Shed
	tools    []tool.Tool
	notifyCh chan<- tool.Tool
}

// Len returns the number of tools in the InstallSet.
func (is *InstallSet) Len() int {
	return len(is.tools)
}

// Notify causes the InstallSet to relay completed actions to ch.
// This is useful to keep track of the progress of installation.
// You should receive from ch on a separate goroutine than the one that
// Apply is called on, since Apply will block until all tools are installed.
func (is *InstallSet) Notify(ch chan<- tool.Tool) {
	is.notifyCh = ch
}

// Apply will install each tool in the InstallSet and add them to the lockfile.
//
// The provided context is used to terminate the install if the context becomes
// done before the install completes on its own.
func (is *InstallSet) Apply(ctx context.Context) error {
	successCh := make(chan tool.Tool)
	failedCh := make(chan error)
	for _, tl := range is.tools {
		go func(t tool.Tool) {
			// go get supports the special version suffix '@none' which means remove the module.
			// See https://golang.org/ref/mod#go-get for more details.
			// Support this for consistency since we want to shed to just work with all module queries.
			if t.Version == noneVersion {
				is.s.logger.Debugf("Uninstalling tool: %s", t.ImportPath)
				successCh <- t
				return
			}

			is.s.logger.Debugf("Installing tool: %v", t)
			installed, err := is.s.cache.Install(ctx, t)
			if err != nil {
				failedCh <- errors.WithMessagef(err, "failed to install tool %s", t)
				return
			}
			successCh <- installed
		}(tl)
	}

	var completedTools []tool.Tool
	var errs lockfile.ErrorList
	for i := 0; i < len(is.tools); i++ {
		select {
		case t := <-successCh:
			completedTools = append(completedTools, t)
			if is.notifyCh != nil {
				is.notifyCh <- t
			}
		case err := <-failedCh:
			// Continue even if a tool failed because they are cached so it will
			// save work on subsequent runs.
			errs = append(errs, err)
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "installation was aborted")
		}
	}
	if len(errs) > 0 {
		return errs
	}

	for _, t := range completedTools {
		if t.Version == noneVersion {
			// Uninstall the tool by removing it from the lockfile.
			// Unlike Uninstall() this will not error if the tool is not in the lockfile,
			// instead it will be silently ignored.
			t.Version = ""
			is.s.lf.DeleteTool(t)
			continue
		}
		if err := is.s.lf.PutTool(t); err != nil {
			return errors.Wrapf(err, "failed to add tool %v to lockfile", t)
		}
	}
	if err := is.s.writeLockfile(); err != nil {
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

// ListOptions is used to configure Shed.List.
type ListOptions struct {
	// ShowUpdates makes List check if a newer version of each tool is available.
	ShowUpdates bool
}

// ToolInfo contains information about a tool returned by Shed.List.
type ToolInfo struct {
	// Tool contains the details of the installed tool.
	Tool tool.Tool
	// LatestVersion specifies the latest version of the tool
	// if ShowUpdates was set to true and a newer version was found.
	// Otherwise it is an empty string.
	LatestVersion string
}

// List returns a list of all the tools specified in the lockfile.
// opts can be used to customize how List behaves.
func (s *Shed) List(ctx context.Context, opts ListOptions) ([]ToolInfo, error) {
	var tools []ToolInfo
	it := s.lf.Iter()
	for it.Next() {
		info := ToolInfo{Tool: it.Value()}
		if opts.ShowUpdates {
			latest, err := s.cache.FindUpdate(ctx, info.Tool)
			if err != nil {
				return nil, err
			}
			info.LatestVersion = latest
		}
		tools = append(tools, info)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Tool.ImportPath < tools[j].Tool.ImportPath
	})
	return tools, nil
}
