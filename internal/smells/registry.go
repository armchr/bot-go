package smells

import (
	"context"
	"fmt"
	"sync"

	"bot-go/internal/signals"

	"go.uber.org/zap"
)

// DetectorRegistry manages all smell detectors
type DetectorRegistry struct {
	detectors map[string]Detector
	logger    *zap.Logger
	mu        sync.RWMutex
}

// NewDetectorRegistry creates a new detector registry
func NewDetectorRegistry(logger *zap.Logger) *DetectorRegistry {
	return &DetectorRegistry{
		detectors: make(map[string]Detector),
		logger:    logger,
	}
}

// Register adds a detector to the registry
func (r *DetectorRegistry) Register(detector Detector) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.detectors[detector.Name()] = detector
	r.logger.Info("Registered smell detector",
		zap.String("detector", detector.Name()),
		zap.String("smell_type", string(detector.SmellType())))
}

// Get retrieves a detector by name
func (r *DetectorRegistry) Get(name string) (Detector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	detector, ok := r.detectors[name]
	if !ok {
		return nil, fmt.Errorf("detector not found: %s", name)
	}
	return detector, nil
}

// DetectAll runs all registered detectors on a class
func (r *DetectorRegistry) DetectAll(ctx context.Context, classInfo *signals.ClassInfo) ([]*DetectionResult, error) {
	r.mu.RLock()
	detectors := make([]Detector, 0, len(r.detectors))
	for _, detector := range r.detectors {
		detectors = append(detectors, detector)
	}
	r.mu.RUnlock()

	results := make([]*DetectionResult, 0, len(detectors))

	for _, detector := range detectors {
		result, err := detector.Detect(ctx, classInfo)
		if err != nil {
			r.logger.Warn("Detector failed",
				zap.String("detector", detector.Name()),
				zap.String("class", classInfo.ClassName),
				zap.Error(err))
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GetAllDetectors returns all registered detectors
func (r *DetectorRegistry) GetAllDetectors() []Detector {
	r.mu.RLock()
	defer r.mu.RUnlock()

	detectors := make([]Detector, 0, len(r.detectors))
	for _, detector := range r.detectors {
		detectors = append(detectors, detector)
	}
	return detectors
}
