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
-md,   --model-draft FNAME              draft model
`

var newCaps = []string{"flash-attn", "gpu-layers", "ctx-size", "threads", "host", "port",
	"api-key", "draft-max", "model-draft"}

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
