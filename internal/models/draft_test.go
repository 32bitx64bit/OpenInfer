package models

import (
	"encoding/json"
	"testing"
)

func metaTok(tok string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"tokenizer": tok})
	return b
}

func TestDraftCompatibleSameModel(t *testing.T) {
	m := Model{ID: "a", Architecture: "llama", Parameters: 7e9, Metadata: metaTok("llama")}
	ok, reason := DraftCompatible(m, m)
	if ok {
		t.Fatal("same model must be incompatible")
	}
	if reason == "" {
		t.Fatal("expected reason")
	}
}

func TestDraftCompatibleArchMismatch(t *testing.T) {
	target := Model{ID: "t", Architecture: "qwen3", Parameters: 8e9, Metadata: metaTok("gpt2")}
	draft := Model{ID: "d", Architecture: "llama", Parameters: 1e9, Metadata: metaTok("gpt2")}
	ok, reason := DraftCompatible(target, draft)
	if ok {
		t.Fatalf("arch mismatch should fail: %s", reason)
	}
}

func TestDraftCompatibleTokenizerMismatch(t *testing.T) {
	target := Model{ID: "t", Architecture: "llama", Parameters: 8e9, Metadata: metaTok("llama")}
	draft := Model{ID: "d", Architecture: "llama", Parameters: 1e9, Metadata: metaTok("gpt2")}
	ok, _ := DraftCompatible(target, draft)
	if ok {
		t.Fatal("tokenizer mismatch should fail")
	}
}

func TestDraftCompatibleTooLarge(t *testing.T) {
	target := Model{ID: "t", Architecture: "llama", Parameters: 1e9, Metadata: metaTok("llama")}
	draft := Model{ID: "d", Architecture: "llama", Parameters: 7e9, Metadata: metaTok("llama")}
	ok, _ := DraftCompatible(target, draft)
	if ok {
		t.Fatal("larger draft should fail")
	}
}

func TestDraftCompatibleProjector(t *testing.T) {
	target := Model{ID: "t", Architecture: "qwen2vl", Parameters: 7e9}
	draft := Model{ID: "d", PrimaryPath: "/models/mmproj-f16.gguf", SizeBytes: 600}
	ok, reason := DraftCompatible(target, draft)
	if ok {
		t.Fatalf("projector should fail: %s", reason)
	}
}

func TestDraftCompatibleOK(t *testing.T) {
	target := Model{ID: "t", Architecture: "llama", Parameters: 8e9, SizeBytes: 5e9, Metadata: metaTok("llama")}
	draft := Model{ID: "d", Architecture: "llama", Parameters: 1e9, SizeBytes: 700e6, Metadata: metaTok("llama")}
	ok, reason := DraftCompatible(target, draft)
	if !ok {
		t.Fatalf("compatible pair rejected: %s", reason)
	}
}

func TestDraftCompatibleSpecializedArch(t *testing.T) {
	target := Model{ID: "t", Architecture: "qwen3", Parameters: 8e9, SizeBytes: 5e9, Metadata: metaTok("gpt2")}
	draft := Model{
		ID: "d", Architecture: "eagle3", Parameters: 1e8, SizeBytes: 200e6,
		PrimaryPath: "/m/Qwen3-8B-eagle3.gguf",
		Metadata:    json.RawMessage(`{"tokenizer":"gpt2","speculative_draft":true}`),
	}
	ok, reason := DraftCompatible(target, draft)
	if !ok {
		t.Fatalf("eagle3 draft should be compatible despite arch mismatch: %s", reason)
	}
}

func TestFilterDraftCandidates(t *testing.T) {
	target := Model{ID: "t", Architecture: "llama", Parameters: 8e9, Metadata: metaTok("llama")}
	lib := []Model{
		target,
		{ID: "good", Architecture: "llama", Parameters: 1e9, Metadata: metaTok("llama")},
		{ID: "bad-arch", Architecture: "qwen3", Parameters: 1e9, Metadata: metaTok("llama")},
		{
			ID: "mtp", Architecture: "llama", Parameters: 1e8, SizeBytes: 100e6,
			PrimaryPath: "/m/mtp-llama-draft.gguf",
			Metadata:    json.RawMessage(`{"tokenizer":"llama","speculative_draft":true,"spec_type":"draft-mtp"}`),
		},
	}
	filtered := FilterDraftCandidates(target, lib, true)
	if len(filtered) != 1 || filtered[0].ID != "mtp" {
		t.Fatalf("filtered should only list speculative drafts, got %+v", filtered)
	}
	all := FilterDraftCandidates(target, lib, false)
	if len(all) != 3 {
		t.Fatalf("unfiltered want 3, got %d", len(all))
	}
	var bad *DraftCandidate
	for i := range all {
		if all[i].ID == "bad-arch" {
			bad = &all[i]
		}
	}
	if bad == nil || bad.Compatible {
		t.Fatalf("bad-arch should be marked incompatible: %+v", bad)
	}
}
