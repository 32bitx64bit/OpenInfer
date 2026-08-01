package huggingface

import (
	"testing"
)

func TestQuantOf(t *testing.T) {
	cases := map[string]string{
		"model-Q4_K_M.gguf":                "Q4_K_M",
		"some-model.IQ4_XS.gguf":           "IQ4_XS",
		"weird.name-q8_0.gguf":             "Q8_0",
		"model-F16.gguf":                   "F16",
		"model.gguf":                       "",
		"model-Q4_K_M-00001-of-00002.gguf": "Q4_K_M",
	}
	for in, want := range cases {
		if got := quantOf(in); got != want {
			t.Errorf("quantOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGroupFilesSplitSet(t *testing.T) {
	files := []FileEntry{
		{Path: "model-IQ4_XS-00001-of-00003.gguf", Size: 100},
		{Path: "model-IQ4_XS-00002-of-00003.gguf", Size: 100},
		{Path: "model-IQ4_XS-00003-of-00003.gguf", Size: 100},
		{Path: "README.md", Size: 10},
	}
	groups, _ := GroupFiles(files)
	if len(groups) != 1 {
		t.Fatalf("want 1 group, got %d: %+v", len(groups), groups)
	}
	g := groups[0]
	if !g.Split || g.Parts != 3 || len(g.Files) != 3 {
		t.Errorf("split set malformed: %+v", g)
	}
	if g.TotalBytes != 300 {
		t.Errorf("total = %d, want 300", g.TotalBytes)
	}
}

func TestGroupFilesVisionPairing(t *testing.T) {
	files := []FileEntry{
		{Path: "llava-Q4_K_M.gguf", Size: 4000},
		{Path: "mmproj-llava-f16.gguf", Size: 600},
	}
	groups, projectors := GroupFiles(files)
	if len(groups) != 1 {
		t.Fatalf("want exactly one model group (no vision variant), got %+v", groups)
	}
	if groups[0].Vision || len(groups[0].Files) != 1 {
		t.Errorf("base group must not include the projector: %+v", groups[0])
	}
	if len(projectors) != 1 || projectors[0].Path != "mmproj-llava-f16.gguf" {
		t.Errorf("projector must be returned separately: %+v", projectors)
	}
}

func TestGroupFilesProjectorOnlyRepo(t *testing.T) {
	files := []FileEntry{
		{Path: "mmproj-a-f16.gguf", Size: 600},
		{Path: "mmproj-b-f16.gguf", Size: 700},
	}
	groups, projectors := GroupFiles(files)
	if len(groups) != 1 || !groups[0].Vision || len(groups[0].Files) != 2 {
		t.Fatalf("projector-only repo should offer one projector group: %+v", groups)
	}
	if len(projectors) != 0 {
		t.Errorf("projectors already in the group must not be returned again: %+v", projectors)
	}
}

func TestGroupFilesExcludesNonGGUF(t *testing.T) {
	files := []FileEntry{
		{Path: "model.safetensors", Size: 1},
		{Path: "config.json", Size: 1},
		{Path: "tokenizer.json", Size: 1},
	}
	if groups, _ := GroupFiles(files); len(groups) != 0 {
		t.Fatalf("non-GGUF files must be excluded, got %+v", groups)
	}
}

func TestGroupUniqueIDs(t *testing.T) {
	files := []FileEntry{
		{Path: "a/model-Q4_K_M.gguf", Size: 1},
		{Path: "b/model-Q4_K_M.gguf", Size: 1},
	}
	groups, _ := GroupFiles(files)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(groups))
	}
	if groups[0].ID == groups[1].ID {
		t.Errorf("group IDs must be unique: %q", groups[0].ID)
	}
}
