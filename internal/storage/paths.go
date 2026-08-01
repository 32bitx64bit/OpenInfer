// Package storage validates filesystem paths used by the backend: every path
// arriving over the API must resolve inside an allowed root.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateInside resolves p (which may be relative to root) and requires the
// result to stay inside root after symlink resolution of the root. It rejects
// traversal such as "../../etc/passwd".
func ValidateInside(root, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(absRoot, p)
	}
	clean := filepath.Clean(abs)
	if clean != absRoot && !strings.HasPrefix(clean, absRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes managed root %q", p, root)
	}
	return clean, nil
}

// SafeJoin joins elements to root and validates containment in one call.
func SafeJoin(root string, elems ...string) (string, error) {
	return ValidateInside(root, filepath.Join(elems...))
}

// AtomicWrite writes data to a temp file in the same directory and renames it
// over dst, so readers never observe a partially written file.
func AtomicWrite(dst string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".atomic-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dst)
}
