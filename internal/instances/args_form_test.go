package instances

import (
	"strings"
	"testing"
)

// Real-flavor help from llama.cpp b10212 (valued --flash-attn, removed
// --draft-max).
const newHelp = `
-fa,   --flash-attn [on|off|auto]       set Flash Attention use
-ngl,  --gpu-layers, --n-gpu-layers N   max. number of layers in VRAM
-c,    --ctx-size N                     context size
-t,    --threads N                      threads
--host HOST
--port PORT
--api-key KEY
--draft, --draft-n, --draft-max N       the argument has been removed. use --spec-draft-n-max or
--spec-draft-n-max N                    max draft tokens
--spec-draft-n-min N                    min draft tokens
--spec-type TYPE                        speculative decoding type
-md,   --model-draft FNAME              draft model
--parallel N
`

var newCaps = []string{"flash-attn", "gpu-layers", "ctx-size", "threads", "host", "port",
	"api-key", "draft-max", "draft-min", "model-draft", "spec-type", "parallel"}

func argVal(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestFlashAttnValuedRuntime(t *testing.T) {
	s := DefaultSettings() // flash_attention auto
	br := BuildArgs(s, "/m.gguf", "", newCaps, newHelp, "127.0.0.1", 1, "k")
	if got := argVal(t, br.Args, "--flash-attn"); got != "auto" {
		t.Errorf("auto → --flash-attn auto, got %q in %v", got, br.Args)
	}
	// The value must not swallow the next flag.
	joined := strings.Join(br.Args, " ")
	if strings.Contains(joined, "--flash-attn --") {
		t.Errorf("bare --flash-attn emitted against valued runtime: %q", joined)
	}

	s.FlashAttention = "off"
	br = BuildArgs(s, "/m.gguf", "", newCaps, newHelp, "127.0.0.1", 1, "k")
	if got := argVal(t, br.Args, "--flash-attn"); got != "off" {
		t.Errorf("off → --flash-attn off, got %q", got)
	}
}

func TestFlashAttnBooleanRuntime(t *testing.T) {
	oldHelp := "-fa, --flash-attn    enable Flash Attention\n--host H\n--port P\n"
	oldCaps := []string{"flash-attn", "host", "port"}
	s := DefaultSettings()
	br := BuildArgs(s, "/m.gguf", "", oldCaps, oldHelp, "127.0.0.1", 1, "k")
	if got := argVal(t, br.Args, "--flash-attn"); got != "" {
		t.Errorf("boolean runtime must get bare --flash-attn, got value %q", got)
	}
	found := false
	for _, a := range br.Args {
		if a == "--flash-attn" {
			found = true
		}
	}
	if !found {
		t.Error("bare --flash-attn missing")
	}

	// off on a boolean runtime = omit the flag entirely.
	s.FlashAttention = "off"
	br = BuildArgs(s, "/m.gguf", "", oldCaps, oldHelp, "127.0.0.1", 1, "k")
	for _, a := range br.Args {
		if a == "--flash-attn" {
			t.Error("off must omit bare --flash-attn on boolean runtimes")
		}
	}
}

func TestRemovedFlagFallsForward(t *testing.T) {
	s := DefaultSettings()
	s.DraftMax = 8
	br := BuildArgs(s, "/m.gguf", "", newCaps, newHelp, "127.0.0.1", 1, "k")
	joined := strings.Join(br.Args, " ")
	if strings.Contains(joined, "--draft-max ") {
		t.Errorf("removed --draft-max must not be emitted: %q", joined)
	}
	if !strings.Contains(joined, "--spec-draft-n-max 8") {
		t.Errorf("want fallback --spec-draft-n-max 8 in %q", joined)
	}
}

func TestSpecTypeAutoWhenDraftSet(t *testing.T) {
	s := DefaultSettings()
	s.DraftModel = "/draft.gguf"
	br := BuildArgs(s, "/m.gguf", "", newCaps, newHelp, "127.0.0.1", 1, "k")
	if got := argVal(t, br.Args, "--spec-type"); got != "draft-simple" {
		t.Errorf("auto spec-type = %q, want draft-simple; args=%v", got, br.Args)
	}
	if got := argVal(t, br.Args, "--model-draft"); got != "/draft.gguf" {
		t.Errorf("draft model = %q", got)
	}
}

func TestSpecTypeMTPSidecar(t *testing.T) {
	s := DefaultSettings()
	s.DraftModel = "/models/mtp-Qwen3.6-27B.gguf"
	br := BuildArgs(s, "/m.gguf", "", newCaps, newHelp, "127.0.0.1", 1, "k")
	if got := argVal(t, br.Args, "--spec-type"); got != "draft-mtp" {
		t.Errorf("mtp sidecar → draft-mtp, got %q in %v", got, br.Args)
	}
	if got := argVal(t, br.Args, "--parallel"); got != "1" {
		t.Errorf("speculative load must pin --parallel 1 when unset, got %q", got)
	}
}

func TestSpecTypeFromStaleCapsViaHelp(t *testing.T) {
	// Caps without "spec-type" (install before we recognized the flag).
	stale := []string{"flash-attn", "gpu-layers", "ctx-size", "threads", "host", "port",
		"api-key", "draft-max", "model-draft", "parallel"}
	s := DefaultSettings()
	s.DraftModel = "/models/mtp-gemma-4-12B-it.gguf"
	br := BuildArgs(s, "/m.gguf", "", stale, newHelp, "127.0.0.1", 1, "k")
	if got := argVal(t, br.Args, "--spec-type"); got != "draft-mtp" {
		t.Errorf("stale caps + help must still emit --spec-type draft-mtp, got %q in %v", got, br.Args)
	}
}

func TestSpecTypeExplicit(t *testing.T) {
	s := DefaultSettings()
	s.DraftModel = "/draft.gguf"
	s.SpecType = "draft-eagle3"
	br := BuildArgs(s, "/m.gguf", "", newCaps, newHelp, "127.0.0.1", 1, "k")
	if got := argVal(t, br.Args, "--spec-type"); got != "draft-eagle3" {
		t.Errorf("spec-type = %q", got)
	}
}

func TestSpecTypeMTPWithoutDraft(t *testing.T) {
	s := DefaultSettings()
	s.SpecType = "draft-mtp"
	br := BuildArgs(s, "/m.gguf", "", newCaps, newHelp, "127.0.0.1", 1, "k")
	if got := argVal(t, br.Args, "--spec-type"); got != "draft-mtp" {
		t.Errorf("fused mtp: %q", got)
	}
	joined := strings.Join(br.Args, " ")
	if strings.Contains(joined, "--model-draft") || strings.Contains(joined, "--spec-draft-model") {
		t.Errorf("fused mtp must not invent a draft path: %q", joined)
	}
}

func TestSpecTypeMTPWithDraftMax(t *testing.T) {
	// Matches LoadConfigDialog default for fused has_mtp trunks.
	s := DefaultSettings()
	s.SpecType = "draft-mtp"
	s.DraftMax = 2
	br := BuildArgs(s, "/m.gguf", "", newCaps, newHelp, "127.0.0.1", 1, "k")
	if got := argVal(t, br.Args, "--spec-type"); got != "draft-mtp" {
		t.Errorf("spec-type = %q", got)
	}
	gotMax := argVal(t, br.Args, "--spec-draft-n-max")
	if gotMax == "" {
		gotMax = argVal(t, br.Args, "--draft-max")
	}
	if gotMax != "2" {
		t.Errorf("draft_max=2 → --spec-draft-n-max/--draft-max 2, got %q in %v", gotMax, br.Args)
	}
	joined := strings.Join(br.Args, " ")
	if strings.Contains(joined, "--model-draft") || strings.Contains(joined, "--spec-draft-model") {
		t.Errorf("fused mtp must not invent a draft path: %q", joined)
	}
}

func TestParallelWarnWithDraft(t *testing.T) {
	s := DefaultSettings()
	s.DraftModel = "/draft.gguf"
	s.Parallel = 4
	br := BuildArgs(s, "/m.gguf", "", newCaps, newHelp, "127.0.0.1", 1, "k")
	found := false
	for _, w := range br.Warnings {
		if strings.Contains(w, "parallel") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected parallel warning, got %v", br.Warnings)
	}
}
