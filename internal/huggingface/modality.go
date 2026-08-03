package huggingface

import (
	"path/filepath"
	"strings"
)

// DetectModalities infers audio / vision capabilities from Hugging Face
// pipeline tags, repo tags, repository id heuristics, and optional file
// paths (siblings / tree). Returns a stable ordered slice: "audio" and/or
// "vision". Empty means unknown (text-only or insufficient signals).
// Speculative draft / speculator repos are never labeled multimodal — they
// are companions to a target model, not chat models.
//
// Name heuristics alone never claim audio for broad families (e.g. gemma-4):
// those need an mmproj (or an audio pipeline/tag) before audio is advertised.
func DetectModalities(repoID, pipelineTag string, tags []string, filePaths []string) []string {
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
	hasMmproj := false
	audioFileHint, visionFileHint := false, false
	for _, p := range filePaths {
		base := strings.ToLower(filepath.Base(p))
		if strings.Contains(base, "mmproj") || strings.Contains(base, "mm-proj") ||
			strings.Contains(base, "projector") {
			hasMmproj = true
		}
		if fileSuggestsAudio(base) {
			audioFileHint = true
		}
		if fileSuggestsVision(base) {
			visionFileHint = true
		}
	}

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

	// Specific audio model families — many ggml-org audio GGUFs lack pipeline tags.
	for _, h := range []string{
		"ultravox", "voxtral", "qwen3-asr", "qwen2-audio", "whisper",
		"seallm-audio", "glm-asr", "lfm2-audio", "lfm2.5-audio",
	} {
		if strings.Contains(lowerID, h) {
			audio = true
		}
	}
	// Token-bounded omni (avoid matching unrelated substrings).
	if containsToken(lowerID, "omni") {
		audio, vision = true, true
	}

	for _, h := range []string{
		"llava", "vision", "-vl-", "-vl_", "pixtral", "internvl",
		"moondream", "smolvlm", "minicpm-v", "qwen2-vl", "qwen2.5-vl",
	} {
		if strings.Contains(lowerID, h) {
			vision = true
		}
	}

	// File evidence: mmproj implies at least vision unless audio-only names.
	if hasMmproj {
		if audioFileHint {
			audio = true
		}
		if visionFileHint || !audio {
			vision = true
		}
		// Official Gemma 4 mmproj ships both encoders; only claim when a
		// projector is actually present (text-only gemma-4 quants stay plain).
		if strings.Contains(lowerID, "gemma-4") || strings.Contains(lowerID, "gemma4") {
			audio, vision = true, true
		}
	}

	if audioFileHint {
		audio = true
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

func fileSuggestsAudio(base string) bool {
	for _, h := range []string{
		"ultravox", "voxtral", "whisper", "qwen2-audio", "qwen3-asr",
		"seallm-audio", "-asr-", "_asr_", "-audio-", "_audio_",
		"-omni-", "_omni_", "omni-",
	} {
		if strings.Contains(base, h) {
			return true
		}
	}
	return containsToken(base, "asr") || containsToken(base, "audio") || containsToken(base, "omni")
}

func fileSuggestsVision(base string) bool {
	for _, h := range []string{"llava", "vision", "vl-", "pixtral", "internvl", "smolvlm", "minicpm"} {
		if strings.Contains(base, h) {
			return true
		}
	}
	return false
}

// containsToken reports whether needle appears as a path/id token bounded by
// non-alphanumeric characters (or string edges).
func containsToken(s, needle string) bool {
	if needle == "" || !strings.Contains(s, needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] != needle {
			continue
		}
		leftOK := i == 0 || !isASCIIAlnum(s[i-1])
		rightOK := i+len(needle) == len(s) || !isASCIIAlnum(s[i+len(needle)])
		if leftOK && rightOK {
			return true
		}
	}
	return false
}

func isASCIIAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z')
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
