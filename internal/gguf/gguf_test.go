package gguf

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildGGUF assembles a minimal valid GGUF header in memory.
func buildGGUF(t *testing.T, kvs map[string]any) []byte {
	t.Helper()
	var b bytes.Buffer
	w32 := func(v uint32) { binary.Write(&b, binary.LittleEndian, v) }
	w64 := func(v uint64) { binary.Write(&b, binary.LittleEndian, v) }
	wstr := func(s string) { w64(uint64(len(s))); b.WriteString(s) }

	binary.Write(&b, binary.LittleEndian, uint32(magicGGUF))
	w32(3)                // version
	w64(0)                // tensor count
	w64(uint64(len(kvs))) // kv count
	for k, v := range kvs {
		wstr(k)
		switch val := v.(type) {
		case string:
			w32(tString)
			wstr(val)
		case uint32:
			w32(tUint32)
			w32(val)
		case uint64:
			w32(tUint64)
			w64(val)
		case float32:
			w32(tFloat32)
			binary.Write(&b, binary.LittleEndian, val)
		}
	}
	return b.Bytes()
}

func TestParseValid(t *testing.T) {
	data := buildGGUF(t, map[string]any{
		"general.name":            "TestModel",
		"general.architecture":    "llama",
		"general.file_type":       uint32(15), // Q4_K_M
		"general.parameter_count": uint64(7_000_000_000),
		"llama.context_length":    uint32(4096),
		"llama.embedding_length":  uint32(4096),
		"tokenizer.ggml.model":    "llama",
	})
	md, err := parse(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if md.Name != "TestModel" || md.Architecture != "llama" {
		t.Errorf("unexpected name/arch: %+v", md)
	}
	if md.Quantization != "Q4_K_M" {
		t.Errorf("quant = %q, want Q4_K_M", md.Quantization)
	}
	if md.Parameters != 7_000_000_000 {
		t.Errorf("params = %d", md.Parameters)
	}
	if md.ContextLength != 4096 || md.Embedding != 4096 {
		t.Errorf("ctx/emb = %d/%d", md.ContextLength, md.Embedding)
	}
}

func TestParseBadMagic(t *testing.T) {
	_, err := parse(bytes.NewReader([]byte("NOPE........")), 12)
	if err != ErrBadMagic {
		t.Fatalf("want ErrBadMagic, got %v", err)
	}
}

func TestParseBadVersion(t *testing.T) {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint32(magicGGUF))
	binary.Write(&b, binary.LittleEndian, uint32(99))
	_, err := parse(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("version")) {
		t.Fatalf("want version error, got %v", err)
	}
}

func TestParseTruncated(t *testing.T) {
	data := buildGGUF(t, map[string]any{"general.name": "x"})
	// Cut the file in half — must fail cleanly, not panic.
	_, err := parse(bytes.NewReader(data[:len(data)/2]), int64(len(data)/2))
	if err == nil {
		t.Fatal("expected truncation error")
	}
}

func TestParseMaliciousCounts(t *testing.T) {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint32(magicGGUF))
	binary.Write(&b, binary.LittleEndian, uint32(3))
	binary.Write(&b, binary.LittleEndian, uint64(1<<40)) // absurd tensor count
	binary.Write(&b, binary.LittleEndian, uint64(1<<40)) // absurd kv count
	_, err := parse(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err == nil {
		t.Fatal("expected bounds error")
	}
}

func TestParseHugeStringLength(t *testing.T) {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint32(magicGGUF))
	binary.Write(&b, binary.LittleEndian, uint32(3))
	binary.Write(&b, binary.LittleEndian, uint64(0))
	binary.Write(&b, binary.LittleEndian, uint64(1))
	// Key with impossible length.
	binary.Write(&b, binary.LittleEndian, uint64(1<<62))
	_, err := parse(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err == nil {
		t.Fatal("expected bounds error for oversized string")
	}
}
