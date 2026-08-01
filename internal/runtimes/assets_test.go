package runtimes

import "testing"

func TestClassifyAsset(t *testing.T) {
	cases := []struct {
		name           string
		platform, arch string
		backend        string
	}{
		{"llama-b6801-bin-ubuntu-x64.zip", "linux", "amd64", BackendCPU},
		{"llama-b6801-bin-ubuntu-vulkan-x64.zip", "linux", "amd64", BackendVulkan},
		{"llama-b6801-ubuntu-cuda-12-x64.zip", "linux", "amd64", BackendCUDA},
		{"llama-b6801-bin-win-cuda-12.4-x64.zip", "windows", "amd64", BackendCUDA},
		{"llama-b6801-bin-win-hip-x64-gfx1100.zip", "windows", "amd64", BackendHIP},
		{"llama-b6801-bin-macos-arm64.zip", "darwin", "arm64", BackendMetal},
		{"llama-b6801-bin-macos-x64.zip", "darwin", "amd64", BackendMetal},
		{"llama-b6801-bin-win-x64.zip", "windows", "amd64", BackendCPU},
		{"llama-b6801-bin-ubuntu-sycl-x64.zip", "linux", "amd64", BackendSYCL},
	}
	for _, c := range cases {
		p, a, b := ClassifyAsset(c.name)
		if p != c.platform || a != c.arch || b != c.backend {
			t.Errorf("ClassifyAsset(%q) = (%q,%q,%q), want (%q,%q,%q)",
				c.name, p, a, b, c.platform, c.arch, c.backend)
		}
	}
}

func TestResolveAssetsPrefersCUDAForNVIDIA(t *testing.T) {
	rel := Release{Tag: "b6801", Assets: []Asset{
		{Name: "llama-b6801-bin-ubuntu-x64.zip"},
		{Name: "llama-b6801-bin-ubuntu-vulkan-x64.zip"},
		{Name: "llama-b6801-ubuntu-cuda-12-x64.zip"},
		{Name: "llama-b6801-bin-win-x64.zip"},
		{Name: "llama-b6801-bin-macos-arm64.zip"},
	}}
	m := MachineProfile{OS: "linux", Arch: "amd64", CUDA: true, Vulkan: true, GPUVendor: "nvidia"}
	matches := ResolveAssets(rel, m, "")
	if len(matches) == 0 {
		t.Fatal("no matches")
	}
	if matches[0].Backend != BackendCUDA {
		t.Errorf("top match = %s, want cuda; all: %+v", matches[0].Backend, matches)
	}
	// Windows/macOS assets must be excluded.
	for _, mt := range matches {
		p, _, _ := ClassifyAsset(mt.Asset.Name)
		if p != "" && p != "linux" {
			t.Errorf("foreign platform asset not filtered: %s", mt.Asset.Name)
		}
	}
}

func TestResolveAssetsPrefersMetalForAppleSilicon(t *testing.T) {
	rel := Release{Tag: "b6801", Assets: []Asset{
		{Name: "llama-b6801-bin-macos-arm64.zip"},
		{Name: "llama-b6801-bin-macos-x64.zip"},
	}}
	m := MachineProfile{OS: "darwin", Arch: "arm64", Metal: true, GPUVendor: "apple"}
	matches := ResolveAssets(rel, m, "")
	if len(matches) != 1 || matches[0].Backend != BackendMetal {
		t.Fatalf("want single metal match, got %+v", matches)
	}
}

func TestResolveAssetsUserPreference(t *testing.T) {
	rel := Release{Tag: "b6801", Assets: []Asset{
		{Name: "llama-b6801-bin-ubuntu-x64.zip"},
		{Name: "llama-b6801-bin-ubuntu-vulkan-x64.zip"},
	}}
	m := MachineProfile{OS: "linux", Arch: "amd64", Vulkan: true, GPUVendor: "amd"}
	matches := ResolveAssets(rel, m, BackendVulkan)
	if matches[0].Backend != BackendVulkan {
		t.Errorf("user-preferred vulkan not first: %+v", matches)
	}
	// CPU must remain listed as fallback.
	if matches[len(matches)-1].Backend != BackendCPU {
		t.Errorf("cpu fallback missing: %+v", matches)
	}
}

func TestStrongestVendor(t *testing.T) {
	if got := StrongestVendor([]string{"intel", "nvidia", "amd"}); got != "nvidia" {
		t.Errorf("got %s", got)
	}
	if got := StrongestVendor(nil); got != "" {
		t.Errorf("got %q", got)
	}
}

var _ = runtimeGOOS // referenced for symmetry; platform comes from MachineProfile in tests
