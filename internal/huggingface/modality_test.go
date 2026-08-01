package huggingface

import (
	"reflect"
	"testing"
)

func TestDetectModalities(t *testing.T) {
	cases := []struct {
		id, pipe string
		tags     []string
		want     []string
	}{
		{"ggml-org/ultravox-v0_5-llama-3_2-1b-GGUF", "audio-text-to-text",
			[]string{"gguf", "audio-text-to-text"}, []string{"audio"}},
		{"ggml-org/Voxtral-Mini-3B-2507-GGUF", "",
			[]string{"gguf", "conversational"}, []string{"audio"}},
		{"ggml-org/Qwen3-ASR-0.6B-GGUF", "", nil, []string{"audio"}},
		{"ggml-org/llava-v1.6-mistral-7b-GGUF", "image-text-to-text",
			[]string{"image-text-to-text"}, []string{"vision"}},
		{"ggml-org/Qwen2.5-Omni-3B-GGUF", "any-to-any",
			[]string{"multimodal", "any-to-any"}, []string{"audio", "vision"}},
		{"bartowski/Llama-3.2-3B-Instruct-GGUF", "",
			[]string{"gguf", "conversational"}, nil},
		{"someone/gemma-4-E2B-it-GGUF", "", []string{"gguf"}, []string{"audio", "vision"}},
	}
	for _, tc := range cases {
		got := DetectModalities(tc.id, tc.pipe, tc.tags)
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("DetectModalities(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestModalityLabel(t *testing.T) {
	if ModalityLabel([]string{"audio"}) != "audio" {
		t.Fatal("audio")
	}
	if ModalityLabel([]string{"vision"}) != "vision" {
		t.Fatal("vision")
	}
	if ModalityLabel([]string{"audio", "vision"}) != "audio+vision" {
		t.Fatal("mixed")
	}
	if ModalityLabel(nil) != "" {
		t.Fatal("empty")
	}
}
