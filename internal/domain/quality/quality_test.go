package quality

import (
	"testing"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

func TestNormalizeELO(t *testing.T) {
	tests := []struct {
		name     string
		elo      float64
		expected float64
	}{
		{"max_elo", 1350, 100},
		{"min_elo", 800, 0},
		{"mid_elo", 1075, 50},
		{"typical_gpt4", 1215, 75.45},
		{"zero_elo", 0, 0},
		{"negative_elo", -100, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeELO(tt.elo)
			if result < tt.expected-0.1 || result > tt.expected+0.1 {
				t.Errorf("normalizeELO(%v) = %v, want ~%v", tt.elo, result, tt.expected)
			}
		})
	}
}

func TestCalculateArenaQuality(t *testing.T) {
	entry := domain.ModelEntry{
		ArenaELO: 1215,
	}

	score, confidence := CalculateArenaQuality(entry)
	
	if score < 75 || score > 76 {
		t.Errorf("Expected score ~75.45, got %v", score)
	}
	
	if confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %v", confidence)
	}
}

func TestCalculateArenaQuality_Missing(t *testing.T) {
	entry := domain.ModelEntry{
		ArenaELO: 0,
	}

	score, confidence := CalculateArenaQuality(entry)
	
	if score != 0 {
		t.Errorf("Expected score 0 for missing ELO, got %v", score)
	}
	
	if confidence != 0 {
		t.Errorf("Expected confidence 0 for missing ELO, got %v", confidence)
	}
}

func TestGetBenchmarkSignals(t *testing.T) {
	signals := GetBenchmarkSignals(
		1215,  // Arena ELO
		85.3,  // MMLU
		89.1,  // IFEval
		92.4,  // GSM8K
		96.2,  // ARC
	)

	if len(signals) != 5 {
		t.Errorf("Expected 5 signals, got %d", len(signals))
	}

	// Check Arena ELO signal
	arenaSignal := signals[0]
	if arenaSignal.Name != "arena_elo" {
		t.Errorf("First signal should be arena_elo, got %s", arenaSignal.Name)
	}
	if arenaSignal.Weight != WeightArenaELO {
		t.Errorf("Arena weight should be %v, got %v", WeightArenaELO, arenaSignal.Weight)
	}
	if arenaSignal.Value < 75 || arenaSignal.Value > 76 {
		t.Errorf("Arena value should be ~75.45, got %v", arenaSignal.Value)
	}
}

func TestGetBenchmarkSignals_Partial(t *testing.T) {
	signals := GetBenchmarkSignals(
		1215,  // Arena ELO
		85.3,  // MMLU
		0,     // IFEval missing
		0,     // GSM8K missing
		0,     // ARC missing
	)

	if len(signals) != 2 {
		t.Errorf("Expected 2 signals (arena + mmlu), got %d", len(signals))
	}
}

func TestGetBenchmarkSignals_None(t *testing.T) {
	signals := GetBenchmarkSignals(0, 0, 0, 0, 0)

	if len(signals) != 0 {
		t.Errorf("Expected 0 signals when all missing, got %d", len(signals))
	}
}

func TestBayesianFusion_NoSignals(t *testing.T) {
	score, confidence := BayesianFusion([]BenchmarkSignal{})

	if score != 50.0 {
		t.Errorf("Expected neutral score 50, got %v", score)
	}

	if confidence != 0.0 {
		t.Errorf("Expected confidence 0, got %v", confidence)
	}
}

func TestBayesianFusion_SingleSignal(t *testing.T) {
	signals := []BenchmarkSignal{
		{Name: "arena_elo", Value: 80, Weight: 0.5, Confidence: 0.95},
	}

	score, confidence := BayesianFusion(signals)

	// Score should be close to the signal value
	if score < 75 || score > 85 {
		t.Errorf("Expected score around 80, got %v", score)
	}

	// Confidence should be penalized for single signal
	if confidence <= 0 || confidence >= 0.95 {
		t.Errorf("Expected penalized confidence < 0.95, got %v", confidence)
	}
}

func TestBayesianFusion_MultipleSignals(t *testing.T) {
	signals := []BenchmarkSignal{
		{Name: "arena_elo", Value: 85, Weight: 0.5, Confidence: 0.95},
		{Name: "mmlu_pro", Value: 80, Weight: 0.3, Confidence: 0.85},
		{Name: "gsm8k", Value: 90, Weight: 0.1, Confidence: 0.60},
	}

	score, confidence := BayesianFusion(signals)

	// Score should be weighted average
	if score < 80 || score > 90 {
		t.Errorf("Expected score around 84-86, got %v", score)
	}

	// Confidence should be higher with more signals
	if confidence < 0.6 {
		t.Errorf("Expected confidence > 0.6 for 3 signals, got %v", confidence)
	}
}

func TestBayesianFusion_ConfidencePenalty(t *testing.T) {
	oneSignal := []BenchmarkSignal{
		{Name: "arena_elo", Value: 80, Weight: 0.5, Confidence: 0.95},
	}

	twoSignals := []BenchmarkSignal{
		{Name: "arena_elo", Value: 80, Weight: 0.5, Confidence: 0.95},
		{Name: "mmlu_pro", Value: 80, Weight: 0.3, Confidence: 0.85},
	}

	_, conf1 := BayesianFusion(oneSignal)
	_, conf2 := BayesianFusion(twoSignals)

	if conf1 >= conf2 {
		t.Errorf("Two signals should have higher confidence than one: %v < %v", conf1, conf2)
	}
}

func TestCalculateConfidenceInterval(t *testing.T) {
	tests := []struct {
		name       string
		score      float64
		confidence float64
		wantLower  float64
		wantUpper  float64
	}{
		{"high_confidence", 85, 0.95, 77, 93},  // ±2 points
		{"low_confidence", 85, 0.3, 65, 100},   // Wider interval
		{"no_confidence", 50, 0, 0, 100},       // Maximum uncertainty
		{"perfect_confidence", 85, 1.0, 83, 87}, // ±2 points minimum
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower, upper := CalculateConfidenceInterval(tt.score, tt.confidence)
			
			if lower < tt.wantLower-5 || lower > tt.wantLower+5 {
				t.Errorf("Lower bound: got %v, want ~%v", lower, tt.wantLower)
			}
			
			if upper < tt.wantUpper-5 || upper > tt.wantUpper+5 {
				t.Errorf("Upper bound: got %v, want ~%v", upper, tt.wantUpper)
			}
			
			if lower > upper {
				t.Errorf("Lower %v should be <= upper %v", lower, upper)
			}
		})
	}
}

func TestCalculateCategoryScore(t *testing.T) {
	signals := GetBenchmarkSignals(
		1215,  // Arena ELO
		85.3,  // MMLU
		89.1,  // IFEval
		92.4,  // GSM8K
		96.2,  // ARC
	)

	tests := []struct {
		name     string
		category string
	}{
		{"general_chat", "general_chat"},
		{"coding", "coding"},
		{"reasoning", "reasoning"},
		{"long_context", "long_context"},
		{"multimodal", "multimodal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateCategoryScore(tt.category, signals)
			
			if score < 0 || score > 100 {
				t.Errorf("Category score %v out of range [0, 100]", score)
			}
			
			// Reasoning should weight GSM8K more heavily
			if tt.category == "reasoning" && score < 70 {
				t.Errorf("Reasoning score should be > 70 with strong signals, got %v", score)
			}
		})
	}
}

func TestCalculateAllCategoryScores(t *testing.T) {
	signals := GetBenchmarkSignals(
		1215,  // Arena ELO
		85.3,  // MMLU
		89.1,  // IFEval
		92.4,  // GSM8K
		96.2,  // ARC
	)

	scores := CalculateAllCategoryScores(signals)

	expectedCategories := []string{
		"general_chat",
		"coding",
		"reasoning",
		"long_context",
		"multimodal",
	}

	for _, cat := range expectedCategories {
		if _, exists := scores[cat]; !exists {
			t.Errorf("Missing category: %s", cat)
		}
	}
}

func TestAssignQualityTier(t *testing.T) {
	tests := []struct {
		name       string
		percentile int
		confidence float64
		wantTier   domain.QualityTier
	}{
		{"s_tier_high_conf", 97, 0.95, domain.TierS},
		{"a_tier", 90, 0.85, domain.TierA},
		{"b_tier", 75, 0.70, domain.TierB},
		{"c_tier", 50, 0.50, domain.TierC},
		{"d_tier", 20, 0.30, domain.TierD},
		{"s_tier_low_conf_downgraded", 96, 0.40, domain.TierA}, // Should downgrade
		{"a_tier_low_conf_downgraded", 88, 0.40, domain.TierB}, // Should downgrade
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := AssignQualityTier(tt.percentile, tt.confidence)
			if tier != tt.wantTier {
				t.Errorf("AssignQualityTier(%d, %.2f) = %v, want %v",
					tt.percentile, tt.confidence, tier, tt.wantTier)
			}
		})
	}
}

func TestScorer_Score(t *testing.T) {
	scorer := NewScorer()

	entry := domain.ModelEntry{
		ID:           "test-model",
		DisplayName:  "Test Model",
		ArenaELO:     1215,
		MMLUScore:    85.3,
		IFEvalScore:  89.1,
		GSM8KScore:   92.4,
		ARCScore:     96.2,
	}

	quality := scorer.Score(entry)

	if quality.Overall < 70 || quality.Overall > 95 {
		t.Errorf("Expected overall score 70-95, got %v", quality.Overall)
	}

	if quality.Confidence <= 0 || quality.Confidence > 1 {
		t.Errorf("Expected confidence in (0, 1], got %v", quality.Confidence)
	}

	if quality.Tier == "" {
		t.Error("Expected tier to be set")
	}

	if len(quality.CategoryScores) == 0 {
		t.Error("Expected category scores to be populated")
	}
}

func TestScorer_ScoreAll(t *testing.T) {
	scorer := NewScorer()

	entries := []domain.ModelEntry{
		{ID: "model-a", DisplayName: "Model A", ArenaELO: 1215, MMLUScore: 85.3},
		{ID: "model-b", DisplayName: "Model B", ArenaELO: 1100, MMLUScore: 75.0},
		{ID: "model-c", DisplayName: "Model C", ArenaELO: 900, MMLUScore: 65.0},
		{ID: "model-d", DisplayName: "Model D", ArenaELO: 1300, MMLUScore: 90.0},
	}

	qualities := scorer.ScoreAll(entries)

	if len(qualities) != 4 {
		t.Errorf("Expected 4 qualities, got %d", len(qualities))
	}

	// Verify percentiles are calculated
	for i, q := range qualities {
		if q.Percentile < 0 || q.Percentile > 100 {
			t.Errorf("Model %d: percentile %d out of range", i, q.Percentile)
		}
	}

	// Verify scores vary (higher ELO should get higher quality score)
	var maxScore float64
	var maxIdx int
	for i, q := range qualities {
		if q.Overall > maxScore {
			maxScore = q.Overall
			maxIdx = i
		}
	}
	
	// Model D (ELO 1300) should have the highest score
	if entries[maxIdx].ID != "model-d" {
		t.Errorf("Model D (ELO 1300) should have highest quality, but model %s has it", entries[maxIdx].ID)
	}
}

func TestScorer_MissingData(t *testing.T) {
	scorer := NewScorer()

	entry := domain.ModelEntry{
		ID:          "unknown-model",
		DisplayName: "Unknown Model",
		// No benchmarks at all
	}

	quality := scorer.Score(entry)

	if quality.Overall != 50.0 {
		t.Errorf("Expected neutral score 50 for missing data, got %v", quality.Overall)
	}

	if quality.Confidence != 0.0 {
		t.Errorf("Expected confidence 0 for missing data, got %v", quality.Confidence)
	}
}
