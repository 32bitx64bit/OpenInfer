package instances

import "testing"

func TestEstimateKVCache(t *testing.T) {
	// 7B-class: 32 layers, 8 KV heads, 128 head dim, 4096 ctx, f16, 1 slot.
	est := EstimateMemory(4_000_000_000, 0, 32, 8, 128, 4096, "", "", 1, true, 32, 8<<30, 32<<30)
	// 2*32*8*128*4096*2 = 536,870,912 (0.5 GiB)
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

func TestEstimateContextScaling(t *testing.T) {
	small := EstimateMemory(4<<30, 0, 32, 8, 128, 4096, "", "", 1, false, 0, 0, 16<<30)
	big := EstimateMemory(4<<30, 0, 32, 8, 128, 131072, "", "", 1, false, 0, 0, 16<<30)
	if big.KVCacheBytes != small.KVCacheBytes*32 {
		t.Errorf("kv should scale linearly with ctx: %d vs %d", big.KVCacheBytes, small.KVCacheBytes)
	}
	if big.BudgetKind != "RAM" {
		t.Errorf("cpu offload → RAM budget, got %s", big.BudgetKind)
	}
}

func TestEstimateQuantizedKV(t *testing.T) {
	f16 := EstimateMemory(0, 0, 32, 8, 128, 8192, "f16", "f16", 1, true, 32, 16<<30, 0)
	q8 := EstimateMemory(0, 0, 32, 8, 128, 8192, "q8_0", "q8_0", 1, true, 32, 16<<30, 0)
	if q8.KVCacheBytes >= f16.KVCacheBytes {
		t.Error("q8_0 KV cache must be smaller than f16")
	}
}

func TestEstimateSlotsMultiply(t *testing.T) {
	one := EstimateMemory(0, 0, 32, 8, 128, 4096, "", "", 1, true, 32, 16<<30, 0)
	four := EstimateMemory(0, 0, 32, 8, 128, 4096, "", "", 4, true, 32, 16<<30, 0)
	if four.KVCacheBytes != one.KVCacheBytes*4 {
		t.Error("kv should scale with parallel slots")
	}
}

func TestEstimateFallbackWithoutMetadata(t *testing.T) {
	est := EstimateMemory(2<<30, 0, 0, 0, 0, 4096, "", "", 1, false, 0, 0, 16<<30)
	if est.KVCacheBytes <= 0 {
		t.Error("fallback kv estimate must be positive")
	}
	if est.Note == "" {
		t.Error("fallback must be flagged as a rough estimate")
	}
}

func TestEstimateFitFails(t *testing.T) {
	est := EstimateMemory(14<<30, 0, 40, 8, 128, 65536, "", "", 1, true, 32, 8<<30, 32<<30)
	if est.Fits {
		t.Error("14 GiB model + 64k ctx must not fit 8 GiB VRAM")
	}
}

func TestEstimatePartialOffload(t *testing.T) {
	// Half of 32 layers on an 8 GiB GPU: GPU holds half weights+KV plus
	// compute; CPU holds the rest.
	est := EstimateMemory(8<<30, 0, 32, 8, 128, 4096, "", "", 1, true, 16, 8<<30, 64<<30)
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
	// Tiny GPU: even partial offload of a big model must fail the GPU side.
	est := EstimateMemory(20<<30, 0, 32, 8, 128, 4096, "", "", 1, true, 16, 4<<30, 128<<30)
	if est.Fits {
		t.Error("half of a 20 GiB model must not fit in 4 GiB VRAM")
	}
}
