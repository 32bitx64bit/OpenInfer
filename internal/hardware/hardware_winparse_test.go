package hardware

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseWinVideoCSV_CIM(t *testing.T) {
	in := "\"Name\",\"DriverVersion\",\"AdapterRAM\"\n" +
		"\"NVIDIA GeForce RTX 4090\",\"32.0.15.6094\",\"25769803776\"\n" +
		"\"Intel(R) UHD Graphics\",\"31.0.101.5333\",\"1073741824\"\n"
	got := parseWinVideoCSV(in)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(got), got)
	}
	if got[0].Vendor != "nvidia" || got[0].VRAM != 25769803776 {
		t.Fatalf("nvidia row: %+v", got[0])
	}
	if got[0].Driver != "32.0.15.6094" {
		t.Fatalf("driver: %q", got[0].Driver)
	}
	if got[1].Vendor != "intel" {
		t.Fatalf("intel row: %+v", got[1])
	}
}

func TestParseWinVideoCSV_WMIC(t *testing.T) {
	in := "Node,AdapterRAM,DriverVersion,Name\n" +
		"DESKTOP-ABC,4293918720,31.0.15.4617,NVIDIA GeForce RTX 3080\n"
	got := parseWinVideoCSV(in)
	if len(got) != 1 || got[0].Vendor != "nvidia" || !strings.Contains(got[0].Name, "3080") {
		t.Fatalf("got %+v", got)
	}
	if got[0].VRAM != 4293918720 {
		t.Fatalf("vram %+v", got[0])
	}
}

func TestParseNvidiaSMI(t *testing.T) {
	in := "NVIDIA GeForce RTX 4090, 24564, 560.94\nNVIDIA GeForce RTX 4090, 24564, 560.94\n"
	got := parseNvidiaSMI(in)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].VRAM != 24564<<20 || got[0].Driver != "560.94" {
		t.Fatalf("%+v", got[0])
	}
}

func TestGPUVendorFromName(t *testing.T) {
	cases := map[string]string{
		"NVIDIA GeForce RTX 4070": "nvidia",
		"AMD Radeon RX 7900 XTX":  "amd",
		"Intel(R) Arc(TM) A770":   "intel",
		"Microsoft Basic Display": "unknown",
	}
	for name, want := range cases {
		if got := gpuVendorFromName(name); got != want {
			t.Errorf("%q: got %s want %s", name, got, want)
		}
	}
}

func TestSplitCSVLine(t *testing.T) {
	got := splitCSVLine(`"a,b",c,"d"`)
	want := []string{"a,b", "c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v != %v", got, want)
	}
}
