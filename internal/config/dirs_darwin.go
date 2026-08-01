//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

// platformDataDir returns ~/Library/Application Support/OpenInfer Studio.
func platformDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "OpenInfer Studio"), nil
}
