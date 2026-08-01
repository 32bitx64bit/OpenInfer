package instances

import (
	"strings"
	"testing"
)

func TestLogSectionForLaunch(t *testing.T) {
	log := "=== openinfer instance old-id starting 2026-07-31T19:51:15Z ===\n" +
		"error while handling argument \"--flash-attn\": unknown value\n" +
		"=== openinfer instance new-id starting 2026-08-01T07:08:55Z ===\n" +
		"model loaded\nfailed to find a memory slot for batch of size 66\n"

	section := LogSectionForLaunch(log, "new-id")
	if strings.Contains(section, "flash-attn") {
		t.Error("section must not contain earlier launches' errors")
	}
	if !strings.Contains(section, "memory slot") {
		t.Error("section must contain the current launch's log")
	}

	// Unknown instance ID: return the tail unchanged.
	if got := LogSectionForLaunch(log, "missing"); got != log {
		t.Error("unknown ID should return the tail unchanged")
	}
}
