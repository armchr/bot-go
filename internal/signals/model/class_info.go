package model

import (
	"bot-go/internal/model/ast"
	"bot-go/internal/service/codegraph"
	"bot-go/internal/service/ngram"
	"bot-go/internal/service/vector"
)

// ClassInfo aggregates all data needed for signal calculation
type ClassInfo struct {
	// Basic metadata
	RepoName  string
	ClassName string
	FilePath  string
	FileID    int32

	// Code graph data
	ClassNode *ast.Node
	Methods   []*MethodInfo
	Fields    []*ast.Node

	// File content
	SourceCode []byte
	StartLine  int
	EndLine    int

	// Services for querying (optional - not all signals need all services)
	CodeGraph    *codegraph.CodeGraph
	VectorDB     vector.VectorDatabase
	NGramService *ngram.NGramService
}

// MethodInfo contains method-level data
type MethodInfo struct {
	Node       *ast.Node
	Name       string
	IsAccessor bool // Computed by accessor detector

	// Source code
	SourceCode []byte
	StartLine  int
	EndLine    int

	// Computed metrics (populated as needed)
	Embedding  []float32 // From vector DB (optional)
	Entropy    float64   // From n-gram (optional, -1 if not computed)
	Complexity int       // Cyclomatic complexity (optional, -1 if not computed)

	// Field access tracking (for cohesion metrics)
	AccessedFields []string // Names of fields this method accesses
}

// GetLOC returns lines of code for the class
func (c *ClassInfo) GetLOC() int {
	if c.EndLine == 0 || c.StartLine == 0 {
		return 0
	}
	return c.EndLine - c.StartLine + 1
}

// GetNOM returns number of methods
func (c *ClassInfo) GetNOM() int {
	return len(c.Methods)
}

// GetNOF returns number of fields
func (c *ClassInfo) GetNOF() int {
	return len(c.Fields)
}

// GetNonAccessorMethods returns methods that are not accessors/mutators
func (c *ClassInfo) GetNonAccessorMethods() []*MethodInfo {
	var result []*MethodInfo
	for _, method := range c.Methods {
		if !method.IsAccessor {
			result = append(result, method)
		}
	}
	return result
}

// GetAccessorMethods returns methods that are accessors/mutators
func (c *ClassInfo) GetAccessorMethods() []*MethodInfo {
	var result []*MethodInfo
	for _, method := range c.Methods {
		if method.IsAccessor {
			result = append(result, method)
		}
	}
	return result
}

// GetMethodByName finds a method by name
func (c *ClassInfo) GetMethodByName(name string) *MethodInfo {
	for _, method := range c.Methods {
		if method.Name == name {
			return method
		}
	}
	return nil
}
