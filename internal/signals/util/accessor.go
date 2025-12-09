package util

import (
	"regexp"
	"strings"

	"bot-go/internal/signals"
)

// AccessorType represents the type of accessor method
type AccessorType string

const (
	AccessorTypeNone         AccessorType = "none"
	AccessorTypeGetter       AccessorType = "getter"
	AccessorTypeSetter       AccessorType = "setter"
	AccessorTypeGetterSetter AccessorType = "getter_setter"
)

// Thresholds for simple accessor detection
const (
	// MaxAccessorLOC is the maximum lines of code for a simple accessor
	MaxAccessorLOC = 5
	// MaxAccessorComplexity is the maximum cyclomatic complexity for an accessor
	MaxAccessorComplexity = 1
)

// AccessorPattern defines a pattern for accessor detection
type AccessorPattern struct {
	NamePattern   string // Regex for method name
	MaxComplexity int    // Max cyclomatic complexity
	MaxLOC        int    // Max lines of code
	RequiresField bool   // Must access exactly one field
}

// AccessorDetector identifies getter/setter methods
type AccessorDetector struct {
	// Language-specific patterns
	patterns map[string][]AccessorPattern
	// Compiled regex patterns cache
	compiledPatterns map[string][]*regexp.Regexp
}

// Default getter patterns for common languages
var defaultGetterPatterns = map[string][]string{
	"go":         {`^Get[A-Z]`, `^Is[A-Z]`, `^Has[A-Z]`, `^Can[A-Z]`},
	"java":       {`^get[A-Z]`, `^is[A-Z]`, `^has[A-Z]`},
	"python":     {`^get_`, `^is_`, `^has_`},
	"javascript": {`^get[A-Z]`, `^is[A-Z]`, `^has[A-Z]`},
	"typescript": {`^get[A-Z]`, `^is[A-Z]`, `^has[A-Z]`},
}

// Default setter patterns for common languages
var defaultSetterPatterns = map[string][]string{
	"go":         {`^Set[A-Z]`},
	"java":       {`^set[A-Z]`},
	"python":     {`^set_`},
	"javascript": {`^set[A-Z]`},
	"typescript": {`^set[A-Z]`},
}

// NewAccessorDetector creates a new accessor detector with default patterns
func NewAccessorDetector() *AccessorDetector {
	d := &AccessorDetector{
		patterns:         make(map[string][]AccessorPattern),
		compiledPatterns: make(map[string][]*regexp.Regexp),
	}
	// Register default patterns for all languages
	for lang, patterns := range defaultGetterPatterns {
		for _, p := range patterns {
			d.RegisterPattern(lang, AccessorPattern{
				NamePattern:   p,
				MaxComplexity: MaxAccessorComplexity,
				MaxLOC:        MaxAccessorLOC,
				RequiresField: true,
			})
		}
	}
	for lang, patterns := range defaultSetterPatterns {
		for _, p := range patterns {
			d.RegisterPattern(lang, AccessorPattern{
				NamePattern:   p,
				MaxComplexity: MaxAccessorComplexity,
				MaxLOC:        MaxAccessorLOC,
				RequiresField: true,
			})
		}
	}
	return d
}

// RegisterPattern registers a pattern for a language
func (d *AccessorDetector) RegisterPattern(language string, pattern AccessorPattern) {
	if d.patterns == nil {
		d.patterns = make(map[string][]AccessorPattern)
	}
	d.patterns[language] = append(d.patterns[language], pattern)

	// Compile and cache the regex
	if d.compiledPatterns == nil {
		d.compiledPatterns = make(map[string][]*regexp.Regexp)
	}
	if re, err := regexp.Compile(pattern.NamePattern); err == nil {
		d.compiledPatterns[language] = append(d.compiledPatterns[language], re)
	}
}

// IsAccessor checks if a method is a simple accessor/mutator
func (d *AccessorDetector) IsAccessor(methodInfo *signals.MethodInfo) bool {
	if methodInfo == nil {
		return false
	}
	return d.IsGetter(methodInfo) || d.IsSetter(methodInfo)
}

// IsGetter checks if a method is a getter
func (d *AccessorDetector) IsGetter(methodInfo *signals.MethodInfo) bool {
	if methodInfo == nil {
		return false
	}

	// Check if already marked as accessor
	if methodInfo.IsAccessor {
		// Verify it's specifically a getter by name pattern
		language := d.detectLanguage(methodInfo.FilePath)
		if d.matchesGetterPattern(methodInfo.Name, language) {
			return true
		}
	}

	// Check if it matches getter patterns and has simple body
	language := d.detectLanguage(methodInfo.FilePath)
	if d.matchesGetterPattern(methodInfo.Name, language) && d.isSimpleGetter(methodInfo) {
		return true
	}

	return false
}

// IsSetter checks if a method is a setter
func (d *AccessorDetector) IsSetter(methodInfo *signals.MethodInfo) bool {
	if methodInfo == nil {
		return false
	}

	// Check if already marked as accessor
	if methodInfo.IsAccessor {
		// Verify it's specifically a setter by name pattern
		language := d.detectLanguage(methodInfo.FilePath)
		if d.matchesSetterPattern(methodInfo.Name, language) {
			return true
		}
	}

	// Check if it matches setter patterns and has simple body
	language := d.detectLanguage(methodInfo.FilePath)
	if d.matchesSetterPattern(methodInfo.Name, language) && d.isSimpleSetter(methodInfo) {
		return true
	}

	return false
}

// ClassifyMethod returns the accessor type
func (d *AccessorDetector) ClassifyMethod(methodInfo *signals.MethodInfo) AccessorType {
	if methodInfo == nil {
		return AccessorTypeNone
	}

	isGetter := d.IsGetter(methodInfo)
	isSetter := d.IsSetter(methodInfo)

	if isGetter && isSetter {
		return AccessorTypeGetterSetter
	}
	if isGetter {
		return AccessorTypeGetter
	}
	if isSetter {
		return AccessorTypeSetter
	}
	return AccessorTypeNone
}

// GetAccessorMethods returns accessor methods from a list
func (d *AccessorDetector) GetAccessorMethods(methods []*signals.MethodInfo) []*signals.MethodInfo {
	if methods == nil {
		return nil
	}

	result := make([]*signals.MethodInfo, 0)
	for _, m := range methods {
		if d.IsAccessor(m) {
			result = append(result, m)
		}
	}
	return result
}

// GetNonAccessorMethods returns non-accessor methods from a list
func (d *AccessorDetector) GetNonAccessorMethods(methods []*signals.MethodInfo) []*signals.MethodInfo {
	if methods == nil {
		return nil
	}

	result := make([]*signals.MethodInfo, 0)
	for _, m := range methods {
		if !d.IsAccessor(m) {
			result = append(result, m)
		}
	}
	return result
}

// matchesGetterPattern checks if method name matches getter patterns
func (d *AccessorDetector) matchesGetterPattern(name string, language string) bool {
	patterns, ok := defaultGetterPatterns[language]
	if !ok {
		// Fall back to Go patterns as default
		patterns = defaultGetterPatterns["go"]
	}

	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			if re.MatchString(name) {
				return true
			}
		}
	}
	return false
}

// matchesSetterPattern checks if method name matches setter patterns
func (d *AccessorDetector) matchesSetterPattern(name string, language string) bool {
	patterns, ok := defaultSetterPatterns[language]
	if !ok {
		// Fall back to Go patterns as default
		patterns = defaultSetterPatterns["go"]
	}

	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			if re.MatchString(name) {
				return true
			}
		}
	}
	return false
}

// isSimpleGetter checks if method body is a simple return of a field
func (d *AccessorDetector) isSimpleGetter(methodInfo *signals.MethodInfo) bool {
	if methodInfo == nil {
		return false
	}

	// Check LOC threshold - a simple getter should have very few lines
	loc := d.getMethodLOC(methodInfo)
	if loc > MaxAccessorLOC {
		return false
	}

	// Check complexity - a simple getter should have no branching
	decisionPoints := len(methodInfo.Conditionals) + len(methodInfo.Loops)
	if decisionPoints > 0 {
		return false
	}

	// A getter should have at least one field access (read)
	hasFieldRead := false
	for _, fa := range methodInfo.FieldAccesses {
		if fa.AccessType == signals.AccessTypeRead {
			hasFieldRead = true
			break
		}
	}

	// If we have field accesses info, require at least one read
	if len(methodInfo.FieldAccesses) > 0 && !hasFieldRead {
		return false
	}

	// Simple getter: few lines, no branching, reads a field
	return true
}

// isSimpleSetter checks if method body is a simple field assignment
func (d *AccessorDetector) isSimpleSetter(methodInfo *signals.MethodInfo) bool {
	if methodInfo == nil {
		return false
	}

	// Check LOC threshold - a simple setter should have very few lines
	loc := d.getMethodLOC(methodInfo)
	if loc > MaxAccessorLOC {
		return false
	}

	// Check complexity - a simple setter should have no branching
	decisionPoints := len(methodInfo.Conditionals) + len(methodInfo.Loops)
	if decisionPoints > 0 {
		return false
	}

	// A setter should have at least one field access (write)
	hasFieldWrite := false
	for _, fa := range methodInfo.FieldAccesses {
		if fa.AccessType == signals.AccessTypeWrite {
			hasFieldWrite = true
			break
		}
	}

	// If we have field accesses info, require at least one write
	if len(methodInfo.FieldAccesses) > 0 && !hasFieldWrite {
		return false
	}

	// A setter typically has exactly one parameter
	if len(methodInfo.Parameters) != 1 {
		// Allow 0 params for Go where receiver is implicit
		if len(methodInfo.Parameters) > 1 {
			return false
		}
	}

	// Simple setter: few lines, no branching, writes a field
	return true
}

// getMethodLOC returns the lines of code for a method
func (d *AccessorDetector) getMethodLOC(methodInfo *signals.MethodInfo) int {
	// Calculate LOC from Range if available
	/*if methodInfo.Range.End.Line > methodInfo.Range.Start.Line {
		return int(methodInfo.Range.End.Line - methodInfo.Range.Start.Line + 1)
	}*/

	// If Range is not set, use the GetLOC method which may get it from code graph
	return methodInfo.GetLOC()
}

// detectLanguage detects the programming language from file path
func (d *AccessorDetector) detectLanguage(filePath string) string {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".go"):
		return "go"
	case strings.HasSuffix(lower, ".java"):
		return "java"
	case strings.HasSuffix(lower, ".py"):
		return "python"
	case strings.HasSuffix(lower, ".js"):
		return "javascript"
	case strings.HasSuffix(lower, ".ts"), strings.HasSuffix(lower, ".tsx"):
		return "typescript"
	default:
		return "go" // Default to Go patterns
	}
}
