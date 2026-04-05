package hardware

import (
	"github.com/kevo-1/KnowURLLM/internal/models"
)

// HardwareDetector is an interface for hardware detection, enabling mocking in tests.
// The default implementation is RealDetector, which calls actual system commands.
type HardwareDetector interface {
	// Detect performs full hardware detection
	Detect() (models.HardwareProfile, error)
	// DetectCPU detects CPU model and core count
	DetectCPU() (string, int, error)
	// DetectMemory detects total and available RAM
	DetectMemory() (uint64, uint64, error)
	// DetectGPU detects GPUs
	DetectGPU() ([]models.GPUInfo, error)
}

// RealDetector is the default hardware detector that calls system commands
type RealDetector struct{}

// NewRealDetector creates a new RealDetector
func NewRealDetector() *RealDetector {
	return &RealDetector{}
}

// Detect performs full hardware detection using system commands
func (d *RealDetector) Detect() (models.HardwareProfile, error) {
	return Detect()
}

// DetectCPU detects CPU model and core count
func (d *RealDetector) DetectCPU() (string, int, error) {
	return detectCPU()
}

// DetectMemory detects total and available RAM
func (d *RealDetector) DetectMemory() (uint64, uint64, error) {
	return memory()
}

// DetectGPU detects GPUs
func (d *RealDetector) DetectGPU() ([]models.GPUInfo, error) {
	return gpu()
}

// MockDetector is a test double that returns predefined values
type MockDetector struct {
	Profile models.HardwareProfile
	Err     error
}

// NewMockDetector creates a MockDetector with the given profile
func NewMockDetector(profile models.HardwareProfile, err error) *MockDetector {
	return &MockDetector{
		Profile: profile,
		Err:     err,
	}
}

// Detect returns the mocked profile
func (d *MockDetector) Detect() (models.HardwareProfile, error) {
	return d.Profile, d.Err
}

// DetectCPU returns the mocked CPU info
func (d *MockDetector) DetectCPU() (string, int, error) {
	return d.Profile.CPUModel, d.Profile.CPUCores, d.Err
}

// DetectMemory returns the mocked memory info
func (d *MockDetector) DetectMemory() (uint64, uint64, error) {
	return d.Profile.TotalRAM, d.Profile.AvailableRAM, d.Err
}

// DetectGPU returns the mocked GPU info
func (d *MockDetector) DetectGPU() ([]models.GPUInfo, error) {
	return d.Profile.GPUs, d.Err
}
