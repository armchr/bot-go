package ngram

import (
	"hash/fnv"
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
)

// TrieNode represents a node in the n-gram trie
type TrieNode struct {
	tokenID  uint32               // Token ID at this node
	count    int64                // Frequency of n-gram ending at this node
	children map[uint32]*TrieNode // Children indexed by token ID
}

// NewTrieNode creates a new trie node
func NewTrieNode(tokenID uint32) *TrieNode {
	return &TrieNode{
		tokenID:  tokenID,
		count:    0,
		children: make(map[uint32]*TrieNode),
	}
}

// NGramTrie stores n-grams in a trie structure with string interning
type NGramTrie struct {
	root        *TrieNode          // Root of the trie
	tokenToID   map[string]uint32  // String to token ID mapping
	idToToken   []string           // Token ID to string reverse mapping
	nextID      uint32             // Next available token ID
	totalTokens int64              // Total number of tokens seen
	totalNGrams int64              // Total number of n-grams stored
	bloomFilter *bloom.BloomFilter // Bloom filter for singleton detection
	useBloom    bool               // Whether to use bloom filter for singletons
	mu          sync.RWMutex       // Protects all data structures
}

// NewNGramTrie creates a new n-gram trie without bloom filter
func NewNGramTrie() *NGramTrie {
	return NewNGramTrieWithBloom(false, 100000, 0.01)
}

// NewNGramTrieWithBloom creates a new n-gram trie with optional bloom filter
// If useBloom is true, only n-grams seen more than once will be stored in the trie
func NewNGramTrieWithBloom(useBloom bool, expectedItems uint, falsePositiveRate float64) *NGramTrie {
	trie := &NGramTrie{
		root:        NewTrieNode(0), // Root has ID 0 (sentinel)
		tokenToID:   make(map[string]uint32),
		idToToken:   []string{"<ROOT>"}, // ID 0 is reserved for root
		nextID:      1,                  // Start from 1
		totalTokens: 0,
		totalNGrams: 0,
		useBloom:    useBloom,
	}

	if useBloom {
		trie.bloomFilter = bloom.NewWithEstimates(expectedItems, falsePositiveRate)
	}

	return trie
}

// internToken converts a token string to its ID, creating a new ID if needed
func (t *NGramTrie) internToken(token string) uint32 {
	if id, exists := t.tokenToID[token]; exists {
		return id
	}

	// Assign new ID
	id := t.nextID
	t.nextID++
	t.tokenToID[token] = id
	t.idToToken = append(t.idToToken, token)
	return id
}

// getToken returns the token string for a given ID
func (t *NGramTrie) getToken(id uint32) string {
	if int(id) < len(t.idToToken) {
		return t.idToToken[id]
	}
	return ""
}

// Insert adds an n-gram to the trie and increments its count
// If bloom filter is enabled, only stores n-grams that appear more than once
func (t *NGramTrie) Insert(tokens []string) {
	if len(tokens) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// If bloom filter is enabled, check if this is a singleton
	if t.useBloom {
		ngramKey := t.tokensToKey(tokens)

		// Check if we've seen this n-gram before
		if !t.bloomFilter.TestString(ngramKey) {
			// First time seeing this n-gram - add to bloom filter but not trie
			t.bloomFilter.AddString(ngramKey)
			return
		}
		// Second time (or more) - add to trie
	}

	// Convert tokens to IDs
	tokenIDs := make([]uint32, len(tokens))
	for i, token := range tokens {
		tokenIDs[i] = t.internToken(token)
	}

	// Traverse/create path in trie
	current := t.root
	for _, tokenID := range tokenIDs {
		child, exists := current.children[tokenID]
		if !exists {
			child = NewTrieNode(tokenID)
			current.children[tokenID] = child
		}
		current = child
	}

	// Increment count at the final node
	current.count++
	t.totalNGrams++
}

// tokensToKey creates a unique string key for an n-gram (for bloom filter)
func (t *NGramTrie) tokensToKey(tokens []string) string {
	// Use a fast hash-based key instead of concatenating strings
	h := fnv.New64a()
	for _, token := range tokens {
		h.Write([]byte(token))
		h.Write([]byte{0}) // Separator
	}
	// Convert hash to string
	return string(h.Sum(nil))
}

// GetCount returns the frequency of an n-gram
func (t *NGramTrie) GetCount(tokens []string) int64 {
	if len(tokens) == 0 {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Convert tokens to IDs
	tokenIDs := make([]uint32, len(tokens))
	for i, token := range tokens {
		id, exists := t.tokenToID[token]
		if !exists {
			return 0 // Token never seen
		}
		tokenIDs[i] = id
	}

	// Traverse trie
	current := t.root
	for _, tokenID := range tokenIDs {
		child, exists := current.children[tokenID]
		if !exists {
			return 0 // N-gram not found
		}
		current = child
	}

	return current.count
}

// Remove decrements the count of an n-gram (for incremental updates)
func (t *NGramTrie) Remove(tokens []string) {
	if len(tokens) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Convert tokens to IDs
	tokenIDs := make([]uint32, len(tokens))
	for i, token := range tokens {
		id, exists := t.tokenToID[token]
		if !exists {
			return // Token never seen, nothing to remove
		}
		tokenIDs[i] = id
	}

	// Traverse trie
	current := t.root
	for _, tokenID := range tokenIDs {
		child, exists := current.children[tokenID]
		if !exists {
			return // N-gram not found
		}
		current = child
	}

	// Decrement count
	if current.count > 0 {
		current.count--
		t.totalNGrams--
	}

	// Note: We don't remove nodes even if count reaches 0
	// This keeps the trie structure stable for concurrent access
	// Optional: implement garbage collection separately
}

// GetAllWithPrefix returns all n-grams with a given prefix
func (t *NGramTrie) GetAllWithPrefix(prefix []string) []NGramWithCount {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Navigate to prefix node
	current := t.root
	prefixIDs := make([]uint32, len(prefix))

	for i, token := range prefix {
		id, exists := t.tokenToID[token]
		if !exists {
			return nil // Prefix not found
		}
		prefixIDs[i] = id

		child, exists := current.children[id]
		if !exists {
			return nil // Prefix path not found
		}
		current = child
	}

	// Collect all n-grams from this point
	var results []NGramWithCount
	var currentPath []uint32
	currentPath = append(currentPath, prefixIDs...)

	t.collectNGrams(current, currentPath, &results)
	return results
}

// collectNGrams recursively collects all n-grams from a node
func (t *NGramTrie) collectNGrams(node *TrieNode, path []uint32, results *[]NGramWithCount) {
	if node.count > 0 {
		// Convert IDs back to tokens
		tokens := make([]string, len(path))
		for i, id := range path {
			tokens[i] = t.getToken(id)
		}
		*results = append(*results, NGramWithCount{
			Tokens: tokens,
			Count:  node.count,
		})
	}

	// Recurse into children
	for tokenID, child := range node.children {
		newPath := append(path, tokenID)
		t.collectNGrams(child, newPath, results)
	}
}

// VocabularySize returns the number of unique tokens
func (t *NGramTrie) VocabularySize() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.tokenToID)
}

// TotalNGrams returns the total number of n-grams stored
func (t *NGramTrie) TotalNGrams() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.totalNGrams
}

// GetVocabulary returns all unique tokens
func (t *NGramTrie) GetVocabulary() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	vocab := make([]string, 0, len(t.tokenToID))
	for token := range t.tokenToID {
		vocab = append(vocab, token)
	}
	return vocab
}

// Prune removes n-grams with count below threshold
func (t *NGramTrie) Prune(minCount int64) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	var pruned int64
	t.pruneNode(t.root, minCount, &pruned)
	return pruned
}

// pruneNode recursively prunes nodes with low counts
func (t *NGramTrie) pruneNode(node *TrieNode, minCount int64, pruned *int64) {
	// Prune children first
	for tokenID, child := range node.children {
		t.pruneNode(child, minCount, pruned)

		// Remove child if it has no count and no children
		if child.count < minCount && len(child.children) == 0 {
			if child.count > 0 {
				t.totalNGrams -= child.count
				*pruned += child.count
			}
			delete(node.children, tokenID)
		}
	}

	// Clear count if below threshold
	if node.count < minCount && node.count > 0 {
		t.totalNGrams -= node.count
		*pruned += node.count
		node.count = 0
	}
}

// MemoryStats returns memory usage statistics
func (t *NGramTrie) MemoryStats() TrieMemoryStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var nodeCount int64
	t.countNodes(t.root, &nodeCount)

	// Rough memory estimation
	vocabMemory := int64(0)
	for token := range t.tokenToID {
		vocabMemory += int64(len(token)) + 16 // String header + content
	}

	return TrieMemoryStats{
		VocabularySize:   len(t.tokenToID),
		TotalNodes:       nodeCount,
		TotalNGrams:      t.totalNGrams,
		VocabMemoryBytes: vocabMemory,
		NodeMemoryBytes:  nodeCount * 56, // Approx: tokenID(4) + count(8) + map(24) + pointers(20)
	}
}

// countNodes recursively counts all nodes in the trie
func (t *NGramTrie) countNodes(node *TrieNode, count *int64) {
	*count++
	for _, child := range node.children {
		t.countNodes(child, count)
	}
}

// NGramWithCount represents an n-gram with its frequency
type NGramWithCount struct {
	Tokens []string
	Count  int64
}

// TrieMemoryStats contains memory usage statistics
type TrieMemoryStats struct {
	VocabularySize   int   `json:"vocabulary_size"`
	TotalNodes       int64 `json:"total_nodes"`
	TotalNGrams      int64 `json:"total_ngrams"`
	VocabMemoryBytes int64 `json:"vocab_memory_bytes"`
	NodeMemoryBytes  int64 `json:"node_memory_bytes"`
}

// TotalMemoryBytes returns the estimated total memory usage
func (s TrieMemoryStats) TotalMemoryBytes() int64 {
	return s.VocabMemoryBytes + s.NodeMemoryBytes
}
