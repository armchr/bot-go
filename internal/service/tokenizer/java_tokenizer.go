package tokenizer

import (
	"bot-go/internal/model/ngram"
	"context"
	"fmt"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// JavaTokenizer implements tokenization for Java source code
type JavaTokenizer struct {
	parser   *tree_sitter.Parser
	language *tree_sitter.Language
	mu       sync.Mutex // Protects parser (tree-sitter parsers are not thread-safe)
}

// NewJavaTokenizer creates a new Java tokenizer
func NewJavaTokenizer() (*JavaTokenizer, error) {
	parser := tree_sitter.NewParser()
	language := tree_sitter.NewLanguage(java.Language())

	err := parser.SetLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to set Java language: %w", err)
	}

	return &JavaTokenizer{
		parser:   parser,
		language: language,
	}, nil
}

func (t *JavaTokenizer) Tokenize(ctx context.Context, source []byte) (ngram.TokenSequence, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	tree := t.parser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse Java source")
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	var tokens ngram.TokenSequence

	t.traverseNode(rootNode, source, &tokens)

	return tokens, nil
}

func (t *JavaTokenizer) traverseNode(node *tree_sitter.Node, source []byte, tokens *ngram.TokenSequence) {
	if node == nil {
		return
	}

	// If this is a leaf node (no children), extract the token
	if node.ChildCount() == 0 {
		nodeType := node.Kind()
		content := node.Utf8Text(source)

		// Skip empty tokens, whitespace, and comments
		if content == "" || nodeType == "comment" || nodeType == "line_comment" || nodeType == "block_comment" {
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

func (t *JavaTokenizer) Normalize(token ngram.Token) string {
	// Normalize based on token type
	switch token.Type {
	case "identifier", "type_identifier":
		return "ID"
	case "decimal_integer_literal", "hex_integer_literal", "octal_integer_literal",
		"binary_integer_literal", "decimal_floating_point_literal", "hex_floating_point_literal":
		return "NUM"
	case "string_literal", "character_literal":
		return "STR"
	case "true", "false":
		return "BOOL"
	case "null":
		return "NULL"
	default:
		// Return the actual value for keywords, operators, and punctuation
		return token.Value
	}
}

func (t *JavaTokenizer) Language() string {
	return "java"
}
