// Package client provides the high level API for using shed.
package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/cszatmary/shed/cache"
	"github.com/cszatmary/shed/errors"
	"github.com/cszatmary/shed/internal/util"
	"github.com/cszatmary/shed/lockfile"
	"github.com/cszatmary/shed/tool"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

const LockfileName = "shed.lock"

const (
	// noneVersion is a special module version that signifies the module should be removed.
	noneVersion = "none"
	// latestVersion is a special module version that signifies the latest
	// available version should be installed.
	latestVersion = "latest"
)

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
	const op = errors.Op("client.NewShed")
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
			return nil, errors.New(errors.Invalid, "unable to find user cache directory", op, err)
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
		return nil, errors.New(errors.IO, fmt.Sprintf("failed to open file %q", s.lockfilePath), op, err)
	}
	defer f.Close()

	s.lf, err = lockfile.Parse(f)
	if err != nil {
		return nil, errors.New(errors.Internal, fmt.Sprintf("failed to parse lockfile %q", s.lockfilePath), op, err)
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

func (s *Shed) writeLockfile(op errors.Op) error {
	f, err := os.OpenFile(s.lockfilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return errors.New(errors.IO, fmt.Sprintf("failed to create/open file %q", s.lockfilePath), op, err)
	}
	defer f.Close()
	if _, err = s.lf.WriteTo(f); err != nil {
		return errors.New(errors.Internal, fmt.Sprintf("failed to write lockfile to %q", s.lockfilePath), op, err)
	}
	return nil
}

// GetOptions is used to configure Shed.Get.
type GetOptions struct {
	// ToolNames is a list of tools that should be installed.
	// These will be unioned with the tools specified in the lockfile.
	ToolNames []string
	// Update sets whether or not tools should be updated to the latest available
	// minor or patch version. If ToolNames is not empty, only those tools will be
	// updated. Otherwise, all tools in the lockfile will be updated.
	Update bool
}

// Get computes a set of tools that should be installed. Zero or more tools can be
// specified in opts. These will be unioned with the tools in the lockfile
// to produce a final set of tools to install. Get will return an InstallSet instance
// which can be used to perform the actual installation.
//
// Get does not modify any state, therefore, if you wish to abort the install simply
// discard the returned InstallSet.
//
// All tool names provided must be full import paths, not binary names.
// If a tool name is invalid, Get will return an error.
//
// If opts.Update is set, tool names must not include version suffixes.
func (s *Shed) Get(opts GetOptions) (*InstallSet, error) {
	const op = errors.Op("Shed.Get")
	// Collect all the tools that need to be installed.
	// Merge the given tools with what exists in the lockfile.
	seenTools := make(map[string]bool)
	var tools []tool.Tool

	var errs errors.List
	for _, toolName := range opts.ToolNames {
		// This also serves to validate the the given tool name is a valid module name
		// Use ParseLax since the version might be a query that should be passed to go get.
		t, err := tool.ParseLax(toolName)
		if err != nil {
			errs = append(errs, errors.New(fmt.Sprintf("invalid tool name %s", toolName), op, err))
			continue
		}
		if opts.Update {
			// Version is not allowed if updating, since the latest version will be installed.
			if t.Version != "" && t.Version != noneVersion && t.Version != latestVersion {
				msg := fmt.Sprintf("tool %s must not have a version when updating", t)
				errs = append(errs, errors.New(errors.Invalid, msg, op))
				continue
			}
			t.Version = latestVersion
		}
		seenTools[t.ImportPath] = true
		tools = append(tools, t)
	}
	if len(errs) > 0 {
		return nil, errs
	}

	// If update and no tools provided update all in the lockfile.
	updateAll := opts.Update && len(opts.ToolNames) == 0
	// Take union with lockfile
	it := s.lf.Iter()
	for it.Next() {
		t := it.Value()
		if ok := seenTools[t.ImportPath]; ok {
			continue
		}
		// Skip tools with a prelease version installed since the latest version might
		// actually be older than the current version which was explicitly installed.
		if updateAll && semver.Prerelease(t.Version) == "" {
			t.Version = latestVersion
		}
		tools = append(tools, t)
	}
	return &InstallSet{s: s, tools: tools}, nil
}

// InstallSet represents a set of tools that are to be installed.
// To perform the installation call the Apply method.
// To abort the install, simply discard the InstallSet object.
type InstallSet struct {
	// Concurrency sets the amount of installs that will run concurrently.
	// It defaults to the number of CPUs available.
	Concurrency uint

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
	const op = errors.Op("InstallSet.Apply")

	type result struct {
		t   tool.Tool
		err error
	}
	resultCh := make(chan result, len(is.tools))
	concurrency := getConcurrency(is.Concurrency)
	is.s.logger.Debugf("Using concurrency %d", concurrency)
	semCh := make(chan struct{}, concurrency)
	for _, tl := range is.tools {
		semCh <- struct{}{}
		go func(t tool.Tool) {
			defer func() {
				<-semCh
			}()

			// go get supports the special version suffix '@none' which means remove the module.
			// See https://golang.org/ref/mod#go-get for more details.
			// Support this for consistency since we want to shed to just work with all module queries.
			if t.Version == noneVersion {
				is.s.logger.Debugf("Uninstalling tool: %s", t.ImportPath)
				resultCh <- result{t: t}
				return
			}

			is.s.logger.Debugf("Installing tool: %v", t)
			installed, err := is.s.cache.Install(ctx, t)
			if err != nil {
				resultCh <- result{err: errors.New(fmt.Sprintf("failed to install tool %s", t), op, err)}
				return
			}
			resultCh <- result{t: installed}
		}(tl)
	}

	var completedTools []tool.Tool
	var errs errors.List
	for i := 0; i < len(is.tools); i++ {
		select {
		case r := <-resultCh:
			if r.err != nil {
				// Continue even if a tool failed because they are cached so it will
				// save work on subsequent runs.
				errs = append(errs, r.err)
				continue
			}
			completedTools = append(completedTools, r.t)
			if is.notifyCh != nil {
				is.notifyCh <- r.t
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if len(errs) > 0 {
		return errs
	}

	for _, t := range completedTools {
		if t.Version == noneVersion {
			// Uninstall the tool by removing it from the lockfile.
			// This will not error if the tool is not in the lockfile,
			// instead it will be silently ignored.
			t.Version = ""
			is.s.lf.DeleteTool(t)
			continue
		}
		if err := is.s.lf.PutTool(t); err != nil {
			return errors.New(errors.Internal, fmt.Sprintf("failed to add tool %s to lockfile", t), op, err)
		}
	}
	if err := is.s.writeLockfile(op); err != nil {
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
	// Concurrency sets the amount of update checks that will happen
	// concurrently when ShowUpdates is true.
	// It defaults to the number of CPUs available.
	Concurrency uint
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
	// If not checking updates, then skip any concurrency
	if !opts.ShowUpdates {
		var tools []ToolInfo
		it := s.lf.Iter()
		for it.Next() {
			tools = append(tools, ToolInfo{Tool: it.Value()})
		}
		sort.Slice(tools, func(i, j int) bool {
			return tools[i].Tool.ImportPath < tools[j].Tool.ImportPath
		})
		return tools, nil
	}

	// Create a cancel context so we can bail and stop any remaining
	// update checks if one fails
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		info ToolInfo
		err  error
	}
	resultCh := make(chan result, s.lf.LenTools())
	concurrency := getConcurrency(opts.Concurrency)
	s.logger.Debugf("Using concurrency %d", concurrency)
	semCh := make(chan struct{}, concurrency)
	it := s.lf.Iter()
	for it.Next() {
		semCh <- struct{}{}
		go func(t tool.Tool) {
			defer func() {
				<-semCh
			}()

			latest, err := s.cache.FindUpdate(ctx, t)
			if err != nil {
				resultCh <- result{err: err}
				return
			}
			resultCh <- result{info: ToolInfo{Tool: t, LatestVersion: latest}}
		}(it.Value())
	}

	var tools []ToolInfo
	for i := 0; i < s.lf.LenTools(); i++ {
		select {
		case r := <-resultCh:
			if r.err != nil {
				// Stop any update checks that are still in progress
				cancel()
				return nil, r.err
			}
			tools = append(tools, r.info)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Tool.ImportPath < tools[j].Tool.ImportPath
	})
	return tools, nil
}

// getConcurrency returns either concurrency or the number of CPUs if
// concurrency is 0. If the number of CPUs cannot be determined,
// 1 will be returned.
func getConcurrency(concurrency uint) uint {
	if concurrency != 0 {
		return concurrency
	}
	numCPUs := runtime.NumCPU()
	// Check for negative number just to be safe since the type is int.
	// Better safe than sorry and having an overflow.
	if numCPUs > 0 {
		return uint(numCPUs)
	}
	// If we get here somehow just execute everything serially.
	return 1
}
