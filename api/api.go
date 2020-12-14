package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cszatmary/shed/cache"
	"github.com/cszatmary/shed/lock"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// This package contains the high level API for using shed.
// This delegates to the lower level cache and lock APIs to
// actually perform the necessary actions.

var setupRun = false

// setup peforms any required setup before other functions can be used.
// This function is idempotent, multiple calls will no-op.
func setup() error {
	if setupRun {
		return nil
	}

	if err := cache.Init(); err != nil {
		return err
	}

	setupRun = true
	return nil
}

// Install installs zero or more given tools and add them to the lockfile.
// It also checks if any tools in the lockfile are not installed and installs
// them if so.
// If a tool name is provided with a version and the same tool already exists in the
// lockfile with a different version, then Install will return an error, unless allowUpdates
// is set in which case the given tool version will overwrite the one in the lockfile.
func Install(allowUpdates bool, toolNames ...string) error {
	if err := setup(); err != nil {
		return err
	}

	lockfile, err := lock.Read()
	if err != nil {
		return err
	}

	tools := make(map[string]lock.Tool)
	var errMessages []string
	for _, toolName := range toolNames {
		t, err := lock.ParseTool(toolName)
		if err != nil {
			errMessages = append(errMessages, err.Error())
			continue
		}

		if existingTool, ok := lockfile.Tools[t.ImportPath]; ok {
			if t.Version != "" && t.Version != existingTool.Version && !allowUpdates {
				msg := fmt.Sprintf("trying to install version %s but lockfile has version %s for tool %s", t.Version, existingTool.Version, t.ImportPath)
				errMessages = append(errMessages, msg)
				continue
			}
		}

		tools[t.ImportPath] = t
	}

	if len(errMessages) > 0 {
		return errors.Errorf("errors encountered in tool names: %s", strings.Join(errMessages, "\n"))
	}

	// Install also installs any missing tools that are present in the lockfile
	for _, t := range lockfile.Tools {
		if _, ok := tools[t.ImportPath]; ok {
			continue
		}

		tools[t.ImportPath] = t
	}

	var modules []cache.Module
	for _, t := range tools {
		modules = append(modules, cache.Module{
			ImportPath: t.ImportPath,
			Version:    t.Version,
			BinaryName: t.Name(),
		})
	}

	// Sort so they are always installed in the same order
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].ImportPath < modules[j].ImportPath
	})

	for i, m := range modules {
		log.Debugf("Installing tool: %v", m)
		installedModule, err := cache.Install(m)
		if err != nil {
			return errors.WithMessagef(err, "failed to install %s", m)
		}
		modules[i] = installedModule
	}

	// Update lockfile
	for _, m := range modules {
		t, ok := tools[m.ImportPath]
		if !ok {
			// This is a bug
			return errors.Errorf("tool %v not found, this is a bug", m.ImportPath)
		}

		t.Version = m.Version
		lockfile.Tools[t.ImportPath] = t
	}

	err = lock.Write(lockfile)
	if err != nil {
		return err
	}

	return nil
}

func BinaryPath(toolName string) (string, error) {
	if err := setup(); err != nil {
		return "", err
	}

	lockfile, err := lock.Read()
	if err != nil {
		return "", err
	}

	// Tool name can either be the full import path, or just the binary name
	tool, ok := lockfile.Tools[toolName]
	if !ok {
		// Loop through and find any matching binary names
		// If we find more than 1 tool with the same binary name then this is
		// an error as we can't know which one the user intended
		var foundTools []lock.Tool
		for _, t := range lockfile.Tools {
			if t.Name() == toolName {
				foundTools = append(foundTools, t)
			}
		}

		// TODO(@cszatmary): Figure out how to do better error messaging
		if len(foundTools) == 0 {
			return "", errors.Errorf("no tool named %s found", toolName)
		} else if len(foundTools) > 1 {
			return "", errors.Errorf("multiple tools named %s found", toolName)
		}

		tool = foundTools[0]
	}

	m := cache.Module{ImportPath: tool.ImportPath, Version: tool.Version, BinaryName: tool.Name()}
	binPath, err := m.BinaryPath()
	if err != nil {
		return "", err
	}

	return binPath, nil
}
