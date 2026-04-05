package scorer

// ValidProfiles returns all supported use-case profile names.
func ValidProfiles() []string {
	return []string{"General", "Coding", "Reasoning", "Chat", "Multimodal", "Embedding"}
}
