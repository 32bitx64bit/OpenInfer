//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// platformDataDir returns %LOCALAPPDATA%\OpenInfer Studio. Runtime state and
// models are machine-local; roaming config is handled by os.UserConfigDir.
func platformDataDir() (string, error) {
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return filepath.Join(v, "OpenInfer Studio"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "AppData", "Local", "OpenInfer Studio"), nil
}
