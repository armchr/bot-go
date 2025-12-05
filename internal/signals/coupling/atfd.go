package coupling

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"bot-go/internal/model/ast"
	"bot-go/internal/service/codegraph"
	"bot-go/internal/signals"
)

// ATFDSignal measures Access To Foreign Data
// Counts number of attributes from external classes accessed by this class
type ATFDSignal struct {
	accessorPattern *regexp.Regexp
}

// NewATFDSignal creates a new ATFD signal
func NewATFDSignal() *ATFDSignal {
	return &ATFDSignal{
		// Pattern to match accessor calls like: obj.getField(), obj.field, obj.Field()
		accessorPattern: regexp.MustCompile(`\w+\.(get|Get|is|Is|has|Has)[A-Z]\w*\(|\.get[A-Z]\w*\(|\.is[A-Z]\w*\(`),
	}
}

func (s *ATFDSignal) Name() string {
	return "ATFD"
}

func (s *ATFDSignal) Category() signals.SignalCategory {
	return signals.CategoryCoupling
}

func (s *ATFDSignal) Description() string {
	return "Access To Foreign Data - number of external class attributes accessed"
}

func (s *ATFDSignal) Calculate(ctx context.Context, classInfo *signals.ClassInfo) (float64, error) {
	// Track unique external attributes accessed
	externalAccesses := make(map[string]bool)

	// Analyze all methods in the class
	for _, method := range classInfo.Methods {
		accesses := s.findExternalAccesses(ctx, classInfo, method)
		for _, access := range accesses {
			externalAccesses[access] = true
		}
	}

	return float64(len(externalAccesses)), nil
}

// findExternalAccesses identifies external field accesses in a method
func (s *ATFDSignal) findExternalAccesses(ctx context.Context, classInfo *signals.ClassInfo, method *signals.MethodInfo) []string {
	var accesses []string

	source := string(method.SourceCode)

	// Method 1: Find accessor method calls via code graph (more accurate)
	if classInfo.CodeGraph != nil {
		graphAccesses := s.findAccessesViaGraph(ctx, classInfo, method)
		accesses = append(accesses, graphAccesses...)
	}

	// Method 2: Heuristic pattern matching (fallback/supplement)
	patternAccesses := s.findAccessesViaPattern(source)
	accesses = append(accesses, patternAccesses...)

	return accesses
}

// findAccessesViaGraph uses code graph to find external field accesses
func (s *ATFDSignal) findAccessesViaGraph(ctx context.Context, classInfo *signals.ClassInfo, method *signals.MethodInfo) []string {
	var accesses []string

	// Get all CALLS relationships from this method
	calls, err := classInfo.CodeGraph.GetOutgoingRelations(ctx, method.Node.ID, "CALLS")
	if err != nil {
		return accesses
	}

	// For each call, check if it's to an external class
	for _, call := range calls {
		targetNode, err := classInfo.CodeGraph.GetNodeByID(ctx, call.ToNodeID)
		if err != nil {
			continue
		}

		// Check if target is in a different class
		targetClassName := s.getContainingClassName(ctx, classInfo.CodeGraph, targetNode)
		if targetClassName != "" && targetClassName != classInfo.ClassName {
			// Check if it's an accessor method
			if s.isAccessorMethodName(targetNode.Name) {
				accessKey := fmt.Sprintf("%s.%s", targetClassName, targetNode.Name)
				accesses = append(accesses, accessKey)
			}
		}
	}

	return accesses
}

// findAccessesViaPattern uses regex patterns to find potential external accesses
func (s *ATFDSignal) findAccessesViaPattern(source string) []string {
	var accesses []string

	// Find all accessor-like method calls
	matches := s.accessorPattern.FindAllString(source, -1)
	for _, match := range matches {
		// Extract the call pattern
		accesses = append(accesses, strings.TrimSpace(match))
	}

	return accesses
}

// getContainingClassName finds which class contains a given node
func (s *ATFDSignal) getContainingClassName(ctx context.Context, codeGraph *codegraph.CodeGraph, node *ast.Node) string {
	// Walk up the containment hierarchy to find the class
	current := node
	for current != nil {
		if current.NodeType == ast.NodeTypeClass {
			return current.Name
		}

		// Get parent via CONTAINS relationship (inverse)
		parents, err := codeGraph.GetIncomingRelations(ctx, current.ID, "CONTAINS")
		if err != nil || len(parents) == 0 {
			break
		}

		parentNode, err := codeGraph.GetNodeByID(ctx, parents[0].FromNodeID)
		if err != nil {
			break
		}
		current = parentNode
	}

	return ""
}

// isAccessorMethodName checks if a method name follows accessor naming pattern
func (s *ATFDSignal) isAccessorMethodName(name string) bool {
	return regexp.MustCompile(`^(get|Get|is|Is|has|Has)[A-Z]`).MatchString(name)
}
