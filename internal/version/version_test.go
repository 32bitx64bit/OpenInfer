package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestVersionSemverShape(t *testing.T) {
	v := Version()
	if v == "" {
		t.Fatal("empty version")
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		t.Fatalf("version %q is not semver-like", v)
	}
	for _, p := range parts {
		if p == "" {
			t.Fatalf("empty semver component in %q", v)
		}
	}
}

func TestInfoKeys(t *testing.T) {
	info := Info()
	for _, k := range []string{"version", "commit", "date", "goos", "goarch"} {
		if _, ok := info[k]; !ok {
			t.Fatalf("missing key %q", k)
		}
	}
	if info["version"] != Version() {
		t.Fatalf("info version = %v, want %s", info["version"], Version())
	}
	if info["goos"] != runtime.GOOS {
		t.Fatalf("goos = %v", info["goos"])
	}
}

func TestUserAgent(t *testing.T) {
	ua := UserAgent()
	if !strings.HasPrefix(ua, "OpenInferStudio/") {
		t.Fatalf("user agent = %q", ua)
	}
}
