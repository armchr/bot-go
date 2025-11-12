package tokenizer

import (
	"bot-go/internal/model/ngram"
	"context"
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	golang "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// GoTokenizer implements tokenization for Go source code
type GoTokenizer struct {
	parser   *tree_sitter.Parser
	language *tree_sitter.Language
}

// NewGoTokenizer creates a new Go tokenizer
func NewGoTokenizer() (*GoTokenizer, error) {
	parser := tree_sitter.NewParser()
	language := tree_sitter.NewLanguage(golang.Language())

	err := parser.SetLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to set Go language: %w", err)
	}

	return &GoTokenizer{
		parser:   parser,
		language: language,
	}, nil
}

func (t *GoTokenizer) Tokenize(ctx context.Context, source []byte) (ngram.TokenSequence, error) {
	tree := t.parser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse Go source")
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	var tokens ngram.TokenSequence

	t.traverseNode(rootNode, source, &tokens)

	return tokens, nil
}

func (t *GoTokenizer) traverseNode(node *tree_sitter.Node, source []byte, tokens *ngram.TokenSequence) {
	if node == nil {
		return
	}

	// If this is a leaf node (no children), extract the token
	if node.ChildCount() == 0 {
		nodeType := node.Kind()
		content := node.Utf8Text(source)

		// Skip empty tokens and whitespace
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

func (t *GoTokenizer) Normalize(token ngram.Token) string {
	// Normalize based on token type
	switch token.Type {
	case "identifier":
		return "ID"
	case "int_literal", "float_literal", "imaginary_literal":
		return "NUM"
	case "raw_string_literal", "interpreted_string_literal":
		return "STR"
	case "rune_literal":
		return "CHAR"
	case "true", "false":
		return "BOOL"
	case "nil":
		return "NIL"
	default:
		// Return the actual value for keywords, operators, and punctuation
		return token.Value
	}
}

func (t *GoTokenizer) Language() string {
	return "go"
}
