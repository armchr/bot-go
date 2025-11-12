package main

import (
	"bot-go/internal/config"
	"bot-go/internal/service"
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
)

func main() {
	// Create logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create temporary test directory
	testDir := "./test_ngram_output"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	fmt.Println("=== N-gram Persistence Test ===\n")

	// Create n-gram service
	ngramService, err := service.NewNGramServiceWithOutputDir(testDir, logger)
	if err != nil {
		fmt.Printf("Failed to create n-gram service: %v\n", err)
		return
	}

	// Create a test repository
	testRepo := &config.Repository{
		Name: "test-repo",
		Path: "/Users/anindya/src/armchr/bot-go/internal/service", // Use our own service directory as test
	}

	// Test 1: Process repository (first time - should build and save)
	fmt.Println("--- Test 1: First processing (build and save) ---")
	ctx := context.Background()
	if err := ngramService.ProcessRepository(ctx, testRepo, 3, false); err != nil {
		fmt.Printf("Failed to process repository: %v\n", err)
		return
	}

	stats1, err := ngramService.GetRepositoryStats(ctx, testRepo.Name)
	if err != nil {
		fmt.Printf("Failed to get stats: %v\n", err)
		return
	}
	fmt.Printf("Processed repository:\n")
	fmt.Printf("  Total tokens: %d\n", stats1.TotalTokens)
	fmt.Printf("  Vocabulary size: %d\n", stats1.GlobalModel.VocabularySize)
	fmt.Printf("  N-gram count: %d\n", stats1.GlobalModel.NGramCount)
	fmt.Printf("  Average entropy: %.4f\n\n", stats1.AverageEntropy)

	// Test 2: Load from disk (should load saved model)
	fmt.Println("--- Test 2: Second processing (load from disk) ---")
	ngramService2, err := service.NewNGramServiceWithOutputDir(testDir, logger)
	if err != nil {
		fmt.Printf("Failed to create second n-gram service: %v\n", err)
		return
	}

	if err := ngramService2.ProcessRepository(ctx, testRepo, 3, false); err != nil {
		fmt.Printf("Failed to load repository: %v\n", err)
		return
	}

	stats2, err := ngramService2.GetRepositoryStats(ctx, testRepo.Name)
	if err != nil {
		fmt.Printf("Failed to get stats: %v\n", err)
		return
	}
	fmt.Printf("Loaded from disk:\n")
	fmt.Printf("  Total tokens: %d\n", stats2.TotalTokens)
	fmt.Printf("  Vocabulary size: %d\n", stats2.GlobalModel.VocabularySize)
	fmt.Printf("  N-gram count: %d\n", stats2.GlobalModel.NGramCount)
	fmt.Printf("  Average entropy: %.4f\n\n", stats2.AverageEntropy)

	// Verify stats match
	if stats1.TotalTokens != stats2.TotalTokens ||
		stats1.GlobalModel.VocabularySize != stats2.GlobalModel.VocabularySize ||
		stats1.GlobalModel.NGramCount != stats2.GlobalModel.NGramCount {
		fmt.Println("❌ FAILED: Stats don't match!")
		return
	}
	fmt.Println("✅ SUCCESS: Stats match after save/load")

	// Test 3: Override (force rebuild)
	fmt.Println("\n--- Test 3: Override (force rebuild) ---")
	if err := ngramService2.ProcessRepository(ctx, testRepo, 3, true); err != nil {
		fmt.Printf("Failed to override repository: %v\n", err)
		return
	}

	stats3, err := ngramService2.GetRepositoryStats(ctx, testRepo.Name)
	if err != nil {
		fmt.Printf("Failed to get stats: %v\n", err)
		return
	}
	fmt.Printf("After override:\n")
	fmt.Printf("  Total tokens: %d\n", stats3.TotalTokens)
	fmt.Printf("  Vocabulary size: %d\n", stats3.GlobalModel.VocabularySize)
	fmt.Printf("  N-gram count: %d\n", stats3.GlobalModel.NGramCount)
	fmt.Printf("  Average entropy: %.4f\n\n", stats3.AverageEntropy)

	if stats1.TotalTokens != stats3.TotalTokens {
		fmt.Println("❌ FAILED: Override stats don't match original!")
		return
	}
	fmt.Println("✅ SUCCESS: Override rebuilds correctly")

	// Test 4: Verify file exists
	fmt.Println("\n--- Test 4: Verify saved file ---")
	modelPath := fmt.Sprintf("%s/%s_ngram.gob", testDir, testRepo.Name)
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		fmt.Printf("❌ FAILED: Model file not found at %s\n", modelPath)
		return
	}
	fileInfo, _ := os.Stat(modelPath)
	fmt.Printf("Model file: %s\n", modelPath)
	fmt.Printf("File size: %.2f KB\n", float64(fileInfo.Size())/1024)
	fmt.Println("✅ SUCCESS: Model file exists")

	fmt.Println("\n=== All tests passed! ===")
}
