package utils

import (
	"regexp"
	"strings"
)

// AccessorDetector identifies getter/setter methods
type AccessorDetector struct {
	getterPattern *regexp.Regexp
	setterPattern *regexp.Regexp
	boolPattern   *regexp.Regexp
}

// NewAccessorDetector creates a new accessor detector
func NewAccessorDetector() *AccessorDetector {
	return &AccessorDetector{
		getterPattern: regexp.MustCompile(`^(get|Get)[A-Z]`),
		setterPattern: regexp.MustCompile(`^(set|Set)[A-Z]`),
		boolPattern:   regexp.MustCompile(`^(is|Is|has|Has)[A-Z]`),
	}
}

// IsAccessor determines if a method is a simple getter/setter
func (d *AccessorDetector) IsAccessor(methodName string, methodSource []byte) bool {
	// Check name pattern first
	if !d.matchesAccessorPattern(methodName) {
		return false
	}

	// Check if method body is simple (heuristic: short and simple)
	return d.hasSimpleBody(string(methodSource))
}

// matchesAccessorPattern checks if method name follows accessor naming convention
func (d *AccessorDetector) matchesAccessorPattern(name string) bool {
	return d.getterPattern.MatchString(name) ||
		d.setterPattern.MatchString(name) ||
		d.boolPattern.MatchString(name)
}

// hasSimpleBody checks if method body is simple enough to be an accessor
func (d *AccessorDetector) hasSimpleBody(source string) bool {
	// Remove comments and whitespace for analysis
	cleaned := d.cleanSource(source)

	// Count statements (very rough heuristic)
	// Simple accessor should have 1-2 statements (e.g., just return, or set + return)
	statementCount := strings.Count(cleaned, ";") + strings.Count(cleaned, "return")

	// Simple accessor should be short
	lines := strings.Split(cleaned, "\n")
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	// Heuristic: accessor is simple if it has <= 5 non-empty lines and <= 2 statements
	return nonEmptyLines <= 5 && statementCount <= 2
}

// cleanSource removes comments and extra whitespace
func (d *AccessorDetector) cleanSource(source string) string {
	// Remove single-line comments
	lines := strings.Split(source, "\n")
	var cleaned []string
	for _, line := range lines {
		// Remove // comments
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		cleaned = append(cleaned, line)
	}

	result := strings.Join(cleaned, "\n")

	// Remove multi-line comments (basic - doesn't handle all edge cases)
	result = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(result, "")

	return result
}
