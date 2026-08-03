package gguf

import "testing"

func TestSidecarPrefixes(t *testing.T) {
	cases := map[string]SpecType{
		"mtp-Qwen3.6-27B-Q4_K_M.gguf":      SpecMTP,
		"/models/eagle3-Qwen3-8B-f16.gguf": SpecEagle3,
		"dflash-Qwen3-4B-b16.gguf":         SpecDFlash,
		"dspark-qwen3_4b_block7.gguf":      SpecDSpark,
		"Qwen3.6-27B-MTP-IQ4_XS.gguf":      SpecNone, // fused trunk name, not sidecar
		"Qwen2-VL-7B-Instruct.gguf":        SpecNone,
	}
	for path, want := range cases {
		if got := SidecarSpecType(path); got != want {
			t.Errorf("SidecarSpecType(%q)=%q want %q", path, got, want)
		}
	}
}

func TestDetectSpeculativeDraftArch(t *testing.T) {
	ok, spec := DetectSpeculativeDraft("eagle3", "", "", nil)
	if !ok || spec != SpecEagle3 {
		t.Fatalf("eagle3: ok=%v spec=%q", ok, spec)
	}
	ok, spec = DetectSpeculativeDraft("gemma4-assistant", "Gemma 4 E2B It Assistant",
		"/m/gemma-4-E2B-it-assistant.Q8_0.gguf", nil)
	if !ok || spec != SpecMTP {
		t.Fatalf("gemma4-assistant: ok=%v spec=%q", ok, spec)
	}
	ok, _ = DetectSpeculativeDraft("llama", "", "", nil)
	if ok {
		t.Fatal("llama must not be draft")
	}
}

func TestGemma4AssistantApplyFlags(t *testing.T) {
	// Official E2B MTP file: no mtp- prefix, arch gemma4-assistant + NextN.
	md := &Metadata{
		Architecture: "gemma4-assistant",
		Name:         "Gemma 4 E2B It Assistant",
		Raw: map[string]any{
			"gemma4-assistant.nextn_predict_layers": uint32(4),
		},
	}
	md.extract()
	md.ApplySpeculativeFlags("/models/gemma-4-E2B-it-assistant.Q8_0.gguf")
	if !md.SpeculativeDraft || md.SpecType != SpecMTP {
		t.Fatalf("expected mtp draft: draft=%v type=%q", md.SpeculativeDraft, md.SpecType)
	}
	if !md.HasMTP || md.NextnPredictLayers != 4 {
		t.Fatalf("expected HasMTP/nextn: %+v", md)
	}
}

func TestDetectMTPSidecar(t *testing.T) {
	ok, spec := DetectSpeculativeDraft("qwen35moe", "", "/m/mtp-Qwen3.6-27B.gguf", map[string]any{
		"qwen35moe.nextn_predict_layers": uint32(1),
	})
	if !ok || spec != SpecMTP {
		t.Fatalf("mtp sidecar: ok=%v spec=%q", ok, spec)
	}
}

func TestFusedMTPIsNotDraft(t *testing.T) {
	// Trunk GGUF with NextN heads embedded — chat model, not a draft sidecar.
	md := &Metadata{
		Architecture: "qwen35moe",
		Name:         "Qwen3.6-27B-MTP",
		Raw: map[string]any{
			"qwen35moe.nextn_predict_layers": uint32(1),
			"clip.has_vision_encoder":        true,
		},
	}
	md.extract()
	md.ApplySpeculativeFlags("/models/Qwen3.6-27B-MTP-IQ4_XS.gguf")
	if md.SpeculativeDraft {
		t.Fatal("fused MTP trunk must not be speculative_draft")
	}
	if !md.HasMTP || md.NextnPredictLayers != 1 {
		t.Fatalf("expected HasMTP: %+v", md)
	}
	// Vision from CLIP keys should remain on a fused trunk.
	if !md.HasVision {
		t.Fatal("fused trunk should keep vision if CLIP keys present")
	}
}

func TestMTPSidecarClearsMultimodal(t *testing.T) {
	md := &Metadata{
		Architecture: "qwen35",
		Name:         "mtp head",
		Raw: map[string]any{
			"qwen35.nextn_predict_layers": uint32(1),
			"clip.has_vision_encoder":     true,
		},
	}
	md.extract()
	md.ApplySpeculativeFlags("/models/mtp-Qwen3-VL-4B.gguf")
	if !md.SpeculativeDraft || md.SpecType != SpecMTP {
		t.Fatalf("mtp sidecar: draft=%v type=%q", md.SpeculativeDraft, md.SpecType)
	}
	if md.HasVision || md.Multimodal {
		t.Fatal("mtp sidecar must clear multimodal")
	}
}

func TestInferSpecType(t *testing.T) {
	if got := InferSpecType("draft-eagle3", "/m/mtp-x.gguf", SpecMTP); got != SpecEagle3 {
		t.Fatalf("explicit wins: %q", got)
	}
	if got := InferSpecType("", "/m/mtp-x.gguf", SpecMTP); got != SpecMTP {
		t.Fatalf("draft mtp: %q", got)
	}
	if got := InferSpecType("", "", SpecNone); got != SpecNone {
		t.Fatalf("no draft must not invent mtp: %q", got)
	}
	if got := InferSpecType("draft-mtp", "", SpecNone); got != SpecMTP {
		t.Fatalf("explicit mtp without draft: %q", got)
	}
	if got := InferSpecType("", "/m/small.gguf", SpecNone); got != SpecSimple {
		t.Fatalf("plain draft: %q", got)
	}
}

func TestClearMultimodalOnEagle3(t *testing.T) {
	md := &Metadata{
		Architecture: "eagle3",
		Raw: map[string]any{
			"clip.has_vision_encoder": true,
			"clip.vision.patch_size":  uint32(14),
		},
	}
	md.extract()
	if !md.SpeculativeDraft || md.SpecType != SpecEagle3 {
		t.Fatalf("expected eagle3 draft: %+v", md)
	}
	if md.HasVision || md.HasAudio || md.Multimodal || md.Projector {
		t.Fatalf("draft must not keep multimodal flags: %+v", md)
	}
}
