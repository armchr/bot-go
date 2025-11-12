package service

import (
	"bot-go/internal/model/ngram"
	"math"
	"sync"
)

// NGramModel stores n-gram statistics and provides probability calculations
type NGramModel struct {
	n               int                // N-gram size
	vocabulary      map[string]int64   // token -> frequency
	ngramCounts     map[string]int64   // n-gram string -> count
	contextCounts   map[string]int64   // (n-1)-gram string -> count
	totalTokens     int64              // Total number of tokens
	smoother        Smoother           // Smoothing algorithm
	mu              sync.RWMutex       // Protects all maps
}

// NewNGramModel creates a new n-gram model
func NewNGramModel(n int, smoother Smoother) *NGramModel {
	if n < 1 {
		n = 3 // Default to trigrams
	}
	if smoother == nil {
		smoother = NewAddKSmoother(1.0) // Default to Laplace smoothing
	}
	return &NGramModel{
		n:             n,
		vocabulary:    make(map[string]int64),
		ngramCounts:   make(map[string]int64),
		contextCounts: make(map[string]int64),
		totalTokens:   0,
		smoother:      smoother,
	}
}

// Add adds tokens to the model, updating all counts
func (m *NGramModel) Add(tokens []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update vocabulary
	for _, token := range tokens {
		m.vocabulary[token]++
		m.totalTokens++
	}

	// Extract and count n-grams
	ngrams := m.extractNGrams(tokens)
	for _, ng := range ngrams {
		ngramStr := ng.String()
		m.ngramCounts[ngramStr]++

		// Update context counts
		if len(ng) > 1 {
			contextStr := ng.Context().String()
			m.contextCounts[contextStr]++
		}
	}
}

// Remove removes tokens from the model (for incremental updates)
func (m *NGramModel) Remove(tokens []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update vocabulary
	for _, token := range tokens {
		if count, ok := m.vocabulary[token]; ok {
			if count > 1 {
				m.vocabulary[token]--
				m.totalTokens--
			} else {
				delete(m.vocabulary, token)
				m.totalTokens--
			}
		}
	}

	// Remove n-gram counts
	ngrams := m.extractNGrams(tokens)
	for _, ng := range ngrams {
		ngramStr := ng.String()
		if count, ok := m.ngramCounts[ngramStr]; ok {
			if count > 1 {
				m.ngramCounts[ngramStr]--
			} else {
				delete(m.ngramCounts, ngramStr)
			}
		}

		// Update context counts
		if len(ng) > 1 {
			contextStr := ng.Context().String()
			if count, ok := m.contextCounts[contextStr]; ok {
				if count > 1 {
					m.contextCounts[contextStr]--
				} else {
					delete(m.contextCounts, contextStr)
				}
			}
		}
	}
}

// Merge combines another model into this one
func (m *NGramModel) Merge(other *NGramModel) {
	if other == nil || other.n != m.n {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	// Merge vocabulary
	for token, count := range other.vocabulary {
		m.vocabulary[token] += count
	}
	m.totalTokens += other.totalTokens

	// Merge n-gram counts
	for ngram, count := range other.ngramCounts {
		m.ngramCounts[ngram] += count
	}

	// Merge context counts
	for context, count := range other.contextCounts {
		m.contextCounts[context] += count
	}
}

// Probability calculates the probability of a token given its context
func (m *NGramModel) Probability(token string, context []string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build the n-gram
	ng := append(context, token)
	if len(ng) > m.n {
		ng = ng[len(ng)-m.n:]
	}

	ngramStr := ngram.NGram(ng).String()
	contextStr := ngram.NGram(context).String()

	ngramCount := m.ngramCounts[ngramStr]
	contextCount := m.contextCounts[contextStr]

	// Calculate backoff probability (uniform for now)
	backoffProb := 1.0 / float64(len(m.vocabulary))
	if len(m.vocabulary) == 0 {
		backoffProb = 0.0
	}

	return m.smoother.Smooth(ngramCount, contextCount, backoffProb, len(m.vocabulary))
}

// CrossEntropy calculates the cross-entropy of a token sequence
func (m *NGramModel) CrossEntropy(tokens []string) float64 {
	if len(tokens) == 0 {
		return 0.0
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	totalLogProb := 0.0
	count := 0

	for i := 0; i < len(tokens); i++ {
		// Build context
		contextStart := 0
		if i >= m.n-1 {
			contextStart = i - m.n + 1
		}
		context := tokens[contextStart:i]

		prob := m.Probability(tokens[i], context)
		if prob > 0 {
			totalLogProb += math.Log2(prob)
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return -totalLogProb / float64(count)
}

// Perplexity calculates the perplexity of a token sequence
func (m *NGramModel) Perplexity(tokens []string) float64 {
	entropy := m.CrossEntropy(tokens)
	return math.Pow(2, entropy)
}

// extractNGrams extracts all n-grams from a token sequence
func (m *NGramModel) extractNGrams(tokens []string) []ngram.NGram {
	if len(tokens) == 0 {
		return nil
	}

	var result []ngram.NGram

	// Extract n-grams of size m.n
	for i := 0; i <= len(tokens)-m.n; i++ {
		ng := make(ngram.NGram, m.n)
		copy(ng, tokens[i:i+m.n])
		result = append(result, ng)
	}

	// Handle tail if sequence is shorter than n
	if len(tokens) < m.n {
		ng := make(ngram.NGram, len(tokens))
		copy(ng, tokens)
		result = append(result, ng)
	}

	return result
}

// Stats returns statistics about the model
func (m *NGramModel) Stats() ModelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ModelStats{
		N:               m.n,
		VocabularySize:  len(m.vocabulary),
		NGramCount:      len(m.ngramCounts),
		TotalTokens:     m.totalTokens,
		SmootherName:    m.smoother.Name(),
	}
}

// ModelStats contains statistics about an n-gram model
type ModelStats struct {
	N              int    `json:"n"`
	VocabularySize int    `json:"vocabulary_size"`
	NGramCount     int    `json:"ngram_count"`
	TotalTokens    int64  `json:"total_tokens"`
	SmootherName   string `json:"smoother_name"`
}
