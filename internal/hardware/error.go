package hardware

import (
	"fmt"
)

// GPUDetectionError represents an error during GPU detection
// with information about whether GPUs were found or detection failed.
type GPUDetectionError struct {
	// Message describing the error
	Message string
	// GPUsFound is true if at least one GPU was successfully detected
	// (even if others failed). If false, detection completely failed.
	GPUsFound bool
	// Wrapped is the underlying error
	Wrapped error
}

func (e *GPUDetectionError) Error() string {
	if e.GPUsFound {
		return fmt.Sprintf("partial GPU detection error (some GPUs detected): %v", e.Wrapped)
	}
	return fmt.Sprintf("GPU detection failed (CPU-only mode): %v", e.Wrapped)
}

func (e *GPUDetectionError) Unwrap() error {
	return e.Wrapped
}

// NoGPUError creates a GPUDetectionError indicating no GPUs found
func NoGPUError(wrapped error) *GPUDetectionError {
	return &GPUDetectionError{
		Message:     "no GPUs detected",
		GPUsFound:   false,
		Wrapped:     wrapped,
	}
}

// PartialGPUError creates a GPUDetectionError indicating partial detection
func PartialGPUError(wrapped error) *GPUDetectionError {
	return &GPUDetectionError{
		Message:     "partial GPU detection",
		GPUsFound:   true,
		Wrapped:     wrapped,
	}
}

// IsNoGPUError checks if an error indicates complete GPU detection failure
func IsNoGPUError(err error) bool {
	if err == nil {
		return false
	}
	// Check if it's directly a GPUDetectionError
	if gpuErr, ok := err.(*GPUDetectionError); ok {
		return !gpuErr.GPUsFound
	}
	// Check wrapped errors
	unwrapper, ok := err.(interface{ Unwrap() error })
	if !ok {
		return false
	}
	for unwrapped := unwrapper.Unwrap(); unwrapped != nil; {
		if gpuErr, ok := unwrapped.(*GPUDetectionError); ok {
			return !gpuErr.GPUsFound
		}
		unwrapper, ok = unwrapped.(interface{ Unwrap() error })
		if !ok {
			break
		}
		unwrapped = unwrapper.Unwrap()
	}
	return false
}
