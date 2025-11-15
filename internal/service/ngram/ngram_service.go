package ngram

import (
	"bot-go/internal/config"
	"bot-go/internal/service/tokenizer"
	"bot-go/internal/util"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// NGramService orchestrates n-gram model building for repositories
type NGramService struct {
	corpusManagers map[string]*CorpusManager // repo name -> corpus manager
	registry       *tokenizer.TokenizerRegistry
	persistence    *NGramPersistence // Model persistence
	logger         *zap.Logger
	mu             sync.RWMutex
}

// NewNGramService creates a new n-gram service with default output directory
func NewNGramService(logger *zap.Logger) (*NGramService, error) {
	return NewNGramServiceWithOutputDir("./ngram_models", logger)
}

// NewNGramServiceWithOutputDir creates a new n-gram service with custom output directory
func NewNGramServiceWithOutputDir(outputDir string, logger *zap.Logger) (*NGramService, error) {
	registry := tokenizer.NewTokenizerRegistry()

	// Register all tokenizers
	goTokenizer, err := tokenizer.NewGoTokenizer()
	if err != nil {
		return nil, fmt.Errorf("failed to create Go tokenizer: %w", err)
	}
	registry.Register("go", goTokenizer, []string{".go"})

	pythonTokenizer, err := tokenizer.NewPythonTokenizer()
	if err != nil {
		return nil, fmt.Errorf("failed to create Python tokenizer: %w", err)
	}
	registry.Register("python", pythonTokenizer, []string{".py", ".pyw"})

	jsTokenizer, err := tokenizer.NewJavaScriptTokenizer()
	if err != nil {
		return nil, fmt.Errorf("failed to create JavaScript tokenizer: %w", err)
	}
	registry.Register("javascript", jsTokenizer, []string{".js", ".jsx", ".mjs"})

	tsTokenizer, err := tokenizer.NewTypeScriptTokenizer()
	if err != nil {
		return nil, fmt.Errorf("failed to create TypeScript tokenizer: %w", err)
	}
	registry.Register("typescript", tsTokenizer, []string{".ts", ".tsx"})

	javaTokenizer, err := tokenizer.NewJavaTokenizer()
	if err != nil {
		return nil, fmt.Errorf("failed to create Java tokenizer: %w", err)
	}
	registry.Register("java", javaTokenizer, []string{".java"})

	// Initialize persistence
	persistence, err := NewNGramPersistence(outputDir, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistence: %w", err)
	}

	return &NGramService{
		corpusManagers: make(map[string]*CorpusManager),
		registry:       registry,
		persistence:    persistence,
		logger:         logger,
	}, nil
}

// ProcessRepository processes all files in a repository and builds n-gram models
func (ns *NGramService) ProcessRepository(ctx context.Context, repo *config.Repository, n int, override bool) error {
	ns.logger.Info("Processing repository for n-gram model",
		zap.String("repo", repo.Name),
		zap.String("path", repo.Path),
		zap.Int("n", n),
		zap.Bool("override", override),
	)

	// Check if we should load from disk
	if !override && ns.persistence.ModelExists(repo.Name) {
		ns.logger.Info("Loading existing n-gram model from disk",
			zap.String("repo", repo.Name))

		corpusManager, err := ns.persistence.LoadCorpusManager(repo.Name, ns.registry, ns.logger)
		if err == nil {
			ns.mu.Lock()
			ns.corpusManagers[repo.Name] = corpusManager
			ns.mu.Unlock()

			ns.logger.Info("Successfully loaded n-gram model from disk",
				zap.String("repo", repo.Name))
			return nil
		}

		ns.logger.Warn("Failed to load existing model, will rebuild",
			zap.String("repo", repo.Name),
			zap.Error(err))
	}

	// Create new corpus manager (always Trie+Bloom)
	ns.mu.Lock()
	smoother := NewAddKSmoother(1.0)
	corpusManager := NewCorpusManager(n, smoother, ns.registry, ns.logger)
	ns.corpusManagers[repo.Name] = corpusManager
	ns.mu.Unlock()

	// Walk the repository directory using concurrent walker
	fileCount := 0
	var mu sync.Mutex

	err := util.WalkDirTree(repo.Path,
		// Walk function - called for each file
		func(path string, err error) error {
			if err != nil {
				return err
			}

			// Check if file should be processed
			if !ns.shouldProcessFile(path, repo) {
				return nil
			}

			// Detect language
			language := ns.detectLanguage(path)
			if language == "" {
				return nil
			}

			// Read file
			source, err := ns.readFile(path)
			if err != nil {
				ns.logger.Warn("Failed to read file",
					zap.String("path", path),
					zap.Error(err),
				)
				return nil
			}

			// Add file to corpus
			err = corpusManager.AddFile(ctx, path, source, language)
			if err != nil {
				ns.logger.Warn("Failed to process file",
					zap.String("path", path),
					zap.Error(err),
				)
				return nil
			}

			mu.Lock()
			fileCount++
			currentCount := fileCount
			mu.Unlock()

			if currentCount%100 == 0 {
				ns.logger.Info("Processing progress",
					zap.String("repo", repo.Name),
					zap.Int("files", currentCount),
				)
			}

			return nil
		},
		// Skip function - called to determine if path should be skipped
		func(path string, isDir bool) bool {
			if isDir {
				// Skip common ignored directories
				dirName := filepath.Base(path)
				return ns.shouldSkipDirectory(dirName)
			}
			return false
		},
		ns.logger,
		0, // gcThreshold: 0 = disabled
		2, // numThreads: use 2 workers
	)

	if err != nil {
		return fmt.Errorf("failed to walk repository: %w", err)
	}

	stats := corpusManager.GetStats(ctx)
	ns.logger.Info("Repository processing complete",
		zap.String("repo", repo.Name),
		zap.Int("files_processed", fileCount),
		zap.Int("total_tokens", stats.TotalTokens),
		zap.Float64("avg_entropy", stats.AverageEntropy),
	)

	// Save the model to disk
	if err := ns.persistence.SaveCorpusManager(corpusManager, repo.Name); err != nil {
		ns.logger.Error("Failed to save n-gram model",
			zap.String("repo", repo.Name),
			zap.Error(err))
		return fmt.Errorf("failed to save model: %w", err)
	}

	return nil
}

// GetCorpusManager returns the corpus manager for a repository
func (ns *NGramService) GetCorpusManager(repoName string) (*CorpusManager, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	cm, exists := ns.corpusManagers[repoName]
	if !exists {
		return nil, fmt.Errorf("no corpus manager found for repository: %s", repoName)
	}

	return cm, nil
}

// GetFileEntropy returns the entropy for a specific file
func (ns *NGramService) GetFileEntropy(ctx context.Context, repoName, filePath string) (float64, error) {
	cm, err := ns.GetCorpusManager(repoName)
	if err != nil {
		return 0, err
	}

	return cm.GetFileEntropy(ctx, filePath)
}

// GetRepositoryStats returns statistics for a repository
func (ns *NGramService) GetRepositoryStats(ctx context.Context, repoName string) (*CorpusStats, error) {
	cm, err := ns.GetCorpusManager(repoName)
	if err != nil {
		return nil, err
	}

	stats := cm.GetStats(ctx)
	return &stats, nil
}

// AnalyzeCode analyzes a code snippet and returns its entropy/naturalness
func (ns *NGramService) AnalyzeCode(ctx context.Context, repoName, language string, code []byte) (*CodeAnalysis, error) {
	cm, err := ns.GetCorpusManager(repoName)
	if err != nil {
		return nil, err
	}

	// Get tokenizer for language
	tokenizer, ok := ns.registry.GetTokenizer(language)
	if !ok {
		return nil, fmt.Errorf("no tokenizer found for language: %s", language)
	}

	// Tokenize code
	tokens, err := tokenizer.Tokenize(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	// Normalize tokens
	normalizedTokens := make([]string, 0, len(tokens))
	for _, token := range tokens {
		normalized := tokenizer.Normalize(token)
		normalizedTokens = append(normalizedTokens, normalized)
	}

	// Calculate entropy and perplexity using global model
	globalModel := cm.GetGlobalModel()
	entropy := globalModel.CrossEntropy(normalizedTokens)
	perplexity := globalModel.Perplexity(normalizedTokens)

	return &CodeAnalysis{
		TokenCount: len(normalizedTokens),
		Entropy:    entropy,
		Perplexity: perplexity,
		Language:   language,
	}, nil
}

// CalculateZScore analyzes code and calculates z-score with detailed n-gram information
func (ns *NGramService) CalculateZScore(ctx context.Context, repoName, language string, code []byte) (*ZScoreAnalysis, error) {
	cm, err := ns.GetCorpusManager(repoName)
	if err != nil {
		return nil, err
	}

	// Get tokenizer for language
	tokenizer, ok := ns.registry.GetTokenizer(language)
	if !ok {
		return nil, fmt.Errorf("no tokenizer found for language: %s", language)
	}

	// Tokenize code
	tokens, err := tokenizer.Tokenize(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	// Normalize tokens
	normalizedTokens := make([]string, 0, len(tokens))
	for _, token := range tokens {
		normalized := tokenizer.Normalize(token)
		normalizedTokens = append(normalizedTokens, normalized)
	}

	// Calculate entropy and scores (always Trie+Bloom)
	entropy, ngramScores := ns.calculateEntropyWithScores(normalizedTokens, cm.globalModel, cm.n)

	// Calculate z-score
	zScore := cm.CalculateZScore(ctx, entropy)

	// Get entropy statistics
	entropyStats := cm.GetEntropyStats(ctx)

	// Interpret z-score
	interpretation := interpretZScore(zScore)

	return &ZScoreAnalysis{
		TokenCount:     len(normalizedTokens),
		Entropy:        entropy,
		ZScore:         zScore,
		EntropyStats:   entropyStats,
		NGramScores:    ngramScores,
		Interpretation: interpretation,
	}, nil
}

// calculateEntropyWithScores calculates entropy and returns individual n-gram scores (trie-based)
func (ns *NGramService) calculateEntropyWithScores(tokens []string, model *NGramModelTrie, n int) (float64, []NGramScoreDetail) {
	if len(tokens) < n {
		return 0, []NGramScoreDetail{}
	}

	totalEntropy := 0.0
	ngramScores := make([]NGramScoreDetail, 0, len(tokens)-n+1)

	for i := 0; i <= len(tokens)-n; i++ {
		ngram := tokens[i : i+n]
		// Split into context and token
		context := ngram[:n-1]
		token := ngram[n-1]
		prob := model.Probability(token, context)
		logProb := 0.0
		if prob > 0 {
			logProb = -1.0 * log2(prob)
		} else {
			logProb = 20.0 // High value for zero probability
		}

		totalEntropy += logProb

		ngramScores = append(ngramScores, NGramScoreDetail{
			NGram:       ngram,
			Probability: prob,
			LogProb:     logProb,
			Entropy:     logProb,
		})
	}

	avgEntropy := totalEntropy / float64(len(tokens))
	return avgEntropy, ngramScores
}

// log2 calculates log base 2
func log2(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// log2(x) = ln(x) / ln(2)
	ln2 := 0.693147180559945309417232121458
	lnX := 0.0

	// Natural log using series expansion (for x near 1)
	if x > 0.5 && x < 1.5 {
		y := x - 1.0
		lnX = y - y*y/2 + y*y*y/3 - y*y*y*y/4 + y*y*y*y*y/5
	} else {
		// Use approximation for other values
		for x >= 2.0 {
			x /= 2.0
			lnX += ln2
		}
		for x < 1.0 {
			x *= 2.0
			lnX -= ln2
		}
		y := x - 1.0
		lnX += y - y*y/2 + y*y*y/3 - y*y*y*y/4
	}

	return lnX / ln2
}

// interpretZScore provides human-readable interpretation of z-score
func interpretZScore(zScore float64) ZScoreInterpretation {
	var level, description string
	var percentile float64

	if zScore < -2.0 {
		level = "very_low"
		description = "Extremely typical code - simpler than 97.5% of corpus"
		percentile = 2.5
	} else if zScore < -1.0 {
		level = "low"
		description = "More typical than average - simpler than 84% of corpus"
		percentile = 16.0
	} else if zScore <= 1.0 {
		level = "normal"
		description = "Normal entropy - within 1 standard deviation of mean"
		percentile = 50.0
	} else if zScore <= 2.0 {
		level = "high"
		description = "Unusual code - more complex than 84% of corpus"
		percentile = 84.0
	} else {
		level = "very_high"
		description = "Highly unusual code - more complex than 97.5% of corpus (potential bug indicator)"
		percentile = 97.5
	}

	return ZScoreInterpretation{
		Level:       level,
		Description: description,
		Percentile:  percentile,
	}
}

// Helper functions

func (ns *NGramService) shouldSkipDirectory(dirName string) bool {
	skipDirs := []string{
		".git", "node_modules", ".vscode", ".idea", "vendor", "target",
		"build", "dist", "__pycache__", ".pytest_cache", "coverage",
		"site-packages", ".next", ".nuxt", "venv", "env",
	}

	for _, skip := range skipDirs {
		if dirName == skip {
			return true
		}
	}

	return false
}

func (ns *NGramService) shouldProcessFile(filePath string, repo *config.Repository) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return false
	}

	// Check if we have a tokenizer for this extension
	_, ok := ns.registry.GetTokenizerByExtension(ext)
	return ok
}

func (ns *NGramService) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return "go"
	case ".py", ".pyw":
		return "python"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	default:
		return ""
	}
}

func (ns *NGramService) readFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// CodeAnalysis contains the analysis results for a code snippet
type CodeAnalysis struct {
	TokenCount int     `json:"token_count"`
	Entropy    float64 `json:"entropy"`
	Perplexity float64 `json:"perplexity"`
	Language   string  `json:"language"`
}

// ZScoreAnalysis contains z-score analysis results
type ZScoreAnalysis struct {
	TokenCount     int                  `json:"token_count"`
	Entropy        float64              `json:"entropy"`
	ZScore         float64              `json:"z_score"`
	EntropyStats   EntropyStats         `json:"entropy_stats"`
	NGramScores    []NGramScoreDetail   `json:"ngram_scores"`
	Interpretation ZScoreInterpretation `json:"interpretation"`
}

// NGramScoreDetail contains detailed information about a single n-gram
type NGramScoreDetail struct {
	NGram       []string `json:"ngram"`
	Probability float64  `json:"probability"`
	LogProb     float64  `json:"log_prob"`
	Entropy     float64  `json:"entropy"`
}

// ZScoreInterpretation provides human-readable interpretation of z-score
type ZScoreInterpretation struct {
	Level       string  `json:"level"` // "very_low", "low", "normal", "high", "very_high"
	Description string  `json:"description"`
	Percentile  float64 `json:"percentile"` // Approximate percentile in corpus
}
