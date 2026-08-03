package huggingface

import (
	"strings"
)

// DetectModalities infers audio / vision capabilities from Hugging Face
// pipeline tags, repo tags, and repository id heuristics. Returns a stable
// ordered slice: "audio" and/or "vision". Empty means unknown (text-only or
// insufficient signals). Speculative draft / speculator repos are never
// labeled multimodal — they are companions to a target model, not chat models.
func DetectModalities(repoID, pipelineTag string, tags []string) []string {
	lowerID := strings.ToLower(repoID)
	if looksLikeSpeculativeDraftRepo(lowerID, tags) {
		return nil
	}
	lowerPipe := strings.ToLower(pipelineTag)
	tagSet := map[string]bool{}
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}

	audio, vision := false, false

	switch lowerPipe {
	case "audio-text-to-text", "automatic-speech-recognition":
		audio = true
	case "image-text-to-text", "image-to-text", "visual-question-answering":
		vision = true
	case "any-to-any":
		audio, vision = true, true
	}

	for _, t := range []string{
		"audio-text-to-text", "automatic-speech-recognition", "speech-to-text",
	} {
		if tagSet[t] {
			audio = true
		}
	}
	for _, t := range []string{
		"image-text-to-text", "image-to-text", "visual-question-answering",
	} {
		if tagSet[t] {
			vision = true
		}
	}
	if tagSet["any-to-any"] {
		audio, vision = true, true
	}

	// Name heuristics — many ggml-org audio GGUFs lack pipeline tags.
	audioHints := []string{
		"ultravox", "voxtral", "qwen3-asr", "qwen2-audio", "whisper",
		"seallm-audio", "glm-asr", "lfm2-audio", "lfm2.5-audio",
	}
	visionHints := []string{
		"llava", "vision", "-vl-", "-vl_", "pixtral", "internvl",
		"moondream", "smolvlm", "minicpm-v", "qwen2-vl", "qwen2.5-vl",
	}
	mixedHints := []string{"omni", "gemma-4", "gemma4"}

	for _, h := range audioHints {
		if strings.Contains(lowerID, h) {
			audio = true
		}
	}
	for _, h := range visionHints {
		if strings.Contains(lowerID, h) {
			vision = true
		}
	}
	for _, h := range mixedHints {
		if strings.Contains(lowerID, h) {
			audio, vision = true, true
		}
	}

	// Generic "multimodal" tag alone is not enough to claim audio.
	if tagSet["multimodal"] && !audio && !vision {
		vision = true
	}

	out := make([]string, 0, 2)
	if audio {
		out = append(out, "audio")
	}
	if vision {
		out = append(out, "vision")
	}
	return out
}

func looksLikeSpeculativeDraftRepo(lowerID string, tags []string) bool {
	// Official llama.cpp sidecar prefixes in repo ids / file names.
	for _, h := range []string{
		"/mtp-", "mtp-", "eagle3-", "dflash-", "dspark-",
		"eagle3", "eagle-3", "dflash", "dspark", "speculator",
		"draft-eagle", "draft-dflash", "draft-mtp", "spec-draft",
	} {
		if strings.Contains(lowerID, h) {
			return true
		}
	}
	for _, t := range tags {
		lt := strings.ToLower(t)
		if lt == "speculative-decoding" || lt == "eagle3" || lt == "dflash" ||
			lt == "dspark" || lt == "mtp" || lt == "speculator" || lt == "draft-model" {
			return true
		}
	}
	return false
}

// ModalityLabel returns a short UI label for a modalities slice.
func ModalityLabel(mods []string) string {
	hasA, hasV := false, false
	for _, m := range mods {
		switch m {
		case "audio":
			hasA = true
		case "vision":
			hasV = true
		}
	}
	switch {
	case hasA && hasV:
		return "audio+vision"
	case hasA:
		return "audio"
	case hasV:
		return "vision"
	default:
		return ""
	}
}
