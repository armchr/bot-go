package controller

import (
	"bot-go/internal/service/ngram"
	"bot-go/internal/service/vector"
	"fmt"
	"net/http"

	"bot-go/internal/model"
	"bot-go/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RepoController struct {
	repoService  *service.RepoService
	chunkService *vector.CodeChunkService
	ngramService *ngram.NGramService
	logger       *zap.Logger
}

func NewRepoController(repoService *service.RepoService, chunkService *vector.CodeChunkService, ngramService *ngram.NGramService, logger *zap.Logger) *RepoController {
	return &RepoController{
		repoService:  repoService,
		chunkService: chunkService,
		ngramService: ngramService,
		logger:       logger,
	}
}

type ProcessRepoRequest struct {
	RepoName string `json:"repo_name" binding:"required"`
}

func (rc *RepoController) ProcessRepo(c *gin.Context) {
	var request ProcessRepoRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	rc.logger.Info("Processing repository", zap.String("repo_name", request.RepoName))

	/*response, err := rc.repoService.ProcessRepository(request.RepoName)
	if err != nil {
		rc.logger.Error("Failed to process repository",
			zap.String("repo_name", request.RepoName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to process repository",
			"details": err.Error(),
		})
		return
	}

	rc.logger.Info("Successfully processed repository",
		zap.String("repo_name", request.RepoName),
		zap.Int("files_count", len(response.Files)),
		zap.Int("functions_count", len(response.Functions)))

	rc.logger.Debug("About to send JSON response")
	c.JSON(http.StatusOK, response)
	*/
	c.JSON(http.StatusOK, nil)
	rc.logger.Debug("JSON response sent successfully")
}

func (rc *RepoController) GetFunctionsInFile(c *gin.Context) {
	var request model.GetFunctionsInFileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	rc.logger.Info("Getting functions in file",
		zap.String("repo_name", request.RepoName),
		zap.String("relative_path", request.RelativePath))

	/*response, err := rc.repoService.GetFunctionsInFile(request.RepoName, request.RelativePath)
	if err != nil {
		rc.logger.Error("Failed to get functions in file",
			zap.String("repo_name", request.RepoName),
			zap.String("relative_path", request.RelativePath),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get functions in file",
			"details": err.Error(),
		})
		return
	}

	rc.logger.Info("Successfully got functions in file",
		zap.String("repo_name", request.RepoName),
		zap.String("relative_path", request.RelativePath),
		zap.Int("function_count", len(response.Functions)))

	rc.logger.Debug("About to send JSON response")
	c.JSON(http.StatusOK, response)
	*/
	c.JSON(http.StatusOK, nil)
	rc.logger.Debug("JSON response sent successfully")
}

func (rc *RepoController) GetFunctionDetails(c *gin.Context) {
	var request model.GetFunctionDetailsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	rc.logger.Info("Getting function details",
		zap.String("repo_name", request.RepoName),
		zap.String("relative_path", request.RelativePath),
		zap.String("function_name", request.FunctionName))

	response, err := rc.repoService.GetFunctionDetails(request.RepoName, request.RelativePath, request.FunctionName)
	if err != nil {
		rc.logger.Error("Failed to get function details",
			zap.String("repo_name", request.RepoName),
			zap.String("relative_path", request.RelativePath),
			zap.String("function_name", request.FunctionName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get function details",
			"details": err.Error(),
		})
		return
	}

	rc.logger.Info("Successfully got function details",
		zap.String("repo_name", request.RepoName),
		zap.String("relative_path", request.RelativePath),
		zap.String("function_name", request.FunctionName))

	rc.logger.Debug("About to send JSON response")
	c.JSON(http.StatusOK, response)
	rc.logger.Debug("JSON response sent successfully")
}

func (rc *RepoController) GetFunctionDependencies(c *gin.Context) {
	request := model.GetFunctionDependenciesRequest{
		Depth: 2, // Default depth
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	rc.logger.Info("Getting function dependencies",
		zap.String("repo_name", request.RepoName),
		zap.String("relative_path", request.RelativePath),
		zap.String("function_name", request.FunctionName),
		zap.Int("depth", request.Depth))

	response, err := rc.repoService.GetFunctionDependencies(c, request.RepoName, request.RelativePath, request.FunctionName, request.Depth)
	if err != nil {
		rc.logger.Error("Failed to get function dependencies",
			zap.String("repo_name", request.RepoName),
			zap.String("relative_path", request.RelativePath),
			zap.String("function_name", request.FunctionName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get function dependencies",
			"details": err.Error(),
		})
		return
	}

	rc.logger.Info("Successfully got function dependencies",
		zap.String("repo_name", request.RepoName),
		zap.String("relative_path", request.RelativePath),
		zap.String("function_name", request.FunctionName))

	rc.logger.Debug("About to send JSON response")
	c.JSON(http.StatusOK, response)
	rc.logger.Debug("JSON response sent successfully")
}

func (rc *RepoController) ProcessDirectory(c *gin.Context) {
	var request model.ProcessDirectoryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Check if chunk service is available
	if rc.chunkService == nil {
		rc.logger.Error("Code chunk service not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Code chunk service not available",
		})
		return
	}

	// Get repository configuration
	repo, err := rc.repoService.GetConfig().GetRepository(request.RepoName)
	if err != nil {
		rc.logger.Error("Repository not found",
			zap.String("repo_name", request.RepoName),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Repository not found",
			"details": err.Error(),
		})
		return
	}

	// Use repo name as collection name if not provided
	collectionName := request.CollectionName
	if collectionName == "" {
		collectionName = request.RepoName
	}

	rc.logger.Info("Processing directory for code chunking",
		zap.String("repo_name", request.RepoName),
		zap.String("path", repo.Path),
		zap.String("collection", collectionName))

	// Create collection if it doesn't exist
	if err := rc.chunkService.CreateCollection(c.Request.Context(), collectionName); err != nil {
		rc.logger.Error("Failed to create collection",
			zap.String("collection", collectionName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create collection",
			"details": err.Error(),
		})
		return
	}

	// Process directory with repository configuration
	totalChunks, err := rc.chunkService.ProcessDirectory(c.Request.Context(), repo.Path, collectionName, repo)
	if err != nil {
		rc.logger.Error("Failed to process directory",
			zap.String("repo_name", request.RepoName),
			zap.String("path", repo.Path),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.ProcessDirectoryResponse{
			RepoName:       request.RepoName,
			CollectionName: collectionName,
			TotalChunks:    totalChunks,
			Success:        false,
			Message:        fmt.Sprintf("Failed to process directory: %v", err),
		})
		return
	}

	rc.logger.Info("Successfully processed directory",
		zap.String("repo_name", request.RepoName),
		zap.String("collection", collectionName),
		zap.Int("total_chunks", totalChunks))

	response := model.ProcessDirectoryResponse{
		RepoName:       request.RepoName,
		CollectionName: collectionName,
		TotalChunks:    totalChunks,
		Success:        true,
		Message:        "Directory processed successfully",
	}

	c.JSON(http.StatusOK, response)
}

// SearchSimilarCode handles searching for similar code using a code snippet
func (rc *RepoController) SearchSimilarCode(c *gin.Context) {
	var request model.SearchSimilarCodeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	// Check if chunk service is available
	if rc.chunkService == nil {
		rc.logger.Error("Code chunk service not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Code chunk service not available",
		})
		return
	}

	// Validate language
	validLanguages := map[string]bool{
		"go":         true,
		"python":     true,
		"java":       true,
		"javascript": true,
		"typescript": true,
	}
	if !validLanguages[request.Language] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported language. Supported: go, python, java, javascript, typescript",
		})
		return
	}

	// Use repo name as collection name if not provided
	collectionName := request.CollectionName
	if collectionName == "" {
		collectionName = request.RepoName
	}

	// Set default limit
	limit := request.Limit
	if limit <= 0 {
		limit = 10
	}

	rc.logger.Info("Searching for similar code",
		zap.String("repo_name", request.RepoName),
		zap.String("collection", collectionName),
		zap.String("language", request.Language),
		zap.Int("limit", limit))

	// Search for similar code
	queryChunks, resultChunks, scores, queryChunkIndices, err := rc.chunkService.SearchSimilarCodeBySnippet(
		c.Request.Context(),
		collectionName,
		request.CodeSnippet,
		request.Language,
		limit,
		nil, // no filter
	)
	if err != nil {
		rc.logger.Error("Failed to search for similar code",
			zap.String("repo_name", request.RepoName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.SearchSimilarCodeResponse{
			RepoName:       request.RepoName,
			CollectionName: collectionName,
			Query: model.QueryInfo{
				CodeSnippet: request.CodeSnippet,
				Language:    request.Language,
				ChunksFound: 0,
			},
			Results: []model.SimilarCodeResult{},
			Success: false,
			Message: fmt.Sprintf("Failed to search: %v", err),
		})
		return
	}

	// Build results
	results := make([]model.SimilarCodeResult, len(resultChunks))
	for i, chunk := range resultChunks {
		result := model.SimilarCodeResult{
			Chunk:           chunk,
			Score:           scores[i],
			QueryChunkIndex: queryChunkIndices[i],
		}

		// Fetch code from file if requested
		if request.IncludeCode {
			code, err := rc.chunkService.ReadCodeFromFile(chunk.FilePath, chunk.StartLine, chunk.EndLine)
			if err != nil {
				rc.logger.Warn("Failed to read code from file",
					zap.String("file", chunk.FilePath),
					zap.Int("start_line", chunk.StartLine),
					zap.Int("end_line", chunk.EndLine),
					zap.Error(err))
				// Continue without code rather than failing the entire request
			} else {
				result.Code = code
			}
		}

		results[i] = result
	}

	rc.logger.Info("Successfully found similar code",
		zap.String("repo_name", request.RepoName),
		zap.String("collection", collectionName),
		zap.Int("query_chunks", len(queryChunks)),
		zap.Int("results", len(results)),
		zap.Bool("include_code", request.IncludeCode))

	response := model.SearchSimilarCodeResponse{
		RepoName:       request.RepoName,
		CollectionName: collectionName,
		Query: model.QueryInfo{
			CodeSnippet: request.CodeSnippet,
			Language:    request.Language,
			ChunksFound: len(queryChunks),
			Chunks:      queryChunks,
		},
		Results: results,
		Success: true,
		Message: "Search completed successfully",
	}

	c.JSON(http.StatusOK, response)
}

// ProcessNGram processes a repository and builds n-gram models
func (rc *RepoController) ProcessNGram(c *gin.Context) {
	var request model.ProcessNGramRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Check if n-gram service is available
	if rc.ngramService == nil {
		rc.logger.Error("N-gram service not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "N-gram service not available",
		})
		return
	}

	// Get repository configuration
	repo, err := rc.repoService.GetConfig().GetRepository(request.RepoName)
	if err != nil {
		rc.logger.Error("Repository not found",
			zap.String("repo_name", request.RepoName),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Repository not found",
			"details": err.Error(),
		})
		return
	}

	// Default n to 3 (trigrams) if not specified
	n := request.N
	if n <= 0 {
		n = 3
	}

	rc.logger.Info("Processing repository for n-gram model",
		zap.String("repo_name", request.RepoName),
		zap.String("path", repo.Path),
		zap.Int("n", n))

	// Process repository
	if err := rc.ngramService.ProcessRepository(c.Request.Context(), repo, n, request.Override); err != nil {
		rc.logger.Error("Failed to process repository for n-gram",
			zap.String("repo_name", request.RepoName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.ProcessNGramResponse{
			RepoName: request.RepoName,
			N:        n,
			Success:  false,
			Message:  fmt.Sprintf("Failed to process repository: %v", err),
		})
		return
	}

	// Get statistics
	stats, err := rc.ngramService.GetRepositoryStats(c.Request.Context(), request.RepoName)
	if err != nil {
		rc.logger.Error("Failed to get repository stats",
			zap.String("repo_name", request.RepoName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.ProcessNGramResponse{
			RepoName: request.RepoName,
			N:        n,
			Success:  false,
			Message:  fmt.Sprintf("Failed to get stats: %v", err),
		})
		return
	}

	rc.logger.Info("Successfully processed repository for n-gram",
		zap.String("repo_name", request.RepoName),
		zap.Int("n", n),
		zap.Int("files", stats.TotalFiles),
		zap.Int("tokens", stats.TotalTokens))

	response := model.ProcessNGramResponse{
		RepoName:       request.RepoName,
		N:              n,
		TotalFiles:     stats.TotalFiles,
		TotalTokens:    stats.TotalTokens,
		VocabularySize: stats.GlobalModel.VocabularySize,
		AverageEntropy: stats.AverageEntropy,
		Success:        true,
		Message:        "Repository processed successfully",
	}

	c.JSON(http.StatusOK, response)
}

// GetNGramStats returns statistics for a repository's n-gram model
func (rc *RepoController) GetNGramStats(c *gin.Context) {
	var request model.GetNGramStatsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Check if n-gram service is available
	if rc.ngramService == nil {
		rc.logger.Error("N-gram service not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "N-gram service not available",
		})
		return
	}

	// Get statistics
	stats, err := rc.ngramService.GetRepositoryStats(c.Request.Context(), request.RepoName)
	if err != nil {
		rc.logger.Error("Failed to get repository stats",
			zap.String("repo_name", request.RepoName),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Repository not found or not processed",
			"details": err.Error(),
		})
		return
	}

	response := model.GetNGramStatsResponse{
		RepoName:       request.RepoName,
		N:              stats.GlobalModel.N,
		TotalFiles:     stats.TotalFiles,
		TotalTokens:    stats.TotalTokens,
		VocabularySize: stats.GlobalModel.VocabularySize,
		NGramCount:     stats.GlobalModel.NGramCount,
		AverageEntropy: stats.AverageEntropy,
		LanguageCounts: stats.LanguageCounts,
	}

	c.JSON(http.StatusOK, response)
}

// GetFileEntropy returns the entropy for a specific file
func (rc *RepoController) GetFileEntropy(c *gin.Context) {
	var request model.GetFileEntropyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Check if n-gram service is available
	if rc.ngramService == nil {
		rc.logger.Error("N-gram service not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "N-gram service not available",
		})
		return
	}

	// Get file entropy
	entropy, err := rc.ngramService.GetFileEntropy(c.Request.Context(), request.RepoName, request.FilePath)
	if err != nil {
		rc.logger.Error("Failed to get file entropy",
			zap.String("repo_name", request.RepoName),
			zap.String("file_path", request.FilePath),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "File not found or not processed",
			"details": err.Error(),
		})
		return
	}

	response := model.GetFileEntropyResponse{
		RepoName: request.RepoName,
		FilePath: request.FilePath,
		Entropy:  entropy,
	}

	c.JSON(http.StatusOK, response)
}

// AnalyzeCode analyzes a code snippet and returns naturalness metrics
func (rc *RepoController) AnalyzeCode(c *gin.Context) {
	var request model.AnalyzeCodeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Check if n-gram service is available
	if rc.ngramService == nil {
		rc.logger.Error("N-gram service not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "N-gram service not available",
		})
		return
	}

	// Validate language
	validLanguages := map[string]bool{
		"go":         true,
		"python":     true,
		"java":       true,
		"javascript": true,
		"typescript": true,
	}
	if !validLanguages[request.Language] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported language. Supported: go, python, java, javascript, typescript",
		})
		return
	}

	// Analyze code
	analysis, err := rc.ngramService.AnalyzeCode(
		c.Request.Context(),
		request.RepoName,
		request.Language,
		[]byte(request.Code),
	)
	if err != nil {
		rc.logger.Error("Failed to analyze code",
			zap.String("repo_name", request.RepoName),
			zap.String("language", request.Language),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to analyze code",
			"details": err.Error(),
		})
		return
	}

	response := model.AnalyzeCodeResponse{
		RepoName:   request.RepoName,
		Language:   request.Language,
		TokenCount: analysis.TokenCount,
		Entropy:    analysis.Entropy,
		Perplexity: analysis.Perplexity,
	}

	c.JSON(http.StatusOK, response)
}

// CalculateZScore calculates z-score for a code snippet
func (rc *RepoController) CalculateZScore(c *gin.Context) {
	var request model.CalculateZScoreRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		rc.logger.Error("Invalid request payload", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Check if n-gram service is available
	if rc.ngramService == nil {
		rc.logger.Error("N-gram service not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "N-gram service not available",
		})
		return
	}

	// Validate language
	validLanguages := map[string]bool{
		"go":         true,
		"python":     true,
		"java":       true,
		"javascript": true,
		"typescript": true,
	}
	if !validLanguages[request.Language] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported language. Supported: go, python, java, javascript, typescript",
		})
		return
	}

	// Calculate z-score
	analysis, err := rc.ngramService.CalculateZScore(
		c.Request.Context(),
		request.RepoName,
		request.Language,
		[]byte(request.Code),
	)
	if err != nil {
		rc.logger.Error("Failed to calculate z-score",
			zap.String("repo_name", request.RepoName),
			zap.String("language", request.Language),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to calculate z-score",
			"details": err.Error(),
		})
		return
	}

	// Convert n-gram scores to response format
	ngramScores := make([]model.NGramScore, len(analysis.NGramScores))
	for i, score := range analysis.NGramScores {
		ngramScores[i] = model.NGramScore{
			NGram:       score.NGram,
			Probability: score.Probability,
			LogProb:     score.LogProb,
			Entropy:     score.Entropy,
		}
	}

	response := model.CalculateZScoreResponse{
		RepoName:   request.RepoName,
		Language:   request.Language,
		TokenCount: analysis.TokenCount,
		Entropy:    analysis.Entropy,
		ZScore:     analysis.ZScore,
		CorpusStats: model.ZScoreCorpusStats{
			MeanEntropy:   analysis.EntropyStats.Mean,
			StdDevEntropy: analysis.EntropyStats.StdDev,
			MinEntropy:    analysis.EntropyStats.Min,
			MaxEntropy:    analysis.EntropyStats.Max,
			FileCount:     analysis.EntropyStats.Count,
		},
		NGramScores: ngramScores,
		Interpretation: model.ZScoreInterpretation{
			Level:       analysis.Interpretation.Level,
			Description: analysis.Interpretation.Description,
			Percentile:  analysis.Interpretation.Percentile,
		},
	}

	c.JSON(http.StatusOK, response)
}
