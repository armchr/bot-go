package utils

import (
	"bot-go/internal/signals/model"
	"strings"
)

// FieldAccessAnalyzer tracks which methods access which fields
type FieldAccessAnalyzer struct{}

// NewFieldAccessAnalyzer creates a new field access analyzer
func NewFieldAccessAnalyzer() *FieldAccessAnalyzer {
	return &FieldAccessAnalyzer{}
}

// AnalyzeFieldAccess populates the AccessedFields for each method
func (a *FieldAccessAnalyzer) AnalyzeFieldAccess(classInfo *model.ClassInfo) {
	// Get field names
	fieldNames := make(map[string]bool)
	for _, field := range classInfo.Fields {
		fieldNames[field.Name] = true
	}

	// For each method, find which fields it accesses
	for _, method := range classInfo.Methods {
		method.AccessedFields = a.findAccessedFields(method.SourceCode, fieldNames)
	}
}

// findAccessedFields identifies field references in method source code
func (a *FieldAccessAnalyzer) findAccessedFields(sourceCode []byte, fieldNames map[string]bool) []string {
	source := string(sourceCode)
	var accessed []string
	accessedSet := make(map[string]bool)

	// Simple heuristic: look for field names in the code
	// This is not perfect but works for most cases
	for fieldName := range fieldNames {
		// Look for patterns like: this.field, self.field, field =, field., field)
		patterns := []string{
			"this." + fieldName,
			"self." + fieldName,
			" " + fieldName + " =",
			" " + fieldName + ".",
			" " + fieldName + ")",
			" " + fieldName + ",",
			"(" + fieldName + ")",
			"(" + fieldName + ",",
			"\n" + fieldName + " =",
			"\n" + fieldName + ".",
		}

		for _, pattern := range patterns {
			if strings.Contains(source, pattern) {
				if !accessedSet[fieldName] {
					accessed = append(accessed, fieldName)
					accessedSet[fieldName] = true
				}
				break
			}
		}
	}

	return accessed
}

// BuildMethodFieldMatrix builds a matrix showing which methods access which fields
// Returns: map[methodName]map[fieldName]bool
func (a *FieldAccessAnalyzer) BuildMethodFieldMatrix(classInfo *model.ClassInfo) map[string]map[string]bool {
	matrix := make(map[string]map[string]bool)

	for _, method := range classInfo.Methods {
		fieldAccess := make(map[string]bool)
		for _, fieldName := range method.AccessedFields {
			fieldAccess[fieldName] = true
		}
		matrix[method.Name] = fieldAccess
	}

	return matrix
}

// GetSharedFields returns fields shared between two methods
func (a *FieldAccessAnalyzer) GetSharedFields(method1, method2 *model.MethodInfo) []string {
	// Build sets
	fields1 := make(map[string]bool)
	for _, field := range method1.AccessedFields {
		fields1[field] = true
	}

	var shared []string
	for _, field := range method2.AccessedFields {
		if fields1[field] {
			shared = append(shared, field)
		}
	}

	return shared
}

// DoMethodsShareFields checks if two methods access at least one common field
func (a *FieldAccessAnalyzer) DoMethodsShareFields(method1, method2 *model.MethodInfo) bool {
	fields1 := make(map[string]bool)
	for _, field := range method1.AccessedFields {
		fields1[field] = true
	}

	for _, field := range method2.AccessedFields {
		if fields1[field] {
			return true
		}
	}

	return false
}
