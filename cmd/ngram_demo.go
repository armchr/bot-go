package main

import (
	"bot-go/internal/service"
	"context"
	"fmt"
	"runtime"
	"time"

	"go.uber.org/zap"
)

// Simple test to compare memory usage between map-based and trie-based n-gram models
func TestNGramMemory() {
	fmt.Println("=== N-gram Model Memory Comparison ===\n")

	// Sample code tokens (normalized)
	sampleTokens := []string{
		"func", "ID", "(", "ID", "ID", ")", "{",
		"if", "ID", "==", "NIL", "{",
		"return", "NIL",
		"}",
		"ID", ":=", "ID", "(", "ID", ")",
		"return", "ID",
		"}",
	}

	// Simulate larger corpus by repeating with variations
	largeCorpus := generateLargeCorpus(sampleTokens, 10000)

	fmt.Printf("Corpus size: %d tokens\n", len(largeCorpus))
	fmt.Printf("Unique patterns: ~%d\n\n", 10000)

	// Test map-based model
	fmt.Println("--- Map-based Model ---")
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	mapModel := service.NewNGramModel(3, service.NewAddKSmoother(1.0))
	mapModel.Add(largeCorpus)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	mapMemory := m2.Alloc - m1.Alloc
	mapStats := mapModel.Stats()

	fmt.Printf("Vocabulary size: %d\n", mapStats.VocabularySize)
	fmt.Printf("N-grams stored: %d\n", mapStats.NGramCount)
	fmt.Printf("Memory used: ~%.2f MB\n\n", float64(mapMemory)/(1024*1024))

	// Test trie-based model
	fmt.Println("--- Trie-based Model ---")
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var t1 runtime.MemStats
	runtime.ReadMemStats(&t1)

	trieModel := service.NewNGramModelTrie(3, service.NewAddKSmoother(1.0))
	trieModel.Add(largeCorpus)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var t2 runtime.MemStats
	runtime.ReadMemStats(&t2)

	trieMemory := t2.Alloc - t1.Alloc
	trieStats := trieModel.Stats()
	trieMemStats := trieModel.MemoryStats()

	fmt.Printf("Vocabulary size: %d\n", trieStats.VocabularySize)
	fmt.Printf("N-grams stored: %d\n", trieStats.NGramCount)
	fmt.Printf("Trie nodes: %d\n", trieMemStats.NGramStats.TotalNodes)
	fmt.Printf("Memory used: ~%.2f MB\n", float64(trieMemory)/(1024*1024))
	fmt.Printf("  - Vocabulary: %.2f MB\n", float64(trieMemStats.VocabularyStats.TotalMemoryBytes())/(1024*1024))
	fmt.Printf("  - N-gram trie: %.2f MB\n", float64(trieMemStats.NGramStats.TotalMemoryBytes())/(1024*1024))
	fmt.Printf("  - Context trie: %.2f MB\n\n", float64(trieMemStats.ContextStats.TotalMemoryBytes())/(1024*1024))

	// Comparison
	fmt.Println("--- Comparison ---")
	savings := float64(mapMemory-trieMemory) / float64(mapMemory) * 100
	fmt.Printf("Memory savings: %.1f%%\n", savings)
	fmt.Printf("Reduction factor: %.2fx\n\n", float64(mapMemory)/float64(trieMemory))

	// Test functionality (should produce same results)
	fmt.Println("--- Functionality Test ---")
	testTokens := []string{"func", "ID", "(", "ID", ")"}

	mapEntropy := mapModel.CrossEntropy(testTokens)
	trieEntropy := trieModel.CrossEntropy(testTokens)

	fmt.Printf("Map model entropy: %.4f\n", mapEntropy)
	fmt.Printf("Trie model entropy: %.4f\n", trieEntropy)
	fmt.Printf("Difference: %.6f (should be ~0)\n", mapEntropy-trieEntropy)
}

func generateLargeCorpus(base []string, variations int) []string {
	var corpus []string

	// Add variations by inserting different identifiers and patterns
	identifiers := []string{"ID", "VAR", "CONST", "PARAM"}

	for i := 0; i < variations; i++ {
		for j, token := range base {
			if token == "ID" {
				// Vary the identifier
				corpus = append(corpus, identifiers[i%len(identifiers)])
			} else {
				corpus = append(corpus, token)
			}

			// Add some variation in structure
			if i%10 == 0 && j%5 == 0 {
				corpus = append(corpus, "STR", ",")
			}
		}
	}

	return corpus
}

func main() {
	// Run the memory comparison test
	TestNGramMemory()

	// Example usage with CorpusManager
	fmt.Println("\n=== CorpusManager Example ===\n")

	ctx := context.Background()
	registry := service.NewTokenizerRegistry()

	// Create a simple logger
	logger, _ := zap.NewDevelopment()

	// Create trie-based corpus manager
	cm := service.NewCorpusManagerWithTrie(3, service.NewAddKSmoother(1.0), registry, logger)

	// Simulate adding some code
	goCode := []byte(`
package main

func main() {
	x := 42
	if x > 0 {
		println("positive")
	}
}
`)

	// Add a Go tokenizer manually for this test
	goTokenizer, _ := service.NewGoTokenizer()
	registry.Register("go", goTokenizer, []string{".go"})

	err := cm.AddFile(ctx, "test.go", goCode, "go")
	if err != nil {
		fmt.Printf("Error adding file: %v\n", err)
		return
	}

	stats := cm.GetStats(ctx)
	memStats := cm.GetMemoryStats()

	fmt.Printf("Files: %d\n", stats.TotalFiles)
	fmt.Printf("Tokens: %d\n", stats.TotalTokens)
	fmt.Printf("Vocabulary: %d\n", stats.GlobalModel.VocabularySize)
	fmt.Printf("N-grams: %d\n", stats.GlobalModel.NGramCount)

	if memStats != nil {
		fmt.Printf("Total memory: %.2f KB\n", float64(memStats.TotalMemoryBytes())/1024)
	}

	entropy, _ := cm.GetFileEntropy(ctx, "test.go")
	fmt.Printf("File entropy: %.4f\n", entropy)
}
