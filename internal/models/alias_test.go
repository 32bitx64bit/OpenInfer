package models

import "testing"

func TestDeriveAlias(t *testing.T) {
	cases := []struct {
		path, name, want string
	}{
		// Solid general.name wins.
		{"/models/x/a.gguf", "MiniCPM V 4_6", "MiniCPM V 4_6"},
		// Stub general.name ("Hf") → filename without quant.
		{
			"/home/u/.local/share/openinfer-studio/models/google--gemma-4-E2B-it-qat-q4_0-gguf/gemma-4-e2b_q4_0-it-q4_0/gemma-4-E2B_q4_0-it.gguf",
			"Hf",
			"gemma-4-E2B-it",
		},
		// Empty name → strip trailing quant from file.
		{"/m/repo/q4/MyModel-Q4_K_M.gguf", "", "MyModel"},
		// Fall back to managed repo folder when filename is useless.
		{
			"/models/bartowski--Cool-Model-GGUF/q4/a.gguf",
			"Hf",
			"Cool Model",
		},
		{"/m/x.gguf", "ab", "x"},
	}
	for _, tc := range cases {
		got := deriveAlias(tc.path, tc.name)
		if got != tc.want {
			t.Errorf("deriveAlias(%q, %q) = %q, want %q", tc.path, tc.name, got, tc.want)
		}
	}
}

func TestGoodAlias(t *testing.T) {
	if goodAlias("Hf") || goodAlias("ab") || goodAlias("model") {
		t.Fatal("stubs must be rejected")
	}
	if !goodAlias("Gemma 4") || !goodAlias("MiniCPM V 4_6") {
		t.Fatal("real names must be accepted")
	}
}
