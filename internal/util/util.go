package util

import (
	"os"
)

// FileOrDirExists returns true if the given path exists on the OS filesystem.
func FileOrDirExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}
