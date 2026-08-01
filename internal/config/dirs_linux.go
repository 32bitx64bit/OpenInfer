//go:build linux

package config

import (
	"os"
	"path/filepath"
)

// platformDataDir returns $XDG_DATA_HOME/openinfer-studio, falling back to
// ~/.local/share/openinfer-studio when XDG_DATA_HOME is unset.
func platformDataDir() (string, error) {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, appDirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", appDirName), nil
}
