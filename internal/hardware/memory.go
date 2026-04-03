package hardware

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/mem"
)

// memory detects total system RAM in bytes.
func memory() (uint64, error) {
	// Try gopsutil first
	v, err := mem.VirtualMemory()
	if err == nil && v.Total > 0 {
		return v.Total, nil
	}

	// Fallback: platform-specific detection
	switch runtime.GOOS {
	case "linux":
		return memoryFromProcMeminfo()
	case "darwin":
		return memoryFromSysctl()
	case "windows":
		return memoryFromWMIC()
	default:
		return 0, fmt.Errorf("memory detection not supported on %s", runtime.GOOS)
	}
}

// memoryFromProcMeminfo reads /proc/meminfo on Linux.
func memoryFromProcMeminfo() (uint64, error) {
	out, err := exec.Command("grep", "MemTotal", "/proc/meminfo").Output()
	if err != nil {
		return 0, fmt.Errorf("reading /proc/meminfo: %w", err)
	}
	// Format: "MemTotal:       16384000 kB"
	parts := strings.Fields(string(out))
	if len(parts) < 2 {
		return 0, fmt.Errorf("unexpected /proc/meminfo format")
	}
	kb, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing MemTotal: %w", err)
	}
	return kb * 1024, nil // Convert kB to bytes
}

// memoryFromSysctl uses sysctl on macOS.
func memoryFromSysctl() (uint64, error) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	bytes, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing hw.memsize: %w", err)
	}
	return bytes, nil
}

// memoryFromWMIC uses wmic on Windows.
func memoryFromWMIC() (uint64, error) {
	// wmic ComputerSystem get TotalPhysicalMemory /format:value
	out, err := exec.Command("wmic", "ComputerSystem", "get", "TotalPhysicalMemory", "/format:value").Output()
	if err != nil {
		return 0, fmt.Errorf("wmic ComputerSystem: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TotalPhysicalMemory=") {
			val := strings.TrimPrefix(line, "TotalPhysicalMemory=")
			bytes, err := strconv.ParseUint(strings.TrimSpace(val), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("parsing TotalPhysicalMemory: %w", err)
			}
			return bytes, nil
		}
	}
	return 0, fmt.Errorf("could not parse wmic output")
}
