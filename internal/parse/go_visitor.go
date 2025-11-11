package parse

import (
	"bot-go/internal/model/ast"
	"context"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"go.uber.org/zap"
)

type GoVisitor struct {
	translate *TranslateFromSyntaxTree
	logger    *zap.Logger
}

func NewGoVisitor(logger *zap.Logger, ts *TranslateFromSyntaxTree) *GoVisitor {
	return &GoVisitor{
		translate: ts,
		logger:    logger,
	}
}

func (gv *GoVisitor) TraverseNode(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	if tsNode == nil {
		return ast.InvalidNodeID
	}

	switch tsNode.Kind() {
	case "source_file":
		return gv.handleSourceFile(ctx, tsNode)
	case "function_declaration":
		return gv.handleFunctionDeclaration(ctx, tsNode, scopeID)
	case "method_declaration":
		return gv.handleMethodDeclaration(ctx, tsNode, scopeID)
	case "block":
		return gv.translate.HandleBlock(ctx, tsNode, scopeID)
	case "type_declaration":
		return gv.handleTypeDeclaration(ctx, tsNode, scopeID)
	case "struct_type":
		return gv.handleStructType(ctx, tsNode, scopeID)
	case "interface_type":
		return gv.handleInterfaceType(ctx, tsNode, scopeID)
	case "return_statement":
		return gv.handleReturnStatement(ctx, tsNode, scopeID)
	case "call_expression":
		return gv.handleCallExpression(ctx, tsNode, scopeID)
	case "selector_expression":
		return gv.handleSelectorExpression(ctx, tsNode, scopeID)
	case "identifier":
		return gv.translate.HandleIdentifier(ctx, tsNode, scopeID)
	case "if_statement":
		return gv.handleIfStatement(ctx, tsNode, scopeID)
	case "for_statement":
		return gv.handleForStatement(ctx, tsNode, scopeID)
	case "range_clause":
		return gv.handleRangeClause(ctx, tsNode, scopeID)
	case "assignment_statement":
		return gv.handleAssignmentStatement(ctx, tsNode, scopeID)
	case "short_var_declaration":
		return gv.handleShortVarDeclaration(ctx, tsNode, scopeID)
	case "var_declaration":
		return gv.handleVarDeclaration(ctx, tsNode, scopeID)
	case "const_declaration":
		return gv.handleConstDeclaration(ctx, tsNode, scopeID)
	case "switch_statement":
		return gv.handleSwitchStatement(ctx, tsNode, scopeID)
	case "type_switch_statement":
		return gv.handleTypeSwitchStatement(ctx, tsNode, scopeID)
	case "go_statement":
		return gv.handleGoStatement(ctx, tsNode, scopeID)
	case "defer_statement":
		return gv.handleDeferStatement(ctx, tsNode, scopeID)
	case "select_statement":
		return gv.handleSelectStatement(ctx, tsNode, scopeID)
	default:
		gv.translate.TraverseChildren(ctx, tsNode, scopeID)
		return ast.InvalidNodeID
	}
}

func (gv *GoVisitor) handleSourceFile(ctx context.Context, tsNode *tree_sitter.Node) ast.NodeID {
	moduleNode := ast.NewNode(
		gv.translate.NextNodeID(), ast.NodeTypeModuleScope, gv.translate.FileID,
		gv.translate.GetTreeNodeName(tsNode), gv.translate.ToRange(tsNode), gv.translate.Version,
		ast.NodeID(gv.translate.FileID),
	)
	gv.translate.CodeGraph.CreateModuleScope(ctx, moduleNode)
	gv.translate.PushScope(false)
	defer gv.translate.PopScope(ctx, moduleNode.ID)
	childNodes := gv.translate.TraverseChildren(ctx, tsNode, moduleNode.ID)
	if len(childNodes) > 0 {
		gv.translate.CreateContainsRelations(ctx, moduleNode.ID, childNodes)
	}
	return moduleNode.ID
}

func (gv *GoVisitor) handleFunctionDeclaration(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	paramsNode := gv.translate.TreeChildByFieldName(tsNode, "parameters")
	bodyNode := gv.translate.TreeChildByFieldName(tsNode, "body")

	return gv.translate.CreateFunction(ctx, scopeID, tsNode, gv.translate.NamedChildren(paramsNode), bodyNode)
}

func (gv *GoVisitor) handleMethodDeclaration(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	receiverNode := gv.translate.TreeChildByFieldName(tsNode, "receiver")
	paramsNode := gv.translate.TreeChildByFieldName(tsNode, "parameters")
	bodyNode := gv.translate.TreeChildByFieldName(tsNode, "body")

	var allParams []*tree_sitter.Node
	if receiverNode != nil {
		allParams = append(allParams, gv.translate.NamedChildren(receiverNode)...)
	}
	if paramsNode != nil {
		allParams = append(allParams, gv.translate.NamedChildren(paramsNode)...)
	}

	return gv.translate.CreateFunction(ctx, scopeID, tsNode, allParams, bodyNode)
}

func (gv *GoVisitor) handleTypeDeclaration(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	typeSpecs := gv.translate.TreeChildrenByKind(tsNode, "type_spec")
	var childNodes []ast.NodeID
	for _, typeSpec := range typeSpecs {
		childID := gv.TraverseNode(ctx, typeSpec, scopeID)
		if childID != ast.InvalidNodeID {
			childNodes = append(childNodes, childID)
		}
	}
	return ast.InvalidNodeID
}

func (gv *GoVisitor) handleStructType(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	fieldList := gv.translate.TreeChildByFieldName(tsNode, "fields")
	var methods []*tree_sitter.Node
	if fieldList != nil {
		methods = gv.translate.NamedChildren(fieldList)
	}
	return gv.translate.HandleClass(ctx, scopeID, tsNode, methods, nil)
}

func (gv *GoVisitor) handleInterfaceType(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	methodList := gv.translate.TreeChildByFieldName(tsNode, "methods")
	var methods []*tree_sitter.Node
	if methodList != nil {
		methods = gv.translate.NamedChildren(methodList)
	}
	return gv.translate.HandleClass(ctx, scopeID, tsNode, methods, nil)
}

func (gv *GoVisitor) handleReturnStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	if tsNode.ChildCount() < 2 {
		return ast.InvalidNodeID
	}
	rhsNode := tsNode.Child(1)
	rhs := gv.translate.HandleReturn(ctx, rhsNode, scopeID)
	return rhs
}

func (gv *GoVisitor) handleCallExpression(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	functionNode := gv.translate.TreeChildByFieldName(tsNode, "function")
	argumentsNode := gv.translate.TreeChildByFieldName(tsNode, "arguments")

	var args []*tree_sitter.Node
	if argumentsNode != nil {
		args = gv.translate.NamedChildren(argumentsNode)
	}

	fnNameNodeID := gv.translate.HandleRhsWithFakeVariable(ctx, "__fn__", functionNode, scopeID, nil)
	return gv.translate.HandleCall(ctx, fnNameNodeID, args, scopeID, gv.translate.ToRange(tsNode))
}

func (gv *GoVisitor) handleSelectorExpression(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	operandNode := gv.translate.TreeChildByFieldName(tsNode, "operand")
	fieldNode := gv.translate.TreeChildByFieldName(tsNode, "field")

	var names []*tree_sitter.Node
	if operandNode != nil {
		names = append(names, operandNode)
	}
	if fieldNode != nil {
		names = append(names, fieldNode)
	}

	resolvedNodeId := gv.translate.ResolveNameChain(ctx, names, scopeID)
	if gv.translate.CurrentScope.IsRhs() && resolvedNodeId != ast.InvalidNodeID {
		gv.translate.CurrentScope.AddRhsVar(resolvedNodeId)
	}
	return resolvedNodeId
}

func (gv *GoVisitor) handleIfStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	conditionNode := gv.translate.TreeChildByFieldName(tsNode, "condition")
	consequenceNode := gv.translate.TreeChildByFieldName(tsNode, "consequence")
	alternativeNode := gv.translate.TreeChildByFieldName(tsNode, "alternative")

	conditions := []*tree_sitter.Node{conditionNode}
	branches := []*tree_sitter.Node{consequenceNode}

	if alternativeNode != nil {
		if alternativeNode.Kind() == "if_statement" {
			altCondition := gv.translate.TreeChildByFieldName(alternativeNode, "condition")
			altConsequence := gv.translate.TreeChildByFieldName(alternativeNode, "consequence")
			conditions = append(conditions, altCondition)
			branches = append(branches, altConsequence)

			altAlternative := gv.translate.TreeChildByFieldName(alternativeNode, "alternative")
			if altAlternative != nil {
				branches = append(branches, altAlternative)
			}
		} else {
			branches = append(branches, alternativeNode)
		}
	}

	return gv.translate.HandleConditional(ctx, tsNode, conditions, branches, scopeID)
}

func (gv *GoVisitor) handleForStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	initNode := gv.translate.TreeChildByFieldName(tsNode, "initializer")
	conditionNode := gv.translate.TreeChildByFieldName(tsNode, "condition")
	updateNode := gv.translate.TreeChildByFieldName(tsNode, "update")
	bodyNode := gv.translate.TreeChildByFieldName(tsNode, "body")

	var inits []*tree_sitter.Node
	if initNode != nil {
		inits = append(inits, initNode)
	}
	if conditionNode != nil {
		inits = append(inits, conditionNode)
	}

	gv.translate.PushScope(false)
	defer gv.translate.PopScope(ctx, ast.InvalidNodeID)

	initCondID := ast.InvalidNodeID
	if len(inits) > 0 {
		initCondID = gv.translate.HandleRhsExprsWithFakeVariable(ctx, "__init__", inits, scopeID, nil)
	}

	updateID := ast.InvalidNodeID
	if updateNode != nil {
		updateID = gv.translate.HandleRhsWithFakeVariable(ctx, "__update__", updateNode, scopeID, nil)
	}

	if bodyNode == nil {
		return ast.InvalidNodeID
	}
	return gv.translate.HandleLoop(ctx, tsNode, updateID, initCondID, bodyNode, scopeID)
}

func (gv *GoVisitor) handleRangeClause(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	leftNode := gv.translate.TreeChildByFieldName(tsNode, "left")
	rightNode := gv.translate.TreeChildByFieldName(tsNode, "right")

	var inits []*tree_sitter.Node
	if leftNode != nil {
		inits = append(inits, leftNode)
	}
	if rightNode != nil {
		inits = append(inits, rightNode)
	}

	if len(inits) > 0 {
		return gv.translate.HandleRhsExprsWithFakeVariable(ctx, "__range__", inits, scopeID, nil)
	}
	return ast.InvalidNodeID
}

func (gv *GoVisitor) handleAssignmentStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	leftNode := gv.translate.TreeChildByFieldName(tsNode, "left")
	rightNode := gv.translate.TreeChildByFieldName(tsNode, "right")

	if leftNode == nil || rightNode == nil {
		return ast.InvalidNodeID
	}

	return gv.translate.HandleAssignment(ctx, tsNode, leftNode, rightNode, scopeID)
}

func (gv *GoVisitor) handleShortVarDeclaration(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	leftNode := gv.translate.TreeChildByFieldName(tsNode, "left")
	rightNode := gv.translate.TreeChildByFieldName(tsNode, "right")

	if leftNode == nil || rightNode == nil {
		return ast.InvalidNodeID
	}

	return gv.translate.HandleAssignment(ctx, tsNode, leftNode, rightNode, scopeID)
}

func (gv *GoVisitor) handleVarDeclaration(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	specs := gv.translate.TreeChildrenByKind(tsNode, "var_spec")
	for _, spec := range specs {
		nameNode := gv.translate.TreeChildByFieldName(spec, "name")
		valueNode := gv.translate.TreeChildByFieldName(spec, "value")
		if nameNode != nil && valueNode != nil {
			gv.translate.HandleAssignment(ctx, spec, nameNode, valueNode, scopeID)
		}
	}
	return ast.InvalidNodeID
}

func (gv *GoVisitor) handleConstDeclaration(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	specs := gv.translate.TreeChildrenByKind(tsNode, "const_spec")
	for _, spec := range specs {
		nameNode := gv.translate.TreeChildByFieldName(spec, "name")
		valueNode := gv.translate.TreeChildByFieldName(spec, "value")
		if nameNode != nil && valueNode != nil {
			gv.translate.HandleAssignment(ctx, spec, nameNode, valueNode, scopeID)
		}
	}
	return ast.InvalidNodeID
}

func (gv *GoVisitor) handleSwitchStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	valueNode := gv.translate.TreeChildByFieldName(tsNode, "value")
	bodyNode := gv.translate.TreeChildByFieldName(tsNode, "body")

	var conditions []*tree_sitter.Node
	var branches []*tree_sitter.Node

	if valueNode != nil {
		conditions = append(conditions, valueNode)
	}

	if bodyNode != nil {
		caseClauses := gv.translate.TreeChildrenByKind(bodyNode, "expression_case")
		defaultClauses := gv.translate.TreeChildrenByKind(bodyNode, "default_case")

		for _, clause := range caseClauses {
			valueList := gv.translate.TreeChildByFieldName(clause, "value")
			if valueList != nil {
				conditions = append(conditions, valueList)
			}
			consequence := gv.translate.TreeChildByFieldName(clause, "consequence")
			if consequence != nil {
				branches = append(branches, consequence)
			}
		}

		for _, clause := range defaultClauses {
			consequence := gv.translate.TreeChildByFieldName(clause, "consequence")
			if consequence != nil {
				branches = append(branches, consequence)
			}
		}
	}

	return gv.translate.HandleConditional(ctx, tsNode, conditions, branches, scopeID)
}

func (gv *GoVisitor) handleTypeSwitchStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	aliasNode := gv.translate.TreeChildByFieldName(tsNode, "alias")
	valueNode := gv.translate.TreeChildByFieldName(tsNode, "value")
	bodyNode := gv.translate.TreeChildByFieldName(tsNode, "body")

	var conditions []*tree_sitter.Node
	var branches []*tree_sitter.Node

	if aliasNode != nil {
		conditions = append(conditions, aliasNode)
	}
	if valueNode != nil {
		conditions = append(conditions, valueNode)
	}

	if bodyNode != nil {
		caseClauses := gv.translate.TreeChildrenByKind(bodyNode, "type_case")
		defaultClauses := gv.translate.TreeChildrenByKind(bodyNode, "default_case")

		for _, clause := range caseClauses {
			typeList := gv.translate.TreeChildByFieldName(clause, "type")
			if typeList != nil {
				conditions = append(conditions, typeList)
			}
			consequence := gv.translate.TreeChildByFieldName(clause, "consequence")
			if consequence != nil {
				branches = append(branches, consequence)
			}
		}

		for _, clause := range defaultClauses {
			consequence := gv.translate.TreeChildByFieldName(clause, "consequence")
			if consequence != nil {
				branches = append(branches, consequence)
			}
		}
	}

	return gv.translate.HandleConditional(ctx, tsNode, conditions, branches, scopeID)
}

func (gv *GoVisitor) handleGoStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	callNode := gv.translate.TreeChildByFieldName(tsNode, "call")
	if callNode != nil {
		return gv.TraverseNode(ctx, callNode, scopeID)
	}
	return ast.InvalidNodeID
}

func (gv *GoVisitor) handleDeferStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	callNode := gv.translate.TreeChildByFieldName(tsNode, "call")
	if callNode != nil {
		return gv.TraverseNode(ctx, callNode, scopeID)
	}
	return ast.InvalidNodeID
}

func (gv *GoVisitor) handleSelectStatement(ctx context.Context, tsNode *tree_sitter.Node, scopeID ast.NodeID) ast.NodeID {
	bodyNode := gv.translate.TreeChildByFieldName(tsNode, "body")

	var conditions []*tree_sitter.Node
	var branches []*tree_sitter.Node

	if bodyNode != nil {
		communicationClauses := gv.translate.TreeChildrenByKind(bodyNode, "communication_case")
		defaultClauses := gv.translate.TreeChildrenByKind(bodyNode, "default_case")

		for _, clause := range communicationClauses {
			communication := gv.translate.TreeChildByFieldName(clause, "communication")
			if communication != nil {
				conditions = append(conditions, communication)
			}
			consequence := gv.translate.TreeChildByFieldName(clause, "consequence")
			if consequence != nil {
				branches = append(branches, consequence)
			}
		}

		for _, clause := range defaultClauses {
			consequence := gv.translate.TreeChildByFieldName(clause, "consequence")
			if consequence != nil {
				branches = append(branches, consequence)
			}
		}
	}

	return gv.translate.HandleConditional(ctx, tsNode, conditions, branches, scopeID)
}
