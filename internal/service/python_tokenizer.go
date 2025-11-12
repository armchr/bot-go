package service

import (
	"bot-go/internal/model/ngram"
	"context"
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

// PythonTokenizer implements tokenization for Python source code
type PythonTokenizer struct {
	parser   *tree_sitter.Parser
	language *tree_sitter.Language
}

// NewPythonTokenizer creates a new Python tokenizer
func NewPythonTokenizer() (*PythonTokenizer, error) {
	parser := tree_sitter.NewParser()
	language := tree_sitter.NewLanguage(python.Language())

	err := parser.SetLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to set Python language: %w", err)
	}

	return &PythonTokenizer{
		parser:   parser,
		language: language,
	}, nil
}

func (t *PythonTokenizer) Tokenize(ctx context.Context, source []byte) (ngram.TokenSequence, error) {
	tree := t.parser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse Python source")
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	var tokens ngram.TokenSequence

	t.traverseNode(rootNode, source, &tokens)

	return tokens, nil
}

func (t *PythonTokenizer) traverseNode(node *tree_sitter.Node, source []byte, tokens *ngram.TokenSequence) {
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

func (t *PythonTokenizer) Normalize(token ngram.Token) string {
	// Normalize based on token type
	switch token.Type {
	case "identifier":
		return "ID"
	case "integer", "float":
		return "NUM"
	case "string":
		return "STR"
	case "true", "false", "True", "False":
		return "BOOL"
	case "none", "None":
		return "NONE"
	default:
		// Return the actual value for keywords, operators, and punctuation
		return token.Value
	}
}

func (t *PythonTokenizer) Language() string {
	return "python"
}
