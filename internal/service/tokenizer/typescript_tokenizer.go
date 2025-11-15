package tokenizer

import (
	"bot-go/internal/model/ngram"
	"context"
	"fmt"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// TypeScriptTokenizer implements tokenization for TypeScript source code
type TypeScriptTokenizer struct {
	parser   *tree_sitter.Parser
	language *tree_sitter.Language
	mu       sync.Mutex // Protects parser (tree-sitter parsers are not thread-safe)
}

// NewTypeScriptTokenizer creates a new TypeScript tokenizer
func NewTypeScriptTokenizer() (*TypeScriptTokenizer, error) {
	parser := tree_sitter.NewParser()
	language := tree_sitter.NewLanguage(typescript.LanguageTypescript())

	err := parser.SetLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to set TypeScript language: %w", err)
	}

	return &TypeScriptTokenizer{
		parser:   parser,
		language: language,
	}, nil
}

func (t *TypeScriptTokenizer) Tokenize(ctx context.Context, source []byte) (ngram.TokenSequence, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	tree := t.parser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse TypeScript source")
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	var tokens ngram.TokenSequence

	t.traverseNode(rootNode, source, &tokens)

	return tokens, nil
}

func (t *TypeScriptTokenizer) traverseNode(node *tree_sitter.Node, source []byte, tokens *ngram.TokenSequence) {
	if node == nil {
		return
	}

	// If this is a leaf node (no children), extract the token
	if node.ChildCount() == 0 {
		nodeType := node.Kind()
		content := node.Utf8Text(source)

		// Skip empty tokens, whitespace, and comments
		if content == "" || nodeType == "comment" {
			return
		}

		startPoint := node.StartPosition()
		token := ngram.Token{
			Type:   nodeType,
			Value:  content,
			Line:   int(startPoint.Row) + 1,
			Column: int(startPoint.Column) + 1,
		}
		*tokens = append(*tokens, token)
		return
	}

	// Recursively traverse children
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		t.traverseNode(child, source, tokens)
	}
}

func (t *TypeScriptTokenizer) Normalize(token ngram.Token) string {
	// Normalize based on token type (similar to JavaScript)
	switch token.Type {
	case "identifier", "type_identifier":
		return "ID"
	case "number":
		return "NUM"
	case "string", "template_string":
		return "STR"
	case "regex":
		return "REGEX"
	case "true", "false":
		return "BOOL"
	case "null":
		return "NULL"
	case "undefined":
		return "UNDEF"
	default:
		// Return the actual value for keywords, operators, and punctuation
		return token.Value
	}
}

func (t *TypeScriptTokenizer) Language() string {
	return "typescript"
}
