package main

import (
	"bot-go/internal/service"
	"fmt"
	"runtime"
	"time"
)

// Demonstrate the memory savings with bloom filter for singleton n-grams
func main() {
	fmt.Println("=== N-gram Bloom Filter Demo ===\n")
	fmt.Println("Comparing: Map-based | Trie-based | Trie + Bloom Filter")
	fmt.Println("Strategy: Bloom filter skips singleton n-grams (appear only once)\n")

	// Generate a realistic corpus with many singletons
	// In real code, ~50-70% of n-grams are singletons
	corpus := generateRealisticCorpus(100000) // 100K tokens

	// Count unique n-grams
	uniqueNGrams := countUniqueNGrams(corpus, 3)
	fmt.Printf("Corpus: %d tokens\n", len(corpus))
	fmt.Printf("Unique trigrams: ~%d\n", len(uniqueNGrams))

	// Estimate singletons
	singletonCount := 0
	for _, count := range uniqueNGrams {
		if count == 1 {
			singletonCount++
		}
	}
	singletonPercent := float64(singletonCount) / float64(len(uniqueNGrams)) * 100
	fmt.Printf("Singletons: %d (%.1f%%)\n\n", singletonCount, singletonPercent)

	// Test 1: Map-based model
	fmt.Println("--- Map-based Model ---")
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	mapModel := service.NewNGramModel(3, service.NewAddKSmoother(1.0))
	mapModel.Add(corpus)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	mapMemory := int64(m2.Alloc - m1.Alloc)
	mapStats := mapModel.Stats()

	fmt.Printf("N-grams stored: %d\n", mapStats.NGramCount)
	fmt.Printf("Memory used: %.2f MB\n\n", float64(mapMemory)/(1024*1024))

	// Test 2: Trie-based model (no bloom)
	fmt.Println("--- Trie-based Model (no bloom) ---")
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var t1 runtime.MemStats
	runtime.ReadMemStats(&t1)

	trieModel := service.NewNGramModelTrie(3, service.NewAddKSmoother(1.0))
	trieModel.Add(corpus)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var t2 runtime.MemStats
	runtime.ReadMemStats(&t2)

	trieMemory := int64(t2.Alloc - t1.Alloc)
	trieStats := trieModel.Stats()
	trieMemStats := trieModel.MemoryStats()

	fmt.Printf("N-grams stored: %d\n", trieStats.NGramCount)
	fmt.Printf("Trie nodes: %d\n", trieMemStats.NGramStats.TotalNodes)
	fmt.Printf("Memory used: %.2f MB\n", float64(trieMemory)/(1024*1024))
	fmt.Printf("Savings vs map: %.1f%%\n\n", (1-float64(trieMemory)/float64(mapMemory))*100)

	// Test 3: Trie + Bloom filter
	fmt.Println("--- Trie + Bloom Filter (skip singletons) ---")
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var b1 runtime.MemStats
	runtime.ReadMemStats(&b1)

	bloomModel := service.NewNGramModelTrieWithBloom(3, service.NewAddKSmoother(1.0), true, 100000, 0.01)
	bloomModel.Add(corpus)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var b2 runtime.MemStats
	runtime.ReadMemStats(&b2)

	bloomMemory := int64(b2.Alloc - b1.Alloc)
	bloomStats := bloomModel.Stats()
	bloomMemStats := bloomModel.MemoryStats()

	fmt.Printf("N-grams stored: %d (skipped %d singletons)\n",
		bloomStats.NGramCount, mapStats.NGramCount-bloomStats.NGramCount)
	fmt.Printf("Trie nodes: %d\n", bloomMemStats.NGramStats.TotalNodes)
	fmt.Printf("Memory used: %.2f MB\n", float64(bloomMemory)/(1024*1024))
	fmt.Printf("Savings vs map: %.1f%%\n", (1-float64(bloomMemory)/float64(mapMemory))*100)
	fmt.Printf("Savings vs trie: %.1f%%\n\n", (1-float64(bloomMemory)/float64(trieMemory))*100)

	// Summary
	fmt.Println("=== Summary ===")
	fmt.Printf("Map-based:       %.2f MB (baseline)\n", float64(mapMemory)/(1024*1024))
	fmt.Printf("Trie-based:      %.2f MB (%.1fx smaller)\n",
		float64(trieMemory)/(1024*1024), float64(mapMemory)/float64(trieMemory))
	fmt.Printf("Trie + Bloom:    %.2f MB (%.1fx smaller than map, %.1fx smaller than trie)\n",
		float64(bloomMemory)/(1024*1024),
		float64(mapMemory)/float64(bloomMemory),
		float64(trieMemory)/float64(bloomMemory))

	// Verify accuracy
	fmt.Println("\n=== Accuracy Test ===")
	testTokens := []string{"func", "ID", "(", "ID", ")", "{"}

	mapEntropy := mapModel.CrossEntropy(testTokens)
	trieEntropy := trieModel.CrossEntropy(testTokens)
	bloomEntropy := bloomModel.CrossEntropy(testTokens)

	fmt.Printf("Map entropy:    %.6f\n", mapEntropy)
	fmt.Printf("Trie entropy:   %.6f (diff: %.6f)\n", trieEntropy, trieEntropy-mapEntropy)
	fmt.Printf("Bloom entropy:  %.6f (diff: %.6f)\n", bloomEntropy, bloomEntropy-mapEntropy)
	fmt.Println("\nNote: Bloom filter has slightly different entropy because singletons are not stored.")
	fmt.Println("This is acceptable - singletons contribute little to naturalness models.")
}

func generateRealisticCorpus(size int) []string {
	// Generate a corpus that mimics real code:
	// - Common patterns (func, if, return, etc.)
	// - Many unique identifiers (singletons)
	// - Some repeated patterns
	corpus := make([]string, 0, size)

	commonTokens := []string{
		"func", "ID", "(", ")", "{", "}",
		"if", "==", "!=", "return", "NIL",
		"for", "range", ":=", "var",
	}

	// Add common patterns (30% of corpus)
	for i := 0; i < size*3/10; i++ {
		corpus = append(corpus, commonTokens[i%len(commonTokens)])
	}

	// Add semi-common patterns (20% of corpus)
	semiCommon := []string{
		"struct", "interface", "type", "const",
		"switch", "case", "default", "break",
		"continue", "goto", "defer", "go",
	}
	for i := 0; i < size*2/10; i++ {
		corpus = append(corpus, semiCommon[i%len(semiCommon)])
	}

	// Add unique tokens (singletons) (50% of corpus)
	// These represent unique identifiers, strings, etc.
	for i := 0; i < size*5/10; i++ {
		// Generate unique identifier
		corpus = append(corpus, fmt.Sprintf("ID_%d", i))
	}

	return corpus
}

func countUniqueNGrams(tokens []string, n int) map[string]int {
	ngrams := make(map[string]int)

	for i := 0; i <= len(tokens)-n; i++ {
		ngram := ""
		for j := 0; j < n; j++ {
			if j > 0 {
				ngram += " "
			}
			ngram += tokens[i+j]
		}
		ngrams[ngram]++
	}

	return ngrams
}
