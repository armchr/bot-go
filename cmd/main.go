package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"bot-go/internal/config"
	"bot-go/internal/controller"
	"bot-go/internal/handler"
	"bot-go/internal/service"
	"bot-go/pkg/lsp"
	"bot-go/pkg/mcp"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	var sourceConfigPath = flag.String("source", "source.yaml", "Path to source configuration file")
	var appConfigPath = flag.String("app", "app.yaml", "Path to app configuration file")
	var workDir = flag.String("workdir", "", "Working directory to store files")
	//var port = flag.String("port", "8080", "Server port")
	var test = flag.Bool("test", false, "Run in test mode")
	flag.Parse()

	//logger, err := zap.NewProduction()
	cfgZap := zap.NewProductionConfig()
	cfgZap.Level.SetLevel(zapcore.DebugLevel)
	cfgZap.OutputPaths = []string{"stdout", "all.log"}
	logger, err := cfgZap.Build()
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	defer logger.Sync()

	cfg, err := config.LoadConfig(*appConfigPath, *sourceConfigPath)
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Override workdir from command line if provided
	if *workDir != "" {
		cfg.App.WorkDir = *workDir
	}

	logger.Info("Configuration loaded successfully", zap.Any("config", cfg))

	if test != nil && *test {
		logger.Info("Running in test mode")
		LSPTest(cfg, logger)
		return
	}

	repoService := service.NewRepoService(cfg, logger)
	CodeGraphEntry(cfg, logger, repoService)

	// Initialize CodeChunkService if Qdrant and Ollama are configured
	var chunkService *service.CodeChunkService
	if cfg.Qdrant.Host != "" && cfg.Ollama.URL != "" {
		logger.Info("Initializing code chunk service",
			zap.String("qdrant_host", cfg.Qdrant.Host),
			zap.Int("qdrant_port", cfg.Qdrant.Port),
			zap.String("ollama_url", cfg.Ollama.URL))

		vectorDB, err := service.NewQdrantDatabase(cfg.Qdrant.Host, cfg.Qdrant.Port, cfg.Qdrant.APIKey, logger)
		if err != nil {
			logger.Warn("Failed to initialize Qdrant database, code chunking will be disabled", zap.Error(err))
		} else {
			embeddingModel, err := service.NewOllamaEmbedding(service.OllamaEmbeddingConfig{
				APIURL:    cfg.Ollama.URL,
				APIKey:    cfg.Ollama.APIKey,
				Model:     cfg.Ollama.Model,
				Dimension: cfg.Ollama.Dimension,
			}, logger)
			if err != nil {
				logger.Warn("Failed to initialize Ollama embedding model, code chunking will be disabled", zap.Error(err))
				vectorDB.Close()
			} else {
				// Set default thresholds if not configured
				minConditionalLines := cfg.Chunking.MinConditionalLines
				minLoopLines := cfg.Chunking.MinLoopLines
				if minConditionalLines == 0 {
					minConditionalLines = 5 // default
				}
				if minLoopLines == 0 {
					minLoopLines = 5 // default
				}

				gcThreshold := cfg.App.GCThreshold
				if gcThreshold == 0 {
					gcThreshold = 100 // default
				}

				numFileThreads := cfg.App.NumFileThreads
				if numFileThreads == 0 {
					numFileThreads = 2 // default
				}

				chunkService = service.NewCodeChunkService(
					vectorDB,
					embeddingModel,
					minConditionalLines,
					minLoopLines,
					gcThreshold,
					numFileThreads,
					logger,
				)
				logger.Info("Code chunk service initialized successfully",
					zap.Int("min_conditional_lines", minConditionalLines),
					zap.Int("min_loop_lines", minLoopLines),
					zap.Int64("gc_threshold", gcThreshold))
			}
		}
	} else {
		logger.Info("Code chunk service disabled (Qdrant or Ollama not configured)")
	}

	// Initialize NGramService
	ngramService, err := service.NewNGramService(logger)
	if err != nil {
		logger.Warn("Failed to initialize N-gram service", zap.Error(err))
	} else {
		logger.Info("N-gram service initialized successfully")
	}

	repoController := controller.NewRepoController(repoService, chunkService, ngramService, logger)
	mcpServer := mcp.NewCodeGraphServer(repoService, cfg, logger)

	router := handler.SetupRouter(repoController, mcpServer, logger)

	logger.Info("Starting server", zap.Int("port", cfg.App.Port))
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.App.Port), router); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}

func LSPTest(cfg *config.Config, logger *zap.Logger) {
	logger.Info("Testing LSP client")
	repo, _ := cfg.GetRepository("mcp-server")

	// Initialize the LSP client
	ls, err := lsp.NewLSPLanguageServer(cfg, repo.Language, repo.Path, logger)
	if err != nil {
		logger.Fatal("Failed to create LSP client", zap.Error(err))
	}

	// Create a context for the LSP operations
	ctx := context.Background()

	defer ls.Shutdown(ctx)

	// Initialize the LSP client

	baseClient := ls.(*lsp.TypeScriptLanguageServerClient).BaseClient

	baseClient.TestCommand(ctx)
}

func CodeGraphEntry(cfg *config.Config, logger *zap.Logger, repoService *service.RepoService) {
	if !cfg.App.CodeGraph {
		logger.Info("CodeGraph is disabled in the configuration")
		return
	}
	ctx := context.Background()

	// Initialize CodeGraph service
	codeGraph, err := service.NewCodeGraph(
		cfg.Neo4j.URI,
		cfg.Neo4j.Username,
		cfg.Neo4j.Password,
		cfg,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to initialize CodeGraph", zap.Error(err))
		return
	}
	//defer codeGraph.Close(ctx)

	// Initialize RepoProcessor
	repoProcessor := controller.NewRepoProcessor(cfg, codeGraph, logger)
	postProcessor := controller.NewPostProcessor(codeGraph, repoService.GetLspService(), logger)

	// Start processing repositories in a goroutine
	go func() {
		logger.Info("Starting repository processing thread")
		err := repoProcessor.ProcessAllRepositories(ctx, postProcessor)

		if err != nil {
			logger.Error("Repository processing failed", zap.Error(err))
		}
		logger.Info("Repository processing thread completed")
	}()
}
