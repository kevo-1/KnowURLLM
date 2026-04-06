# MoE (Mixture of Experts) Handling - Current State & Known Issues

## ✅ What's Working

1. **Data Structure**: `ModelEntry` has `IsMoE` and `ActiveParams` fields
2. **Registry Parsing**: `hf_models.json` contains MoE metadata (`is_moe`, `active_parameters`, `num_experts`, `active_experts`)
3. **Field Population**: `hfModelToEntry` now correctly copies `IsMoE` and `ActiveParams` from JSON
4. **Run Mode Detection**: `detectRunMode` checks for MoE and uses active params for VRAM fit
5. **Performance Penalty**: `MODE_MOE = 0.8` multiplier applied in TPS estimation

## ⚠️ Known Issues & Limitations

### Issue 1: Parameter Estimation for Active Size

**Current Logic:**
```go
// In domain/hardware/fit.go
totalParamsEstimate := uint64(float64(modelSizeBytes) / 0.563)
activeSizeBytes := modelSizeBytes * activeParams / totalParamsEstimate
```

**Problem:** This assumes a **linear relationship** between parameter count and model size, which is approximately true but:
- Doesn't account for overhead (routing networks, expert gates)
- May be inaccurate for non-Q4 quants
- Active params in JSON may be in different units than expected

**Impact:** MoE VRAM estimation could be off by 10-20%

### Issue 2: Missing NumExperts and ActiveExperts Fields

**Current State:**
- JSON has: `num_experts`, `active_experts`
- `hfModel` struct parses them
- **NOT copied** to `domain.ModelEntry`

**Why It Matters:**
- Could use `active_experts / num_experts` ratio for better VRAM estimation
- Useful for display ("8 experts, 2 active")
- Helps validate `active_parameters` data

**Fix Needed:** Add these fields to `ModelEntry` and populate them

### Issue 3: MoE Performance Estimation Too Simple

**Current Logic:**
```go
// In domain/hardware/performance.go
estimatedTPS *= MODE_MOE  // 0.8 penalty
```

**Problem:** This is a flat 20% penalty regardless of:
- How many experts are active (2/8 vs 6/8 is very different)
- Expert switching overhead (hardware-dependent)
- Whether experts fit entirely in VRAM or need RAM fetching

**Better Approach:**
```go
// Dynamic MoE penalty based on expert distribution
expertRatio := float64(activeExperts) / float64(numExperts)
moePenalty := 0.9 - (0.2 * expertRatio) // 0.7-0.9 range

// Additional penalty if inactive experts need RAM
if !allExpertsFitInVRAM {
    moePenalty *= 0.7 // RAM access penalty
}
estimatedTPS *= moePenalty
```

### Issue 4: No MoE-Specific Display in TUI

**Current TUI:**
- Shows "MoE ✓" badge (good)
- Doesn't show: number of experts, active experts, expert ratio
- Doesn't explain MoE run mode to users

**Should Show:**
```
Tier: A  |  Fit: MoE ✓
Quality: 85/100
MoE: 8 experts, 2 active (25%)
TPS: ~45 tok/s
```

### Issue 5: MoE Models in Embedded Database

**Statistics from hf_models.json:**
- ~50+ MoE models detected
- Includes: Mixtral 8x7B, Mixtral 8x22B, Qwen MoE variants, etc.
- Wide range of active params: 984K to 39B

**Validation Needed:**
- Verify `active_parameters` values are correct (some seem suspiciously low)
- Cross-reference with official model cards
- Some models may have incorrect `is_moe` flags

## 📋 Recommended Fixes (Priority Order)

### High Priority
1. ✅ **Populate IsMoE and ActiveParams** (DONE - just fixed)
2. ⚠️ **Add NumExperts and ActiveExperts fields** to ModelEntry
3. ⚠️ **Improve MoE performance estimation** with dynamic penalty

### Medium Priority
4. 📊 **Add MoE details to TUI display** (expert count, ratio)
5. 🔍 **Validate MoE data in hf_models.json** against official sources

### Low Priority
6. 🧪 **Add MoE-specific tests** with real model data
7. 📖 **Document MoE assumptions and limitations**

## Example MoE Models in Database

| Model | Total Params | Active Params | Experts | Active Experts |
|-------|-------------|---------------|---------|----------------|
| Mixtral 8x7B | 46.7B | 12.9B | 8 | 2 |
| Mixtral 8x22B | 140.6B | 39B | 8 | 2 |
| Qwen1.5-MoE-A2.7B | 14.3B | 2.7B | 64 | 4 |
| DeepSeekMoE-16B | 15.9B | 2.4B | 160 | 8 |

## Conclusion

The MoE structure is **partially addressed**. The critical bug (fields not populated) is now fixed, but there are still accuracy issues with:
- Parameter estimation
- Performance prediction
- User-facing display
- Data validation

These don't prevent the tool from working, but they reduce accuracy for MoE models specifically.
