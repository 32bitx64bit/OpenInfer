//go:build windows

package hardware

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func detectPlatform(i *Info) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if out, err := exec.CommandContext(ctx, "wmic", "cpu", "get", "name", "/value").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "Name="); ok {
				i.CPUModel = v
				break
			}
		}
	}
	if i.CPUModel == "" {
		i.CPUModel = os.Getenv("PROCESSOR_IDENTIFIER")
	}
	i.PhysicalCores = i.LogicalCores // refining requires WMI; logical is a safe default
	i.Probes = append(i.Probes, Probe{Name: "cpu", OK: i.CPUModel != ""})

	// GlobalMemoryStatusEx for total/available RAM.
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	gms := kernel32.NewProc("GlobalMemoryStatusEx")
	var mem struct {
		Length, MemoryLoad                                                         uint32
		TotalPhys, AvailPhys, TotalPage, AvailPage, TotalVirt, AvailVirt, AvailExt uint64
	}
	mem.Length = uint32(unsafe.Sizeof(mem))
	if r, _, _ := gms.Call(uintptr(unsafe.Pointer(&mem))); r != 0 {
		i.RAMTotal = mem.TotalPhys
		i.RAMAvailable = mem.AvailPhys
		i.Probes = append(i.Probes, Probe{Name: "memory", OK: true})
	} else {
		i.Probes = append(i.Probes, Probe{Name: "memory", OK: false})
	}

	if out, err := exec.CommandContext(ctx, "wmic", "path", "win32_VideoController",
		"get", "name,driverversion", "/format:csv").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			parts := strings.Split(line, ",")
			if len(parts) < 2 {
				continue
			}
			name := strings.TrimSpace(parts[len(parts)-1])
			if name == "" || strings.EqualFold(name, "Name") {
				continue
			}
			vendor := "unknown"
			ln := strings.ToLower(name)
			switch {
			case strings.Contains(ln, "nvidia"):
				vendor = "nvidia"
			case strings.Contains(ln, "amd"), strings.Contains(ln, "radeon"):
				vendor = "amd"
			case strings.Contains(ln, "intel"):
				vendor = "intel"
			}
			i.GPUs = append(i.GPUs, GPU{Name: name, Vendor: vendor})
		}
		i.Probes = append(i.Probes, Probe{Name: "gpu", OK: len(i.GPUs) > 0})
	} else {
		i.Probes = append(i.Probes, Probe{Name: "gpu", OK: false, Detail: err.Error()})
	}

	if _, err := os.Stat(filepath.Join(os.Getenv("SystemRoot"), "System32", "vulkan-1.dll")); err == nil {
		i.Vulkan = true
	}
	i.Probes = append(i.Probes, Probe{Name: "vulkan", OK: i.Vulkan})
	if out, err := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=driver_version",
		"--format=csv,noheader").Output(); err == nil {
		i.CUDA = true
		for idx := range i.GPUs {
			if i.GPUs[idx].Vendor == "nvidia" && i.GPUs[idx].Driver == "" {
				i.GPUs[idx].Driver = strings.TrimSpace(string(out))
			}
		}
	}
	i.Probes = append(i.Probes, Probe{Name: "cuda", OK: i.CUDA})
	if os.Getenv("HIP_PATH") != "" {
		i.HIP = true
	}
	i.Probes = append(i.Probes, Probe{Name: "hip", OK: i.HIP})
}

func diskFree(dir string) uint64 {
	root := filepath.VolumeName(dir) + `\`
	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0
	}
	var avail, total, totalfree uint64
	if err := windows.GetDiskFreeSpaceEx(rootPtr, &avail, &total, &totalfree); err == nil {
		return avail
	}
	return 0
}
