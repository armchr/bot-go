package smells

import (
	"context"

	"bot-go/internal/signals"
)

// Detector is the main interface for code smell detection
type Detector interface {
	// Name returns the unique identifier for this detector
	Name() string

	// SmellType returns the type of smell this detector finds
	SmellType() SmellType

	// Detect analyzes a class and returns detection result
	Detect(ctx context.Context, classInfo *signals.ClassInfo) (*DetectionResult, error)
}
