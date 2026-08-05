package huggingface

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// FileKind classifies a repository file.
type FileKind string

const (
	KindGGUF      FileKind = "gguf"
	KindProjector FileKind = "projector"
	KindOther     FileKind = "other"
)

// GroupedFile is one downloadable file inside a group.
type GroupedFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Kind string `json:"kind"` // gguf|projector
	Part int    `json:"part,omitempty"`
}

// FileGroup is one logical download unit: a single GGUF, a complete split
// set, or a vision set. Sets are only offered whole — never per-shard.
type FileGroup struct {
	ID          string        `json:"id"`    // stable, derived from base name + quant
	Label       string        `json:"label"` // e.g. "Q4_K_M" or "IQ4_XS · MTP"
	Quant       string        `json:"quant"`
	Split       bool          `json:"split"`
	Parts       int           `json:"parts"`
	Vision      bool          `json:"vision"`        // includes an mmproj file
	MTP         string        `json:"mtp,omitempty"` // "" | "mtp" | "mtp-draft"
	TotalBytes  int64         `json:"total_bytes"`
	Files       []GroupedFile `json:"files"`
	EstMemBytes int64         `json:"est_memory_bytes"` // rough estimate, clearly marked
}

var (
	// name-Q4_K_M.gguf, name.IQ4_XS.gguf, name.q8_0.gguf
	quantRe = regexp.MustCompile(`(?i)[.\-_]((?:IQ[1-4]_[A-Z0-9]+|Q[1-8]_[A-Z0-9_]+(?:_[SMXL])?|F16|F32|BF16|TQ[12]_0|MXFP4))`)
	// name-00001-of-00003.gguf
	splitRe = regexp.MustCompile(`(?i)-(\d{5})-of-(\d{5})\.gguf$`)
	// mmproj-model-f16.gguf, model.mmproj-Q8_0.gguf
	projRe = regexp.MustCompile(`(?i)(mmproj|mm-proj|projector)`)
)

// classifyFile returns the kind for a repo path.
func classifyFile(path string) FileKind {
	lower := strings.ToLower(path)
	if !strings.HasSuffix(lower, ".gguf") {
		return KindOther
	}
	base := lower[strings.LastIndex(lower, "/")+1:]
	if projRe.MatchString(base) {
		return KindProjector
	}
	return KindGGUF
}

// quantOf extracts the quantization token from a filename.
func quantOf(path string) string {
	m := quantRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	return strings.ToUpper(m[1])
}

// GroupFiles organizes repository files into logical download units:
// split shards are merged into one set, and non-GGUF files are excluded.
// Non-split GGUFs are one group per file so mixed MTP / non-MTP quants in the
// same repo (same quant token, different filenames) stay distinct.
// Projector (mmproj) files are returned separately: the UI offers a single
// "include vision" toggle that appends them to whichever group is chosen,
// instead of duplicating every quant as a separate vision variant.
func GroupFiles(files []FileEntry) (groups []FileGroup, projectors []GroupedFile) {
	type key struct{ stem, quant, mtp string }
	regular := map[key][]GroupedFile{}
	splits := map[key][]GroupedFile{}
	splitDeclared := map[key]int{} // declared shard count from filename
	projByQuant := map[string][]GroupedFile{}

	for _, f := range files {
		kind := classifyFile(f.Path)
		if kind == KindOther {
			continue
		}
		q := quantOf(f.Path)
		mtp := FileMTP(f.Path)
		gf := GroupedFile{Path: f.Path, Size: f.Size, Kind: string(kind)}
		if kind == KindProjector {
			projByQuant[q] = append(projByQuant[q], gf)
			continue
		}
		if m := splitRe.FindStringSubmatch(f.Path); len(m) == 3 {
			part, _ := strconv.Atoi(m[1])
			declared, _ := strconv.Atoi(m[2])
			gf.Part = part
			k := key{stem: splitStem(f.Path), quant: q, mtp: mtp}
			splits[k] = append(splits[k], gf)
			if declared > splitDeclared[k] {
				splitDeclared[k] = declared
			}
			continue
		}
		// One group per non-split GGUF. Key includes the full relative path so
		// identical basenames in different folders stay separate, and so MTP /
		// non-MTP siblings that share a quant never merge.
		k := key{stem: strings.TrimSuffix(f.Path, ".gguf"), quant: q, mtp: mtp}
		regular[k] = append(regular[k], gf)
	}

	groups = []FileGroup{}
	seenID := map[string]int{}

	uniqID := func(stem, quant, mtp string) string {
		id := strings.ToLower(strings.NewReplacer("/", "-", " ", "-", ".", "-").Replace(stem))
		if quant != "" {
			id += "-" + strings.ToLower(quant)
		}
		if mtp != "" {
			id += "-" + mtp
		}
		if n := seenID[id]; n > 0 {
			seenID[id] = n + 1
			return id + "-" + strconv.Itoa(n+1)
		}
		seenID[id] = 1
		return id
	}

	for k, fs := range regular {
		g := FileGroup{
			ID:    uniqID(k.stem, k.quant, k.mtp),
			Label: quantLabel(k.quant, k.mtp, fs[0].Path),
			Quant: k.quant,
			MTP:   k.mtp,
			Files: sortedFiles(fs),
		}
		for _, f := range fs {
			g.TotalBytes += f.Size
		}
		groups = append(groups, g)
	}
	for k, fs := range splits {
		parts := splitDeclared[k]
		var total int64
		for _, f := range fs {
			if f.Part > parts {
				parts = f.Part
			}
			total += f.Size
		}
		g := FileGroup{
			ID:         uniqID(k.stem, k.quant, k.mtp),
			Label:      quantLabel(k.quant, k.mtp, fs[0].Path) + " split set",
			Quant:      k.quant,
			MTP:        k.mtp,
			Split:      true,
			Parts:      parts,
			Files:      sortedFiles(fs),
			TotalBytes: total,
		}
		groups = append(groups, g)
	}

	// Projectors (vision) are reported once for the whole repo; they pair
	// with any quantization at download time via the UI toggle.
	var allProjectors []GroupedFile
	for _, ps := range projByQuant {
		allProjectors = append(allProjectors, ps...)
	}
	if len(allProjectors) > 0 && len(groups) == 0 {
		// Projector-only repository corner case: offer the set directly and
		// suppress the separate return so it is not added twice.
		g := FileGroup{ID: uniqID("projectors", "", ""), Label: "Projector files", Vision: true, Files: sortedFiles(allProjectors)}
		for _, f := range allProjectors {
			g.TotalBytes += f.Size
		}
		groups = append(groups, g)
		allProjectors = nil
	}

	// Memory estimate: file size + ~25% overhead for context/KV at defaults.
	for i := range groups {
		groups[i].EstMemBytes = groups[i].TotalBytes + groups[i].TotalBytes/4
	}

	// Sort by quantization rank: smallest/heaviest-compressed quants first,
	// full-precision last. Within a quant, plain before MTP, then by size.
	sort.SliceStable(groups, func(a, b int) bool {
		ra, rb := quantRank(groups[a]), quantRank(groups[b])
		if ra != rb {
			return ra < rb
		}
		if groups[a].Quant == groups[b].Quant && groups[a].MTP != groups[b].MTP {
			return groups[a].MTP < groups[b].MTP
		}
		if groups[a].TotalBytes != groups[b].TotalBytes {
			return groups[a].TotalBytes < groups[b].TotalBytes
		}
		return groups[a].Label < groups[b].Label
	})
	return groups, sortedFiles(allProjectors)
}

// quantLabel builds the Discover download-row title for a quant (+ MTP).
func quantLabel(quant, mtp, path string) string {
	label := orDefault(quant, "GGUF")
	if hint := mtpVariantHint(path); hint != "" && mtp == "mtp" {
		label += " · " + hint
	}
	switch mtp {
	case "mtp":
		label += " · MTP"
	case "mtp-draft":
		label += " · MTP draft"
	}
	return label
}

// mtpVariantHint pulls a short build tag immediately before MTP in the
// filename (e.g. AMD / LOW in …-AMD-MTP-IQ4_XS), so same-quant MTP builds
// stay distinguishable in the UI.
func mtpVariantHint(path string) string {
	base := strings.ToUpper(stemOf(path))
	parts := strings.FieldsFunc(base, func(r rune) bool {
		return r == '-' || r == '_'
	})
	for i, p := range parts {
		if p != "MTP" || i == 0 {
			continue
		}
		prev := parts[i-1]
		switch prev {
		case "AMD", "LOW", "HIGH", "ULTRA", "FAST", "SPARSE", "DENSE":
			return prev
		}
	}
	return ""
}

// Known quantization sort order (smallest → largest).
var quantRanks = map[string]int{
	"IQ1_S": 1, "IQ1_M": 2,
	"IQ2_XXS": 3, "IQ2_XS": 4, "Q2_K_S": 5, "Q2_K": 6, "IQ2_S": 7, "IQ2_M": 8,
	"IQ3_XXS": 9, "IQ3_XS": 10, "Q3_K_S": 11, "IQ3_S": 12, "IQ3_M": 13,
	"Q3_K_M": 14, "Q3_K_L": 15,
	"IQ4_NL": 16, "IQ4_XS": 17, "Q4_0": 18, "Q4_K_S": 19, "Q4_K_M": 20, "Q4_1": 21,
	"Q5_0": 22, "Q5_K_S": 23, "Q5_K_M": 24, "Q5_1": 25,
	"Q6_K": 26, "Q8_0": 27,
	"TQ1_0": 28, "TQ2_0": 29, "MXFP4": 30,
	"F16": 40, "BF16": 41, "F32": 42,
}

// quantRank returns the sort rank of a group; unknown quants rank by a
// size-derived estimate between Q8_0 and F16.
func quantRank(g FileGroup) int {
	if r, ok := quantRanks[g.Quant]; ok {
		return r
	}
	return 35
}

func sortedFiles(fs []GroupedFile) []GroupedFile {
	out := append([]GroupedFile{}, fs...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Part != out[j].Part {
			return out[i].Part < out[j].Part
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// stemOf returns the filename without directory and .gguf extension.
func stemOf(path string) string {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".gguf")
}

// splitStem strips the -00001-of-00003 suffix.
func splitStem(path string) string {
	return splitRe.ReplaceAllString(path, "")
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
