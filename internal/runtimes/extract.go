package runtimes

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// archivePathUnsafe reports whether an archive entry name must be rejected
// before joining. Zip/tar attackers use both / and \; on Windows,
// filepath.IsAbs("/etc/passwd") is false and Join would place it under dest.
func archivePathUnsafe(name string) bool {
	if name == "" {
		return true
	}
	n := strings.ReplaceAll(name, `\`, "/")
	if n[0] == '/' {
		return true
	}
	// Drive / UNC / volume forms (C:/..., //host/share).
	if len(n) >= 2 && n[1] == ':' {
		return true
	}
	if strings.HasPrefix(n, "//") {
		return true
	}
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" || filepath.VolumeName(n) != "" {
		return true
	}
	clean := path.Clean(n)
	return clean == ".." || strings.HasPrefix(clean, "../")
}

// safeJoin prevents zip-slip / tar path traversal: the joined path must stay
// inside dest after cleaning.
func safeJoin(dest, name string) (string, error) {
	if archivePathUnsafe(name) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	clean := filepath.FromSlash(path.Clean(strings.ReplaceAll(name, `\`, "/")))
	full := filepath.Join(dest, clean)
	rel, err := filepath.Rel(dest, full)
	if err != nil || archivePathUnsafe(filepath.ToSlash(rel)) {
		return "", fmt.Errorf("archive entry %q escapes destination", name)
	}
	return full, nil
}

const maxExtractedBytes = 8 << 30 // 8 GiB sanity cap for one archive

// ExtractArchive unpacks a .zip or .tar.gz into dest with traversal
// protection and a total-size cap. Returns the list of extracted files.
func ExtractArchive(archivePath, dest string) ([]string, error) {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(archivePath, dest)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(archivePath, dest)
	default:
		return nil, fmt.Errorf("unsupported archive type: %s", archivePath)
	}
}

func extractZip(path, dest string) ([]string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var files []string
	var total int64
	for _, f := range zr.File {
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return nil, err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > maxExtractedBytes {
			return nil, fmt.Errorf("archive too large (> %d bytes)", maxExtractedBytes)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		// Zip entries can encode symlinks via the Unix mode bits.
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			linkTarget, rerr := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if rerr != nil {
				return nil, rerr
			}
			if err := safeSymlink(dest, target, string(linkTarget)); err != nil {
				return nil, err
			}
			files = append(files, target)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		mode := f.FileInfo().Mode()
		if mode == 0 {
			mode = 0o644
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			rc.Close()
			return nil, err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return nil, err
		}
		files = append(files, target)
	}
	return files, nil
}

func extractTarGz(path, dest string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var files []string
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return nil, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			total += hdr.Size
			if total > maxExtractedBytes {
				return nil, fmt.Errorf("archive too large (> %d bytes)", maxExtractedBytes)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return nil, err
			}
			mode := os.FileMode(hdr.Mode)
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return nil, err
			}
			out.Close()
			files = append(files, target)
		case tar.TypeSymlink:
			// Official llama.cpp archives use SONAME symlinks
			// (libllama.so.0 → libllama.so.0.0.10212). They are required by
			// the dynamic loader. Allow only relative in-tree targets.
			if err := safeSymlink(dest, target, hdr.Linkname); err != nil {
				return nil, err
			}
			files = append(files, target)
		default:
			// Skip devices, fifos and hardlinks: runtimes must be plain files.
			continue
		}
	}
	return files, nil
}

// safeSymlink creates a symlink at linkPath (inside dest) pointing to target,
// rejecting absolute/rooted targets and any target that resolves outside dest.
func safeSymlink(dest, linkPath, target string) error {
	if archivePathUnsafe(target) {
		return fmt.Errorf("unsafe symlink target %q", target)
	}
	// Resolve the target relative to the link's directory and require it to
	// stay inside dest after cleaning.
	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), target))
	rel, err := filepath.Rel(dest, resolved)
	if err != nil || archivePathUnsafe(filepath.ToSlash(rel)) {
		return fmt.Errorf("symlink %q → %q escapes destination", linkPath, target)
	}
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	os.Remove(linkPath) // replace stale entry
	return os.Symlink(target, linkPath)
}
