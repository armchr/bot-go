package utils

import (
	"context"
	"strings"

	"bot-go/internal/model/ast"
	"bot-go/internal/service/codegraph"
)

// ComplexityCalculator calculates cyclomatic complexity using the code graph.
//
// ## Implementation Notes
//
// This calculator uses the code graph to count decision points (Conditional and Loop nodes)
// rather than parsing source code text. This approach is more accurate for explicit control
// flow structures but has some limitations.
//
// ## Limitations (compared to source-based counting)
//
// The following decision points are NOT counted by the graph-based approach because they
// are not stored as separate nodes in the code graph:
//
//   - Logical operators (&&, ||, and, or) - these add implicit branches but are embedded
//     within expression nodes
//   - Ternary operators (? :) - stored as expressions, not as Conditional nodes
//   - Null coalescing operators (??) - embedded in expressions
//   - Catch/except blocks - not currently tracked as separate node types
//   - Case statements in switch - the switch itself is a Conditional, but individual
//     cases may not be counted separately depending on the language visitor implementation
//
// ## Accuracy
//
// This implementation follows the classic McCabe cyclomatic complexity definition more closely:
// M = E - N + 2P (simplified to: 1 + number of decision points for a single function)
//
// The graph-based approach counts:
//   - Conditional nodes (if, else if, switch statements)
//   - Loop nodes (for, while, do-while loops)
//
// For a more comprehensive count that includes logical operators and other constructs,
// see the legacy CalculateFromSource method which uses text-based pattern matching.
type ComplexityCalculator struct {
	codeGraph *codegraph.CodeGraph
}

// NewComplexityCalculator creates a new complexity calculator
// If codeGraph is nil, the calculator will fall back to source-based calculation
func NewComplexityCalculator(codeGraph *codegraph.CodeGraph) *ComplexityCalculator {
	return &ComplexityCalculator{
		codeGraph: codeGraph,
	}
}

// Calculate computes cyclomatic complexity for a method using the code graph.
// It counts Conditional and Loop nodes that are descendants of the given function node.
//
// Cyclomatic Complexity = 1 + number of decision points
// Decision points counted: Conditional nodes, Loop nodes
//
// If codeGraph is nil or methodNodeID is invalid, returns 1 (base complexity).
func (c *ComplexityCalculator) Calculate(ctx context.Context, methodNodeID ast.NodeID) (int, error) {
	if c.codeGraph == nil || methodNodeID == ast.InvalidNodeID {
		return 1, nil // Base complexity only
	}

	decisionPoints, err := c.codeGraph.CountDecisionPoints(ctx, methodNodeID)
	if err != nil {
		return 1, err
	}

	return 1 + decisionPoints, nil
}

// CalculateForClass computes total complexity for all methods in a class
func (c *ComplexityCalculator) CalculateForClass(ctx context.Context, methodNodeIDs []ast.NodeID) (int, error) {
	total := 0
	for _, methodID := range methodNodeIDs {
		complexity, err := c.Calculate(ctx, methodID)
		if err != nil {
			return total, err
		}
		total += complexity
	}
	return total, nil
}

// CalculateFromSource computes cyclomatic complexity using source code text matching.
// This is the legacy implementation preserved for comparison and fallback purposes.
//
// This method counts more decision points than the graph-based approach, including:
//   - Control flow keywords (if, else if, for, while, case, catch)
//   - Logical operators (&&, ||, and, or)
//   - Ternary operators (? :)
//   - Null coalescing (??)
//
// However, it is less accurate because it uses simple string matching which can
// produce false positives (e.g., keywords in strings or comments).
func (c *ComplexityCalculator) CalculateFromSource(sourceCode []byte) int {
	source := string(sourceCode)

	complexity := 1 // Base complexity

	// Count decision points (simplified heuristic-based approach)
	// This is not as accurate as AST-based counting but works for estimates

	// Control flow keywords
	complexity += strings.Count(source, " if ")
	complexity += strings.Count(source, " if(")
	complexity += strings.Count(source, "\nif ")
	complexity += strings.Count(source, "\nif(")
	complexity += strings.Count(source, ";if ")
	complexity += strings.Count(source, ";if(")

	complexity += strings.Count(source, " else if ")
	complexity += strings.Count(source, " else if(")

	complexity += strings.Count(source, " for ")
	complexity += strings.Count(source, " for(")
	complexity += strings.Count(source, "\nfor ")
	complexity += strings.Count(source, "\nfor(")

	complexity += strings.Count(source, " while ")
	complexity += strings.Count(source, " while(")
	complexity += strings.Count(source, "\nwhile ")
	complexity += strings.Count(source, "\nwhile(")

	complexity += strings.Count(source, " case ")
	complexity += strings.Count(source, "\ncase ")

	complexity += strings.Count(source, " catch ")
	complexity += strings.Count(source, " catch(")
	complexity += strings.Count(source, "\ncatch ")
	complexity += strings.Count(source, "\ncatch(")

	// Logical operators (each adds a branch)
	complexity += strings.Count(source, " && ")
	complexity += strings.Count(source, " || ")
	complexity += strings.Count(source, " and ")
	complexity += strings.Count(source, " or ")

	// Ternary operators
	complexity += strings.Count(source, " ? ")

	// Null coalescing (some languages)
	complexity += strings.Count(source, " ?? ")

	return complexity
}

// CalculateForClassFromSource computes total complexity for all methods using source code
// This is the legacy implementation preserved for comparison.
func (c *ComplexityCalculator) CalculateForClassFromSource(methods [][]byte) int {
	total := 0
	for _, methodSource := range methods {
		total += c.CalculateFromSource(methodSource)
	}
	return total
}

/*
// LEGACY IMPLEMENTATION - Preserved for reference
// The original Calculate method used source code text matching:
//
// func (c *ComplexityCalculator) Calculate(sourceCode []byte) int {
// 	source := string(sourceCode)
//
// 	complexity := 1 // Base complexity
//
// 	// Count decision points (simplified heuristic-based approach)
// 	// This is not as accurate as AST-based counting but works for estimates
//
// 	// Control flow keywords
// 	complexity += strings.Count(source, " if ")
// 	complexity += strings.Count(source, " if(")
// 	complexity += strings.Count(source, "\nif ")
// 	complexity += strings.Count(source, "\nif(")
// 	complexity += strings.Count(source, ";if ")
// 	complexity += strings.Count(source, ";if(")
//
// 	complexity += strings.Count(source, " else if ")
// 	complexity += strings.Count(source, " else if(")
//
// 	complexity += strings.Count(source, " for ")
// 	complexity += strings.Count(source, " for(")
// 	complexity += strings.Count(source, "\nfor ")
// 	complexity += strings.Count(source, "\nfor(")
//
// 	complexity += strings.Count(source, " while ")
// 	complexity += strings.Count(source, " while(")
// 	complexity += strings.Count(source, "\nwhile ")
// 	complexity += strings.Count(source, "\nwhile(")
//
// 	complexity += strings.Count(source, " case ")
// 	complexity += strings.Count(source, "\ncase ")
//
// 	complexity += strings.Count(source, " catch ")
// 	complexity += strings.Count(source, " catch(")
// 	complexity += strings.Count(source, "\ncatch ")
// 	complexity += strings.Count(source, "\ncatch(")
//
// 	// Logical operators (each adds a branch)
// 	complexity += strings.Count(source, " && ")
// 	complexity += strings.Count(source, " || ")
// 	complexity += strings.Count(source, " and ")
// 	complexity += strings.Count(source, " or ")
//
// 	// Ternary operators
// 	complexity += strings.Count(source, " ? ")
//
// 	// Null coalescing (some languages)
// 	complexity += strings.Count(source, " ?? ")
//
// 	return complexity
// }
//
// func (c *ComplexityCalculator) CalculateForClass(methods [][]byte) int {
// 	total := 0
// 	for _, methodSource := range methods {
// 		total += c.Calculate(methodSource)
// 	}
// 	return total
// }
*/
