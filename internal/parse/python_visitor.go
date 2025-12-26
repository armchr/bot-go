package parse

import (
	"bot-go/internal/model/ast"
	"context"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"go.uber.org/zap"
)

type PythonVisitor struct {
	translate *TranslateFromSyntaxTree
	logger    *zap.Logger
}

func NewPythonVisitor(logger *zap.Logger, ts *TranslateFromSyntaxTree) *PythonVisitor {
	return &PythonVisitor{
		translate: ts,
		logger:    logger,
	}
}

func (pv *PythonVisitor) TraverseNode(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	if tsNode == nil {
		return ast.InvalidNodeID
	}

	nodeKind := tsNode.Kind()

	// Handle import statements first (before other cases)
	// Check multiple possible node type names
	if nodeKind == "import_statement" || nodeKind == "import" {
		return pv.handleImportStatement(ctx, tsNode, scopeID)
	}
	if nodeKind == "import_from_statement" || nodeKind == "import_from" {
		return pv.handleImportFromStatement(ctx, tsNode, scopeID)
	}

	switch nodeKind {
	case "module":
		return pv.handleModule(ctx, tsNode)
	case "function_definition":
		return pv.handleFunctionDefinition(ctx, tsNode, scopeID)
	case "block":
		return pv.translate.HandleBlock(ctx, tsNode, scopeID)
	case "class_definition":
		return pv.handleClassDefinition(ctx, tsNode, scopeID)
	case "return_statement":
		return pv.handleReturnStatement(ctx, tsNode, scopeID)
	case "call":
		return pv.handleCall(ctx, tsNode, scopeID)
	case "attribute":
		return pv.handleAttribute(ctx, tsNode, scopeID)
	case "identifier":
		return pv.translate.HandleIdentifier(ctx, tsNode, scopeID)
	case "if_statement":
		return pv.handleIfStatement(ctx, tsNode, scopeID)
	case "for_statement":
		return pv.handleForStatement(ctx, tsNode, scopeID)
	case "while_statement":
		return pv.handleWhileStatement(ctx, tsNode, scopeID)
	case "assignment":
		return pv.handleAssignment(ctx, tsNode, scopeID)
	case "expression_statement":
		// Expression statements at module level (e.g., function calls, assignments, imports)
		// Check if this contains an import statement first
		importStmt := pv.translate.TreeChildByKind(tsNode, "import_statement")
		if importStmt != nil {
			return pv.handleImportStatement(ctx, importStmt, scopeID)
		}
		importFromStmt := pv.translate.TreeChildByKind(tsNode, "import_from_statement")
		if importFromStmt != nil {
			return pv.handleImportFromStatement(ctx, importFromStmt, scopeID)
		}
		// Otherwise, traverse children to process the underlying expression
		return pv.handleExpressionStatement(ctx, tsNode, scopeID)
	case "decorated_definition":
		// Decorated functions/classes - traverse to get the actual definition
		return pv.handleDecoratedDefinition(ctx, tsNode, scopeID)
	case "try_statement":
		return pv.handleTryStatement(ctx, tsNode, scopeID)
	case "with_statement":
		return pv.handleWithStatement(ctx, tsNode, scopeID)
	case "lambda":
		return pv.handleLambda(ctx, tsNode, scopeID)
	case "match_statement":
		return pv.handleMatchStatement(ctx, tsNode, scopeID)
	// Add more cases as needed for other node types
	default:
		// Check if this is an import-related node that we haven't handled
		if strings.Contains(nodeKind, "import") {
			// Try to handle it as import_statement or import_from_statement
			if nodeKind == "import_statement" || strings.HasPrefix(nodeKind, "import_statement") {
				return pv.handleImportStatement(ctx, tsNode, scopeID)
			}
			if nodeKind == "import_from_statement" || strings.HasPrefix(nodeKind, "import_from_statement") {
				return pv.handleImportFromStatement(ctx, tsNode, scopeID)
			}
			pv.logger.Warn("Unhandled import-related node type",
				zap.String("node_kind", nodeKind),
				zap.Uint("child_count", tsNode.ChildCount()))
		}
		// For unhandled node types, traverse children but don't create a node
		// This allows processing of nested structures
		pv.translate.TraverseChildren(ctx, tsNode, scopeID)
		return ast.InvalidNodeID
	}
}

func (pv *PythonVisitor) handleModule(ctx context.Context, tsNode *tree_sitter.Node) ast.NodeID {
	// For Python modules, use "__main__" as default name (or extract from file path if available)
	// Module nodes don't have a name field, so we use a default
	moduleName := "__main__"

	moduleNode := ast.NewNode(
		pv.translate.NextNodeID(), ast.NodeTypeModuleScope, pv.translate.FileID,
		moduleName, pv.translate.ToRange(tsNode), pv.translate.Version,
		ast.NodeID(pv.translate.FileID),
	)
	pv.translate.CodeGraph.CreateModuleScope(ctx, moduleNode)
	pv.translate.PushScope(false)
	defer pv.translate.PopScope(ctx, moduleNode.ID)

	// Manually process children to ensure imports are handled
	// This is similar to how Go handles imports
	var childIDs []ast.NodeID
	for i := uint(0); i < tsNode.ChildCount(); i++ {
		child := tsNode.Child(i)
		childID := pv.TraverseNode(ctx, child, moduleNode.ID)
		if childID != ast.InvalidNodeID {
			childIDs = append(childIDs, childID)
		}
	}

	if len(childIDs) > 0 {
		pv.translate.CreateContainsRelations(ctx, moduleNode.ID, childIDs)
	}
	return moduleNode.ID
}

func (pv *PythonVisitor) handleFunctionDefinition(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	// Extract function name
	funcName := ""
	nameNode := pv.translate.TreeChildByFieldName(tsNode, "name")
	if nameNode != nil {
		funcName = pv.translate.String(nameNode)
	}

	paramsNode := pv.translate.TreeChildByFieldName(tsNode, "parameters")
	bodyNode := pv.translate.TreeChildByFieldName(tsNode, "body")

	return pv.translate.CreateFunction(ctx, scopeID, tsNode, funcName, pv.translate.NamedChildren(paramsNode), bodyNode)
}

func (pv *PythonVisitor) handleClassDefinition(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	// Extract class name
	className := ""
	nameNode := pv.translate.TreeChildByFieldName(tsNode, "name")
	if nameNode != nil {
		className = pv.translate.String(nameNode)
	}

	body := pv.translate.TreeChildByFieldName(tsNode, "body")
	var methods []*tree_sitter.Node
	if body != nil {
		methods = pv.translate.TreeChildrenByKind(body, "function_definition")
	}
	return pv.translate.HandleClass(ctx, scopeID, tsNode, className, methods, nil)
}

func (pv *PythonVisitor) handleReturnStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	if tsNode.ChildCount() < 2 {
		return ast.InvalidNodeID
	}
	rhsNode := tsNode.Child(1)
	rhs := pv.translate.HandleReturn(ctx, rhsNode, scopeID)
	return rhs
}

func (pv *PythonVisitor) handleCall(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	argList := pv.translate.TreeChildByKind(tsNode, "argument_list")
	var args []*tree_sitter.Node
	if argList != nil {
		args = pv.translate.NamedChildren(argList)
	}
	fnNameNodeID := pv.translate.HandleRhsWithFakeVariable(ctx, "__fn__", tsNode.Child(0), scopeID, nil)
	return pv.translate.HandleCall(ctx, fnNameNodeID, args, scopeID, pv.translate.ToRange(tsNode))
}

func (pv *PythonVisitor) handleAttribute(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	var names []*tree_sitter.Node

	for i := uint(0); i < tsNode.ChildCount(); i++ {
		child := tsNode.Child(i)
		if child.Kind() == "." {
			continue
		}
		names = append(names, child)
	}
	resolvedNodeId := pv.translate.ResolveNameChain(ctx, names, scopeID)
	if pv.translate.CurrentScope.IsRhs() && resolvedNodeId != ast.InvalidNodeID {
		pv.translate.CurrentScope.AddRhsVar(resolvedNodeId)
	}
	return resolvedNodeId
}

func (pv *PythonVisitor) handleIfStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	conditionNode := tsNode.Child(1) // if is 0, condition is 1
	branch := tsNode.Child(3)        // body is 3

	conditions := []*tree_sitter.Node{conditionNode}
	branches := []*tree_sitter.Node{branch}

	elifNodes := pv.translate.TreeChildrenByKind(tsNode, "elif_clause")
	for _, elif := range elifNodes {
		cond := elif.Child(1) // elif is 0, condition is 1
		br := elif.Child(3)   // body is 3
		conditions = append(conditions, cond)
		branches = append(branches, br)
	}

	elseNode := pv.translate.TreeChildByKind(tsNode, "else_clause")
	if elseNode != nil {
		branches = append(branches, elseNode)
	}
	return pv.translate.HandleConditional(ctx, tsNode, conditions, branches, scopeID)
}

func (pv *PythonVisitor) handleForStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	inits := make([]*tree_sitter.Node, 0)
	initVars := tsNode.Child(1) // target is 1
	if initVars != nil {
		inits = append(inits, initVars)
	}
	initCond := tsNode.Child(3)
	if initCond == nil {
		return ast.InvalidNodeID
	}
	inits = append(inits, initCond)

	pv.translate.PushScope(false)
	defer pv.translate.PopScope(ctx, ast.InvalidNodeID)

	initCondID := pv.translate.HandleRhsExprsWithFakeVariable(ctx, "__init__", inits, scopeID, nil)
	body := tsNode.Child(5) // body is 5
	if body == nil {
		return ast.InvalidNodeID
	}
	return pv.translate.HandleLoop(ctx, tsNode, ast.InvalidNodeID, initCondID, body, scopeID)
}

func (pv *PythonVisitor) handleWhileStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	conditionNode := tsNode.Child(1) // while is 0, condition is 1
	if conditionNode == nil {
		return ast.InvalidNodeID
	}
	conditionID := pv.translate.HandleRhsWithFakeVariable(ctx, "__cond__", conditionNode, scopeID, nil)
	body := tsNode.Child(3) // body is 3
	if body == nil {
		return ast.InvalidNodeID
	}
	return pv.translate.HandleLoop(ctx, tsNode, ast.InvalidNodeID, conditionID, body, scopeID)
}

func (pv *PythonVisitor) handleAssignment(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	if tsNode.ChildCount() < 3 {
		return ast.InvalidNodeID
	}
	lhsNode := tsNode.Child(0)
	rhsNode := tsNode.Child(2)

	return pv.translate.HandleAssignment(ctx, tsNode, lhsNode, rhsNode, scopeID)
}

// handleImportStatement processes Python import statements
// Handles:
//   - import module
//   - import module as alias
//   - import module.submodule
//   - import module.submodule as alias
//   - import a, b, c (multiple imports)
func (pv *PythonVisitor) handleImportStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	var importNodeIDs []ast.NodeID
	seenImportKeyword := false

	for i := uint(0); i < tsNode.ChildCount(); i++ {
		child := tsNode.Child(i)
		kind := child.Kind()

		if kind == "import" {
			seenImportKeyword = true
			continue
		}
		if !seenImportKeyword {
			// Skip everything until we reach the imported names
			continue
		}

		// Skip punctuation like commas or parentheses that group imports
		if kind == "," || kind == "(" || kind == ")" {
			continue
		}

		var importID ast.NodeID
		switch kind {
		case "dotted_name":
			importID = pv.handleDottedImport(ctx, child, nil, scopeID)
		case "aliased_import":
			nameNode := pv.translate.TreeChildByFieldName(child, "name")
			if nameNode == nil {
				nameNode = pv.translate.TreeChildByKind(child, "dotted_name")
			}
			aliasNode := pv.translate.TreeChildByFieldName(child, "alias")
			if aliasNode == nil {
				identifiers := pv.translate.TreeChildrenByKind(child, "identifier")
				if len(identifiers) > 0 {
					aliasNode = identifiers[len(identifiers)-1]
				}
			}
			importID = pv.handleDottedImport(ctx, nameNode, aliasNode, scopeID)
		default:
			// Ignore whitespace/comments or unexpected tokens
			continue
		}

		if importID != ast.InvalidNodeID {
			importNodeIDs = append(importNodeIDs, importID)
		}
	}

	if len(importNodeIDs) > 0 {
		return importNodeIDs[0]
	}
	return ast.InvalidNodeID
}

// handleImportFromStatement processes Python "from ... import ..." statements
// Handles:
//   - from module import name
//   - from module import name as alias
//   - from module import name1, name2
//   - from module import name1 as alias1, name2 as alias2
//   - from .module import name (relative imports)
//   - from module import * (wildcard imports)
func (pv *PythonVisitor) handleImportFromStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	// Get the module name (can be dotted_name or relative_import)
	moduleNameNode := pv.translate.TreeChildByFieldName(tsNode, "module_name")
	relativeImport := pv.translate.TreeChildByKind(tsNode, "relative_import")

	var modulePath string
	if relativeImport != nil {
		// Handle relative imports: from .module import name or from ..module import name
		modulePath = pv.getRelativeImportPath(relativeImport)
	} else if moduleNameNode != nil {
		// Regular import: from module import name
		dottedName := pv.translate.TreeChildByKind(moduleNameNode, "dotted_name")
		if dottedName != nil {
			modulePath = pv.translate.String(dottedName)
		} else {
			// Module name might be directly a dotted_name
			modulePath = pv.translate.String(moduleNameNode)
		}
	}

	if modulePath == "" {
		return ast.InvalidNodeID
	}

	var importNodeIDs []ast.NodeID
	seenImportKeyword := false
	for i := uint(0); i < tsNode.ChildCount(); i++ {
		child := tsNode.Child(i)
		kind := child.Kind()

		if kind == "import" {
			seenImportKeyword = true
			continue
		}
		if !seenImportKeyword {
			continue
		}
		if kind == "," || kind == "(" || kind == ")" {
			continue
		}

		switch kind {
		case "wildcard_import":
			fullPath := pv.combineImportPath(modulePath, "*")
			importID := pv.createImportNode(ctx, child, "*", fullPath, scopeID)
			if importID != ast.InvalidNodeID {
				importNodeIDs = append(importNodeIDs, importID)
			}
		case "aliased_import":
			nameNode := pv.translate.TreeChildByFieldName(child, "name")
			if nameNode == nil {
				nameNode = pv.translate.TreeChildByKind(child, "dotted_name")
			}
			if nameNode == nil {
				continue
			}
			aliasNode := pv.translate.TreeChildByFieldName(child, "alias")
			if aliasNode == nil {
				identifiers := pv.translate.TreeChildrenByKind(child, "identifier")
				if len(identifiers) > 0 {
					aliasNode = identifiers[len(identifiers)-1]
				}
			}
			originalPath := pv.translate.String(nameNode)
			fullPath := pv.combineImportPath(modulePath, originalPath)
			symbolName := ""
			if aliasNode != nil {
				symbolName = pv.translate.String(aliasNode)
			}
			if symbolName == "" {
				symbolName = pv.getLastComponent(originalPath)
			}
			if symbolName == "" || fullPath == "" {
				continue
			}
			importID := pv.createImportNode(ctx, child, symbolName, fullPath, scopeID)
			if importID != ast.InvalidNodeID {
				importNodeIDs = append(importNodeIDs, importID)
			}
		case "dotted_name":
			importedPath := pv.translate.String(child)
			if importedPath == "" {
				continue
			}
			fullPath := pv.combineImportPath(modulePath, importedPath)
			symbolName := pv.getLastComponent(importedPath)
			if symbolName == "" {
				continue
			}
			importID := pv.createImportNode(ctx, child, symbolName, fullPath, scopeID)
			if importID != ast.InvalidNodeID {
				importNodeIDs = append(importNodeIDs, importID)
			}
		default:
			// Ignore unexpected tokens like newlines or comments
			continue
		}
	}

	// Return first import ID if any, so TraverseChildren can collect it
	// The CONTAINS relationships are already created in createImportNode
	if len(importNodeIDs) > 0 {
		return importNodeIDs[0]
	}
	return ast.InvalidNodeID
}

// handleDottedImport processes a dotted import name (e.g., "module.submodule")
// and creates an import node with the appropriate symbol name
func (pv *PythonVisitor) handleDottedImport(ctx context.Context, dottedName *tree_sitter.Node, alias *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	if dottedName == nil {
		return ast.InvalidNodeID
	}

	importPath := pv.translate.String(dottedName)
	if importPath == "" {
		return ast.InvalidNodeID
	}

	// Determine symbol name
	var symbolName string
	if alias != nil {
		// Use alias if provided: import module as alias
		symbolName = pv.translate.String(alias)
	} else {
		// Extract the last component: import module.submodule -> "submodule"
		symbolName = pv.getLastComponent(importPath)
	}

	if symbolName == "" {
		return ast.InvalidNodeID
	}

	return pv.createImportNode(ctx, dottedName, symbolName, importPath, scopeID)
}

// createImportNode creates an Import AST node and adds it to the code graph and scope
func (pv *PythonVisitor) createImportNode(ctx context.Context, tsNode *tree_sitter.Node, symbolName string, importPath string, scopeID ast.NodeID) ast.NodeID {
	if symbolName == "" || importPath == "" {
		return ast.InvalidNodeID
	}

	// Create the Import node
	importNode := ast.NewNode(
		pv.translate.NextNodeID(),
		ast.NodeTypeImport,
		pv.translate.FileID,
		symbolName,
		pv.translate.ToRange(tsNode),
		pv.translate.Version,
		scopeID,
	)

	// Store the full import path in metadata
	importNode.MetaData = map[string]any{
		"importPath": importPath,
	}

	// Write node to code graph
	pv.translate.CodeGraph.CreateImport(ctx, importNode)

	// Link import to its parent scope via CONTAINS relationship
	if scopeID != ast.InvalidNodeID {
		pv.translate.CreateContainsRelation(ctx, scopeID, importNode.ID, pv.translate.FileID)
	}

	// Add to current scope so it can be resolved when used
	pv.translate.CurrentScope.AddSymbol(NewSymbol(importNode))

	// Track in translator's node map
	pv.translate.Nodes[importNode.ID] = importNode

	return importNode.ID
}

// getRelativeImportPath extracts the import path from a relative_import node
// Handles: .module, ..module, ...module, etc.
func (pv *PythonVisitor) getRelativeImportPath(relativeImport *tree_sitter.Node) string {
	if relativeImport == nil {
		return ""
	}

	var pathBuilder string

	// Count the dots (., .., ...)
	for i := uint(0); i < relativeImport.ChildCount(); i++ {
		child := relativeImport.Child(i)
		if child.Kind() == "." {
			pathBuilder += "."
		} else if child.Kind() == "dotted_name" {
			pathBuilder += pv.translate.String(child)
		}
	}

	return pathBuilder
}

// combineImportPath joins the module/relative portion with the imported symbol
func (pv *PythonVisitor) combineImportPath(modulePath, importedItem string) string {
	if modulePath == "" {
		return importedItem
	}
	if importedItem == "" {
		return modulePath
	}
	if strings.HasSuffix(modulePath, ".") {
		return modulePath + importedItem
	}
	return modulePath + "." + importedItem
}

// getLastComponent extracts the last component from a dotted path
// For "module.submodule.item", returns "item"
// For "module", returns "module"
func (pv *PythonVisitor) getLastComponent(path string) string {
	if path == "" {
		return ""
	}

	// Find the last "." and take everything after it
	lastDot := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			lastDot = i
			break
		}
	}

	if lastDot == -1 {
		return path
	}

	return path[lastDot+1:]
}

// handleExpressionStatement processes expression statements (e.g., function calls at module level)
func (pv *PythonVisitor) handleExpressionStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	// Check if this expression statement contains an import statement
	// Python tree-sitter may wrap imports in expression_statement nodes
	if pv.translate.TreeChildByKind(tsNode, "import_statement") != nil {
		return pv.handleImportStatement(ctx, pv.translate.TreeChildByKind(tsNode, "import_statement"), scopeID)
	}
	if pv.translate.TreeChildByKind(tsNode, "import_from_statement") != nil {
		return pv.handleImportFromStatement(ctx, pv.translate.TreeChildByKind(tsNode, "import_from_statement"), scopeID)
	}

	// Expression statements contain the actual expression (call, assignment, etc.)
	// Traverse children to process the underlying expression
	childNodes := pv.translate.TraverseChildren(ctx, tsNode, scopeID)
	// Don't create a node for the expression statement itself, just process children
	if len(childNodes) > 0 {
		// Children are already processed, just return the first valid child ID if any
		for _, childID := range childNodes {
			if childID != ast.InvalidNodeID {
				return childID
			}
		}
	}
	return ast.InvalidNodeID
}

// handleDecoratedDefinition processes decorated functions/classes
// Handles: @decorator def func(): ... or @decorator class Class: ...
func (pv *PythonVisitor) handleDecoratedDefinition(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	// Get the actual definition (function_definition or class_definition)
	definition := pv.translate.TreeChildByFieldName(tsNode, "definition")
	if definition == nil {
		// Fallback: try to find by kind
		definitions := pv.translate.TreeChildrenByKind(tsNode, "function_definition")
		if len(definitions) == 0 {
			definitions = pv.translate.TreeChildrenByKind(tsNode, "class_definition")
		}
		if len(definitions) > 0 {
			definition = definitions[0]
		}
	}

	if definition != nil {
		// Process the underlying definition (function or class)
		return pv.TraverseNode(ctx, definition, scopeID)
	}

	// If no definition found, traverse children
	pv.translate.TraverseChildren(ctx, tsNode, scopeID)
	return ast.InvalidNodeID
}

// handleTryStatement processes try/except/finally blocks
func (pv *PythonVisitor) handleTryStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	// Create a block node for the try statement
	blockNode := pv.translate.HandleBlock(ctx, tsNode, scopeID)

	// Process except clauses and finally block if present
	pv.translate.TraverseChildren(ctx, tsNode, scopeID)

	return blockNode
}

// handleWithStatement processes context managers (with statements)
func (pv *PythonVisitor) handleWithStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	// Process the with statement body
	bodyNode := pv.translate.TreeChildByFieldName(tsNode, "body")
	if bodyNode != nil {
		return pv.translate.HandleBlock(ctx, bodyNode, scopeID)
	}

	// Traverse children to process context items
	pv.translate.TraverseChildren(ctx, tsNode, scopeID)
	return ast.InvalidNodeID
}

// handleLambda processes lambda expressions
func (pv *PythonVisitor) handleLambda(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	// Lambda expressions are anonymous functions
	// Extract parameters and body
	paramsNode := pv.translate.TreeChildByFieldName(tsNode, "parameters")
	bodyNode := pv.translate.TreeChildByFieldName(tsNode, "body")

	// Create an anonymous function (empty name)
	var params []*tree_sitter.Node
	if paramsNode != nil {
		params = pv.translate.NamedChildren(paramsNode)
	}

	return pv.translate.CreateFunction(ctx, scopeID, tsNode, "", params, bodyNode)
}

// handleMatchStatement processes Python 3.10+ match/case statements
func (pv *PythonVisitor) handleMatchStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	// Match statements are similar to switch statements
	// Extract subject and cases
	subjectNode := pv.translate.TreeChildByFieldName(tsNode, "subject")
	bodyNode := pv.translate.TreeChildByFieldName(tsNode, "body")

	var conditions []*tree_sitter.Node
	var branches []*tree_sitter.Node

	if subjectNode != nil {
		conditions = append(conditions, subjectNode)
	}

	if bodyNode != nil {
		// Get case clauses
		caseClauses := pv.translate.TreeChildrenByKind(bodyNode, "case_clause")
		for _, clause := range caseClauses {
			pattern := pv.translate.TreeChildByFieldName(clause, "pattern")
			if pattern != nil {
				conditions = append(conditions, pattern)
			}
			consequence := pv.translate.TreeChildByFieldName(clause, "consequence")
			if consequence != nil {
				branches = append(branches, consequence)
			}
		}
	}

	return pv.translate.HandleConditional(ctx, tsNode, conditions, branches, scopeID)
}
