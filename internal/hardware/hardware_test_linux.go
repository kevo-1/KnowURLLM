//go:build linux

package hardware

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseAMDSysfsLinux tests the sysfs reader using temporary files.
// This file only compiles on Linux where readSysfsUint64 and readSysfsString are defined.
func TestParseAMDSysfsLinux(t *testing.T) {
	// Create temp directory structure mimicking sysfs
	tmpDir := t.TempDir()

	// Create card0 structure
	card0Dir := filepath.Join(tmpDir, "card0", "device")
	if err := os.MkdirAll(card0Dir, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Write mock VRAM info
	if err := os.WriteFile(filepath.Join(card0Dir, "mem_info_vram_total"), []byte("17179869184"), 0644); err != nil {
		t.Fatalf("failed to write vram file: %v", err)
	}

	// Write mock product name
	if err := os.WriteFile(filepath.Join(card0Dir, "product_name"), []byte("AMD Radeon RX 7900 XTX\n"), 0644); err != nil {
		t.Fatalf("failed to write product_name file: %v", err)
	}

	// Create card1 structure
	card1Dir := filepath.Join(tmpDir, "card1", "device")
	if err := os.MkdirAll(card1Dir, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(card1Dir, "mem_info_vram_total"), []byte("8589934592"), 0644); err != nil {
		t.Fatalf("failed to write vram file: %v", err)
	}

	// Test reading individual files
	t.Run("read sysfs uint64", func(t *testing.T) {
		vramPath := filepath.Join(card0Dir, "mem_info_vram_total")
		val, err := readSysfsUint64(vramPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 17179869184 {
			t.Errorf("expected 17179869184, got %d", val)
		}
	})

	t.Run("read sysfs string", func(t *testing.T) {
		namePath := filepath.Join(card0Dir, "product_name")
		val, err := readSysfsString(namePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "AMD Radeon RX 7900 XTX\n"
		if val != expected {
			t.Errorf("expected '%s', got '%s'", expected, val)
		}
	})
}
