package ngram

// Token represents a single lexical token in source code
type Token struct {
	Type    string // Token type (e.g., "IDENTIFIER", "KEYWORD", "LITERAL", etc.)
	Value   string // Original token value
	Line    int    // Line number in source
	Column  int    // Column number in source
}

// TokenSequence is a slice of tokens
type TokenSequence []Token

// NGram represents an n-gram (sequence of n tokens)
type NGram []string

// String returns the n-gram as a space-separated string
func (ng NGram) String() string {
	result := ""
	for i, token := range ng {
		if i > 0 {
			result += " "
		}
		result += token
	}
	return result
}

// Context returns the context (all tokens except the last one)
func (ng NGram) Context() NGram {
	if len(ng) <= 1 {
		return NGram{}
	}
	return ng[:len(ng)-1]
}

// LastToken returns the last token in the n-gram
func (ng NGram) LastToken() string {
	if len(ng) == 0 {
		return ""
	}
	return ng[len(ng)-1]
}
