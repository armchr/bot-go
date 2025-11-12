package service

import (
	"bot-go/internal/model/ngram"
	"context"
)

// Tokenizer defines the interface for language-specific tokenization
type Tokenizer interface {
	// Tokenize converts source code into a sequence of tokens
	Tokenize(ctx context.Context, source []byte) (ngram.TokenSequence, error)

	// Normalize applies language-specific normalization (e.g., all identifiers -> "ID")
	Normalize(token ngram.Token) string

	// Language returns the language this tokenizer handles
	Language() string
}

// TokenizerRegistry manages tokenizers for different languages
type TokenizerRegistry struct {
	tokenizers map[string]Tokenizer
	extensions map[string]string // file extension -> language
}

// NewTokenizerRegistry creates a new tokenizer registry
func NewTokenizerRegistry() *TokenizerRegistry {
	return &TokenizerRegistry{
		tokenizers: make(map[string]Tokenizer),
		extensions: make(map[string]string),
	}
}

// Register adds a tokenizer for a specific language
func (tr *TokenizerRegistry) Register(language string, tokenizer Tokenizer, extensions []string) {
	tr.tokenizers[language] = tokenizer
	for _, ext := range extensions {
		tr.extensions[ext] = language
	}
}

// GetTokenizer returns the tokenizer for a given language
func (tr *TokenizerRegistry) GetTokenizer(language string) (Tokenizer, bool) {
	tokenizer, ok := tr.tokenizers[language]
	return tokenizer, ok
}

// GetTokenizerByExtension returns the tokenizer for a given file extension
func (tr *TokenizerRegistry) GetTokenizerByExtension(extension string) (Tokenizer, bool) {
	language, ok := tr.extensions[extension]
	if !ok {
		return nil, false
	}
	return tr.GetTokenizer(language)
}

// SupportedLanguages returns a list of all supported languages
func (tr *TokenizerRegistry) SupportedLanguages() []string {
	languages := make([]string, 0, len(tr.tokenizers))
	for lang := range tr.tokenizers {
		languages = append(languages, lang)
	}
	return languages
}
