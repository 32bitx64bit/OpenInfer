package runtimes

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func makeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	path := filepath.Join(t.TempDir(), "test.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractZipNormal(t *testing.T) {
	z := makeZip(t, map[string]string{
		"bin/llama-server": "fake-binary",
		"README.md":        "hello",
	})
	dest := t.TempDir()
	files, err := ExtractArchive(z, dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("extracted %d files", len(files))
	}
	if _, err := os.Stat(filepath.Join(dest, "bin", "llama-server")); err != nil {
		t.Errorf("expected file missing: %v", err)
	}
}

func TestExtractZipSlipRejected(t *testing.T) {
	evil := []string{
		"../../../etc/evil",
		"..\\..\\windows\\system32\\evil",
		"/absolute/path/evil",
	}
	for _, name := range evil {
		z := makeZip(t, map[string]string{name: "pwned"})
		if _, err := ExtractArchive(z, t.TempDir()); err == nil {
			t.Errorf("zip-slip entry %q was not rejected", name)
		}
	}
}

func TestSafeJoin(t *testing.T) {
	dest := t.TempDir()
	ok, err := safeJoin(dest, "a/b/c.txt")
	if err != nil {
		t.Errorf("safe path rejected: %v", err)
	} else {
		rel, rerr := filepath.Rel(dest, ok)
		if rerr != nil || archivePathUnsafe(filepath.ToSlash(rel)) {
			t.Errorf("safe path escaped dest: ok=%q dest=%q", ok, dest)
		}
	}
	for _, bad := range []string{"../x", "a/../../x", "/etc/passwd", "..", `C:\Windows\evil`, `\rooted\evil`} {
		if _, err := safeJoin(dest, bad); err == nil {
			t.Errorf("unsafe path %q accepted", bad)
		}
	}
}
