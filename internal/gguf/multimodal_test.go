package gguf

import "testing"

func TestExtractMultimodalFlags(t *testing.T) {
	md := &Metadata{Raw: map[string]any{
		"clip.has_audio_encoder": true,
		"audio.block_count":      uint32(12),
	}}
	md.extract()
	if !md.HasAudio || !md.Multimodal || md.HasVision {
		t.Fatalf("audio-only: %+v", md)
	}

	md2 := &Metadata{Raw: map[string]any{
		"clip.has_vision_encoder": true,
		"clip.vision.patch_size":  uint32(14),
	}, Architecture: "clip"}
	md2.extract()
	if !md2.HasVision || !md2.Multimodal || md2.HasAudio || !md2.Projector {
		t.Fatalf("vision: %+v", md2)
	}

	md3 := &Metadata{Raw: map[string]any{
		"clip.has_vision_encoder": true,
		"clip.has_audio_encoder":  true,
	}}
	md3.extract()
	if !md3.HasVision || !md3.HasAudio {
		t.Fatalf("mixed: %+v", md3)
	}
}
