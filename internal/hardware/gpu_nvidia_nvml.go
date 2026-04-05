//go:build linux

package hardware

import (
	"fmt"

	"github.com/kevo-1/KnowURLLM/internal/models"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// detectNVMLFallback is the real NVML implementation for Linux.
// This overrides the stub in gpu_nvidia.go when building on Linux.
func detectNVMLFallback() ([]models.GPUInfo, error) {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("nvml init failed: %v", ret)
	}
	defer nvml.Shutdown()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("nvml DeviceGetCount failed: %v", ret)
	}

	var gpus []models.GPUInfo
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		name, ret := device.GetName()
		if ret != nvml.SUCCESS {
			continue
		}

		memory, ret := device.GetMemoryInfo()
		if ret != nvml.SUCCESS {
			continue
		}

		gpus = append(gpus, models.GPUInfo{
			Vendor: normalizeGPUVendor(name),
			Model:  name,
			VRAM:   memory.Total,
		})
	}

	if len(gpus) == 0 {
		return nil, fmt.Errorf("nvml found no GPUs")
	}

	return gpus, nil
}
