package cache

// The cache package is responsible to managing the actual installing of tools.
// It handles downloading and building the go modules.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TouchBistro/goutils/file"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

var (
	// rootDir is the root directory that shed manages
	// this will be CACHE_DIR/shed where CACHE_DIR
	// is the OS specific cache directory retrieved by
	// os.UserCacheDir
	rootDir string
	binDir  string
	srcDir  string
)

// Dir returns the absolute path to the cache dir on the OS filesystem.
// This is OS dependent and the location is based on os.UserCacheDir.
func Dir() (string, error) {
	// TODO(@cszatmary): This is a smell. Cache should be redesigned
	// to not required Init to be called before using. Hidden interfaces are awkward.
	if rootDir == "" {
		err := Init()
		if err != nil {
			return "", err
		}
	}

	return rootDir, nil
}

// Init loads and resolves all necessary cache information.
// It also ensures all required directories exist in the OS filesystem.
func Init() error {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return errors.Wrapf(err, "failed to find user cache directory")
	}
	log.WithFields(log.Fields{
		"userCacheDir": userCacheDir,
	}).Debug("Resolved user cache dir")

	rootDir = filepath.Join(userCacheDir, "shed")
	binDir = filepath.Join(rootDir, "bin")
	srcDir = filepath.Join(rootDir, "src")
	log.WithFields(log.Fields{
		"cacheDir": rootDir,
		"binDir":   binDir,
		"srcDir":   srcDir,
	}).Debug("Resolved shed cache dirs")

	// Make sure dirs exist
	// Don't need an explicit check for rootDir, because if bin & src don't exist,
	// they will be created mkdir -p style, which will create rootDir if it's missing
	if !file.FileOrDirExists(binDir) {
		err := os.MkdirAll(binDir, 0o755)
		if err != nil {
			return errors.Wrapf(err, "failed to create directory %q", binDir)
		}

		log.WithFields(log.Fields{
			"binDir": binDir,
		}).Debug("Created bin dir")
	}

	if !file.FileOrDirExists(srcDir) {
		err := os.MkdirAll(srcDir, 0o755)
		if err != nil {
			return errors.Wrapf(err, "failed to create directory %q", srcDir)
		}

		log.WithFields(log.Fields{
			"srcDir": srcDir,
		}).Debug("Created src dir")
	}

	return nil
}

// Module represents the details of an installed Go module.
type Module struct {
	Version     string
	ImportPath  string
	BinaryName  string
	relativeDir string
}

func (m Module) String() string {
	if m.Version == "" {
		return m.ImportPath
	}

	return fmt.Sprintf("%s@%s", m.ImportPath, m.Version)
}

func (m Module) BinaryPath() (string, error) {
	if m.relativeDir != "" {
		modBinDir := filepath.Join(binDir, m.relativeDir)
		return filepath.Join(modBinDir, m.BinaryName), nil
	}

	escapedPath, err := module.EscapePath(m.ImportPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to escape path %q", m.ImportPath)
	}

	escapedVersion, err := module.EscapeVersion(m.Version)
	if err != nil {
		return "", errors.Wrapf(err, "failed to escape version %q", m.Version)
	}

	escapedPath += "@" + escapedVersion
	relativeDir := filepath.FromSlash(escapedPath)
	modBinDir := filepath.Join(binDir, relativeDir)
	return filepath.Join(modBinDir, m.BinaryName), nil
}

// Install installs the module. The returned module will have its
// Version field set to the installed version if the given module
// has no version.
func Install(m Module) (Module, error) {
	// Make sure required fields are set
	if m.ImportPath == "" {
		return m, errors.New("import path is required on module")
	}
	if m.BinaryName == "" {
		return m, errors.New("binary name is required on module")
	}

	log.WithFields(log.Fields{
		"module": m,
	}).Debug("module name is valid")

	if err := download(&m); err != nil {
		return m, err
	}
	if err := build(m); err != nil {
		return m, err
	}

	log.WithFields(log.Fields{
		"module": m,
	}).Debug("module installed")
	return m, nil
}

// download does half the work of Install. It is responsible for downloading the module
// using go get -d. It does this by creating an empty go.mod which can then be used to install
// the desired module. If no version is specified for the module, the latest version will be resolved
// by go get.
// go.mod files are stored in a directory the is represented by the module import path.
// For example if the import path is golang.org/x/tools/cmd/stringer then download will create
// SRC_DIR/golang.org/x/tools/cmd/stringer@VERSION/go.mod where SRC_DIR is the srcDir variable
// and VERSION is the version of the module (either explicit or resolved).
func download(m *Module) error {
	// First check if it already exists
	// The filepath is based off the import path
	escapedPath, err := module.EscapePath(m.ImportPath)
	if err != nil {
		return errors.Wrapf(err, "failed to escape path %q", m.ImportPath)
	}

	relativeDir := filepath.FromSlash(escapedPath)
	modDir := filepath.Join(srcDir, relativeDir)

	// If we have the version the process is pretty easy
	if m.Version != "" {
		escapedVersion, err := module.EscapeVersion(m.Version)
		if err != nil {
			return errors.Wrapf(err, "failed to escape version %q", m.Version)
		}

		modDir += "@" + escapedVersion
		// Save the actual dir so we have it for later
		m.relativeDir = fmt.Sprintf("%s@%s", relativeDir, escapedVersion)

		modfilePath := filepath.Join(modDir, "go.mod")
		if file.FileOrDirExists(modfilePath) {
			// If go.mod already exists, make sure there's no issues with it
			data, err := ioutil.ReadFile(modfilePath)
			if err != nil {
				return errors.Wrapf(err, "failed to read file %q", modfilePath)
			}

			gomod, err := modfile.Parse(modfilePath, data, nil)
			if err != nil {
				return errors.Wrapf(err, "failed to parse go.mod file %q", modfilePath)
			}

			modfileOK := true
			// There should only be a single require, otherwise something is wrong
			if len(gomod.Require) != 1 {
				modfileOK = false
				log.Debugf("expected 1 required statement in go.mod, found %d", len(gomod.Require))
			}

			mod := gomod.Require[0].Mod
			// Use contains since actual module could have less then what we are installing
			// Ex: golang.org/x/tools vs golang.org/x/tools/cmd/stringer
			if !strings.Contains(m.ImportPath, mod.Path) {
				modfileOK = false
				log.WithFields(log.Fields{
					"expected": m.ImportPath,
					"received": mod.Path,
				}).Debug("incorrect dependency in go.mod")
			}

			if m.Version != mod.Version {
				modfileOK = false
				log.WithFields(log.Fields{
					"expected": m.Version,
					"received": mod.Version,
				}).Debug("incorrect dependency version go.mod")
			}

			if modfileOK {
				log.WithFields(log.Fields{
					"module": m,
				}).Debug("Module already exists, skipping install")
				return nil
			}

			log.WithFields(log.Fields{
				"module": m,
			}).Debug("Module exists but issues found, reinstalling")

			if err := os.Remove(modfilePath); err != nil {
				return errors.Wrapf(err, "failed to remove file %q", modfilePath)
			}
		}

		// It's easier to just mkdir -p right now instead of
		// check if the dir exists beforehand
		// We can improve this later if needed
		err = os.MkdirAll(modDir, 0o755)
		if err != nil {
			return errors.Wrapf(err, "failed to create directory %q", modDir)
		}

		// Create empty go.mod file so we can install module
		// Can just use _ as the module name since this is a "fake" module
		err = execGo(modDir, "mod", "init", "_")
		if err != nil {
			return err

		}

		// Download using go get -d to get the source
		// What's nice here is we leverage the power of go get so we don't need to
		// reinvent the module resolution & downloading. Also we can reuse an existing
		// download that's already cached.
		// Always download even if the modfile existed, just to be safe.

		err = execGo(modDir, "get", "-d", m.String())
		if err != nil {
			return err
		}

		log.WithFields(log.Fields{
			"module":  m,
			"srcPath": modDir,
		}).Debug("downloaded module")
		return nil
	}

	// Don't have the version, this process is a bit more complicated because
	// we need to figure out what the latest version is

	// Use import path without version and install latest version
	if !file.FileOrDirExists(modDir) {
		err := os.MkdirAll(modDir, 0o755)
		if err != nil {
			return errors.Wrapf(err, "failed to create directory %q", modDir)
		}
	}

	// If modfile already exists, delete it and create a fresh one to be safe since
	// it's likely a leftover that wasn't cleaned up properly
	modfilePath := filepath.Join(modDir, "go.mod")
	if file.FileOrDirExists(modfilePath) {
		err := os.Remove(modfilePath)
		if err != nil {
			return errors.Wrapf(err, "failed to remove file %q", modfilePath)
		}
	}

	// Create empty go.mod file so we can install module
	// Can just use _ as the module name since this is a "fake" module
	err = execGo(modDir, "mod", "init", "_")
	if err != nil {
		return err
	}

	// Download using got get -d to get the source
	// go get will do the heavy lifting to figure out the latest version
	err = execGo(modDir, "get", "-d", m.String())
	if err != nil {
		return err
	}

	// Need to read go.mod file so we can figure out what version was installed
	data, err := ioutil.ReadFile(modfilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read file %q", modfilePath)
	}

	gomod, err := modfile.Parse(modfilePath, data, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to parse go.mod file %q", modfilePath)
	}

	// There should only be a single require, otherwise we have a bug
	if len(gomod.Require) != 1 {
		return errors.Errorf("expected 1 required statement in go.mod, found %d", len(gomod.Require))
	}
	m.Version = gomod.Require[0].Mod.Version

	// We got the version, now we need to rename the dir so it includes the version
	escapedVersion, err := module.EscapeVersion(m.Version)
	if err != nil {
		return errors.Wrapf(err, "failed to escape version %q", m.Version)
	}

	modVersionDir := fmt.Sprintf("%s@%s", modDir, escapedVersion)
	// Save the actual dir so we have it for later
	m.relativeDir = fmt.Sprintf("%s@%s", relativeDir, escapedVersion)

	if file.FileOrDirExists(modVersionDir) {
		// This version was already installed
		// We can leave the current dir, since future latest installs will
		// make use of it
		return nil
	}

	err = os.Rename(modDir, modVersionDir)
	if err != nil {
		return errors.Wrapf(err, "failed to rename %q to %q", modDir, modVersionDir)
	}

	log.WithFields(log.Fields{
		"module":  m,
		"srcPath": modVersionDir,
	}).Debug("downloaded module")
	return nil
}

// build does half the work of Install. It is responsible for building the module
// that was previously downloaded.
func build(m Module) error {

	// Check if already built
	// Default bin name should be the basename of the module
	modBinDir := filepath.Join(binDir, m.relativeDir)
	modBinPath := filepath.Join(modBinDir, m.BinaryName)
	if file.FileOrDirExists(modBinPath) {
		log.WithFields(log.Fields{
			"module":  m,
			"binPath": modBinPath,
		}).Debug("module binary already exists, skipping build")
		return nil
	}

	// build using go build and output to bin dir
	modSrcDir := filepath.Join(srcDir, m.relativeDir)
	err := execGo(modSrcDir, "build", "-o", modBinPath, m.ImportPath)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"module":  m,
		"binPath": modBinPath,
	}).Debug("module built")
	return nil
}

func execGo(dir string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir

	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil {
		argsStr := strings.Join(args, " ")
		return errors.Wrapf(err, "failed to run 'go %s', stderr: %s", argsStr, stderr.String())
	}

	return nil
}
