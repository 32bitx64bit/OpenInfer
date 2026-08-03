package runtimes

import "testing"

// Excerpt of b10212's real --help: valued and boolean forms side by side.
const b10212Help = `
-fa,   --flash-attn [on|off|auto]       set Flash Attention use ('on', 'off', or 'auto', default: 'auto')
--mlock                                 DEPRECATED in favor of --load-mode: force system to keep model in
--mmap, --no-mmap                       DEPRECATED in favor of --load-mode
--jinja, --no-jinja                     whether to use jinja template engine for chat (default: enabled)
-cb,   --cont-batching, -nocb, --no-cont-batching
--draft, --draft-n, --draft-max N       the argument has been removed. use --spec-draft-n-max or
-c,    --ctx-size N                     size of the prompt context (default: 0)
`

const oldHelp = `
-fa,   --flash-attn                     enable Flash Attention
-c,    --ctx-size N                     size of the prompt context
`

func TestFlagTakesValue(t *testing.T) {
	if !FlagTakesValue(b10212Help, "--flash-attn") {
		t.Error("--flash-attn [on|off|auto] must be detected as valued")
	}
	if FlagTakesValue(oldHelp, "--flash-attn") {
		t.Error("bare --flash-attn (old builds) must be detected as boolean")
	}
	if !FlagTakesValue(b10212Help, "--ctx-size") {
		t.Error("--ctx-size N must be valued")
	}
	if FlagTakesValue(b10212Help, "--jinja") {
		t.Error("--jinja must be boolean")
	}
	if FlagTakesValue(b10212Help, "--mlock") {
		t.Error("--mlock must be boolean (DEPRECATED but functional)")
	}
	if FlagTakesValue(b10212Help, "--cont-batching") {
		t.Error("--cont-batching must be boolean")
	}
}

func TestRemovedFlagsExcluded(t *testing.T) {
	caps := ParseCapabilities(b10212Help)
	// The removed spelling must be rejected…
	if SupportsFlag(caps, b10212Help, "--draft-max") {
		t.Error("removed flag must be unsupported")
	}
	if SupportsFlag(caps, b10212Help, "--draft-n") {
		t.Error("--draft-n (unknown to us) on a removed line must be unsupported")
	}
	// …while the capability survives through the replacement spelling.
	found := false
	for _, c := range caps {
		if c == "draft-max" {
			found = true
		}
	}
	if !found {
		t.Error("draft-max capability should exist via --spec-draft-n-max")
	}
	if !SupportsFlag(caps, b10212Help, "--spec-draft-n-max") {
		t.Error("--spec-draft-n-max must be supported")
	}
	// DEPRECATED but functional flags remain supported.
	if !SupportsFlag(caps, b10212Help, "--mlock") {
		t.Error("--mlock is deprecated but functional and must stay supported")
	}
}

func TestSupportsFlagFallsBackToHelpForStaleCaps(t *testing.T) {
	// Caps snapshotted before --spec-type was added to knownFlags.
	stale := []string{"model-draft", "draft-max", "host", "port"}
	help := `
--spec-draft-model, -md, --model-draft FNAME
--spec-type none,draft-simple,draft-eagle3,draft-mtp,draft-dflash,draft-dspark
`
	if !SupportsFlag(stale, help, "--spec-type") {
		t.Error("stale caps must still allow --spec-type when help advertises it")
	}
	if SupportsFlag(stale, "", "--spec-type") {
		t.Error("without help text, missing caps must not invent support")
	}
}
