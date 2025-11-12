package service

import (
	"bot-go/internal/model/ngram"
	"math"
	"sync"
)

// NGramModelTrie stores n-gram statistics using a trie structure
type NGramModelTrie struct {
	n            int          // N-gram size
	ngramTrie    *NGramTrie   // Trie for full n-grams
	contextTrie  *NGramTrie   // Trie for (n-1)-grams (contexts)
	vocabulary   *NGramTrie   // Trie for unigrams (vocabulary)
	totalTokens  int64        // Total number of tokens
	smoother     Smoother     // Smoothing algorithm
	mu           sync.RWMutex // Protects totalTokens
}

// NewNGramModelTrie creates a new trie-based n-gram model without bloom filter
func NewNGramModelTrie(n int, smoother Smoother) *NGramModelTrie {
	return NewNGramModelTrieWithBloom(n, smoother, false, 100000, 0.01)
}

// NewNGramModelTrieWithBloom creates a new trie-based n-gram model with optional bloom filter
func NewNGramModelTrieWithBloom(n int, smoother Smoother, useBloom bool, expectedItems uint, falsePositiveRate float64) *NGramModelTrie {
	if n < 1 {
		n = 3 // Default to trigrams
	}
	if smoother == nil {
		smoother = NewAddKSmoother(1.0) // Default to Laplace smoothing
	}

	// Create tries with bloom filter if enabled
	var ngramTrie, contextTrie, vocabulary *NGramTrie
	if useBloom {
		ngramTrie = NewNGramTrieWithBloom(true, expectedItems, falsePositiveRate)
		contextTrie = NewNGramTrieWithBloom(true, expectedItems, falsePositiveRate)
		vocabulary = NewNGramTrieWithBloom(false, expectedItems/10, falsePositiveRate) // Smaller for vocabulary
	} else {
		ngramTrie = NewNGramTrie()
		contextTrie = NewNGramTrie()
		vocabulary = NewNGramTrie()
	}

	return &NGramModelTrie{
		n:           n,
		ngramTrie:   ngramTrie,
		contextTrie: contextTrie,
		vocabulary:  vocabulary,
		totalTokens: 0,
		smoother:    smoother,
	}
}

// Add adds tokens to the model, updating all counts
func (m *NGramModelTrie) Add(tokens []string) {
	if len(tokens) == 0 {
		return
	}

	m.mu.Lock()
	m.totalTokens += int64(len(tokens))
	m.mu.Unlock()

	// Update vocabulary (unigrams)
	for _, token := range tokens {
		m.vocabulary.Insert([]string{token})
	}

	// Extract and count n-grams
	ngrams := m.extractNGrams(tokens)
	for _, ng := range ngrams {
		// Add full n-gram
		m.ngramTrie.Insert(ng)

		// Add context (n-1 gram) if applicable
		if len(ng) > 1 {
			context := ng[:len(ng)-1]
			m.contextTrie.Insert(context)
		}
	}
}

// Remove removes tokens from the model (for incremental updates)
func (m *NGramModelTrie) Remove(tokens []string) {
	if len(tokens) == 0 {
		return
	}

	m.mu.Lock()
	m.totalTokens -= int64(len(tokens))
	if m.totalTokens < 0 {
		m.totalTokens = 0
	}
	m.mu.Unlock()

	// Remove from vocabulary
	for _, token := range tokens {
		m.vocabulary.Remove([]string{token})
	}

	// Remove n-grams
	ngrams := m.extractNGrams(tokens)
	for _, ng := range ngrams {
		m.ngramTrie.Remove(ng)

		if len(ng) > 1 {
			context := ng[:len(ng)-1]
			m.contextTrie.Remove(context)
		}
	}
}

// Merge combines another trie-based model into this one
func (m *NGramModelTrie) Merge(other *NGramModelTrie) {
	if other == nil || other.n != m.n {
		return
	}

	// Note: This is a simplified merge that re-adds all n-grams
	// A more efficient implementation would merge the tries directly
	// For now, we just update the total tokens
	m.mu.Lock()
	other.mu.RLock()
	m.totalTokens += other.totalTokens
	other.mu.RUnlock()
	m.mu.Unlock()

	// TODO: Implement efficient trie merging
	// For now, this is a placeholder - the tries are independent structures
}

// Probability calculates the probability of a token given its context
func (m *NGramModelTrie) Probability(token string, context []string) float64 {
	// Build the n-gram
	ng := append(context, token)
	if len(ng) > m.n {
		ng = ng[len(ng)-m.n:]
	}

	ngramCount := m.ngramTrie.GetCount(ng)

	// Get context count
	contextCount := int64(0)
	if len(ng) > 1 {
		ctx := ng[:len(ng)-1]
		contextCount = m.contextTrie.GetCount(ctx)
	}

	// Calculate backoff probability (uniform for now)
	vocabSize := m.vocabulary.VocabularySize()
	backoffProb := 1.0 / float64(vocabSize)
	if vocabSize == 0 {
		backoffProb = 0.0
	}

	return m.smoother.Smooth(ngramCount, contextCount, backoffProb, vocabSize)
}

// CrossEntropy calculates the cross-entropy of a token sequence
func (m *NGramModelTrie) CrossEntropy(tokens []string) float64 {
	if len(tokens) == 0 {
		return 0.0
	}

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
func (m *NGramModelTrie) Perplexity(tokens []string) float64 {
	entropy := m.CrossEntropy(tokens)
	return math.Pow(2, entropy)
}

// extractNGrams extracts all n-grams from a token sequence (returns as []string slices)
func (m *NGramModelTrie) extractNGrams(tokens []string) [][]string {
	if len(tokens) == 0 {
		return nil
	}

	var result [][]string

	// Extract n-grams of size m.n
	for i := 0; i <= len(tokens)-m.n; i++ {
		ng := make([]string, m.n)
		copy(ng, tokens[i:i+m.n])
		result = append(result, ng)
	}

	// Handle tail if sequence is shorter than n
	if len(tokens) < m.n {
		ng := make([]string, len(tokens))
		copy(ng, tokens)
		result = append(result, ng)
	}

	return result
}

// Stats returns statistics about the model
func (m *NGramModelTrie) Stats() ModelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ModelStats{
		N:              m.n,
		VocabularySize: m.vocabulary.VocabularySize(),
		NGramCount:     int(m.ngramTrie.TotalNGrams()),
		TotalTokens:    m.totalTokens,
		SmootherName:   m.smoother.Name(),
	}
}

// MemoryStats returns detailed memory usage statistics
func (m *NGramModelTrie) MemoryStats() TrieModelMemoryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return TrieModelMemoryStats{
		N:               m.n,
		TotalTokens:     m.totalTokens,
		VocabularyStats: m.vocabulary.MemoryStats(),
		NGramStats:      m.ngramTrie.MemoryStats(),
		ContextStats:    m.contextTrie.MemoryStats(),
	}
}

// Prune removes n-grams with count below threshold
func (m *NGramModelTrie) Prune(minCount int64) (int64, int64) {
	ngramPruned := m.ngramTrie.Prune(minCount)
	contextPruned := m.contextTrie.Prune(minCount)
	return ngramPruned, contextPruned
}

// GetNGramsWithPrefix returns all n-grams starting with a given prefix
func (m *NGramModelTrie) GetNGramsWithPrefix(prefix []string) []NGramWithCount {
	return m.ngramTrie.GetAllWithPrefix(prefix)
}

// TrieModelMemoryStats contains detailed memory statistics for the trie model
type TrieModelMemoryStats struct {
	N               int             `json:"n"`
	TotalTokens     int64           `json:"total_tokens"`
	VocabularyStats TrieMemoryStats `json:"vocabulary_stats"`
	NGramStats      TrieMemoryStats `json:"ngram_stats"`
	ContextStats    TrieMemoryStats `json:"context_stats"`
}

// TotalMemoryBytes returns the estimated total memory usage
func (s TrieModelMemoryStats) TotalMemoryBytes() int64 {
	return s.VocabularyStats.TotalMemoryBytes() +
		s.NGramStats.TotalMemoryBytes() +
		s.ContextStats.TotalMemoryBytes()
}

// ConvertToTrieModel converts a map-based NGramModel to a trie-based model
func ConvertToTrieModel(model *NGramModel) *NGramModelTrie {
	trieModel := NewNGramModelTrie(model.n, model.smoother)

	model.mu.RLock()
	defer model.mu.RUnlock()

	// Copy vocabulary
	for token, count := range model.vocabulary {
		for i := int64(0); i < count; i++ {
			trieModel.vocabulary.Insert([]string{token})
		}
	}

	// Copy n-grams
	for ngramStr, count := range model.ngramCounts {
		// Parse n-gram string back to tokens
		ng := ngram.NGram{}
		// This is a simplified parser - assumes space-separated tokens
		tokens := parseNGramString(ngramStr)
		for i := int64(0); i < count; i++ {
			trieModel.ngramTrie.Insert(tokens)
		}
		_ = ng // Suppress unused warning
	}

	// Copy contexts
	for contextStr, count := range model.contextCounts {
		tokens := parseNGramString(contextStr)
		for i := int64(0); i < count; i++ {
			trieModel.contextTrie.Insert(tokens)
		}
	}

	trieModel.totalTokens = model.totalTokens

	return trieModel
}

// parseNGramString is a helper to parse space-separated n-gram strings
// Note: This is simplified and doesn't handle tokens with spaces
func parseNGramString(ngramStr string) []string {
	if ngramStr == "" {
		return []string{}
	}
	// Simple split by space - may need more sophisticated parsing
	tokens := []string{}
	current := ""
	for _, ch := range ngramStr {
		if ch == ' ' {
			if current != "" {
				tokens = append(tokens, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		tokens = append(tokens, current)
	}
	return tokens
}
