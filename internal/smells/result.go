package smells

import "time"

// Severity levels for code smells
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// SmellType represents the type of code smell
type SmellType string

const (
	SmellTypeGodClass    SmellType = "god_class"
	SmellTypeFeatureEnvy SmellType = "feature_envy"
	SmellTypeLongMethod  SmellType = "long_method"
	SmellTypeDataClass   SmellType = "data_class"
	SmellTypeLazyClass   SmellType = "lazy_class"
)

// DetectionResult contains the outcome of code smell detection
type DetectionResult struct {
	SmellType   SmellType `json:"smell_type"`
	RepoName    string  `json:"repo_name"`
	ClassName   string  `json:"class_name"`
	FilePath    string  `json:"file_path"`
	IsSmell     bool    `json:"is_smell"`
	Severity    Severity `json:"severity"`
	Confidence  float64 `json:"confidence"` // 0.0 - 1.0
	Strategy    string  `json:"strategy"`   // "rule_based", "score_based", etc.

	// Metrics
	SignalValues    map[string]float64 `json:"signal_values"`     // All computed signal values
	ViolatedSignals []string           `json:"violated_signals"`  // Which signals exceeded thresholds

	// Recommendations
	Recommendations []Recommendation `json:"recommendations"`

	// Metadata
	DetectedAt time.Time `json:"detected_at"`
}

// Recommendation suggests a refactoring action
type Recommendation struct {
	Type        string   `json:"type"`        // "extract_class", "move_method", "reduce_complexity", etc.
	Description string   `json:"description"` // Human-readable description
	Priority    int      `json:"priority"`    // 1-5, lower is higher priority
	TargetCode  []string `json:"target_code"` // Method names, line ranges, etc.
}

// NewDetectionResult creates a new detection result
func NewDetectionResult(smellType SmellType, repoName, className, filePath string) *DetectionResult {
	return &DetectionResult{
		SmellType:       smellType,
		RepoName:        repoName,
		ClassName:       className,
		FilePath:        filePath,
		SignalValues:    make(map[string]float64),
		ViolatedSignals: []string{},
		Recommendations: []Recommendation{},
		DetectedAt:      time.Now(),
	}
}
