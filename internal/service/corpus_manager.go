package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FileModel represents the n-gram model for a single file
type FileModel struct {
	FilePath      string
	Language      string
	TokenCount    int
	LastModified  time.Time
	Model         *NGramModel
	TrieModel     *NGramModelTrie // Optional trie-based model
	Entropy       float64         // Cached entropy value
}

// CorpusManager manages both file-level and global n-gram models
type CorpusManager struct {
	globalModel     *NGramModel            // Aggregate model across all files
	globalTrieModel *NGramModelTrie        // Optional trie-based global model
	fileModels      map[string]*FileModel  // file path -> file model
	tokenizer       *TokenizerRegistry
	n               int                    // N-gram size
	smoother        Smoother
	useTrie         bool                   // Use trie-based storage
	useBloom        bool                   // Use bloom filter for singleton detection
	logger          *zap.Logger
	mu              sync.RWMutex           // Protects fileModels map
}

// NewCorpusManager creates a new corpus manager
func NewCorpusManager(n int, smoother Smoother, tokenizer *TokenizerRegistry, logger *zap.Logger) *CorpusManager {
	return NewCorpusManagerWithOptions(n, smoother, tokenizer, false, false, logger)
}

// NewCorpusManagerWithTrie creates a new corpus manager using trie-based storage
func NewCorpusManagerWithTrie(n int, smoother Smoother, tokenizer *TokenizerRegistry, logger *zap.Logger) *CorpusManager {
	return NewCorpusManagerWithOptions(n, smoother, tokenizer, true, false, logger)
}

// NewCorpusManagerWithTrieAndBloom creates a new corpus manager using trie + bloom filter
func NewCorpusManagerWithTrieAndBloom(n int, smoother Smoother, tokenizer *TokenizerRegistry, logger *zap.Logger) *CorpusManager {
	return NewCorpusManagerWithOptions(n, smoother, tokenizer, true, true, logger)
}

// NewCorpusManagerWithOptions creates a new corpus manager with custom options
func NewCorpusManagerWithOptions(n int, smoother Smoother, tokenizer *TokenizerRegistry, useTrie bool, useBloom bool, logger *zap.Logger) *CorpusManager {
	if n < 1 {
		n = 3 // Default to trigrams
	}
	if smoother == nil {
		smoother = NewAddKSmoother(1.0)
	}
	if tokenizer == nil {
		tokenizer = NewTokenizerRegistry()
	}

	cm := &CorpusManager{
		fileModels: make(map[string]*FileModel),
		tokenizer:  tokenizer,
		n:          n,
		smoother:   smoother,
		useTrie:    useTrie,
		useBloom:   useBloom,
		logger:     logger,
	}

	if useTrie {
		if useBloom {
			// Create trie model with bloom filter
			// Estimate based on typical code corpus: ~100K n-grams per 10K LOC
			cm.globalTrieModel = NewNGramModelTrieWithBloom(n, smoother, true, 100000, 0.01)
		} else {
			cm.globalTrieModel = NewNGramModelTrie(n, smoother)
		}
	} else {
		cm.globalModel = NewNGramModel(n, smoother)
	}

	return cm
}

// AddFile adds a file to the corpus, updating both file-level and global models
func (cm *CorpusManager) AddFile(ctx context.Context, filePath string, source []byte, language string) error {
	// Get the appropriate tokenizer
	tok, ok := cm.tokenizer.GetTokenizer(language)
	if !ok {
		return fmt.Errorf("no tokenizer found for language: %s", language)
	}

	// Tokenize the source
	tokenSeq, err := tok.Tokenize(ctx, source)
	if err != nil {
		return fmt.Errorf("tokenization failed: %w", err)
	}

	// Normalize tokens
	normalizedTokens := make([]string, 0, len(tokenSeq))
	for _, token := range tokenSeq {
		normalized := tok.Normalize(token)
		normalizedTokens = append(normalizedTokens, normalized)
	}

	// Check if file already exists and update
	cm.mu.Lock()
	if _, exists := cm.fileModels[filePath]; exists {
		cm.mu.Unlock()
		return cm.UpdateFile(ctx, filePath, source, language)
	}
	cm.mu.Unlock()

	// Create new file model (map-based or trie-based)
	var entropy float64
	fm := &FileModel{
		FilePath:     filePath,
		Language:     language,
		TokenCount:   len(normalizedTokens),
		LastModified: time.Now(),
	}

	if cm.useTrie {
		var fileTrieModel *NGramModelTrie
		if cm.useBloom {
			fileTrieModel = NewNGramModelTrieWithBloom(cm.n, cm.smoother, true, 10000, 0.01)
		} else {
			fileTrieModel = NewNGramModelTrie(cm.n, cm.smoother)
		}
		fileTrieModel.Add(normalizedTokens)
		entropy = fileTrieModel.CrossEntropy(normalizedTokens)
		fm.TrieModel = fileTrieModel

		// Update global trie model
		cm.globalTrieModel.Add(normalizedTokens)
	} else {
		fileModel := NewNGramModel(cm.n, cm.smoother)
		fileModel.Add(normalizedTokens)
		entropy = fileModel.CrossEntropy(normalizedTokens)
		fm.Model = fileModel

		// Update global model
		cm.globalModel.Add(normalizedTokens)
	}

	fm.Entropy = entropy

	// Store file model
	cm.mu.Lock()
	cm.fileModels[filePath] = fm
	cm.mu.Unlock()

	cm.logger.Debug("Added file to corpus",
		zap.String("path", filePath),
		zap.String("language", language),
		zap.Int("tokens", len(normalizedTokens)),
		zap.Float64("entropy", entropy),
	)

	return nil
}

// UpdateFile updates an existing file in the corpus
func (cm *CorpusManager) UpdateFile(ctx context.Context, filePath string, source []byte, language string) error {
	cm.mu.RLock()
	existingModel, exists := cm.fileModels[filePath]
	cm.mu.RUnlock()

	if !exists {
		return cm.AddFile(ctx, filePath, source, language)
	}

	// Get the appropriate tokenizer
	tok, ok := cm.tokenizer.GetTokenizer(language)
	if !ok {
		return fmt.Errorf("no tokenizer found for language: %s", language)
	}

	// Tokenize the new source
	tokenSeq, err := tok.Tokenize(ctx, source)
	if err != nil {
		return fmt.Errorf("tokenization failed: %w", err)
	}

	// Normalize tokens
	normalizedTokens := make([]string, 0, len(tokenSeq))
	for _, token := range tokenSeq {
		normalized := tok.Normalize(token)
		normalizedTokens = append(normalizedTokens, normalized)
	}

	// Create new file model
	var entropy float64
	fm := &FileModel{
		FilePath:     filePath,
		Language:     language,
		TokenCount:   len(normalizedTokens),
		LastModified: time.Now(),
	}

	if cm.useTrie {
		var newFileModel *NGramModelTrie
		if cm.useBloom {
			newFileModel = NewNGramModelTrieWithBloom(cm.n, cm.smoother, true, 10000, 0.01)
		} else {
			newFileModel = NewNGramModelTrie(cm.n, cm.smoother)
		}
		newFileModel.Add(normalizedTokens)
		entropy = newFileModel.CrossEntropy(normalizedTokens)
		fm.TrieModel = newFileModel

		// Update global model (simplified merge)
		cm.globalTrieModel.Add(normalizedTokens)
	} else {
		newFileModel := NewNGramModel(cm.n, cm.smoother)
		newFileModel.Add(normalizedTokens)
		entropy = newFileModel.CrossEntropy(normalizedTokens)
		fm.Model = newFileModel

		// Update global model
		cm.globalModel.Merge(newFileModel)
	}

	fm.Entropy = entropy

	// Update file model
	cm.mu.Lock()
	cm.fileModels[filePath] = fm
	cm.mu.Unlock()

	cm.logger.Debug("Updated file in corpus",
		zap.String("path", filePath),
		zap.String("language", language),
		zap.Int("old_tokens", existingModel.TokenCount),
		zap.Int("new_tokens", len(normalizedTokens)),
		zap.Float64("old_entropy", existingModel.Entropy),
		zap.Float64("new_entropy", entropy),
	)

	return nil
}

// RemoveFile removes a file from the corpus
func (cm *CorpusManager) RemoveFile(ctx context.Context, filePath string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	fileModel, exists := cm.fileModels[filePath]
	if !exists {
		return fmt.Errorf("file not found in corpus: %s", filePath)
	}

	// Note: Removing from global model is complex without tracking
	// In a production system, we'd need better bookkeeping
	delete(cm.fileModels, filePath)

	cm.logger.Debug("Removed file from corpus",
		zap.String("path", filePath),
	)

	// Suppress unused variable warning
	_ = fileModel

	return nil
}

// GetFileEntropy returns the entropy for a specific file
func (cm *CorpusManager) GetFileEntropy(ctx context.Context, filePath string) (float64, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	fileModel, exists := cm.fileModels[filePath]
	if !exists {
		return 0, fmt.Errorf("file not found in corpus: %s", filePath)
	}

	return fileModel.Entropy, nil
}

// GetFilePerplexity returns the perplexity for a specific file
func (cm *CorpusManager) GetFilePerplexity(ctx context.Context, filePath string) (float64, error) {
	_, err := cm.GetFileEntropy(ctx, filePath)
	if err != nil {
		return 0, err
	}
	return cm.globalModel.Perplexity([]string{}), nil // Simplified
}

// GetGlobalEntropy returns the entropy of the global model
func (cm *CorpusManager) GetGlobalEntropy(ctx context.Context) float64 {
	// For global entropy, we'd ideally compute across all files
	// For now, return an aggregate metric
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if len(cm.fileModels) == 0 {
		return 0.0
	}

	totalEntropy := 0.0
	for _, fm := range cm.fileModels {
		totalEntropy += fm.Entropy
	}

	return totalEntropy / float64(len(cm.fileModels))
}

// GetFileModel returns the n-gram model for a specific file
func (cm *CorpusManager) GetFileModel(ctx context.Context, filePath string) (*FileModel, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	fileModel, exists := cm.fileModels[filePath]
	if !exists {
		return nil, fmt.Errorf("file not found in corpus: %s", filePath)
	}

	return fileModel, nil
}

// GetGlobalModel returns the global n-gram model
func (cm *CorpusManager) GetGlobalModel() *NGramModel {
	return cm.globalModel
}

// GetStats returns statistics about the corpus
func (cm *CorpusManager) GetStats(ctx context.Context) CorpusStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	languageCounts := make(map[string]int)
	totalTokens := 0
	entropies := make([]float64, 0, len(cm.fileModels))

	for _, fm := range cm.fileModels {
		languageCounts[fm.Language]++
		totalTokens += fm.TokenCount
		entropies = append(entropies, fm.Entropy)
	}

	var globalModelStats ModelStats
	if cm.useTrie {
		globalModelStats = cm.globalTrieModel.Stats()
	} else {
		globalModelStats = cm.globalModel.Stats()
	}

	// Calculate entropy statistics
	entropyStats := calculateEntropyStatistics(entropies)

	return CorpusStats{
		TotalFiles:     len(cm.fileModels),
		TotalTokens:    totalTokens,
		LanguageCounts: languageCounts,
		GlobalModel:    globalModelStats,
		AverageEntropy: entropyStats.Mean,
		EntropyStdDev:  entropyStats.StdDev,
		EntropyMin:     entropyStats.Min,
		EntropyMax:     entropyStats.Max,
	}
}

// GetMemoryStats returns memory usage statistics (for trie-based models)
func (cm *CorpusManager) GetMemoryStats() *TrieModelMemoryStats {
	if !cm.useTrie || cm.globalTrieModel == nil {
		return nil
	}
	stats := cm.globalTrieModel.MemoryStats()
	return &stats
}

// PruneGlobalModel prunes low-frequency n-grams from the global model
func (cm *CorpusManager) PruneGlobalModel(minCount int64) (int64, int64) {
	if cm.useTrie && cm.globalTrieModel != nil {
		return cm.globalTrieModel.Prune(minCount)
	}
	return 0, 0
}

// ListFiles returns a list of all files in the corpus
func (cm *CorpusManager) ListFiles(ctx context.Context) []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	files := make([]string, 0, len(cm.fileModels))
	for path := range cm.fileModels {
		files = append(files, path)
	}

	return files
}

// CorpusStats contains statistics about the entire corpus
type CorpusStats struct {
	TotalFiles         int            `json:"total_files"`
	TotalTokens        int            `json:"total_tokens"`
	LanguageCounts     map[string]int `json:"language_counts"`
	GlobalModel        ModelStats     `json:"global_model"`
	AverageEntropy     float64        `json:"average_entropy"`
	EntropyStdDev      float64        `json:"entropy_std_dev"`       // Standard deviation of file entropies
	EntropyMin         float64        `json:"entropy_min"`           // Minimum file entropy
	EntropyMax         float64        `json:"entropy_max"`           // Maximum file entropy
}

// EntropyStats contains detailed entropy statistics for z-score calculation
type EntropyStats struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Count  int     `json:"count"`
}

// GetEntropyStats returns entropy statistics for z-score calculation
func (cm *CorpusManager) GetEntropyStats(ctx context.Context) EntropyStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	entropies := make([]float64, 0, len(cm.fileModels))
	for _, fm := range cm.fileModels {
		entropies = append(entropies, fm.Entropy)
	}

	return calculateEntropyStatistics(entropies)
}

// CalculateZScore calculates the z-score for a given entropy value
// Z-score = (entropy - mean) / stddev
// Higher z-score indicates more unusual/buggy code
func (cm *CorpusManager) CalculateZScore(ctx context.Context, entropy float64) float64 {
	stats := cm.GetEntropyStats(ctx)

	if stats.StdDev == 0 {
		return 0 // Avoid division by zero
	}

	return (entropy - stats.Mean) / stats.StdDev
}

// calculateEntropyStatistics computes mean, stddev, min, max from entropy values
func calculateEntropyStatistics(entropies []float64) EntropyStats {
	if len(entropies) == 0 {
		return EntropyStats{}
	}

	// Calculate mean
	sum := 0.0
	min := entropies[0]
	max := entropies[0]

	for _, e := range entropies {
		sum += e
		if e < min {
			min = e
		}
		if e > max {
			max = e
		}
	}

	mean := sum / float64(len(entropies))

	// Calculate standard deviation
	varianceSum := 0.0
	for _, e := range entropies {
		diff := e - mean
		varianceSum += diff * diff
	}

	variance := varianceSum / float64(len(entropies))
	stddev := 0.0
	if variance > 0 {
		stddev = 1.0
		// Newton's method for square root
		for i := 0; i < 10; i++ {
			stddev = (stddev + variance/stddev) / 2
		}
	}

	return EntropyStats{
		Mean:   mean,
		StdDev: stddev,
		Min:    min,
		Max:    max,
		Count:  len(entropies),
	}
}
