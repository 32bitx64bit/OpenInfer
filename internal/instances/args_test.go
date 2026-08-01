package instances

import (
	"strings"
	"testing"
)

const testHelp = `
  --ctx-size N
  --n-gpu-layers N
  --threads N
  --flash-attn
  --host HOST
  --port PORT
  --api-key KEY
  --mmproj FILE
  --alias NAME
  --batch-size N
`
const testHelpMinimal = `
  --host HOST
  --port PORT
`

func testCaps() []string {
	return []string{"ctx-size", "gpu-layers", "threads", "flash-attn", "host", "port",
		"api-key", "mmproj", "alias", "batch-size"}
}

func TestBuildArgsBasics(t *testing.T) {
	s := DefaultSettings()
	s.ContextLength = 8192
	s.Threads = 8
	br := BuildArgs(s, "/models/m.gguf", "", testCaps(), testHelp, "127.0.0.1", 8123, "k")
	joined := strings.Join(br.Args, " ")
	for _, want := range []string{"--model /models/m.gguf", "--ctx-size 8192", "--threads 8",
		"--host 127.0.0.1", "--port 8123", "--api-key k", "--flash-attn"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	if len(br.Resolutions) == 0 {
		t.Error("expected auto resolutions to be reported")
	}
}

func TestBuildArgsParallelFullContextPerSlot(t *testing.T) {
	s := DefaultSettings()
	s.ContextLength = 8192
	s.Parallel = 3
	br := BuildArgs(s, "/models/m.gguf", "", append(testCaps(), "parallel"), testHelp+"  --parallel N\n", "127.0.0.1", 8123, "k")
	joined := strings.Join(br.Args, " ")
	// llama.cpp splits --ctx-size across slots; 3 × 8192 gives each the full window.
	if !strings.Contains(joined, "--ctx-size 24576") {
		t.Errorf("expected --ctx-size 24576 in %q", joined)
	}
	if !strings.Contains(joined, "--parallel 3") {
		t.Errorf("expected --parallel 3 in %q", joined)
	}
}

func TestBuildArgsParallelWarnsOnAutoContext(t *testing.T) {
	s := DefaultSettings()
	s.ContextLength = 0 // model default
	s.Parallel = 2
	br := BuildArgs(s, "/models/m.gguf", "", append(testCaps(), "parallel"), testHelp+"  --parallel N\n", "127.0.0.1", 8123, "k")
	found := false
	for _, w := range br.Warnings {
		if strings.Contains(w, "slots share it") {
			found = true
		}
	}
	if !found {
		t.Error("expected a warning about shared model-default context")
	}
}

func TestBuildArgsDropsUnsupported(t *testing.T) {
	s := DefaultSettings()
	s.ContextLength = 8192 // unsupported in minimal help
	br := BuildArgs(s, "/m.gguf", "", []string{"host", "port"}, testHelpMinimal, "127.0.0.1", 1, "k")
	joined := strings.Join(br.Args, " ")
	if strings.Contains(joined, "--ctx-size") {
		t.Error("unsupported --ctx-size must be dropped")
	}
	if len(br.Warnings) == 0 {
		t.Error("expected a warning about the dropped flag")
	}
}

func TestBuildArgsGPUOffloadModes(t *testing.T) {
	for mode, want := range map[string]string{"all": "999", "none": "0", "auto": "999"} {
		s := DefaultSettings()
		s.GPUOffload = mode
		br := BuildArgs(s, "/m.gguf", "", testCaps(), testHelp, "127.0.0.1", 1, "k")
		if !strings.Contains(strings.Join(br.Args, " "), "--n-gpu-layers "+want) {
			t.Errorf("mode %s: want n-gpu-layers %s in %v", mode, want, br.Args)
		}
	}
	s := DefaultSettings()
	s.GPUOffload = "custom"
	s.GPULayers = 17
	br := BuildArgs(s, "/m.gguf", "", testCaps(), testHelp, "127.0.0.1", 1, "k")
	if !strings.Contains(strings.Join(br.Args, " "), "--n-gpu-layers 17") {
		t.Errorf("custom layers missing: %v", br.Args)
	}
}

func TestBuildArgsRawArgsValidation(t *testing.T) {
	// Unsafe input: rejected wholesale, nothing leaks through.
	s := DefaultSettings()
	s.RawArgs = "--batch-size 512; rm -rf / && echo pwned"
	br := BuildArgs(s, "/m.gguf", "", testCaps(), testHelp, "127.0.0.1", 1, "k")
	joined := strings.Join(br.Args, " ")
	if strings.Contains(joined, "rm") || strings.Contains(joined, "--batch-size 512;") {
		t.Errorf("unsafe raw args leaked: %q", joined)
	}
	if len(br.Warnings) == 0 {
		t.Error("expected a wholesale-rejection warning")
	}

	// Safe input: supported flags pass with one value each; unsupported flags
	// and orphan tokens are dropped with warnings.
	s2 := DefaultSettings()
	s2.RawArgs = "--batch-size 512 --future-flag stray-token"
	br2 := BuildArgs(s2, "/m.gguf", "", testCaps(), testHelp, "127.0.0.1", 1, "k")
	joined2 := strings.Join(br2.Args, " ")
	if !strings.Contains(joined2, "--batch-size 512") {
		t.Errorf("valid raw arg dropped: %q", joined2)
	}
	if strings.Contains(joined2, "--future-flag") || strings.Contains(joined2, "stray-token") {
		t.Errorf("unsupported/orphan tokens not filtered: %q", joined2)
	}
}

func TestFilterEnv(t *testing.T) {
	allowed, rejected := FilterEnv(map[string]string{
		"GGML_LOG_LEVEL": "debug",
		"LD_PRELOAD":     "/evil.so",
		"HOME":           "/somewhere",
	})
	if allowed["GGML_LOG_LEVEL"] != "debug" {
		t.Error("allowlisted var dropped")
	}
	if len(rejected) != 2 {
		t.Errorf("want 2 rejected, got %v", rejected)
	}
	if _, ok := allowed["LD_PRELOAD"]; ok {
		t.Error("LD_PRELOAD must never pass")
	}
}

func TestQuoteCommand(t *testing.T) {
	q := quoteCommand([]string{"--model", "/path with spaces/m.gguf", "--ctx-size", "4096"})
	if !strings.Contains(q, `"/path with spaces/m.gguf"`) {
		t.Errorf("spaces not quoted: %q", q)
	}
	if !strings.Contains(q, "--ctx-size 4096") {
		t.Errorf("simple args should stay bare: %q", q)
	}
}
