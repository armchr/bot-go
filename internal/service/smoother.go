package service

// Smoother defines the interface for n-gram probability smoothing algorithms
type Smoother interface {
	// Smooth computes the smoothed probability for an n-gram
	// ngramCount: count of the full n-gram
	// contextCount: count of the context (n-1 gram)
	// backoffProb: probability from lower-order model
	// vocabularySize: size of the vocabulary
	Smooth(ngramCount, contextCount int64, backoffProb float64, vocabularySize int) float64

	// Name returns the name of the smoothing algorithm
	Name() string
}

// AddKSmoother implements simple add-k (Laplace) smoothing
type AddKSmoother struct {
	k float64
}

// NewAddKSmoother creates a new add-k smoother
func NewAddKSmoother(k float64) *AddKSmoother {
	if k <= 0 {
		k = 1.0 // Default to Laplace smoothing
	}
	return &AddKSmoother{k: k}
}

func (s *AddKSmoother) Smooth(ngramCount, contextCount int64, backoffProb float64, vocabularySize int) float64 {
	if contextCount == 0 {
		return 1.0 / float64(vocabularySize)
	}
	numerator := float64(ngramCount) + s.k
	denominator := float64(contextCount) + (s.k * float64(vocabularySize))
	return numerator / denominator
}

func (s *AddKSmoother) Name() string {
	return "AddK"
}

// WittenBellSmoother implements Witten-Bell smoothing
type WittenBellSmoother struct{}

// NewWittenBellSmoother creates a new Witten-Bell smoother
func NewWittenBellSmoother() *WittenBellSmoother {
	return &WittenBellSmoother{}
}

func (s *WittenBellSmoother) Smooth(ngramCount, contextCount int64, backoffProb float64, vocabularySize int) float64 {
	if contextCount == 0 {
		return 1.0 / float64(vocabularySize)
	}

	if ngramCount > 0 {
		// Seen n-gram: use MLE with discounting
		uniqueTypes := float64(vocabularySize) // Simplified: should be unique continuations
		lambda := float64(contextCount) / (float64(contextCount) + uniqueTypes)
		return lambda * (float64(ngramCount) / float64(contextCount))
	}

	// Unseen n-gram: use backoff
	uniqueTypes := float64(vocabularySize)
	lambda := uniqueTypes / (float64(contextCount) + uniqueTypes)
	return lambda * backoffProb
}

func (s *WittenBellSmoother) Name() string {
	return "WittenBell"
}
