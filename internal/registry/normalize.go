// Package registry normalizes raw API/JSON response types into ModelEntry.
package registry

import (
	"math"
	"regexp"
	"strings"
)

// parseParameterSize converts a parameter size string like "8B" to bytes.
// E.g., "8B" -> 8_000_000_000, "70B" -> 70_000_000_000, "0.5B" -> 500_000_000
func parseParameterSize(s string) uint64 {
	if s == "" {
		return 0
	}

	s = strings.ToUpper(strings.TrimSpace(s))

	// Look for 'B' suffix
	if idx := strings.Index(s, "B"); idx != -1 && idx > 0 {
		numStr := s[:idx]
		var multiplier float64 = 1_000_000_000

		num, err := parseFloat(numStr)
		if err != nil {
			return 0
		}

		return uint64(num * multiplier)
	}

	// Try parsing as raw number (bytes)
	num, err := parseFloat(s)
	if err != nil {
		return 0
	}

	return uint64(num)
}

// parseFloat is a simple float64 parser.
func parseFloat(s string) (float64, error) {
	var result float64
	var frac float64
	var div float64 = 1
	var isFrac bool

	for _, c := range s {
		if c == '.' {
			isFrac = true
			frac = 0
			div = 1
			continue
		}
		if c >= '0' && c <= '9' {
			digit := float64(c - '0')
			if isFrac {
				div *= 10
				frac += digit / div
			} else {
				result = result*10 + digit
			}
		} else {
			return 0, nil
		}
	}

	return result + frac, nil
}

// parseQuantizationFromFilename extracts quantization label from GGUF filenames.
// E.g., "model-Q4_K_M.gguf" -> "Q4_K_M", "llama-3.1-8b-q8_0.gguf" -> "Q8_0"
func parseQuantizationFromFilename(filename string) string {
	filename = strings.ToUpper(filename)

	patterns := []string{
		"Q2_K",
		"Q3_K_S", "Q3_K_M", "Q3_K_L", "Q3_K",
		"Q4_0", "Q4_1", "Q4_K_S", "Q4_K_M", "Q4_K",
		"Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M", "Q5_K",
		"Q6_K",
		"Q8_0",
		"FP16", "F16",
		"FP32", "F32",
	}

	for _, pattern := range patterns {
		if strings.Contains(filename, pattern) {
			switch pattern {
			case "Q4_K_S", "Q4_K":
				return "Q4_K_M"
			case "Q5_K_S", "Q5_K":
				return "Q5_K_M"
			case "Q3_K_S", "Q3_K_M", "Q3_K_L", "Q3_K":
				return "Q3_K_M"
			case "F16":
				return "FP16"
			case "F32":
				return "FP32"
			default:
				return pattern
			}
		}
	}

	return "unknown"
}

// bytesPerParam returns the approximate bytes per parameter for a given quantization level.
func bytesPerParam(quant string) float64 {
	switch strings.ToUpper(quant) {
	case "Q2_K":
		return 0.313
	case "Q3_K", "Q3_K_M":
		return 0.438
	case "Q4_K_M", "Q4_K", "Q4_K_S":
		return 0.563
	case "Q5_K_M", "Q5_K", "Q5_K_S":
		return 0.688
	case "Q8_0":
		return 1.0
	case "FP16", "F16":
		return 2.0
	case "FP32", "F32":
		return 4.0
	default:
		return 0.563
	}
}

// normalizeModelName normalizes a model name for deduplication.
// E.g., "meta-llama/Llama-3.1-8B-Instruct" -> "llama318"
func normalizeModelName(name string) string {
	name = strings.ToLower(name)

	// Remove path prefix
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	// Remove quantization suffix
	qIdx := regexp.MustCompile(`[-_]q\d`).FindStringIndex(name)
	if qIdx != nil {
		name = name[:qIdx[0]]
	}

	// Remove common separators
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, ":", "")

	// Extract model family
	familyPattern := regexp.MustCompile(`(llama|mistral|qwen|phi|gemma|falcon|deepseek)`)
	familyMatch := familyPattern.FindString(name)
	if familyMatch == "" {
		familyMatch = name
	}

	// Extract numbers
	numberPattern := regexp.MustCompile(`\d+\.?\d*`)
	numberMatches := numberPattern.FindAllString(name, -1)

	result := familyMatch
	for i, num := range numberMatches {
		if i >= 2 {
			break
		}
		num = strings.ReplaceAll(num, ".", "")
		result += num
	}

	return result
}

// roundTo rounds a float to the nearest integer.
func roundTo(f float64) float64 {
	return math.Round(f*100) / 100
}
