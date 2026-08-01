package runtimes

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

type tarEntry struct {
	name, body, link string
	typ              byte
}

func makeTarGz(t *testing.T, entries []tarEntry) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0o755, Typeflag: e.typ}
		if e.typ == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		} else if e.typ == tar.TypeSymlink {
			hdr.Linkname = e.link
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if e.typ == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	path := filepath.Join(t.TempDir(), "test.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExtractTarGzSonameSymlinks mirrors the official llama.cpp layout:
// real versioned libraries plus SONAME symlinks pointing at them.
func TestExtractTarGzSonameSymlinks(t *testing.T) {
	arc := makeTarGz(t, []tarEntry{
		{"llama-b1/llama-server", "binary", "", tar.TypeReg},
		{"llama-b1/libllama.so.0.0.1", "lib", "", tar.TypeReg},
		{"llama-b1/libllama.so.0", "", "libllama.so.0.0.1", tar.TypeSymlink},
		{"llama-b1/libllama.so", "", "libllama.so.0", tar.TypeSymlink},
	})
	dest := t.TempDir()
	files, err := ExtractArchive(arc, dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Errorf("extracted %d files, want 4", len(files))
	}
	link := filepath.Join(dest, "llama-b1", "libllama.so.0")
	target, err := os.Readlink(link)
	if err != nil || target != "libllama.so.0.0.1" {
		t.Errorf("symlink = %q, %v", target, err)
	}
	// Reading through the chain must reach the real file.
	if b, err := os.ReadFile(link); err != nil || string(b) != "lib" {
		t.Errorf("read through symlink failed: %v", err)
	}
}

// TestExtractTarGzEvilSymlinks rejects escaping or absolute link targets.
func TestExtractTarGzEvilSymlinks(t *testing.T) {
	for _, evil := range []string{"../../etc/passwd", "/etc/passwd", "../../../root/.ssh/id"} {
		arc := makeTarGz(t, []tarEntry{
			{"x/link", "", evil, tar.TypeSymlink},
		})
		if _, err := ExtractArchive(arc, t.TempDir()); err == nil {
			t.Errorf("evil symlink target %q accepted", evil)
		}
	}
}

func TestSafeSymlinkBasics(t *testing.T) {
	dest := t.TempDir()
	link := filepath.Join(dest, "a", "lib.so")
	if err := safeSymlink(dest, link, "lib.so.1"); err != nil {
		t.Fatal(err)
	}
	if target, _ := os.Readlink(link); target != "lib.so.1" {
		t.Errorf("target = %q", target)
	}
}
