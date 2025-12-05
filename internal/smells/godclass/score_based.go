package godclass

import (
	"context"
	"fmt"

	"bot-go/internal/signals/utils"
	"bot-go/internal/smells"
)

// ScoreBasedStrategy implements weighted scoring for god class detection
type ScoreBasedStrategy struct {
	normalizer *utils.Normalizer
}

// NewScoreBasedStrategy creates a new score-based strategy
func NewScoreBasedStrategy() *ScoreBasedStrategy {
	return &ScoreBasedStrategy{
		normalizer: utils.NewNormalizer(),
	}
}

func (s *ScoreBasedStrategy) Name() string {
	return "score_based"
}

func (s *ScoreBasedStrategy) Detect(ctx context.Context, signalValues map[string]float64) (*StrategyResult, error) {
	result := &StrategyResult{
		IsGodClass:      false,
		Severity:        smells.SeverityLow,
		Confidence:      0.0,
		ViolatedSignals: []string{},
		Explanation:     "",
	}

	// Calculate weighted score
	score := 0.0
	var violations []string

	// Component 1: LOCNAMM (weight: 0.15)
	if locnamm, ok := signalValues["LOCNAMM"]; ok {
		normalized := s.normalizer.Normalize(locnamm, NormLOCNAMMMin, NormLOCNAMMMax)
		score += WeightLOCNAMM * normalized
		if locnamm >= ThresholdLOCNAMM {
			violations = append(violations, fmt.Sprintf("LOCNAMM (%.0f)", locnamm))
		}
	}

	// Component 2: WMCNAMM (weight: 0.15)
	if wmcnamm, ok := signalValues["WMCNAMM"]; ok {
		normalized := s.normalizer.Normalize(wmcnamm, NormWMCNAMMMin, NormWMCNAMMMax)
		score += WeightWMCNAMM * normalized
		if wmcnamm >= ThresholdWMCNAMM {
			violations = append(violations, fmt.Sprintf("WMCNAMM (%.0f)", wmcnamm))
		}
	}

	// Component 3: NOMNAMM (weight: 0.10)
	if nomnamm, ok := signalValues["NOMNAMM"]; ok {
		normalized := s.normalizer.Normalize(nomnamm, NormNOMAMMMin, NormNOMAMMMax)
		score += WeightNOMAMM * normalized
		if nomnamm >= ThresholdNOMAMM {
			violations = append(violations, fmt.Sprintf("NOMNAMM (%.0f)", nomnamm))
		}
	}

	// Component 4: TCC (weight: 0.15, inverted since low is bad)
	if tcc, ok := signalValues["TCC"]; ok {
		// Invert: low TCC = high score contribution
		invertedTCC := 1.0 - tcc
		score += WeightTCC * invertedTCC
		if tcc <= ThresholdTCCLow {
			violations = append(violations, fmt.Sprintf("TCC (%.2f)", tcc))
		}
	}

	// Component 5: ATFD (weight: 0.10)
	if atfd, ok := signalValues["ATFD"]; ok {
		normalized := s.normalizer.Normalize(atfd, NormATFDMin, NormATFDMax)
		score += WeightATFD * normalized
		if atfd >= ThresholdATFD {
			violations = append(violations, fmt.Sprintf("ATFD (%.0f)", atfd))
		}
	}

	// Component 6: RFC (weight: 0.10)
	if rfc, ok := signalValues["RFC"]; ok {
		normalized := s.normalizer.Normalize(rfc, NormRFCMin, NormRFCMax)
		score += WeightRFC * normalized
		if rfc >= ThresholdRFC {
			violations = append(violations, fmt.Sprintf("RFC (%.0f)", rfc))
		}
	}

	// Component 7: CBO (weight: 0.10)
	if cbo, ok := signalValues["CBO"]; ok {
		normalized := s.normalizer.Normalize(cbo, NormCBOMin, NormCBOMax)
		score += WeightCBO * normalized
		if cbo >= ThresholdCBO {
			violations = append(violations, fmt.Sprintf("CBO (%.0f)", cbo))
		}
	}

	// Component 8: Method Similarity (weight: 0.10, inverted)
	if methodSim, ok := signalValues["MethodSimilarity"]; ok {
		// Low similarity = high score contribution
		invertedSim := 1.0 - methodSim
		score += WeightMethodSimilarity * invertedSim
		if methodSim < ThresholdMethodSimilarityLow {
			violations = append(violations, fmt.Sprintf("MethodSimilarity (%.2f)", methodSim))
		}
	}

	// Component 9: High Entropy Methods (weight: 0.05)
	if highEntropy, ok := signalValues["HighEntropyMethods"]; ok {
		normalized := s.normalizer.Normalize(highEntropy, NormHighEntropyMethodsMin, NormHighEntropyMethodsMax)
		score += WeightHighEntropyMethods * normalized
		if highEntropy >= ThresholdHighEntropyMethods {
			violations = append(violations, fmt.Sprintf("HighEntropyMethods (%.0f)", highEntropy))
		}
	}

	result.ViolatedSignals = violations

	// Determine classification based on score
	if score >= ScoreThresholdDefinite {
		result.IsGodClass = true
		result.Severity = smells.SeverityCritical
		result.Confidence = score
		result.Explanation = fmt.Sprintf("Definite god class (score: %.2f)", score)
	} else if score >= ScoreThresholdLikely {
		result.IsGodClass = true
		result.Severity = smells.SeverityHigh
		result.Confidence = score
		result.Explanation = fmt.Sprintf("Likely god class (score: %.2f)", score)
	} else if score >= ScoreThresholdModerate {
		result.IsGodClass = true
		result.Severity = smells.SeverityMedium
		result.Confidence = score
		result.Explanation = fmt.Sprintf("Moderate god class concerns (score: %.2f)", score)
	} else {
		result.IsGodClass = false
		result.Severity = smells.SeverityLow
		result.Confidence = score
		result.Explanation = fmt.Sprintf("Not a god class (score: %.2f)", score)
	}

	return result, nil
}
