package service

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// SerializableNGramModel is a serializable representation of the n-gram model
type SerializableNGramModel struct {
	Version       string                 // Format version
	N             int                    // N-gram size
	UseTrie       bool                   // Whether this is a trie-based model
	UseBloom      bool                   // Whether bloom filter was used
	TotalTokens   int64                  // Total tokens processed
	CreatedAt     time.Time              // When the model was created
	RepoName      string                 // Repository name
	SmootherName  string                 // Smoother type

	// File-level metadata (for GetStats)
	FileMetadata  map[string]FileMetadata // path -> metadata

	// For trie-based models
	TokenToID     map[string]uint32      // String interning map
	IDToToken     []string               // Reverse lookup
	TrieNodes     []SerializableTrieNode // Flattened trie structure
	VocabNodes    []SerializableTrieNode // Vocabulary trie
	ContextNodes  []SerializableTrieNode // Context trie

	// Trie counters
	NGramTrieTotalNGrams    int64  // Total n-grams in ngramTrie
	NGramTrieTotalTokens    int64  // Total tokens in ngramTrie
	ContextTrieTotalNGrams  int64  // Total n-grams in contextTrie
	ContextTrieTotalTokens  int64  // Total tokens in contextTrie

	// For map-based models (fallback)
	Vocabulary    map[string]int64       // token -> frequency
	NGramCounts   map[string]int64       // n-gram -> count
	ContextCounts map[string]int64       // context -> count
}

// FileMetadata stores minimal file information for statistics
type FileMetadata struct {
	Path       string  `json:"path"`
	Language   string  `json:"language"`
	TokenCount int     `json:"token_count"`
	Entropy    float64 `json:"entropy"`
}

// SerializableTrieNode represents a serialized trie node
type SerializableTrieNode struct {
	ID          int               // Node ID in serialized form
	TokenID     uint32            // Token ID
	Count       int64             // Frequency
	ChildrenIDs map[uint32]int    // TokenID -> child node ID
	ParentID    int               // Parent node ID (-1 for root)
}

// NGramPersistence handles saving and loading n-gram models
type NGramPersistence struct {
	outputDir string
	logger    *zap.Logger
}

// NewNGramPersistence creates a new persistence manager
func NewNGramPersistence(outputDir string, logger *zap.Logger) (*NGramPersistence, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &NGramPersistence{
		outputDir: outputDir,
		logger:    logger,
	}, nil
}

// GetModelPath returns the file path for a repository's n-gram model
func (p *NGramPersistence) GetModelPath(repoName string) string {
	return filepath.Join(p.outputDir, fmt.Sprintf("%s_ngram.gob", repoName))
}

// SaveCorpusManager saves a corpus manager to disk
func (p *NGramPersistence) SaveCorpusManager(cm *CorpusManager, repoName string) error {
	model := &SerializableNGramModel{
		Version:      "1.0",
		N:            cm.n,
		UseTrie:      cm.useTrie,
		UseBloom:     cm.useBloom,
		CreatedAt:    time.Now(),
		RepoName:     repoName,
		FileMetadata: make(map[string]FileMetadata),
	}

	// Save file metadata
	cm.mu.RLock()
	for path, fm := range cm.fileModels {
		model.FileMetadata[path] = FileMetadata{
			Path:       path,
			Language:   fm.Language,
			TokenCount: fm.TokenCount,
			Entropy:    fm.Entropy,
		}
	}
	cm.mu.RUnlock()

	// Serialize based on model type
	if cm.useTrie && cm.globalTrieModel != nil {
		if err := p.serializeTrieModel(cm.globalTrieModel, model); err != nil {
			return fmt.Errorf("failed to serialize trie model: %w", err)
		}
	} else if cm.globalModel != nil {
		p.serializeMapModel(cm.globalModel, model)
	} else {
		return fmt.Errorf("no global model found")
	}

	// Save to file
	modelPath := p.GetModelPath(repoName)
	if err := p.saveToFile(model, modelPath); err != nil {
		return fmt.Errorf("failed to save to file: %w", err)
	}

	p.logger.Info("Saved n-gram model",
		zap.String("repo", repoName),
		zap.String("path", modelPath),
		zap.Int("n", model.N),
		zap.Bool("trie", model.UseTrie),
		zap.Int64("tokens", model.TotalTokens))

	return nil
}

// LoadCorpusManager loads a corpus manager from disk
func (p *NGramPersistence) LoadCorpusManager(repoName string, tokenizer *TokenizerRegistry, logger *zap.Logger) (*CorpusManager, error) {
	modelPath := p.GetModelPath(repoName)

	// Check if file exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no saved model found for repository: %s", repoName)
	}

	// Load from file
	model, err := p.loadFromFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load from file: %w", err)
	}

	// Create smoother (default to AddK for now)
	var smoother Smoother = NewAddKSmoother(1.0)
	if model.SmootherName == "WittenBell" {
		smoother = NewWittenBellSmoother()
	}

	// Create corpus manager
	cm := NewCorpusManagerWithOptions(model.N, smoother, tokenizer, model.UseTrie, model.UseBloom, logger)

	// Restore file metadata
	cm.mu.Lock()
	for path, metadata := range model.FileMetadata {
		cm.fileModels[path] = &FileModel{
			FilePath:     metadata.Path,
			Language:     metadata.Language,
			TokenCount:   metadata.TokenCount,
			Entropy:      metadata.Entropy,
			LastModified: model.CreatedAt,
		}
	}
	cm.mu.Unlock()

	// Deserialize model
	if model.UseTrie {
		if err := p.deserializeTrieModel(model, cm); err != nil {
			return nil, fmt.Errorf("failed to deserialize trie model: %w", err)
		}
	} else {
		p.deserializeMapModel(model, cm)
	}

	p.logger.Info("Loaded n-gram model",
		zap.String("repo", repoName),
		zap.String("path", modelPath),
		zap.Int("n", model.N),
		zap.Bool("trie", model.UseTrie),
		zap.Int64("tokens", model.TotalTokens))

	return cm, nil
}

// ModelExists checks if a saved model exists for a repository
func (p *NGramPersistence) ModelExists(repoName string) bool {
	modelPath := p.GetModelPath(repoName)
	_, err := os.Stat(modelPath)
	return err == nil
}

// DeleteModel deletes a saved model for a repository
func (p *NGramPersistence) DeleteModel(repoName string) error {
	modelPath := p.GetModelPath(repoName)
	if err := os.Remove(modelPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete model: %w", err)
	}
	p.logger.Info("Deleted n-gram model", zap.String("repo", repoName))
	return nil
}

// serializeTrieModel serializes a trie-based model
func (p *NGramPersistence) serializeTrieModel(trieModel *NGramModelTrie, target *SerializableNGramModel) error {
	stats := trieModel.Stats()
	target.TotalTokens = stats.TotalTokens
	target.SmootherName = stats.SmootherName

	// Serialize string interning
	target.TokenToID = trieModel.vocabulary.tokenToID
	target.IDToToken = trieModel.vocabulary.idToToken

	// Serialize trie counters
	target.NGramTrieTotalNGrams = trieModel.ngramTrie.totalNGrams
	target.NGramTrieTotalTokens = trieModel.ngramTrie.totalTokens
	target.ContextTrieTotalNGrams = trieModel.contextTrie.totalNGrams
	target.ContextTrieTotalTokens = trieModel.contextTrie.totalTokens

	// Serialize tries
	target.TrieNodes = p.flattenTrie(trieModel.ngramTrie.root)
	target.VocabNodes = p.flattenTrie(trieModel.vocabulary.root)
	target.ContextNodes = p.flattenTrie(trieModel.contextTrie.root)

	return nil
}

// serializeMapModel serializes a map-based model
func (p *NGramPersistence) serializeMapModel(mapModel *NGramModel, target *SerializableNGramModel) {
	mapModel.mu.RLock()
	defer mapModel.mu.RUnlock()

	target.TotalTokens = mapModel.totalTokens
	target.SmootherName = mapModel.smoother.Name()
	target.Vocabulary = make(map[string]int64)
	target.NGramCounts = make(map[string]int64)
	target.ContextCounts = make(map[string]int64)

	for k, v := range mapModel.vocabulary {
		target.Vocabulary[k] = v
	}
	for k, v := range mapModel.ngramCounts {
		target.NGramCounts[k] = v
	}
	for k, v := range mapModel.contextCounts {
		target.ContextCounts[k] = v
	}
}

// flattenTrie converts a trie to a flat array for serialization
func (p *NGramPersistence) flattenTrie(root *TrieNode) []SerializableTrieNode {
	if root == nil {
		return nil
	}

	nodes := []SerializableTrieNode{}
	nodeMap := make(map[*TrieNode]int)
	nextID := 0

	// BFS traversal to assign IDs
	var flatten func(*TrieNode, int)
	flatten = func(node *TrieNode, parentID int) {
		nodeID := nextID
		nextID++
		nodeMap[node] = nodeID

		sNode := SerializableTrieNode{
			ID:          nodeID,
			TokenID:     node.tokenID,
			Count:       node.count,
			ChildrenIDs: make(map[uint32]int),
			ParentID:    parentID,
		}

		// Process children
		for tokenID, child := range node.children {
			childID := nextID
			sNode.ChildrenIDs[tokenID] = childID
			flatten(child, nodeID)
		}

		nodes = append(nodes, sNode)
	}

	flatten(root, -1)
	return nodes
}

// deserializeTrieModel reconstructs a trie-based model
func (p *NGramPersistence) deserializeTrieModel(model *SerializableNGramModel, cm *CorpusManager) error {
	if cm.globalTrieModel == nil {
		return fmt.Errorf("corpus manager has no trie model")
	}

	// Restore string interning
	cm.globalTrieModel.vocabulary.tokenToID = model.TokenToID
	cm.globalTrieModel.vocabulary.idToToken = model.IDToToken
	cm.globalTrieModel.vocabulary.nextID = uint32(len(model.IDToToken))

	// Restore tries
	cm.globalTrieModel.ngramTrie.root = p.reconstructTrie(model.TrieNodes)
	cm.globalTrieModel.vocabulary.root = p.reconstructTrie(model.VocabNodes)
	cm.globalTrieModel.contextTrie.root = p.reconstructTrie(model.ContextNodes)

	// Restore trie counters
	cm.globalTrieModel.ngramTrie.totalNGrams = model.NGramTrieTotalNGrams
	cm.globalTrieModel.ngramTrie.totalTokens = model.NGramTrieTotalTokens
	cm.globalTrieModel.contextTrie.totalNGrams = model.ContextTrieTotalNGrams
	cm.globalTrieModel.contextTrie.totalTokens = model.ContextTrieTotalTokens

	// Update total tokens
	cm.globalTrieModel.totalTokens = model.TotalTokens

	return nil
}

// deserializeMapModel reconstructs a map-based model
func (p *NGramPersistence) deserializeMapModel(model *SerializableNGramModel, cm *CorpusManager) {
	cm.globalModel.mu.Lock()
	defer cm.globalModel.mu.Unlock()

	cm.globalModel.totalTokens = model.TotalTokens
	cm.globalModel.vocabulary = model.Vocabulary
	cm.globalModel.ngramCounts = model.NGramCounts
	cm.globalModel.contextCounts = model.ContextCounts
}

// reconstructTrie rebuilds a trie from serialized nodes
func (p *NGramPersistence) reconstructTrie(nodes []SerializableTrieNode) *TrieNode {
	if len(nodes) == 0 {
		return NewTrieNode(0)
	}

	// Create all nodes first
	nodeMap := make(map[int]*TrieNode)
	for _, sNode := range nodes {
		node := NewTrieNode(sNode.TokenID)
		node.count = sNode.Count
		nodeMap[sNode.ID] = node
	}

	// Connect children
	for _, sNode := range nodes {
		node := nodeMap[sNode.ID]
		for tokenID, childID := range sNode.ChildrenIDs {
			if child, exists := nodeMap[childID]; exists {
				node.children[tokenID] = child
			}
		}
	}

	// Return root (ID 0)
	return nodeMap[0]
}

// saveToFile saves a model to a file using gob encoding
func (p *NGramPersistence) saveToFile(model *SerializableNGramModel, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(model); err != nil {
		return err
	}

	return nil
}

// loadFromFile loads a model from a file using gob decoding
func (p *NGramPersistence) loadFromFile(path string) (*SerializableNGramModel, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var model SerializableNGramModel
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&model); err != nil {
		return nil, err
	}

	return &model, nil
}
