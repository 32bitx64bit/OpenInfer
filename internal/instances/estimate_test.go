package instances

import (
	"strings"
	"testing"
)

func baseIn() EstimateInput {
	return EstimateInput{
		Weights: 4_000_000_000, Layers: 32, KVHeads: 8, HeadDim: 128,
		Ctx: 4096, Slots: 1, GPUOffload: true, OffloadedLayers: 32,
		VRAM: 8 << 30, RAM: 32 << 30,
	}
}

func TestEstimateKVCache(t *testing.T) {
	// 7B-class: 32 layers, 8 KV heads, 128 head dim, 4096 ctx, f16, 1 slot.
	// layers×kv_heads×(k_dim+v_dim)×bytes×ctx×slots = 32×8×(128+128)×2×4096
	est := EstimateMemory(baseIn())
	wantKV := int64(536_870_912)
	if est.KVCacheBytes != wantKV {
		t.Errorf("kv = %d, want %d", est.KVCacheBytes, wantKV)
	}
	if est.BudgetKind != "VRAM" || est.BudgetBytes != 8<<30 {
		t.Errorf("budget = %s %d", est.BudgetKind, est.BudgetBytes)
	}
	if !est.Fits {
		t.Error("4 GiB model + small ctx should fit 8 GiB VRAM")
	}
}

func TestEstimateSeparateKVDtypes(t *testing.T) {
	in := baseIn()
	in.Weights = 0
	in.CacheK = "q4_0"
	in.CacheV = "f16"
	mixed := EstimateMemory(in)
	in.CacheK, in.CacheV = "f16", "f16"
	bothF16 := EstimateMemory(in)
	in.CacheK, in.CacheV = "q4_0", "q4_0"
	bothQ4 := EstimateMemory(in)
	if mixed.KVCacheBytes >= bothF16.KVCacheBytes {
		t.Error("mixed q4/f16 must be smaller than both f16")
	}
	if mixed.KVCacheBytes <= bothQ4.KVCacheBytes {
		t.Error("mixed q4/f16 must be larger than both q4")
	}
}

func TestEstimateContextScaling(t *testing.T) {
	smallIn := baseIn()
	smallIn.GPUOffload = false
	smallIn.OffloadedLayers = 0
	smallIn.VRAM = 0
	smallIn.RAM = 16 << 30
	small := EstimateMemory(smallIn)
	bigIn := smallIn
	bigIn.Ctx = 131072
	big := EstimateMemory(bigIn)
	if big.KVCacheBytes != small.KVCacheBytes*32 {
		t.Errorf("kv should scale linearly with ctx: %d vs %d", big.KVCacheBytes, small.KVCacheBytes)
	}
	if big.BudgetKind != "RAM" {
		t.Errorf("cpu offload → RAM budget, got %s", big.BudgetKind)
	}
}

func TestEstimateModelContextDefault(t *testing.T) {
	in := baseIn()
	in.Ctx = 0
	in.ModelContext = 8192
	est := EstimateMemory(in)
	in.Ctx = 8192
	in.ModelContext = 0
	want := EstimateMemory(in)
	if est.KVCacheBytes != want.KVCacheBytes {
		t.Errorf("Ctx=0 should use ModelContext: got %d want %d", est.KVCacheBytes, want.KVCacheBytes)
	}
}

func TestEstimateQuantizedKV(t *testing.T) {
	in := baseIn()
	in.Weights = 0
	in.Ctx = 8192
	in.VRAM = 16 << 30
	in.CacheK, in.CacheV = "f16", "f16"
	f16 := EstimateMemory(in)
	in.CacheK, in.CacheV = "q8_0", "q8_0"
	q8 := EstimateMemory(in)
	if q8.KVCacheBytes >= f16.KVCacheBytes {
		t.Error("q8_0 KV cache must be smaller than f16")
	}
}

func TestEstimateSlotsMultiply(t *testing.T) {
	in := baseIn()
	in.Weights = 0
	in.VRAM = 16 << 30
	one := EstimateMemory(in)
	in.Slots = 4
	four := EstimateMemory(in)
	if four.KVCacheBytes != one.KVCacheBytes*4 {
		t.Error("kv should scale with parallel slots")
	}
}

func TestEstimateFallbackWithoutMetadata(t *testing.T) {
	est := EstimateMemory(EstimateInput{
		Weights: 2 << 30, Ctx: 4096, RAM: 16 << 30,
	})
	if est.KVCacheBytes <= 0 {
		t.Error("fallback kv estimate must be positive")
	}
	if est.Note == "" {
		t.Error("fallback must be flagged as a rough estimate")
	}
}

func TestEstimateFitFails(t *testing.T) {
	est := EstimateMemory(EstimateInput{
		Weights: 14 << 30, Layers: 40, KVHeads: 8, HeadDim: 128,
		Ctx: 65536, GPUOffload: true, OffloadedLayers: 32,
		VRAM: 8 << 30, RAM: 32 << 30,
	})
	if est.Fits {
		t.Error("14 GiB model + 64k ctx must not fit 8 GiB VRAM")
	}
}

func TestEstimatePartialOffload(t *testing.T) {
	est := EstimateMemory(EstimateInput{
		Weights: 8 << 30, Layers: 32, KVHeads: 8, HeadDim: 128,
		Ctx: 4096, GPUOffload: true, OffloadedLayers: 16,
		VRAM: 8 << 30, RAM: 64 << 30,
	})
	if est.GPUBytes == 0 || est.CPUBytes == 0 {
		t.Fatalf("partial offload must split: gpu=%d cpu=%d", est.GPUBytes, est.CPUBytes)
	}
	if est.GPUBytes+est.CPUBytes != est.TotalBytes {
		t.Errorf("gpu+cpu (%d) != total (%d)", est.GPUBytes+est.CPUBytes, est.TotalBytes)
	}
	halfW := int64(0.5 * float64(8<<30))
	if est.CPUBytes != halfW+est.KVCacheBytes/2 {
		t.Errorf("cpu share wrong: %d (want weights/2 + kv/2 = %d)", est.CPUBytes, halfW+est.KVCacheBytes/2)
	}
}

func TestEstimatePartialOffloadFitFailure(t *testing.T) {
	est := EstimateMemory(EstimateInput{
		Weights: 20 << 30, Layers: 32, KVHeads: 8, HeadDim: 128,
		Ctx: 4096, GPUOffload: true, OffloadedLayers: 16,
		VRAM: 4 << 30, RAM: 128 << 30,
	})
	if est.Fits {
		t.Error("half of a 20 GiB model must not fit in 4 GiB VRAM")
	}
}

func TestEstimateMultimodalMedia(t *testing.T) {
	est := EstimateMemory(EstimateInput{
		Weights: 4 << 30, Projector: 800 << 20, Layers: 32, KVHeads: 8, HeadDim: 128,
		Ctx: 4096, GPUOffload: true, OffloadedLayers: 32,
		VRAM: 24 << 30, RAM: 64 << 30, HasVision: true,
	})
	if est.MediaBytes <= 0 || est.ProjectorBytes != 800<<20 {
		t.Fatalf("media=%d projector=%d", est.MediaBytes, est.ProjectorBytes)
	}
	if est.TotalBytes <= est.WeightsBytes+est.ProjectorBytes+est.KVCacheBytes {
		t.Error("total must include media + overhead")
	}
}

func TestEstimateNoMmproj(t *testing.T) {
	with := EstimateMemory(EstimateInput{
		Weights: 4 << 30, Projector: 800 << 20, Layers: 32, KVHeads: 8, HeadDim: 128,
		Ctx: 4096, GPUOffload: true, OffloadedLayers: 32,
		VRAM: 24 << 30, RAM: 64 << 30, HasVision: true,
	})
	skip := EstimateMemory(EstimateInput{
		Weights: 4 << 30, Projector: 800 << 20, Layers: 32, KVHeads: 8, HeadDim: 128,
		Ctx: 4096, GPUOffload: true, OffloadedLayers: 32,
		VRAM: 24 << 30, RAM: 64 << 30, HasVision: true, NoMmproj: true,
	})
	if skip.ProjectorBytes != 0 || skip.MediaBytes != 0 {
		t.Fatalf("skip must zero projector/media: projector=%d media=%d", skip.ProjectorBytes, skip.MediaBytes)
	}
	if skip.TotalBytes >= with.TotalBytes {
		t.Errorf("skip total %d should be less than with projector %d", skip.TotalBytes, with.TotalBytes)
	}
	if !strings.Contains(skip.Note, "projector skipped") {
		t.Errorf("expected skip note, got %q", skip.Note)
	}
}

func TestEstimateValueDim(t *testing.T) {
	in := baseIn()
	in.Weights = 0
	same := EstimateMemory(in)
	in.ValueDim = 64 // half of head dim
	half := EstimateMemory(in)
	if half.KVCacheBytes >= same.KVCacheBytes {
		t.Error("smaller value_dim must shrink KV")
	}
}

func TestEstimateSlidingWindow(t *testing.T) {
	// Gemma-4-like: 6 layers, pattern SWA/SWA/SWA/SWA/SWA/global, window 1024.
	pattern := []bool{true, true, true, true, true, false}
	heads := []uint32{8, 8, 8, 8, 8, 1}
	naive := EstimateMemory(EstimateInput{
		Weights: 0, Layers: 6, KVHeads: 8, HeadDim: 512, ValueDim: 512,
		Ctx: 65536, GPUOffload: true, OffloadedLayers: 6,
		VRAM: 64 << 30, RAM: 64 << 30,
	})
	swa := EstimateMemory(EstimateInput{
		Weights: 0, Layers: 6, KVHeads: 8, HeadDim: 512, ValueDim: 512,
		HeadDimSWA: 256, ValueDimSWA: 256, SlidingWindow: 1024,
		SlidingWindowPattern: pattern, HeadCountKVLayers: heads,
		Ctx: 65536, GPUOffload: true, OffloadedLayers: 6,
		VRAM: 64 << 30, RAM: 64 << 30,
	})
	if swa.KVCacheBytes >= naive.KVCacheBytes/4 {
		t.Fatalf("SWA should cut KV sharply: swa=%d naive=%d", swa.KVCacheBytes, naive.KVCacheBytes)
	}
	// 5 SWA layers: 8 heads × (256+256)×2 × 1024 = 8*512*2*1024 = 8_388_608 each → 41_943_040
	// 1 global: 1 × (512+512)×2 × 65536 = 134_217_728
	want := int64(5*8*512*2*1024 + 1*1024*2*65536)
	if swa.KVCacheBytes != want {
		t.Fatalf("kv=%d want %d", swa.KVCacheBytes, want)
	}
}

func TestEstimateHybridInterval(t *testing.T) {
	// Qwen3.5-like: full attention every 4th layer.
	full := EstimateMemory(EstimateInput{
		Weights: 0, Layers: 24, KVHeads: 2, HeadDim: 256, ValueDim: 256,
		Ctx: 4096, GPUOffload: true, OffloadedLayers: 24,
		VRAM: 16 << 30, RAM: 32 << 30,
	})
	hybrid := EstimateMemory(EstimateInput{
		Weights: 0, Layers: 24, KVHeads: 2, HeadDim: 256, ValueDim: 256,
		FullAttentionInterval: 4, SSMStateSize: 128, SSMInnerSize: 2048,
		Ctx: 4096, GPUOffload: true, OffloadedLayers: 24,
		VRAM: 16 << 30, RAM: 32 << 30,
	})
	if hybrid.KVCacheBytes*4 != full.KVCacheBytes {
		t.Fatalf("interval=4 should use 1/4 KV layers: hybrid=%d full=%d", hybrid.KVCacheBytes, full.KVCacheBytes)
	}
	if hybrid.RecurrentBytes <= 0 {
		t.Fatal("hybrid should estimate recurrent state for non-KV layers")
	}
}

func TestEstimateSharedKVLayers(t *testing.T) {
	all := EstimateMemory(EstimateInput{
		Weights: 0, Layers: 20, KVHeads: 1, HeadDim: 512, ValueDim: 512,
		Ctx: 4096, GPUOffload: true, OffloadedLayers: 20,
		VRAM: 16 << 30, RAM: 32 << 30,
	})
	shared := EstimateMemory(EstimateInput{
		Weights: 0, Layers: 20, KVHeads: 1, HeadDim: 512, ValueDim: 512,
		SharedKVLayers: 10, Ctx: 4096, GPUOffload: true, OffloadedLayers: 20,
		VRAM: 16 << 30, RAM: 32 << 30,
	})
	if shared.KVCacheBytes*2 != all.KVCacheBytes {
		t.Fatalf("shared_kv=10 of 20 should halve KV: shared=%d all=%d", shared.KVCacheBytes, all.KVCacheBytes)
	}
}
