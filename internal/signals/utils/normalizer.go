package utils

import "math"

// Normalizer provides utility functions for normalizing signal values
type Normalizer struct{}

// NewNormalizer creates a new normalizer
func NewNormalizer() *Normalizer {
	return &Normalizer{}
}

// Normalize converts a value to 0-1 scale based on min/max thresholds
// Values below min return 0, values above max return 1
func (n *Normalizer) Normalize(value, min, max float64) float64 {
	if value <= min {
		return 0.0
	}
	if value >= max {
		return 1.0
	}
	return (value - min) / (max - min)
}

// InvertNormalize is for metrics where LOW values are bad (e.g., TCC)
// Returns 1.0 for low values, 0.0 for high values
func (n *Normalizer) InvertNormalize(value, min, max float64) float64 {
	normalized := n.Normalize(value, min, max)
	return 1.0 - normalized
}

// NormalizeWithSigmoid uses sigmoid function for smoother normalization
// Useful when values can extend beyond typical ranges
func (n *Normalizer) NormalizeWithSigmoid(value, midpoint, steepness float64) float64 {
	// Sigmoid: 1 / (1 + e^(-steepness * (value - midpoint)))
	exponent := -steepness * (value - midpoint)
	return 1.0 / (1.0 + math.Exp(exponent))
}

// Clamp constrains a value to [min, max] range
func (n *Normalizer) Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
