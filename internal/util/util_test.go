package util_test

import (
	"path/filepath"
	"testing"

	"github.com/cszatmary/shed/internal/util"
)

func TestFileOrDirExists(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"dir exists", filepath.Join("."), true},
		// Test that this file exists, how meta!
		{"file exists", "util_test.go", true},
		{"file exists", "if_this_exists_all_hope_is_lost.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := util.FileOrDirExists(tt.path)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
