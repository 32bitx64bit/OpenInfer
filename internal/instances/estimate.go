package instances

// Memory estimation for the load dialog. These are informed heuristics in
// the spirit of other local-inference tools — always displayed as estimates.
//
// Budget math:
//   weights   = model file size (+ projector when used)
//   KV cache  = 2 (K,V) × layers × kv_heads × head_dim × ctx × bytes/elem × slots
//   compute   = max(512 MiB, 5% of weights) — activation & graph buffers
//   overhead  = ~256 MiB fixed runtime + display reserve when on GPU

// Estimate breaks down projected memory use for one configuration.
type Estimate struct {
	WeightsBytes  int64  `json:"weights_bytes"`
	KVCacheBytes  int64  `json:"kv_cache_bytes"`
	ComputeBytes  int64  `json:"compute_bytes"`
	OverheadBytes int64  `json:"overhead_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
	GPUBytes      int64  `json:"gpu_bytes"`    // portion projected onto the GPU
	CPUBytes      int64  `json:"cpu_bytes"`    // portion projected onto system RAM
	BudgetBytes   uint64 `json:"budget_bytes"` // VRAM when offloading, else RAM
	BudgetKind    string `json:"budget_kind"`  // "VRAM", "unified RAM", "RAM"
	Fits          bool   `json:"fits"`
	Note          string `json:"note"` // caveat shown next to the numbers
}

// kvBytesPerElem approximates one KV element for a cache dtype.
func kvBytesPerElem(dtype string) float64 {
	switch dtype {
	case "q8_0":
		return 1.06
	case "q4_0", "q4_1", "iq4_nl":
		return 0.56
	case "q5_0", "q5_1":
		return 0.68
	default: // f16, bf16, "" (default f16)
		return 2.0
	}
}

// kvDtype resolves the effective element size when K and V types differ
// (llama.cpp allocates one buffer per type; we approximate with the larger).
func kvDtype(k, v string) float64 {
	ek, ev := kvBytesPerElem(k), kvBytesPerElem(v)
	if ek > ev {
		return ek
	}
	return ev
}

// EstimateMemory computes the projection. weights/projector are file sizes;
// layers, kvHeads, headDim come from GGUF metadata (0 = unknown); ctx/slots
// come from load settings; offloadedLayers is how many layers go to the GPU
// (0 with gpuOffload=false = CPU only; layers = all). vram/ram are the
// detected capacities.
func EstimateMemory(weights, projector int64, layers, kvHeads, headDim uint32,
	ctx int, cacheK, cacheV string, slots int, gpuOffload bool, offloadedLayers uint32,
	vram, ram uint64) Estimate {

	if ctx <= 0 {
		ctx = 4096
	}
	if slots <= 0 {
		slots = 1
	}
	total := weights + projector
	est := Estimate{WeightsBytes: total}

	// KV cache: needs architecture metadata; otherwise estimate from a
	// conservative per-layer/per-token heuristic.
	var kvBytes int64
	if layers > 0 && kvHeads > 0 && headDim > 0 {
		kv := 2.0 * float64(layers) * float64(kvHeads) * float64(headDim) *
			float64(ctx) * kvDtype(cacheK, cacheV) * float64(slots)
		kvBytes = int64(kv)
	} else {
		// Fallback: ~1 MiB per token of context per slot at f16 — typical for
		// 7–13B dense models; flagged as a rougher estimate.
		kvBytes = int64(float64(int64(ctx)*int64(slots)) * float64(int64(1)<<20) * kvDtype(cacheK, cacheV) / 2)
		est.Note = "rough estimate (architecture metadata incomplete)"
	}
	est.KVCacheBytes = kvBytes

	est.ComputeBytes = total / 20
	if est.ComputeBytes < 512<<20 {
		est.ComputeBytes = 512 << 20
	}
	est.OverheadBytes = 256 << 20

	est.TotalBytes = est.WeightsBytes + est.KVCacheBytes + est.ComputeBytes + est.OverheadBytes

	switch {
	case gpuOffload && vram > 0:
		est.BudgetBytes = vram
		est.BudgetKind = "VRAM"
	case gpuOffload && vram == 0:
		// Unified memory (Apple Silicon, AMD iGPU): GPU shares system RAM.
		est.BudgetBytes = ram
		est.BudgetKind = "unified RAM"
		est.Note = "GPU shares system memory on this machine"
	default:
		est.BudgetBytes = ram
		est.BudgetKind = "RAM"
	}

	// Partial offload: split weights + KV between GPU and CPU by layer share.
	if gpuOffload && layers > 0 && offloadedLayers > 0 && offloadedLayers < layers {
		frac := float64(offloadedLayers) / float64(layers)
		gpuW := int64(frac * float64(est.WeightsBytes))
		gpuKV := int64(frac * float64(est.KVCacheBytes))
		est.GPUBytes = gpuW + gpuKV + est.ComputeBytes + est.OverheadBytes
		est.CPUBytes = (est.WeightsBytes - gpuW) + (est.KVCacheBytes - gpuKV)
		gpuFits := uint64(est.GPUBytes) <= est.BudgetBytes-est.BudgetBytes/10
		cpuFits := uint64(est.CPUBytes) <= ram-ram/10
		est.Fits = gpuFits && cpuFits
		return est
	}

	// Full offload or CPU-only: single budget.
	if gpuOffload {
		est.GPUBytes = est.TotalBytes
	} else {
		est.CPUBytes = est.TotalBytes
	}
	// Leave ~10% headroom: fits when under 90% of budget.
	est.Fits = uint64(est.TotalBytes) <= est.BudgetBytes-est.BudgetBytes/10
	return est
}
