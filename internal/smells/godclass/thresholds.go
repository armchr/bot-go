package godclass

// God Class Detection Thresholds (Lanza & Marinescu + Extended)
const (
	// Size thresholds
	ThresholdLOCNAMM   = 176.0
	ThresholdNOMAMM    = 18.0
	ThresholdNOF       = 10.0

	// Complexity thresholds
	ThresholdWMC       = 47.0
	ThresholdWMCNAMM   = 22.0
	ThresholdAMC       = 3.0

	// Cohesion thresholds (low values indicate problems)
	ThresholdTCCLow    = 0.33

	// Coupling thresholds
	ThresholdATFD      = 6.0
	ThresholdCe        = 20.0
	ThresholdCa        = 30.0
	ThresholdCBO       = 30.0
	ThresholdRFC       = 95.0

	// Semantic thresholds
	ThresholdMethodSimilarityLow = 0.5 // Low similarity = poor cohesion
	ThresholdSemanticClusters    = 3.0 // 3+ clusters = multiple responsibilities

	// Statistical thresholds
	ThresholdHighEntropyMethods = 3.0   // 3+ high-entropy methods
	ThresholdEntropyZScore      = 2.0   // Z-score > 2 = highly unusual

	// Score-based weights (sum to 1.0)
	WeightLOCNAMM               = 0.15
	WeightWMCNAMM               = 0.15
	WeightNOMAMM                = 0.10
	WeightTCC                   = 0.15
	WeightATFD                  = 0.10
	WeightRFC                   = 0.10
	WeightCBO                   = 0.10
	WeightMethodSimilarity      = 0.10
	WeightHighEntropyMethods    = 0.05

	// Normalization ranges (min, max for each metric)
	NormLOCNAMMMin              = 176.0
	NormLOCNAMMMax              = 400.0
	NormWMCNAMMMin              = 22.0
	NormWMCNAMMMax              = 100.0
	NormNOMAMMMin               = 18.0
	NormNOMAMMMax               = 50.0
	NormATFDMin                 = 6.0
	NormATFDMax                 = 20.0
	NormRFCMin                  = 95.0
	NormRFCMax                  = 200.0
	NormCBOMin                  = 30.0
	NormCBOMax                  = 80.0
	NormHighEntropyMethodsMin   = 3.0
	NormHighEntropyMethodsMax   = 10.0

	// Score thresholds
	ScoreThresholdDefinite = 0.75  // Definite god class
	ScoreThresholdLikely   = 0.60  // Likely god class
	ScoreThresholdModerate = 0.40  // Moderate concern
)
