//go:build darwin

package hardware

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func detectPlatform(i *Info) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if out, err := exec.CommandContext(ctx, "sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
		i.CPUModel = strings.TrimSpace(string(out))
		i.Probes = append(i.Probes, Probe{Name: "cpu-brand", OK: true})
	} else {
		i.Probes = append(i.Probes, Probe{Name: "cpu-brand", OK: false, Detail: err.Error()})
	}
	if out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.physicalcpu").Output(); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &i.PhysicalCores)
	}
	if i.PhysicalCores == 0 {
		i.PhysicalCores = i.LogicalCores
	}
	var mem uint64
	if out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output(); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &mem)
	}
	i.RAMTotal = mem
	i.RAMAvailable = mem // approximation without vm_stat parsing

	if out, err := exec.CommandContext(ctx, "sw_vers", "-productVersion").Output(); err == nil {
		i.OSVersion = "macOS " + strings.TrimSpace(string(out))
		i.Probes = append(i.Probes, Probe{Name: "sw_vers", OK: true})
	}

	if runtime.GOARCH == "arm64" {
		i.Metal = true
		i.GPUs = append(i.GPUs, GPU{Name: "Apple Silicon GPU", Vendor: "apple"})
		i.Probes = append(i.Probes, Probe{Name: "metal", OK: true})
	} else {
		// Intel Macs: Metal exists but llama.cpp Metal build targets arm64;
		// Vulkan via MoltenVK is not shipped in official builds — CPU is safe.
		i.Probes = append(i.Probes, Probe{Name: "metal", OK: false, Detail: "Intel Mac; CPU backend recommended"})
	}
	if out, err := exec.CommandContext(ctx, "sysctl", "-n", "machdep.cpu.features").Output(); err == nil {
		for _, f := range strings.Fields(strings.ToLower(string(out))) {
			switch f {
			case "avx", "avx2", "fma", "f16c", "sse4.2":
				i.CPUFeatures = append(i.CPUFeatures, f)
			}
		}
	}
}

func diskFree(dir string) uint64 {
	var st syscall.Statfs_t
	for dir != "" && dir != "/" {
		if err := syscall.Statfs(dir, &st); err == nil {
			return st.Bavail * uint64(st.Bsize)
		}
		if idx := strings.LastIndex(dir, "/"); idx > 0 {
			dir = dir[:idx]
		} else {
			break
		}
	}
	return 0
}
