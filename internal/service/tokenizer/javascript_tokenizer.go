package tokenizer

import (
	"bot-go/internal/model/ngram"
	"context"
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

// JavaScriptTokenizer implements tokenization for JavaScript source code
type JavaScriptTokenizer struct {
	parser   *tree_sitter.Parser
	language *tree_sitter.Language
}

// NewJavaScriptTokenizer creates a new JavaScript tokenizer
func NewJavaScriptTokenizer() (*JavaScriptTokenizer, error) {
	parser := tree_sitter.NewParser()
	language := tree_sitter.NewLanguage(javascript.Language())

	err := parser.SetLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to set JavaScript language: %w", err)
	}

	return &JavaScriptTokenizer{
		parser:   parser,
		language: language,
	}, nil
}

func (t *JavaScriptTokenizer) Tokenize(ctx context.Context, source []byte) (ngram.TokenSequence, error) {
	tree := t.parser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse JavaScript source")
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	var tokens ngram.TokenSequence

	t.traverseNode(rootNode, source, &tokens)

	return tokens, nil
}

func (t *JavaScriptTokenizer) traverseNode(node *tree_sitter.Node, source []byte, tokens *ngram.TokenSequence) {
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

func (t *JavaScriptTokenizer) Normalize(token ngram.Token) string {
	// Normalize based on token type
	switch token.Type {
	case "identifier":
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

func (t *JavaScriptTokenizer) Language() string {
	return "javascript"
}
