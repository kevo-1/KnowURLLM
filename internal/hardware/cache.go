package hardware

import (
	"sync"

	"github.com/kevo-1/KnowURLLM/internal/models"
)

var (
	// detectOnce ensures Detect() is only called once per program execution
	detectOnce sync.Once
	// cachedProfile stores the result of the first Detect() call
	cachedProfile models.HardwareProfile
	// cachedErr stores the error from the first Detect() call
	cachedErr error
)

// DetectCached returns the hardware profile, caching the result after the first call.
// This is more efficient than Detect() when called multiple times, as hardware
// doesn't change during runtime.
//
// The first call performs full hardware detection and caches the result.
// Subsequent calls return the cached profile immediately.
func DetectCached() (models.HardwareProfile, error) {
	detectOnce.Do(func() {
		cachedProfile, cachedErr = Detect()
	})
	return cachedProfile, cachedErr
}

// ResetCache clears the cached hardware profile, forcing the next DetectCached()
// call to perform fresh hardware detection.
// This is primarily useful for testing or if hardware somehow changes at runtime.
func ResetCache() {
	detectOnce = sync.Once{}
	cachedProfile = models.HardwareProfile{}
	cachedErr = nil
}
