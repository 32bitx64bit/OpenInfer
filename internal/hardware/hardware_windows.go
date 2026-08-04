//go:build windows

package hardware

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/cpu"
	"golang.org/x/sys/windows"
)

func detectPlatform(i *Info) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	detectOSVersionWin(ctx, i)
	detectCPUWin(ctx, i)
	detectMemWin(i)
	detectCPUFeaturesWin(i)
	detectGPUsWin(ctx, i)
	detectAccelWin(ctx, i)
}

func detectOSVersionWin(ctx context.Context, i *Info) {
	// Prefer CIM Caption + DisplayVersion; fall back to RtlGetVersion build.
	if out, err := powershell(ctx,
		`(Get-CimInstance Win32_OperatingSystem).Caption`); err == nil {
		cap := strings.TrimSpace(out)
		disp := ""
		if d, err := powershell(ctx,
			`(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion').DisplayVersion`); err == nil {
			disp = strings.TrimSpace(d)
		}
		if cap != "" {
			if disp != "" {
				i.OSVersion = cap + " " + disp
			} else {
				i.OSVersion = cap
			}
			i.Probes = append(i.Probes, Probe{Name: "os-version", OK: true})
			return
		}
	}
	if ver := rtlGetVersion(); ver != "" {
		i.OSVersion = ver
		i.Probes = append(i.Probes, Probe{Name: "os-version", OK: true, Detail: "RtlGetVersion"})
		return
	}
	i.Probes = append(i.Probes, Probe{Name: "os-version", OK: false})
}

func rtlGetVersion() string {
	ntdll := windows.NewLazySystemDLL("ntdll.dll")
	proc := ntdll.NewProc("RtlGetVersion")
	var vi struct {
		OSVersionInfoSize uint32
		MajorVersion      uint32
		MinorVersion      uint32
		BuildNumber       uint32
		PlatformId        uint32
		CSDVersion        [128]uint16
	}
	vi.OSVersionInfoSize = uint32(unsafe.Sizeof(vi))
	if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&vi))); r != 0 {
		return ""
	}
	return fmt.Sprintf("Windows %d.%d.%d", vi.MajorVersion, vi.MinorVersion, vi.BuildNumber)
}

func detectCPUWin(ctx context.Context, i *Info) {
	if out, err := powershell(ctx, `(Get-CimInstance Win32_Processor | Select-Object -First 1).Name`); err == nil {
		i.CPUModel = strings.TrimSpace(out)
	}
	if i.CPUModel == "" {
		// WMIC is deprecated/removed on some Windows 11 builds — keep as fallback.
		if out, err := exec.CommandContext(ctx, "wmic", "cpu", "get", "name", "/value").Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if v, ok := strings.CutPrefix(strings.TrimSpace(line), "Name="); ok && v != "" {
					i.CPUModel = v
					break
				}
			}
		}
	}
	if i.CPUModel == "" {
		i.CPUModel = os.Getenv("PROCESSOR_IDENTIFIER")
	}

	if out, err := powershell(ctx,
		`(Get-CimInstance Win32_Processor | Measure-Object -Property NumberOfCores -Sum).Sum`); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(out)); err == nil && n > 0 {
			i.PhysicalCores = n
		}
	}
	if i.PhysicalCores == 0 {
		i.PhysicalCores = i.LogicalCores
	}
	i.Probes = append(i.Probes, Probe{Name: "cpu", OK: i.CPUModel != ""})
}

func detectMemWin(i *Info) {
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
}

func detectCPUFeaturesWin(i *Info) {
	// Mirror the Linux feature filter using the portable CPUID package.
	add := func(name string, ok bool) {
		if ok {
			i.CPUFeatures = append(i.CPUFeatures, name)
		}
	}
	add("avx", cpu.X86.HasAVX)
	add("avx2", cpu.X86.HasAVX2)
	add("avx512f", cpu.X86.HasAVX512F)
	add("fma", cpu.X86.HasFMA)
	add("sse4_2", cpu.X86.HasSSE42)
	i.Probes = append(i.Probes, Probe{Name: "cpu-features", OK: len(i.CPUFeatures) > 0})
}

func detectGPUsWin(ctx context.Context, i *Info) {
	// CIM first (WMIC is gone on many Win11 systems).
	gpus := []GPU{}
	if out, err := powershell(ctx,
		`Get-CimInstance Win32_VideoController | Select-Object Name,DriverVersion,AdapterRAM | ConvertTo-Csv -NoTypeInformation`); err == nil {
		gpus = parseWinVideoCSV(out)
	}
	if len(gpus) == 0 {
		if out, err := exec.CommandContext(ctx, "wmic", "path", "win32_VideoController",
			"get", "name,driverversion,adapterram", "/format:csv").Output(); err == nil {
			gpus = parseWinVideoCSV(string(out))
		}
	}

	// Enrich / replace NVIDIA entries via nvidia-smi (names, VRAM, driver).
	if out, err := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=name,memory.total,driver_version", "--format=csv,noheader,nounits").Output(); err == nil {
		nv := parseNvidiaSMI(string(out))
		if len(nv) > 0 {
			// Keep non-NVIDIA adapters (iGPU) alongside discrete NVIDIA cards.
			merged := nv
			for _, g := range gpus {
				if g.Vendor != "nvidia" {
					merged = append(merged, g)
				}
			}
			gpus = merged
			i.CUDA = true
			i.Probes = append(i.Probes, Probe{Name: "nvidia-smi", OK: true})
		}
	} else {
		hasNV := false
		for _, g := range gpus {
			if g.Vendor == "nvidia" {
				hasNV = true
				break
			}
		}
		if hasNV {
			i.Probes = append(i.Probes, Probe{Name: "nvidia-smi", OK: false,
				Detail: "NVIDIA device present but nvidia-smi failed; driver may be missing"})
		}
	}

	i.GPUs = gpus
	i.Probes = append(i.Probes, Probe{Name: "gpu", OK: len(i.GPUs) > 0})
}

func detectAccelWin(ctx context.Context, i *Info) {
	// Vulkan loader.
	roots := []string{
		filepath.Join(os.Getenv("SystemRoot"), "System32", "vulkan-1.dll"),
		filepath.Join(os.Getenv("SystemRoot"), "SysWOW64", "vulkan-1.dll"),
	}
	for _, p := range roots {
		if _, err := os.Stat(p); err == nil {
			i.Vulkan = true
			break
		}
	}
	if !i.Vulkan {
		if _, err := exec.LookPath("vulkaninfo"); err == nil {
			i.Vulkan = true
		}
	}
	i.Probes = append(i.Probes, Probe{Name: "vulkan", OK: i.Vulkan})

	// CUDA: nvidia-smi already sets this; also look for toolkit installs.
	if !i.CUDA {
		if _, err := exec.LookPath("nvidia-smi"); err == nil {
			i.CUDA = true
		}
	}
	if !i.CUDA {
		for _, p := range []string{
			`C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA`,
			os.Getenv("CUDA_PATH"),
		} {
			if p == "" {
				continue
			}
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				i.CUDA = true
				break
			}
		}
	}
	i.Probes = append(i.Probes, Probe{Name: "cuda", OK: i.CUDA})

	// HIP / ROCm on Windows (AMD).
	if os.Getenv("HIP_PATH") != "" || os.Getenv("ROCM_PATH") != "" {
		i.HIP = true
	}
	if !i.HIP {
		for _, p := range []string{
			`C:\Program Files\AMD\ROCm`,
			`C:\Program Files\AMD\HIP`,
		} {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				i.HIP = true
				break
			}
		}
	}
	i.Probes = append(i.Probes, Probe{Name: "hip", OK: i.HIP})

	// Intel oneAPI / SYCL.
	for _, p := range []string{
		os.Getenv("ONEAPI_ROOT"),
		`C:\Program Files (x86)\Intel\oneAPI`,
		`C:\Program Files\Intel\oneAPI`,
	} {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			i.SYCL = true
			break
		}
	}
	i.Probes = append(i.Probes, Probe{Name: "sycl", OK: i.SYCL})
	i.Probes = append(i.Probes, Probe{Name: "metal", OK: false, Detail: "not available on Windows"})
}

func powershell(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
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
