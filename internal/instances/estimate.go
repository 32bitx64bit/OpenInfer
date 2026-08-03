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

// Estimate breaks down projected memory use for one configuration.
type Estimate struct {
	WeightsBytes      int64  `json:"weights_bytes"`
	DraftWeightsBytes int64  `json:"draft_weights_bytes"`
	ProjectorBytes    int64  `json:"projector_bytes"`
	KVCacheBytes      int64  `json:"kv_cache_bytes"`
	RecurrentBytes    int64  `json:"recurrent_bytes"` // hybrid SSM / linear-attn state
	ComputeBytes      int64  `json:"compute_bytes"`
	MediaBytes        int64  `json:"media_bytes"` // multimodal encoder activations
	OverheadBytes     int64  `json:"overhead_bytes"`
	TotalBytes        int64  `json:"total_bytes"`
	GPUBytes          int64  `json:"gpu_bytes"`
	CPUBytes          int64  `json:"cpu_bytes"`
	BudgetBytes       uint64 `json:"budget_bytes"`
	BudgetKind        string `json:"budget_kind"` // "VRAM", "unified RAM", "RAM"
	Fits              bool   `json:"fits"`
	Note              string `json:"note"`
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

	switch {
	case in.GPUOffload && in.VRAM > 0:
		est.BudgetBytes = in.VRAM
		est.BudgetKind = "VRAM"
	case in.GPUOffload && in.VRAM == 0:
		est.BudgetBytes = in.RAM
		est.BudgetKind = "unified RAM"
		if est.Note == "" {
			est.Note = "GPU shares system memory on this machine"
		} else if !strings.Contains(est.Note, "GPU shares") {
			est.Note += "; GPU shares system memory"
		}
	default:
		est.BudgetBytes = in.RAM
		est.BudgetKind = "RAM"
	}

	// Where does the projector / media live?
	projOnGPU := in.GPUOffload && projector > 0 && !in.NoMmprojOffload
	mediaOnGPU := projOnGPU

	// Partial offload: split weights + KV by layer share. Draft weights follow
	// the same GPU/CPU split as the target (llama.cpp loads both in-process).
	if in.GPUOffload && in.Layers > 0 && in.OffloadedLayers > 0 && in.OffloadedLayers < in.Layers {
		frac := float64(in.OffloadedLayers) / float64(in.Layers)
		gpuW := int64(frac * float64(est.WeightsBytes+est.DraftWeightsBytes))
		gpuKV := int64(frac * float64(est.KVCacheBytes+est.RecurrentBytes))
		est.GPUBytes = gpuW + gpuKV + est.ComputeBytes + est.OverheadBytes
		est.CPUBytes = (est.WeightsBytes + est.DraftWeightsBytes - gpuW) + (est.KVCacheBytes + est.RecurrentBytes - gpuKV)
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
		gpuFits := uint64(est.GPUBytes) <= est.BudgetBytes-est.BudgetBytes/10
		cpuFits := uint64(est.CPUBytes) <= in.RAM-in.RAM/10
		est.Fits = gpuFits && cpuFits
		return est
	}

	if in.GPUOffload {
		est.GPUBytes = est.TotalBytes
		if in.NoMmprojOffload && (est.ProjectorBytes > 0 || est.MediaBytes > 0) {
			est.GPUBytes -= est.ProjectorBytes + est.MediaBytes
			est.CPUBytes = est.ProjectorBytes + est.MediaBytes
		}
	} else {
		est.CPUBytes = est.TotalBytes
	}
	est.Fits = uint64(est.TotalBytes) <= est.BudgetBytes-est.BudgetBytes/10
	return est
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
