package controller

import (
	"fmt"
	"net/http"

	"bot-go/internal/model"
	"bot-go/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RepoController struct {
	repoService  *service.RepoService
	chunkService *service.CodeChunkService
	logger       *zap.Logger
}

func NewRepoController(repoService *service.RepoService, chunkService *service.CodeChunkService, logger *zap.Logger) *RepoController {
	return &RepoController{
		repoService:  repoService,
		chunkService: chunkService,
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
