package main

import (
	"bot-go/internal/config"
	"bot-go/internal/service"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

func main() {
	// Command line flags
	appConfig := flag.String("app", "config/app.yaml", "Path to app config file")
	testMode := flag.String("test", "", "Test mode: file, directory, search, all")
	testFile := flag.String("file", "", "Path to file for testing (used with -test file)")
	testDir := flag.String("dir", "", "Path to directory for testing (used with -test directory)")
	searchQuery := flag.String("query", "HTTP request handler", "Search query for testing (used with -test search)")
	collection := flag.String("collection", "code-search", "Collection name for vector DB")
	recreate := flag.Bool("recreate", false, "Recreate collection (delete and create new)")

	flag.Parse()

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadConfig(*appConfig, "config/source.yaml")
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	logger.Info("Loaded configuration",
		zap.String("qdrant_host", cfg.Qdrant.Host),
		zap.Int("qdrant_port", cfg.Qdrant.Port),
		zap.String("ollama_url", cfg.Ollama.URL),
		zap.String("ollama_model", cfg.Ollama.Model),
		zap.Int("embedding_dimension", cfg.Ollama.Dimension))

	// Initialize Qdrant vector database
	vectorDB, err := service.NewQdrantDatabase(cfg.Qdrant.Host, cfg.Qdrant.Port, cfg.Qdrant.APIKey, logger)
	if err != nil {
		logger.Fatal("Failed to create Qdrant database", zap.Error(err))
	}
	defer vectorDB.Close()

	// Check health
	if err := vectorDB.Health(ctx); err != nil {
		logger.Fatal("Qdrant health check failed", zap.Error(err))
	}
	logger.Info("Qdrant connection successful")

	// Initialize Ollama embedding model
	embeddingModel, err := service.NewOllamaEmbedding(service.OllamaEmbeddingConfig{
		APIURL:    cfg.Ollama.URL,
		APIKey:    cfg.Ollama.APIKey,
		Model:     cfg.Ollama.Model,
		Dimension: cfg.Ollama.Dimension,
	}, logger)
	if err != nil {
		logger.Fatal("Failed to create Ollama embedding model", zap.Error(err))
	}

	logger.Info("Ollama embedding model initialized",
		zap.String("model", embeddingModel.GetModelName()),
		zap.Int("dimension", embeddingModel.GetDimension()))

	// Initialize code chunk service
	chunkService := service.NewCodeChunkService(vectorDB, embeddingModel, 5, 5, 100, logger)
	defer chunkService.Close()

	// Handle collection recreation
	if *recreate {
		logger.Info("Recreating collection", zap.String("collection", *collection))
		exists, err := vectorDB.CollectionExists(ctx, *collection)
		if err != nil {
			logger.Fatal("Failed to check collection existence", zap.Error(err))
		}
		if exists {
			if err := chunkService.DeleteCollection(ctx, *collection); err != nil {
				logger.Fatal("Failed to delete collection", zap.Error(err))
			}
		}
	}

	// Create collection if it doesn't exist
	if err := chunkService.CreateCollection(ctx, *collection); err != nil {
		logger.Fatal("Failed to create collection", zap.Error(err))
	}

	// Execute test mode
	switch *testMode {
	case "file":
		if *testFile == "" {
			logger.Fatal("File path required for file test mode (-file)")
		}
		testFileMode(ctx, chunkService, *testFile, *collection, logger)

	case "directory":
		if *testDir == "" {
			logger.Fatal("Directory path required for directory test mode (-dir)")
		}
		testDirectoryMode(ctx, chunkService, *testDir, *collection, logger)

	case "search":
		testSearchMode(ctx, chunkService, *searchQuery, *collection, logger)

	case "all":
		testAllMode(ctx, chunkService, *collection, logger)

	default:
		logger.Fatal("Invalid test mode. Use: file, directory, search, or all")
	}

	logger.Info("Test completed successfully")
}

func testFileMode(ctx context.Context, chunkService *service.CodeChunkService, filePath, collection string, logger *zap.Logger) {
	logger.Info("Testing single file processing", zap.String("file", filePath))

	// Detect language from file extension
	language := detectLanguageFromPath(filePath)
	if language == "" {
		logger.Fatal("Unsupported file type", zap.String("file", filePath))
	}

	// Process file
	chunks, err := chunkService.ProcessFile(ctx, filePath, language, collection)
	if err != nil {
		logger.Fatal("Failed to process file", zap.Error(err))
	}

	// Display results
	logger.Info("File processed successfully",
		zap.String("file", filePath),
		zap.Int("chunks", len(chunks)))

	for i, chunk := range chunks {
		logger.Info(fmt.Sprintf("Chunk %d", i+1),
			zap.String("id", chunk.ID),
			zap.String("type", string(chunk.ChunkType)),
			zap.Int("level", chunk.Level),
			zap.String("name", chunk.Name),
			zap.String("signature", chunk.Signature),
			zap.Int("start_line", chunk.StartLine),
			zap.Int("end_line", chunk.EndLine),
			zap.Int("content_length", len(chunk.Content)),
			zap.Int("embedding_dim", len(chunk.Embedding)))
	}
}

func testDirectoryMode(ctx context.Context, chunkService *service.CodeChunkService, dirPath, collection string, logger *zap.Logger) {
	logger.Info("Testing directory processing", zap.String("directory", dirPath))

	totalChunks, err := chunkService.ProcessDirectory(ctx, dirPath, collection)
	if err != nil {
		logger.Fatal("Failed to process directory", zap.Error(err))
	}

	logger.Info("Directory processed successfully",
		zap.String("directory", dirPath),
		zap.Int("total_chunks", totalChunks))
}

func testSearchMode(ctx context.Context, chunkService *service.CodeChunkService, query, collection string, logger *zap.Logger) {
	logger.Info("Testing search", zap.String("query", query))

	chunks, scores, err := chunkService.SearchSimilarCode(ctx, collection, query, 10, nil)
	if err != nil {
		logger.Fatal("Failed to search", zap.Error(err))
	}

	logger.Info("Search completed", zap.Int("results", len(chunks)))

	for i, chunk := range chunks {
		logger.Info(fmt.Sprintf("Result %d", i+1),
			zap.Float32("score", scores[i]),
			zap.String("type", string(chunk.ChunkType)),
			zap.String("name", chunk.Name),
			zap.String("file", chunk.FilePath),
			zap.Int("line", chunk.StartLine),
			zap.String("signature", chunk.Signature))

		// Print snippet of content
		contentPreview := chunk.Content
		if len(contentPreview) > 200 {
			contentPreview = contentPreview[:200] + "..."
		}
		fmt.Printf("\nContent:\n%s\n\n", contentPreview)
	}
}

func testAllMode(ctx context.Context, chunkService *service.CodeChunkService, collection string, logger *zap.Logger) {
	logger.Info("Running all tests")

	// Test 1: Process a sample Go file
	logger.Info("Test 1: Processing sample Go file")
	testFile1 := "cmd/main.go"
	if _, err := os.Stat(testFile1); err == nil {
		testFileMode(ctx, chunkService, testFile1, collection, logger)
	} else {
		logger.Warn("Sample file not found, skipping", zap.String("file", testFile1))
	}

	// Test 2: Process internal/service directory
	logger.Info("Test 2: Processing internal/service directory")
	testDir := "internal/service"
	if _, err := os.Stat(testDir); err == nil {
		testDirectoryMode(ctx, chunkService, testDir, collection, logger)
	} else {
		logger.Warn("Sample directory not found, skipping", zap.String("dir", testDir))
	}

	// Test 3: Search for different queries
	logger.Info("Test 3: Testing search functionality")
	queries := []string{
		"database connection",
		"HTTP handler",
		"parse syntax tree",
		"embedding generation",
	}

	for _, query := range queries {
		logger.Info("Searching", zap.String("query", query))
		testSearchMode(ctx, chunkService, query, collection, logger)
	}
}

func detectLanguageFromPath(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".go":
		return "go"
	case ".py", ".pyw":
		return "python"
	case ".java":
		return "java"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	default:
		return ""
	}
}
