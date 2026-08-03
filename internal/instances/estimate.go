package instances

import "strings"

// Memory estimation for the load dialog. These are informed heuristics —
// always displayed as estimates, never as guarantees.
//
// Budget math (llama.cpp-shaped):
//   weights   = model GGUF size
//   projector = mmproj file size (when paired)
//   KV cache  = per-layer (heads × (k_dim×bK + v_dim×bV) × tokens × slots)
//               with SWA / hybrid / shared-KV aware layer selection
//   recurrent = SSM / linear-attention state for non-KV hybrid layers
//   compute   = activation/graph scratch
//   overhead  = fixed runtime / driver reserve
//   media     = multimodal encoder activation headroom (when mmproj present)
//
// Placement:
//   full GPU offload → weights/KV/compute on GPU; optional mmproj on CPU
//   partial offload  → layer fraction of weights+KV on GPU, remainder on RAM
//   CPU only         → everything on system RAM
// GPU VRAM and system RAM are budgeted independently; Fits requires both.

// Estimate breaks down projected memory use for one configuration.
type Estimate struct {
	WeightsBytes      int64   `json:"weights_bytes"`
	DraftWeightsBytes int64   `json:"draft_weights_bytes"`
	ProjectorBytes    int64   `json:"projector_bytes"`
	KVCacheBytes      int64   `json:"kv_cache_bytes"`
	RecurrentBytes    int64   `json:"recurrent_bytes"` // hybrid SSM / linear-attn state
	ComputeBytes      int64   `json:"compute_bytes"`
	MediaBytes        int64   `json:"media_bytes"` // multimodal encoder activations
	OverheadBytes     int64   `json:"overhead_bytes"`
	TotalBytes        int64   `json:"total_bytes"`
	GPUBytes          int64   `json:"gpu_bytes"`
	CPUBytes          int64   `json:"cpu_bytes"`
	GPUBudgetBytes    uint64  `json:"gpu_budget_bytes"` // discrete VRAM (0 on unified / CPU-only)
	CPUBudgetBytes    uint64  `json:"cpu_budget_bytes"` // system RAM budget
	BudgetBytes       uint64  `json:"budget_bytes"`     // primary budget for back-compat UI
	BudgetKind        string  `json:"budget_kind"`      // "VRAM", "unified RAM", "RAM", "VRAM+RAM"
	FitsGPU           bool    `json:"fits_gpu"`
	FitsCPU           bool    `json:"fits_cpu"`
	Fits              bool    `json:"fits"`
	OffloadFraction   float64 `json:"offload_fraction"` // 0..1 of layers on GPU
	Note              string  `json:"note"`
}

// EstimateInput collects everything EstimateMemory needs.
type EstimateInput struct {
	Weights           int64
	DraftWeights      int64 // speculative draft GGUF size
	Projector         int64
	Layers            uint32
	KVHeads           uint32
	HeadCountKVLayers []uint32 // per-layer override; empty = use KVHeads
	HeadDim           uint32   // key (and usually value) dimension
	ValueDim          uint32   // 0 = same as HeadDim
	HeadDimSWA        uint32
	ValueDimSWA       uint32
	SlidingWindow     uint32
	// SlidingWindowPattern: true = SWA layer, false = full-context. Empty = all full.
	SlidingWindowPattern  []bool
	SharedKVLayers        uint32
	FullAttentionInterval uint32 // hybrid: dense KV every Nth layer
	SSMStateSize          uint32
	SSMInnerSize          uint32
	EmbeddingLength       uint32
	Ctx                   int
	ModelContext          int // GGUF default when Ctx <= 0
	CacheK                string
	CacheV                string
	Slots                 int
	BatchSize             int
	UBatchSize            int
	GPUOffload            bool
	OffloadedLayers       uint32
	VRAM                  uint64
	RAM                   uint64
	HasVision             bool
	HasAudio              bool
	NoMmproj              bool // skip projector + media entirely
	NoMmprojOffload       bool
	FlashAttention        string // auto|on|off
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

// EstimateMemory computes the projection from EstimateInput.
func EstimateMemory(in EstimateInput) Estimate {
	ctx := in.Ctx
	if ctx <= 0 {
		ctx = in.ModelContext
	}
	if ctx <= 0 {
		ctx = 4096
	}
	slots := in.Slots
	if slots <= 0 {
		slots = 1
	}
	valueDim := in.ValueDim
	if valueDim == 0 {
		valueDim = in.HeadDim
	}

	est := Estimate{
		WeightsBytes:      in.Weights,
		DraftWeightsBytes: in.DraftWeights,
	}
	projector := in.Projector
	if in.NoMmproj {
		projector = 0
	}
	est.ProjectorBytes = projector

	kv, recurrent, kvNote := estimateKVCache(in, ctx, slots, valueDim)
	est.KVCacheBytes = kv
	est.RecurrentBytes = recurrent
	if kvNote != "" {
		est.Note = kvNote
	}

	// Compute / activation scratch — keep modest; llama.cpp graph workspace
	// is typically hundreds of MiB, not a large fraction of weights.
	weightBase := in.Weights + in.DraftWeights + projector
	compute := weightBase / 40
	if compute < 256<<20 {
		compute = 256 << 20
	}
	batch := in.BatchSize
	if batch <= 0 {
		batch = 512
	}
	ubatch := in.UBatchSize
	if ubatch <= 0 {
		ubatch = batch
	}
	if in.EmbeddingLength > 0 {
		scratch := int64(ubatch) * int64(in.EmbeddingLength) * 16 * int64(slots)
		if scratch > compute {
			compute = scratch
		}
	}
	if in.FlashAttention == "on" || in.FlashAttention == "auto" {
		compute = compute * 85 / 100
	}
	est.ComputeBytes = compute

	// Multimodal encoder activations (images/audio through libmtmd).
	if projector > 0 {
		media := projector / 2
		minMedia := int64(256 << 20)
		if in.HasAudio && !in.HasVision {
			minMedia = 128 << 20
		}
		if media < minMedia {
			media = minMedia
		}
		if in.HasVision && in.HasAudio {
			media = media + media/4 // omni models need both encoders
		}
		est.MediaBytes = media
	}
	if in.NoMmproj && in.Projector > 0 {
		if est.Note == "" {
			est.Note = "multimodal projector skipped"
		} else {
			est.Note += "; multimodal projector skipped"
		}
	}

	est.OverheadBytes = 128 << 20
	est.TotalBytes = est.WeightsBytes + est.DraftWeightsBytes + est.ProjectorBytes +
		est.KVCacheBytes + est.RecurrentBytes + est.ComputeBytes + est.MediaBytes + est.OverheadBytes

	// Independent budgets: discrete GPU VRAM vs system RAM.
	unified := in.GPUOffload && in.VRAM == 0
	est.CPUBudgetBytes = in.RAM
	if in.GPUOffload && !unified {
		est.GPUBudgetBytes = in.VRAM
		est.BudgetKind = "VRAM+RAM"
		est.BudgetBytes = in.VRAM // primary bar stays VRAM for back-compat
	} else if unified {
		est.GPUBudgetBytes = 0
		est.BudgetBytes = in.RAM
		est.BudgetKind = "unified RAM"
		if est.Note == "" {
			est.Note = "GPU shares system memory on this machine"
		} else if !strings.Contains(est.Note, "GPU shares") {
			est.Note += "; GPU shares system memory"
		}
	} else {
		est.GPUBudgetBytes = 0
		est.BudgetBytes = in.RAM
		est.BudgetKind = "RAM"
	}

	place(in, &est, projector, unified)
	return est
}

// place assigns component bytes to GPU vs system RAM and evaluates fits.
func place(in EstimateInput, est *Estimate, projector int64, unified bool) {
	projOnGPU := in.GPUOffload && projector > 0 && !in.NoMmprojOffload
	mediaOnGPU := projOnGPU

	frac := offloadFraction(in)
	est.OffloadFraction = frac

	weights := est.WeightsBytes + est.DraftWeightsBytes
	kvAll := est.KVCacheBytes + est.RecurrentBytes

	switch {
	case !in.GPUOffload || frac <= 0:
		est.GPUBytes = 0
		est.CPUBytes = est.TotalBytes
		est.OffloadFraction = 0

	case frac >= 1:
		est.GPUBytes = est.TotalBytes
		est.CPUBytes = 0
		if in.NoMmprojOffload && (est.ProjectorBytes > 0 || est.MediaBytes > 0) {
			est.GPUBytes -= est.ProjectorBytes + est.MediaBytes
			est.CPUBytes = est.ProjectorBytes + est.MediaBytes
		}
		est.OffloadFraction = 1

	default:
		// Partial offload: layer fraction of weights + KV on GPU.
		gpuW := int64(frac * float64(weights))
		gpuKV := int64(frac * float64(kvAll))
		est.GPUBytes = gpuW + gpuKV + est.ComputeBytes + est.OverheadBytes
		est.CPUBytes = (weights - gpuW) + (kvAll - gpuKV)
		if projOnGPU {
			est.GPUBytes += est.ProjectorBytes
		} else {
			est.CPUBytes += est.ProjectorBytes
		}
		if mediaOnGPU {
			est.GPUBytes += est.MediaBytes
		} else {
			est.CPUBytes += est.MediaBytes
		}
	}

	headroom := func(budget uint64) uint64 {
		if budget == 0 {
			return 0
		}
		return budget - budget/10
	}

	if unified {
		// Everything competes for system RAM.
		used := uint64(est.GPUBytes + est.CPUBytes)
		est.FitsCPU = used <= headroom(est.CPUBudgetBytes)
		est.FitsGPU = true
		est.Fits = est.FitsCPU
		return
	}

	if est.GPUBudgetBytes > 0 {
		est.FitsGPU = uint64(est.GPUBytes) <= headroom(est.GPUBudgetBytes)
	} else {
		est.FitsGPU = est.GPUBytes == 0
	}
	est.FitsCPU = uint64(est.CPUBytes) <= headroom(est.CPUBudgetBytes)
	est.Fits = est.FitsGPU && est.FitsCPU
}

// offloadFraction returns how much of the model layers land on the GPU.
// When layer metadata is missing but OffloadedLayers is set (custom slider
// with unknown block_count), treat OffloadedLayers as an absolute request
// against a synthetic denominator so the estimate still reacts to the slider.
func offloadFraction(in EstimateInput) float64 {
	if !in.GPUOffload {
		return 0
	}
	layers := in.Layers
	off := in.OffloadedLayers
	if layers == 0 {
		if off == 0 {
			// "all" / auto with unknown layer count → assume full offload.
			return 1
		}
		// Custom N with unknown block_count: use N/(N) only when N looks like
		// "all" sentinel (999), otherwise scale against a soft max of max(N, 32).
		if off >= 999 {
			return 1
		}
		den := off
		if den < 32 {
			den = 32
		}
		f := float64(off) / float64(den)
		if f > 1 {
			f = 1
		}
		return f
	}
	if off >= layers {
		return 1
	}
	return float64(off) / float64(layers)
}

// estimateKVCache walks layers with SWA / hybrid / shared-KV awareness.
func estimateKVCache(in EstimateInput, ctx, slots int, valueDim uint32) (kvBytes, recurrentBytes int64, note string) {
	layers := int(in.Layers)
	if layers <= 0 || in.HeadDim == 0 || (in.KVHeads == 0 && len(in.HeadCountKVLayers) == 0) {
		// Fallback: ~128 KiB/token/slot at f16 — rough 7–13B GQA class.
		per := float64(int64(128) << 10)
		kvBytes = int64(float64(ctx*slots) * per * kvBytesPerElem(in.CacheK) / 2)
		note = "rough estimate (architecture metadata incomplete)"
		return kvBytes, 0, note
	}

	allocLayers := layers
	if in.SharedKVLayers > 0 && int(in.SharedKVLayers) < layers {
		allocLayers = layers - int(in.SharedKVLayers)
	}

	bytesK := kvBytesPerElem(in.CacheK)
	bytesV := kvBytesPerElem(in.CacheV)
	hasPattern := len(in.SlidingWindowPattern) > 0
	hasInterval := in.FullAttentionInterval > 1

	var total float64
	kvLayers := 0
	for i := 0; i < allocLayers; i++ {
		// Hybrid linear-attn / SSM layers: only every Nth layer holds dense KV.
		if hasInterval && !hasPattern {
			if ((i + 1) % int(in.FullAttentionInterval)) != 0 {
				continue
			}
		}

		isSWA := false
		if hasPattern {
			if i < len(in.SlidingWindowPattern) {
				isSWA = in.SlidingWindowPattern[i]
			}
		}

		heads := in.KVHeads
		if i < len(in.HeadCountKVLayers) && in.HeadCountKVLayers[i] > 0 {
			heads = in.HeadCountKVLayers[i]
		}
		if heads == 0 {
			continue
		}

		kDim := in.HeadDim
		vDim := valueDim
		tokens := ctx
		if isSWA && in.SlidingWindow > 0 {
			tokens = int(in.SlidingWindow)
			if tokens > ctx {
				tokens = ctx
			}
			if in.HeadDimSWA > 0 {
				kDim = in.HeadDimSWA
			}
			if in.ValueDimSWA > 0 {
				vDim = in.ValueDimSWA
			}
		}

		perToken := float64(heads) * (float64(kDim)*bytesK + float64(vDim)*bytesV)
		total += perToken * float64(tokens) * float64(slots)
		kvLayers++
	}
	kvBytes = int64(total)

	// Fixed-size recurrent / SSM state for layers without dense KV.
	if hasInterval && !hasPattern && in.SSMStateSize > 0 {
		nonKV := allocLayers - kvLayers
		if nonKV < 0 {
			nonKV = 0
		}
		inner := in.SSMInnerSize
		if inner == 0 {
			inner = in.EmbeddingLength
		}
		if inner == 0 {
			inner = 2048
		}
		// Empirically ~state×inner×4 bytes per non-KV layer (matches llama.cpp
		// recurrent footprints on Qwen3.5-class hybrids within ~20%).
		recurrentBytes = int64(nonKV) * int64(in.SSMStateSize) * int64(inner) * 4 * int64(slots)
	}

	switch {
	case hasPattern:
		note = "sliding-window layers capped at window size"
	case hasInterval:
		note = "hybrid model: dense KV on a subset of layers"
	case in.SharedKVLayers > 0:
		note = "shared KV layers excluded from cache"
	}
	return kvBytes, recurrentBytes, note
}
