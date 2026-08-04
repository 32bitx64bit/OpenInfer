package hardware

import (
	"strconv"
	"strings"
)

// parseWinVideoCSV parses ConvertTo-Csv / WMIC CSV output for
// Win32_VideoController rows (Name, DriverVersion, AdapterRAM).
func parseWinVideoCSV(out string) []GPU {
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		return nil
	}
	// Detect header to map columns (order differs between CIM and WMIC).
	header := strings.TrimSpace(lines[0])
	header = strings.TrimPrefix(header, "\ufeff")
	cols := splitCSVLine(header)
	idxName, idxDriver, idxRAM := -1, -1, -1
	for i, c := range cols {
		switch strings.ToLower(strings.Trim(c, `"`)) {
		case "name":
			idxName = i
		case "driverversion":
			idxDriver = i
		case "adapterram":
			idxRAM = i
		}
	}
	// WMIC csv often prefixes with Node; Name is last when unsorted.
	if idxName < 0 {
		return parseWinVideoCSVLoose(out)
	}

	var gpus []GPU
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := splitCSVLine(line)
		get := func(idx int) string {
			if idx < 0 || idx >= len(parts) {
				return ""
			}
			return strings.Trim(parts[idx], `"`)
		}
		name := strings.TrimSpace(get(idxName))
		if name == "" || strings.EqualFold(name, "Name") {
			continue
		}
		vendor := gpuVendorFromName(name)
		g := GPU{Name: name, Vendor: vendor, Driver: strings.TrimSpace(get(idxDriver))}
		if idxRAM >= 0 {
			if ram, err := strconv.ParseUint(strings.TrimSpace(get(idxRAM)), 10, 64); err == nil && ram > 0 {
				// AdapterRAM is bytes when present; some drivers report bogus
				// small values — keep only plausible discrete sizes (≥128 MiB).
				if ram >= 128<<20 {
					g.VRAM = ram
				}
			}
		}
		gpus = append(gpus, g)
	}
	return gpus
}

func parseWinVideoCSVLoose(out string) []GPU {
	var gpus []GPU
	for _, line := range strings.Split(out, "\n") {
		parts := splitCSVLine(strings.TrimSpace(line))
		if len(parts) < 2 {
			continue
		}
		name := strings.Trim(parts[len(parts)-1], `"`)
		name = strings.TrimSpace(name)
		if name == "" || strings.EqualFold(name, "Name") {
			continue
		}
		driver := ""
		if len(parts) >= 3 {
			driver = strings.Trim(parts[len(parts)-2], `"`)
			// Heuristic: driver versions look like digits/dots, not Node names.
			if strings.ContainsAny(driver, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz") &&
				!strings.ContainsAny(driver, "0123456789") {
				driver = ""
			}
		}
		gpus = append(gpus, GPU{
			Name: name, Vendor: gpuVendorFromName(name), Driver: strings.TrimSpace(driver),
		})
	}
	return gpus
}

func parseNvidiaSMI(out string) []GPU {
	var gpus []GPU
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		mib, _ := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
		gpus = append(gpus, GPU{
			Name:   strings.TrimSpace(parts[0]),
			Vendor: "nvidia",
			VRAM:   mib << 20,
			Driver: strings.TrimSpace(parts[2]),
		})
	}
	return gpus
}

func gpuVendorFromName(name string) string {
	ln := strings.ToLower(name)
	switch {
	case strings.Contains(ln, "nvidia"), strings.Contains(ln, "geforce"), strings.Contains(ln, "quadro"), strings.Contains(ln, "tesla"), strings.Contains(ln, "rtx "), strings.Contains(ln, "gtx "):
		return "nvidia"
	case strings.Contains(ln, "amd"), strings.Contains(ln, "radeon"), strings.Contains(ln, "rx "):
		return "amd"
	case strings.Contains(ln, "intel"), strings.Contains(ln, "arc "), strings.Contains(ln, "uhd"), strings.Contains(ln, "iris"):
		return "intel"
	default:
		return "unknown"
	}
}

func splitCSVLine(line string) []string {
	var parts []string
	var cur strings.Builder
	inQuotes := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
		case c == ',' && !inQuotes:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	parts = append(parts, cur.String())
	return parts
}
