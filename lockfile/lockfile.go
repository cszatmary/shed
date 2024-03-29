// Package lockfile provides mechanisms for creating and modifying shed lockfiles.
package lockfile

import (
	"encoding/json"
	"fmt"
	"io"
	"path"

	"github.com/cszatmary/shed/errors"
	"github.com/cszatmary/shed/tool"
)

// ErrNotFound is returned when a tool is not found in a lockfile.
var ErrNotFound = errors.Str("lockfile: tool not found")

// ErrIncorrectVersion is returned when the version of a tool found
// is different then the version requested.
var ErrIncorrectVersion = errors.Str("lockfile: incorrect version of tool")

// ErrMultipleTools indicates that multiple tools with the same name were found
// in the lockfile.
var ErrMultipleTools = errors.Str("lockfile: multiple tools found with the same name")

// ErrInvalidVersion is returned when adding a tool to a lockfile that does not have a
// valid SemVer. The version in a lockfile must be an exact version, it cannot be
// a module query (ex: branch name or commit SHA) or a shorthand version.
var ErrInvalidVersion = errors.Str("lockfile: tool has invalid version")

// Lockfile represents a shed lockfile. The lockfile is responsible for keeping
// track of installed tools as well as their versions so shed can always
// re-install the same version of each tool.
//
// A zero value Lockfile is a valid empty lockfile ready for use.
type Lockfile struct {
	// tools stores the tools managed by this lockfile.
	// Tools are not stored in any particular order, and the
	// order can change as tools are added and deleted.
	tools []tool.Tool
	// nameMap is a map of tool binary names to a list of indices
	// for each matching tool in tools. This allows for quick lookup
	// of a tool by its binary name instead of needing to do a linear
	// search through tools. This also allows for quickly determining
	// if multiple tools exist with the same binary name, in which
	// case the full import path is required to retrieve the tool.
	nameMap map[string][]int
}

// LenTools returns the number of tools stored in the lockfile.
func (lf *Lockfile) LenTools() int {
	return len(lf.tools)
}

// GetTool retrieves the tool with the given name from the lockfile.
// Name can either be the name of the tool itself (i.e. the name of the binary)
// or it can be the full import path.
//
// If no tool is found, ErrNotFound is returned. If the name is a full import path
// and it contains a version, then the version will be checked against the tool found.
// If the versions do not match, then ErrIncorrectVersion will be returned along with
// the found version of the tool.
func (lf *Lockfile) GetTool(name string) (tool.Tool, error) {
	// Fast way, assume the name is just the tool name and see if we get a match
	bucket, ok := lf.nameMap[name]
	if ok {
		// Tool names must be unique to use the shorthand, otherwise we have no idea
		// which tool was intended
		if len(bucket) > 1 {
			err := fmt.Errorf("%w: %d tools named %s found", ErrMultipleTools, len(bucket), name)
			return tool.Tool{}, err
		}
		return lf.tools[bucket[0]], nil
	}

	// Check if it was short name so we can report not found instead of trying to parse
	if path.Base(name) == name {
		return tool.Tool{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	// Long way, parse the tool name which should be an import path
	tl, err := tool.ParseLax(name)
	if err != nil {
		return tool.Tool{}, err
	}

	toolName := tl.Name()
	bucket, ok = lf.nameMap[toolName]
	if !ok {
		return tool.Tool{}, fmt.Errorf("%w: %s", ErrNotFound, toolName)
	}

	for _, ti := range bucket {
		t := lf.tools[ti]
		if t.ImportPath != tl.ImportPath {
			continue
		}
		if tl.Version != "" && tl.Version != t.Version {
			return t, fmt.Errorf("%w: wanted %s", ErrIncorrectVersion, tl.Version)
		}
		return t, nil
	}
	return tool.Tool{}, fmt.Errorf("%w: %s", ErrNotFound, toolName)
}

// PutTool adds or replaces the given tool in the lockfile.
//
// t.Version must be a valid SemVer, that is t.HasSemver() must return true.
// If t.Version is not a valid SemVer, ErrInvalidVersion will be returned.
func (lf *Lockfile) PutTool(t tool.Tool) error {
	if lf.nameMap == nil {
		lf.nameMap = make(map[string][]int)
	}

	// Invariant check: A tool inserted into the lockfile must have Version set to
	// a valid SemVer otherwise it defeats the purpose of a lockfile.
	if !t.HasSemver() {
		return fmt.Errorf("%w: %v", ErrInvalidVersion, t)
	}

	toolName := t.Name()
	// Don't need to check whether or not the bucket exists. If it doesn't we will get
	// back a nil slice which we can append to
	bucket := lf.nameMap[toolName]

	// Check if the tool already exists
	foundIndex := -1
	for _, ti := range bucket {
		tl := lf.tools[ti]
		if tl.ImportPath == t.ImportPath {
			foundIndex = ti
			break
		}
	}

	// If an existing tool was found then easy, just update it
	if foundIndex != -1 {
		lf.tools[foundIndex] = t
		return nil
	}

	// No existing one found, add new one
	// New tool is aways appended to the end so the index is easy
	lf.nameMap[toolName] = append(bucket, len(lf.tools))
	lf.tools = append(lf.tools, t)
	return nil
}

// DeleteTool removes the given tool from the lockfile if it exists.
// If t.Version is not empty, the tool will only be deleted from the lockfile
// if it has the same version. If t.Version is empty, it will be deleted from the
// lockfile regardless of version.
func (lf *Lockfile) DeleteTool(t tool.Tool) {
	toolName := t.Name()
	bucket, ok := lf.nameMap[toolName]
	if !ok {
		return
	}

	foundIndex := -1
	bucketIndex := -1
	for i, ti := range bucket {
		tl := lf.tools[ti]
		if t.ImportPath != tl.ImportPath {
			continue
		}
		if t.Version == "" || t.Version == tl.Version {
			foundIndex = ti
			bucketIndex = i
			break
		}
	}
	if foundIndex == -1 {
		return
	}

	// To efficiently delete, simply replace the the tool at the found index with the last
	// tool, then resize the slice to drop the last element
	lf.tools[foundIndex] = lf.tools[len(lf.tools)-1]
	lf.tools = lf.tools[:len(lf.tools)-1]
	// Use the same technique for the bucket
	bucket[bucketIndex] = bucket[len(bucket)-1]
	bucket = bucket[:len(bucket)-1]

	// If bucket is empty, delete it from the map, since no tools with this name exist anymore
	if len(bucket) == 0 {
		delete(lf.nameMap, toolName)
		return
	}
	lf.nameMap[toolName] = bucket
}

// Iterator allows for iteration over the tools within a Lockfile.
// An iterator provides two methods that can be used for iteration, Next and Value.
// Next advances the iterator to the next element and returns a bool indicating if
// it was successful. Value returns the value at the current index.
//
// The iteration order over a lockfile is not specified and is not guaranteed to be the same
// from one iteration to the next. It is not safe to add or delete tools from a lockfile
// during iteration.
type Iterator struct {
	lf *Lockfile
	i  int
}

// Iter creates a new Iterator that can be used to iterate over the tools in a Lockfile.
func (lf *Lockfile) Iter() *Iterator {
	return &Iterator{lf: lf, i: -1}
}

// Next advances the iterator to the next element. Every call to Value, even the
// first one, must be preceded by a call to Next.
//
// Next returns a bool indicating whether or not a next element exists meaning
// it is safe to call Value.
func (it *Iterator) Next() bool {
	it.i++
	return it.i < len(it.lf.tools)
}

// Value returns the current element in the iterator.
// Value will panic if iteration has finished.
func (it *Iterator) Value() tool.Tool {
	if it.i >= len(it.lf.tools) {
		panic("lockfile.Iterator: out of bounds access")
	}
	return it.lf.tools[it.i]
}

// WriteTo serializes and writes the lockfile to w. It returns the
// number of bytes written and any error that occurred.
func (lf *Lockfile) WriteTo(w io.Writer) (int64, error) {
	// Convert lockfile to format that can be serialized into JSON
	lfSchema := lockfileSchema{Tools: make(map[string]toolSchema)}
	for _, t := range lf.tools {
		lfSchema.Tools[t.ImportPath] = toolSchema{Version: t.Version}
	}

	data, err := json.MarshalIndent(lfSchema, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("lockfile: failed to serialize as JSON: %w", err)
	}

	n, err := w.Write(data)
	if err != nil {
		return int64(n), err
	}

	// All bytes should have been written if no error, by definition of
	// io.Writer. io.ErrShortWrite must be returned in this case.
	if n != len(data) {
		return int64(n), io.ErrShortWrite
	}
	return int64(n), nil
}

type toolSchema struct {
	Version string `json:"version"`
}

type lockfileSchema struct {
	Tools map[string]toolSchema `json:"tools"`
}

// Parse reads from r and parses the data into a Lockfile struct.
func Parse(r io.Reader) (*Lockfile, error) {
	lfSchema := lockfileSchema{}
	err := json.NewDecoder(r).Decode(&lfSchema)
	if err != nil {
		return nil, fmt.Errorf("lockfile: failed to deserialize JSON: %w", err)
	}

	lf := &Lockfile{nameMap: make(map[string][]int)}
	// Parse all the tools in the lockfile. If errors are encountered, save
	// them and continue. This way multiple errors can be reported at once.
	var errs errors.List
	for importPath, tlSchema := range lfSchema.Tools {
		t, err := tool.Parse(importPath + "@" + tlSchema.Version)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		toolName := t.Name()
		bucket := lf.nameMap[toolName]
		lf.nameMap[toolName] = append(bucket, len(lf.tools))
		lf.tools = append(lf.tools, t)
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return lf, nil
}
