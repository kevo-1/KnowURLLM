package hardware

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	gpucpu "github.com/shirou/gopsutil/v4/cpu"
)

// detectCPU detects the CPU model string and logical core count.
// Returns (model, cores, error).
func detectCPU() (string, int, error) {
	// Try gopsutil first
	infos, err := gpucpu.Info()
	if err == nil && len(infos) > 0 {
		model := strings.TrimSpace(infos[0].ModelName)
		if model != "" {
			return model, runtime.NumCPU(), nil
		}
	}

	// Fallback: platform-specific detection
	switch runtime.GOOS {
	case "linux":
		return cpuFromProcCpuinfo()
	case "darwin":
		return cpuFromSysctl()
	case "windows":
		return cpuFromWMIC()
	default:
		return fmt.Sprintf("unknown (%s)", runtime.GOOS), runtime.NumCPU(), nil
	}
}

// cpuFromProcCpuinfo reads /proc/cpuinfo on Linux.
func cpuFromProcCpuinfo() (string, int, error) {
	out, err := exec.Command("grep", "-m", "1", "model name", "/proc/cpuinfo").Output()
	if err != nil {
		return "", runtime.NumCPU(), fmt.Errorf("reading /proc/cpuinfo: %w", err)
	}
	// Format: "model name\t: Intel(R) Core(TM) i7-9700K CPU @ 3.60GHz"
	parts := strings.SplitN(string(out), ":", 2)
	if len(parts) != 2 {
		return "", runtime.NumCPU(), fmt.Errorf("unexpected /proc/cpuinfo format")
	}
	model := strings.TrimSpace(parts[1])
	return model, runtime.NumCPU(), nil
}

// cpuFromSysctl uses sysctl on macOS.
func cpuFromSysctl() (string, int, error) {
	out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
	if err != nil {
		return "", runtime.NumCPU(), fmt.Errorf("sysctl machdep.cpu.brand_string: %w", err)
	}
	model := strings.TrimSpace(string(out))
	return model, runtime.NumCPU(), nil
}

// cpuFromWMIC uses wmic on Windows.
func cpuFromWMIC() (string, int, error) {
	// wmic cpu get name /format:value
	out, err := exec.Command("wmic", "cpu", "get", "name", "/format:value").Output()
	if err != nil {
		return "", runtime.NumCPU(), fmt.Errorf("wmic cpu get name: %w", err)
	}
	// Format: "Name=Intel64 Family 6 Model 158 Stepping 10..."
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Name=") {
			model := strings.TrimPrefix(line, "Name=")
			return strings.TrimSpace(model), runtime.NumCPU(), nil
		}
	}
	return "", runtime.NumCPU(), fmt.Errorf("could not parse wmic output")
}
